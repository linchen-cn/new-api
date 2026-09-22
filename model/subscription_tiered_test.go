package model

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tieredTestEnv holds the fixtures created for one tiered test case.
type tieredTestEnv struct {
	planId int
	userId int
	subId  int
}

// setupTieredSubscription creates a tiered plan + active user subscription.
func setupTieredSubscription(t *testing.T, plan SubscriptionPlan, userId int) tieredTestEnv {
	t.Helper()
	require.NoError(t, DB.Create(&plan).Error)
	env := tieredTestEnv{planId: plan.Id, userId: userId}
	sub, err := CreateUserSubscriptionFromPlanTx(DB, userId, &plan, "admin")
	require.NoError(t, err)
	env.subId = sub.Id
	return env
}

func getSubscriptionRow(t *testing.T, subId int) UserSubscription {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", subId).First(&sub).Error)
	return sub
}

func TestAdvanceSessionWindow(t *testing.T) {
	const window int64 = 18000
	now := time.Now().Unix()

	t.Run("first request anchors window", func(t *testing.T) {
		sub := &UserSubscription{SessionWindowSeconds: window}
		advanceSessionWindow(sub, now)
		assert.Equal(t, now, sub.SessionWindowStart)
		assert.EqualValues(t, 0, sub.SessionUsed)
	})

	t.Run("active window keeps anchor", func(t *testing.T) {
		start := now - 100
		sub := &UserSubscription{SessionWindowSeconds: window, SessionWindowStart: start, SessionUsed: 500}
		advanceSessionWindow(sub, now)
		assert.Equal(t, start, sub.SessionWindowStart)
		assert.EqualValues(t, 500, sub.SessionUsed)
	})

	t.Run("expired window re-anchors and clears usage", func(t *testing.T) {
		start := now - window - 1
		sub := &UserSubscription{SessionWindowSeconds: window, SessionWindowStart: start, SessionUsed: 500}
		advanceSessionWindow(sub, now)
		assert.Equal(t, now, sub.SessionWindowStart)
		assert.EqualValues(t, 0, sub.SessionUsed)
	})

	t.Run("non-positive window falls back to 5h", func(t *testing.T) {
		start := now - 18000 - 1
		sub := &UserSubscription{SessionWindowSeconds: 0, SessionWindowStart: start, SessionUsed: 100}
		advanceSessionWindow(sub, now)
		assert.Equal(t, now, sub.SessionWindowStart)
		assert.EqualValues(t, 0, sub.SessionUsed)
	})
}

