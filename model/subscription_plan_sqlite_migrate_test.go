package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestEnsureSubscriptionPlanTableSQLiteAddsTieredColumns guards the SQLite migration path
// for the tiered limit columns. The subscription_plans table is NOT covered by GORM
// AutoMigrate on SQLite (it is managed by ensureSubscriptionPlanTableSQLite), so an
// upgrade from an older schema must ADD COLUMN these fields or plan writes fail with
// "table subscription_plans has no column named tiered_limit_enabled".
func TestEnsureSubscriptionPlanTableSQLiteAddsTieredColumns(t *testing.T) {
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Simulate an OLD table without the tiered columns.
	oldSQL := `CREATE TABLE ` + "`subscription_plans`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`allow_balance_pay`" + ` numeric DEFAULT 1,
` + "`allow_wallet_overflow`" + ` numeric DEFAULT 1,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`waffo_pancake_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`downgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
	require.NoError(t, db.Exec(oldSQL).Error)

	orig := DB
	DB = db
	defer func() { DB = orig }()

	require.NoError(t, ensureSubscriptionPlanTableSQLite())

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	require.NoError(t, db.Raw("PRAGMA table_info(`subscription_plans`)").Scan(&cols).Error)
	got := map[string]bool{}
	for _, c := range cols {
		got[c.Name] = true
	}
	for _, want := range []string{"tiered_limit_enabled", "session_limit_amount", "session_window_seconds", "weekly_limit_amount"} {
		require.True(t, got[want], "missing column %s", want)
	}

	// A fresh insert with the new columns must work (this is the failing write from the bug report).
	require.NoError(t, db.Create(&SubscriptionPlan{
		Title:                "tiered",
		TieredLimitEnabled:   true,
		SessionLimitAmount:   100,
		SessionWindowSeconds: 18000,
		WeeklyLimitAmount:    500,
	}).Error)

	// Running again must be idempotent.
	require.NoError(t, ensureSubscriptionPlanTableSQLite())
}
