package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tier-exhausted mapping must produce a 429 with a distinct error code so
// the relay loop neither retries other channels nor falls back to the wallet.
func TestSubscriptionTierLimitAPIError(t *testing.T) {
	sessionErr := &model.TierExhaustedError{Tier: model.SubscriptionTierSession, ResetAt: 1710000000}
	apiErr := subscriptionTierLimitAPIError(sessionErr)

	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeSubscriptionTierExhausted, apiErr.GetErrorCode())
	// must not be confused with monthly insufficiency, which allows wallet fallback
	assert.NotEqual(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr))
	assert.False(t, types.IsRecordErrorLog(apiErr))
	assert.Contains(t, apiErr.Error(), "会话")

	weeklyErr := &model.TierExhaustedError{Tier: model.SubscriptionTierWeekly, ResetAt: 1710000000}
	weeklyAPIErr := subscriptionTierLimitAPIError(weeklyErr)
	assert.Equal(t, http.StatusTooManyRequests, weeklyAPIErr.StatusCode)
	assert.Contains(t, weeklyAPIErr.Error(), "每周")
}