func TestAdvanceWeeklyWindow(t *testing.T) {
	now := time.Now().Unix()

	t.Run("first use initializes next Monday", func(t *testing.T) {
		sub := &UserSubscription{}
		advanceWeeklyWindow(sub, now)
		reset := time.Unix(sub.WeekNextResetTime, 0)
		assert.Equal(t, time.Monday, reset.Weekday())
		assert.Equal(t, 0, reset.Hour())
		assert.True(t, sub.WeekNextResetTime > now)
	})

	t.Run("window before reset keeps usage", func(t *testing.T) {
		sub := &UserSubscription{WeeklyUsed: 300}
		advanceWeeklyWindow(sub, now)
		// initialize first, then verify usage kept and reset not passed
		assert.EqualValues(t, 300, sub.WeeklyUsed)
		assert.True(t, sub.WeekNextResetTime > now)
	})

	t.Run("expired weekly window rolls and clears usage", func(t *testing.T) {
		// construct a reset time in the past: previous Monday 00:00
		past := time.Now()
		weekday := int(past.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		prevMonday := time.Date(past.Year(), past.Month(), past.Day(), 0, 0, 0, 0, past.Location()).
			AddDate(0, 0, -(weekday - 1))
		sub := &UserSubscription{WeeklyUsed: 700, WeekNextResetTime: prevMonday.Unix()}
		advanceWeeklyWindow(sub, now)
		assert.EqualValues(t, 0, sub.WeeklyUsed)
		assert.True(t, sub.WeekNextResetTime > now, "next reset must be in the future")
	})
}

func TestTieredPreConsumeAndExhaust(t *testing.T) {
	truncateTables(t)

	basePlan := func() SubscriptionPlan {
		return SubscriptionPlan{
			Title:                "tiered-test",
			PriceAmount:          10,
			DurationUnit:         SubscriptionDurationMonth,
			DurationValue:        1,
			Enabled:              true,
			TotalAmount:          1000,
			QuotaResetPeriod:     SubscriptionResetMonthly,
			TieredLimitEnabled:   true,
			SessionLimitAmount:   200,
			SessionWindowSeconds: 18000,
			WeeklyLimitAmount:    600,
		}
	}

	t.Run("normal pre-consume charges all three tiers", func(t *testing.T) {
		env := setupTieredSubscription(t, basePlan(), 501)
		res, err := PreConsumeUserSubscription("tier-normal-req", 501, "gpt-test", 0, 100)
		require.NoError(t, err)
		assert.Equal(t, env.subId, res.UserSubscriptionId)
		assert.EqualValues(t, 100, res.PreConsumed)
		assert.True(t, res.SessionWindowStart > 0)
		assert.True(t, res.WeekNextResetTime > 0)

		sub := getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 100, sub.AmountUsed)
		assert.EqualValues(t, 100, sub.SessionUsed)
		assert.EqualValues(t, 100, sub.WeeklyUsed)
	})

	t.Run("session tier exhausted returns reset time", func(t *testing.T) {
		plan := basePlan()
		plan.SessionLimitAmount = 150
		env := setupTieredSubscription(t, plan, 502)
		_, err := PreConsumeUserSubscription("tier-sess-1", 502, "gpt-test", 0, 100)
		require.NoError(t, err)
		_, err = PreConsumeUserSubscription("tier-sess-2", 502, "gpt-test", 0, 100)
		var tierErr *TierExhaustedError
		require.ErrorAs(t, err, &tierErr)
		assert.Equal(t, SubscriptionTierSession, tierErr.Tier)

		sub := getSubscriptionRow(t, env.subId)
		assert.Equal(t, sub.SessionWindowStart+sub.SessionWindowSeconds, tierErr.ResetAt)
		// failed attempt must not change any counter
		assert.EqualValues(t, 100, sub.AmountUsed)
		assert.EqualValues(t, 100, sub.SessionUsed)
		assert.EqualValues(t, 100, sub.WeeklyUsed)
	})

	t.Run("weekly tier exhausted returns reset time", func(t *testing.T) {
		plan := basePlan()
		plan.WeeklyLimitAmount = 150
		env := setupTieredSubscription(t, plan, 503)
		_, err := PreConsumeUserSubscription("tier-week-1", 503, "gpt-test", 0, 100)
		require.NoError(t, err)
		_, err = PreConsumeUserSubscription("tier-week-2", 503, "gpt-test", 0, 100)
		var tierErr *TierExhaustedError
		require.ErrorAs(t, err, &tierErr)
		assert.Equal(t, SubscriptionTierWeekly, tierErr.Tier)

		sub := getSubscriptionRow(t, env.subId)
		assert.Equal(t, sub.WeekNextResetTime, tierErr.ResetAt)
		assert.EqualValues(t, 100, sub.WeeklyUsed)
	})

	t.Run("monthly exhausted keeps legacy error", func(t *testing.T) {
		plan := basePlan()
		plan.TotalAmount = 150
		plan.SessionLimitAmount = 1000
		plan.WeeklyLimitAmount = 1000
		setupTieredSubscription(t, plan, 504)
		_, err := PreConsumeUserSubscription("tier-month-1", 504, "gpt-test", 0, 100)
		require.NoError(t, err)
		_, err = PreConsumeUserSubscription("tier-month-2", 504, "gpt-test", 0, 100)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "subscription quota insufficient")
	})

	t.Run("idempotent replay deducts once", func(t *testing.T) {
		env := setupTieredSubscription(t, basePlan(), 505)
		first, err := PreConsumeUserSubscription("tier-idem-req", 505, "gpt-test", 0, 100)
		require.NoError(t, err)
		second, err := PreConsumeUserSubscription("tier-idem-req", 505, "gpt-test", 0, 100)
		require.NoError(t, err)
		assert.Equal(t, first.UserSubscriptionId, second.UserSubscriptionId)
		assert.Equal(t, first.PreConsumed, second.PreConsumed)
		assert.Equal(t, first.SessionWindowStart, second.SessionWindowStart)
		assert.Equal(t, first.WeekNextResetTime, second.WeekNextResetTime)

		sub := getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 100, sub.AmountUsed)
		assert.EqualValues(t, 100, sub.SessionUsed)
		assert.EqualValues(t, 100, sub.WeeklyUsed)
	})

	t.Run("expired session window resets before charging", func(t *testing.T) {
		plan := basePlan()
		plan.SessionWindowSeconds = 60
		plan.SessionLimitAmount = 100
		env := setupTieredSubscription(t, plan, 506)
		_, err := PreConsumeUserSubscription("tier-roll-1", 506, "gpt-test", 0, 80)
		require.NoError(t, err)

		// simulate the 60s window having rolled
		sub := getSubscriptionRow(t, env.subId)
		require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", env.subId).
			Update("session_window_start", sub.SessionWindowStart-120).Error)

		_, err = PreConsumeUserSubscription("tier-roll-2", 506, "gpt-test", 0, 80)
		require.NoError(t, err)
		sub = getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 80, sub.SessionUsed, "session usage must reset with the window")
		assert.EqualValues(t, 160, sub.AmountUsed, "monthly usage accumulates")
		assert.EqualValues(t, 160, sub.WeeklyUsed)
	})

	t.Run("non-tiered subscription keeps legacy behavior", func(t *testing.T) {
		plan := basePlan()
		plan.TieredLimitEnabled = false
		plan.TotalAmount = 150
		env := setupTieredSubscription(t, plan, 507)
		_, err := PreConsumeUserSubscription("tier-legacy-1", 507, "gpt-test", 0, 100)
		require.NoError(t, err)
		_, err = PreConsumeUserSubscription("tier-legacy-2", 507, "gpt-test", 0, 100)
		require.ErrorContains(t, err, "subscription quota insufficient")

		sub := getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 100, sub.AmountUsed)
		assert.EqualValues(t, 0, sub.SessionUsed)
		assert.EqualValues(t, 0, sub.WeeklyUsed)
	})
}

func TestTieredRefundAcrossWindowRoll(t *testing.T) {
	truncateTables(t)

	plan := SubscriptionPlan{
		Title:                "tiered-refund-test",
		PriceAmount:          10,
		DurationUnit:         SubscriptionDurationMonth,
		DurationValue:        1,
		Enabled:              true,
		TotalAmount:          1000,
		QuotaResetPeriod:     SubscriptionResetMonthly,
		TieredLimitEnabled:   true,
		SessionLimitAmount:   500,
		SessionWindowSeconds: 18000,
		WeeklyLimitAmount:    900,
	}
	env := setupTieredSubscription(t, plan, 601)
	res, err := PreConsumeUserSubscription("tier-refund-1", 601, "gpt-test", 0, 100)
	require.NoError(t, err)

	// Simulate the windows having rolled since pre-consume: a freshly
	// re-anchored session window and a later weekly window anchor.
	now := GetDBTimestamp()
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", env.subId).
		Updates(map[string]interface{}{
			"session_window_start": now - 60,
			"session_used":         100,
			"week_next_reset_time": res.WeekNextResetTime + 7*86400,
			"weekly_used":          100,
		}).Error)

	// Refund must credit only the monthly tier; the rolled session/weekly
	// windows (anchors no longer match the record) must not receive quota back.
	require.NoError(t, RefundSubscriptionPreConsume("tier-refund-1"))
	sub := getSubscriptionRow(t, env.subId)
	assert.EqualValues(t, 0, sub.AmountUsed, "monthly must be refunded")
	assert.EqualValues(t, 100, sub.SessionUsed, "re-anchored session window must not be credited")
	assert.EqualValues(t, 100, sub.WeeklyUsed, "rolled weekly window must not be credited")

	// refund is idempotent
	require.NoError(t, RefundSubscriptionPreConsume("tier-refund-1"))
	sub = getSubscriptionRow(t, env.subId)
	assert.EqualValues(t, 0, sub.AmountUsed)
	assert.EqualValues(t, 100, sub.SessionUsed)
	assert.EqualValues(t, 100, sub.WeeklyUsed)
}

func TestTieredRefundWithMatchingAnchors(t *testing.T) {
	truncateTables(t)

	plan := SubscriptionPlan{
		Title:                "tiered-refund-match",
		PriceAmount:          10,
		DurationUnit:         SubscriptionDurationMonth,
		DurationValue:        1,
		Enabled:              true,
		TotalAmount:          1000,
		QuotaResetPeriod:     SubscriptionResetMonthly,
		TieredLimitEnabled:   true,
		SessionLimitAmount:   500,
		SessionWindowSeconds: 18000,
		WeeklyLimitAmount:    900,
	}
	env := setupTieredSubscription(t, plan, 602)
	_, err := PreConsumeUserSubscription("tier-refund-match-req", 602, "gpt-test", 0, 100)
	require.NoError(t, err)

	require.NoError(t, RefundSubscriptionPreConsume("tier-refund-match-req"))
	sub := getSubscriptionRow(t, env.subId)
	assert.EqualValues(t, 0, sub.AmountUsed)
	assert.EqualValues(t, 0, sub.SessionUsed, "active window refund must credit session tier")
	assert.EqualValues(t, 0, sub.WeeklyUsed, "active window refund must credit weekly tier")
}

func TestTieredReserveDelta(t *testing.T) {
	truncateTables(t)

	plan := SubscriptionPlan{
		Title:                "tiered-reserve-test",
		PriceAmount:          10,
		DurationUnit:         SubscriptionDurationMonth,
		DurationValue:        1,
		Enabled:              true,
		TotalAmount:          1000,
		QuotaResetPeriod:     SubscriptionResetMonthly,
		TieredLimitEnabled:   true,
		SessionLimitAmount:   200,
		SessionWindowSeconds: 18000,
		WeeklyLimitAmount:    800,
	}
	env := setupTieredSubscription(t, plan, 701)
	res, err := PreConsumeUserSubscription("tier-reserve-req", 701, "gpt-test", 0, 100)
	require.NoError(t, err)

	t.Run("reserve within limits charges all tiers", func(t *testing.T) {
		require.NoError(t, PostConsumeUserSubscriptionTieredDelta(env.subId, 50,
			res.SessionWindowStart, res.WeekNextResetTime))
		sub := getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 150, sub.AmountUsed)
		assert.EqualValues(t, 150, sub.SessionUsed)
		assert.EqualValues(t, 150, sub.WeeklyUsed)
	})

	t.Run("reserve beyond session limit fails with tier error", func(t *testing.T) {
		err := PostConsumeUserSubscriptionTieredDelta(env.subId, 60,
			res.SessionWindowStart, res.WeekNextResetTime)
		var tierErr *TierExhaustedError
		require.ErrorAs(t, err, &tierErr)
		assert.Equal(t, SubscriptionTierSession, tierErr.Tier)

		sub := getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 150, sub.AmountUsed, "failed reserve must not change counters")
		assert.EqualValues(t, 150, sub.SessionUsed)
		assert.EqualValues(t, 150, sub.WeeklyUsed)
	})

	t.Run("settle negative delta clamps at zero", func(t *testing.T) {
		require.NoError(t, PostConsumeUserSubscriptionTieredDelta(env.subId, -1000,
			res.SessionWindowStart, res.WeekNextResetTime))
		sub := getSubscriptionRow(t, env.subId)
		assert.EqualValues(t, 0, sub.AmountUsed)
		assert.EqualValues(t, 0, sub.SessionUsed)
		assert.EqualValues(t, 0, sub.WeeklyUsed)
	})
}

func TestTierExhaustedErrorMessage(t *testing.T) {
	err := &TierExhaustedError{Tier: SubscriptionTierSession, ResetAt: 1710000000}
	assert.Contains(t, err.Error(), SubscriptionTierSession)
	assert.Contains(t, err.Error(), "1710000000")

	var target *TierExhaustedError
	wrapped := fmt.Errorf("wrap: %w", err)
	require.True(t, errors.As(wrapped, &target))
	assert.EqualValues(t, 1710000000, target.ResetAt)
}
