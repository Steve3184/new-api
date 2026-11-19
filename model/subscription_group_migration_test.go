package model

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionGroupAutoCreateKeyMigrationMatrix(t *testing.T) {
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldUsable := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"Go Plan":1,"go plan":1,"vip":1,"\u0301":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"go plan":"Legacy"}`))
	for _, dialect := range []common.DatabaseType{common.DatabaseTypeSQLite, common.DatabaseTypeMySQL, common.DatabaseTypePostgreSQL} {
		t.Run(string(dialect), func(t *testing.T) {
			var dsn string
			switch dialect {
			case common.DatabaseTypeSQLite:
				oldPath := common.SQLitePath
				common.SQLitePath = filepath.Join(t.TempDir(), "old.db")
				t.Cleanup(func() { common.SQLitePath = oldPath })
				dsn = "local"
			case common.DatabaseTypeMySQL:
				dsn = os.Getenv("TEST_MYSQL_DSN")
			case common.DatabaseTypePostgreSQL:
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			if dsn == "" {
				t.Skip("test database DSN is not configured")
			}
			t.Setenv("SUBSCRIPTION_MIGRATION_TEST_DSN", dsn)
			db, _, err := chooseDB("SUBSCRIPTION_MIGRATION_TEST_DSN", false)
			require.NoError(t, err)
			conn, err := db.DB()
			require.NoError(t, err)
			oldDB, oldDialect := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(dialect)
			initCol()
			t.Cleanup(func() {
				DB = oldDB
				common.SetMainDatabaseType(oldDialect)
				initCol()
				assert.NoError(t, conn.Close())
			})

			// Use the real table/column names in the isolated test database.
			tables := []any{&Option{}, &SubscriptionPlan{}, &UserSubscription{}, &SubscriptionOrder{}, &SubscriptionMigration{}, &User{}}
			for _, table := range tables {
				require.False(t, db.Migrator().HasTable(table), "use a fresh, isolated database")
			}
			require.NoError(t, db.AutoMigrate(tables...))
			t.Cleanup(func() {
				assert.NoError(t, db.Migrator().DropTable(tables...))
			})
			if dialect == common.DatabaseTypeMySQL {
				require.NoError(t, db.Exec("ALTER TABLE subscription_plans MODIFY subscription_group VARCHAR(64) COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT ''").Error)
				var equivalent bool
				require.NoError(t, db.Raw("SELECT _utf8mb4 0xCC81 COLLATE utf8mb4_0900_ai_ci = ''").Scan(&equivalent).Error)
				require.True(t, equivalent)
			}
			if dialect == common.DatabaseTypeSQLite {
				require.NoError(t, db.Exec("ALTER TABLE subscription_plans DROP COLUMN auto_create_key").Error)
			} else {
				require.NoError(t, db.Migrator().DropColumn(&SubscriptionPlan{}, "auto_create_key"))
			}
			for _, plan := range []SubscriptionPlan{
				{Id: 11, Title: "Pro", SubscriptionGroup: "Go Plan", PriceAmount: 12},
				{Id: 12, Title: "Archived", SubscriptionGroup: "Go Plan"},
				{Id: 13, Title: "Case variant", SubscriptionGroup: "go plan"},
				{Id: 14, Title: "Legacy", UpgradeGroup: "Go Plan"},
				{Id: 15, Title: "No key", SubscriptionGroup: "vip"},
				{Id: 16, Title: "Collation-ignorable name", SubscriptionGroup: "\u0301"},
			} {
				require.NoError(t, db.Omit("AutoCreateKey").Create(&plan).Error)
			}
			require.NoError(t, db.Model(&SubscriptionPlan{}).Where("id = ?", 12).UpdateColumn("enabled", false).Error)
			now := common.GetTimestamp()
			require.NoError(t, db.Create(&User{Id: 77, Username: "migration legacy holder", Group: "go plan"}).Error)
			legacy := UserSubscription{UserId: 77, PlanId: 14, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100, PriceAmount: common.GetPointer(12.0)}
			legacy.LegacyCycleGroups = "{}"
			require.NoError(t, db.Create(&legacy).Error)
			historical := UserSubscription{UserId: 77, PlanId: 14, Status: "active", Source: "order", StartTime: now - 180, EndTime: now + 3500, AmountTotal: 100}
			require.NoError(t, db.Omit("LegacyCycleGroups").Create(&historical).Error)
			require.NoError(t, db.Model(&historical).UpdateColumn("created_at", historical.StartTime).Error)
			require.NoError(t, db.Create(&SubscriptionOrder{UserId: 77, PlanId: 14, TradeNo: "legacy-paid-before-price-snapshot", Status: common.TopUpStatusSuccess, Money: 12, CreateTime: now - 240, CompleteTime: historical.StartTime + 10}).Error)
			pending := SubscriptionOrder{UserId: 77, PlanId: 14, TradeNo: "legacy-pending-at-upgrade", Status: common.TopUpStatusPending, Money: 12}
			require.NoError(t, db.Omit("LegacyCycleGroups").Create(&pending).Error)
			old := Option{Key: retiredSubscriptionGroupsOptionKey, Value: `{"Go Plan":{"auto_create_key":true},"vip":{"auto_create_key":false},"orphan":{"auto_create_key":true},"\u0301":{"auto_create_key":true}}`}
			require.NoError(t, db.Create(&old).Error)

			migrate := func() {
				if dialect == common.DatabaseTypeSQLite {
					require.NoError(t, ensureSubscriptionPlanTableSQLite())
				} else {
					require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}))
				}
				require.NoError(t, migrateSubscriptionGroupAutoCreateKeys(db))
			}
			require.ErrorIs(t, requireSubscriptionBindingMigration(db), errSubscriptionBindingMigrationPending)
			migrate()
			require.NoError(t, requireSubscriptionBindingMigration(db))
			require.NoError(t, db.Model(&pending).UpdateColumn("legacy_cycle_groups", nil).Error)
			require.NoError(t, requireSubscriptionBindingMigration(db), "an unrelated NULL row after migration does not revoke the committed readiness result")
			require.NoError(t, db.Model(&pending).UpdateColumn("legacy_cycle_groups", "{}").Error)
			var enabled []int
			require.NoError(t, db.Model(&SubscriptionPlan{}).Where("auto_create_key = ?", true).Order("id").Pluck("id", &enabled).Error)
			assert.Equal(t, []int{11, 12, 16}, enabled)
			require.NoError(t, db.First(&legacy, legacy.Id).Error)
			assert.Equal(t, legacyCycleGrant{Access: []string{"go plan"}, Quota: []string{"go plan"}}, legacy.legacyCycleGrant(), "only newly activated, exact group names are grandfathered")
			require.NoError(t, db.First(&historical, historical.Id).Error)
			assert.Equal(t, legacy.legacyCycleGrant(), historical.legacyCycleGrant(), "the actual paid order is honored even though the plan's current price is zero")
			var pro SubscriptionPlan
			require.NoError(t, db.First(&pro, 11).Error)
			assert.Equal(t, 12.0, pro.PriceAmount)
			var retiredCount int64
			require.NoError(t, db.Model(&Option{}).Where(&Option{Key: old.Key}).Count(&retiredCount).Error)
			assert.Zero(t, retiredCount)

			require.NoError(t, db.Model(&pro).UpdateColumn("auto_create_key", false).Error)
			migrate()
			require.NoError(t, db.First(&pro, 11).Error)
			assert.False(t, pro.AutoCreateKey, "a later plan edit survives restart")
			t.Run("reupgrade_applies_explicit_disabled_setting", func(t *testing.T) {
				require.NoError(t, db.Model(&pro).UpdateColumn("auto_create_key", true).Error)
				require.NoError(t, db.Delete(&SubscriptionMigration{Name: subscriptionBindingMigrationName}).Error)
				require.NoError(t, db.Create(&Option{Key: retiredSubscriptionGroupsOptionKey, Value: `{"Go Plan":{"auto_create_key":false}}`}).Error)
				migrate()
				require.NoError(t, db.First(&pro, 11).Error)
				assert.False(t, pro.AutoCreateKey, "the current old-version switch must override the retained column")
			})
			t.Run("reupgrade_disables_keys_for_removed_legacy_option", func(t *testing.T) {
				require.NoError(t, db.Model(&pro).UpdateColumn("auto_create_key", true).Error)
				require.NoError(t, db.Delete(&SubscriptionMigration{Name: subscriptionBindingMigrationName}).Error)
				migrate()
				require.NoError(t, db.First(&pro, 11).Error)
				assert.False(t, pro.AutoCreateKey)
			})
			require.NoError(t, db.Delete(&SubscriptionPlan{}, 11).Error)
			bound, err := IsSubscriptionGroup("Go Plan")
			require.NoError(t, err)
			assert.True(t, bound, "disabled plans still bind the group")
			require.NoError(t, db.Delete(&SubscriptionPlan{}, 12).Error)
			bound, err = IsSubscriptionGroup("Go Plan")
			require.NoError(t, err)
			assert.False(t, bound, "a case variant does not count as the last binding")
			names, err := SubscriptionGroupNames()
			require.NoError(t, err)
			assert.Equal(t, []string{"go plan", "vip", "\u0301"}, names)
			if dialect == common.DatabaseTypeSQLite {
				t.Run("leading_nul_binding_is_not_empty", func(t *testing.T) {
					const group = "\x00bound"
					require.NoError(t, db.Create(&SubscriptionPlan{Title: "Nonempty name", SubscriptionGroup: group}).Error)
					bound, err := IsSubscriptionGroup(group)
					require.NoError(t, err)
					require.True(t, bound)
					names, err := SubscriptionGroupNames()
					require.NoError(t, err)
					require.Contains(t, names, group, "enumeration and exact lookup must agree")
				})
			}
			if dialect != common.DatabaseTypeSQLite {
				t.Run("stale_snapshot_cannot_rebind_after_grant", func(t *testing.T) {
					stale := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
					require.NoError(t, stale.Error)
					defer stale.Rollback()
					var count int64
					require.NoError(t, stale.Model(&UserSubscription{}).Where("plan_id = ?", 15).Count(&count).Error)
					require.Zero(t, count)
					require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
						plan, err := syncSubscriptionPlanEntitlementTx(tx, 15)
						if err != nil {
							return err
						}
						_, err = CreateUserSubscriptionFromPlanTx(tx, 77, plan, "admin")
						return err
					}))
					target := "go plan"
					_, err := CheckSubscriptionPlanGroupChangeTx(stale, 15, &target)
					require.Error(t, err, "the committed active grant must block rebind even with a stale snapshot")
				})
			}
			if dialect == common.DatabaseTypeMySQL {
				t.Run("read_committed_binding_serializes_new_plans", func(t *testing.T) {
					tx := db.Begin(&sql.TxOptions{Isolation: sql.LevelReadCommitted})
					require.NoError(t, tx.Error)
					defer tx.Rollback()
					group := "new-scoped-group"
					_, err := CheckSubscriptionPlanGroupChangeTx(tx, 13, &group)
					require.NoError(t, err)
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					fresh := SubscriptionPlan{Title: "New ordinary plan", TotalAmount: 100}
					err = createSubscriptionPlanWithBinding(db.WithContext(ctx), &fresh)
					require.ErrorIs(t, err, context.DeadlineExceeded, "a new plan must wait for the uncommitted binding change")
					require.NoError(t, tx.Rollback().Error)
					require.NoError(t, CreateSubscriptionPlanWithBinding(&fresh))
				})
				t.Run("binding_uses_latest_legacy_identity", func(t *testing.T) {
					group := "mysql-current-identity"
					require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"Go Plan":1,"go plan":1,"vip":1,"\u0301":1,"mysql-current-identity":1}`))
					binding := SubscriptionPlan{Title: "Change binding", SubscriptionGroup: "other"}
					require.NoError(t, db.Create(&binding).Error)
					user := User{Username: "mysql latest identity", Group: "default", AffCode: "mysql-latest"}
					require.NoError(t, db.Create(&user).Error)
					cycle := UserSubscription{UserId: user.Id, PlanId: 14, PriceAmount: common.GetPointer(1.0), AmountTotal: 100, Status: "active", StartTime: now - 60, EndTime: now + 3600}
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					require.NoError(t, subscriptionPlanTransaction(db.WithContext(ctx), func(tx *gorm.DB) error {
						require.NoError(t, tx.First(&SubscriptionPlan{}, binding.Id).Error)
						require.NoError(t, db.Transaction(func(writer *gorm.DB) error {
							if err := writer.Model(&user).UpdateColumn("group", group).Error; err != nil {
								return err
							}
							return writer.Create(&cycle).Error
						}))
						buyer := db.Begin()
						require.NoError(t, buyer.Error)
						defer buyer.Rollback()
						require.NoError(t, lockForUpdate(buyer).First(&User{}, user.Id).Error)
						_, err := CheckSubscriptionPlanGroupChangeTx(tx, binding.Id, &group)
						return err
					}))
					require.NoError(t, db.First(&cycle, cycle.Id).Error)
					assert.Contains(t, cycle.legacyCycleGrant().Quota, group)
				})
			}
			if dialect == common.DatabaseTypePostgreSQL {
				t.Run("stale_catalog_cannot_omit_new_legacy_plan", func(t *testing.T) {
					binding := SubscriptionPlan{Title: "Change binding", SubscriptionGroup: "other"}
					require.NoError(t, db.Create(&binding).Error)
					stale := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
					require.NoError(t, stale.Error)
					defer stale.Rollback()
					require.NoError(t, stale.First(&SubscriptionPlan{}, binding.Id).Error)
					fresh := SubscriptionPlan{Title: "New ordinary plan", TotalAmount: 100}
					require.NoError(t, CreateSubscriptionPlanWithBinding(&fresh))
					group := "new-bound-group"
					_, err := CheckSubscriptionPlanGroupChangeTx(stale, binding.Id, &group)
					require.ErrorContains(t, err, "40001", "structural writes must not commit a stale catalog")
				})
				t.Run("unchanged_binding_does_not_lock_unrelated_plans", func(t *testing.T) {
					busy := db.Begin()
					require.NoError(t, busy.Error)
					defer busy.Rollback()
					var unrelated SubscriptionPlan
					require.NoError(t, lockForUpdate(busy).First(&unrelated, 14).Error)
					require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
						if err := tx.Exec("SET LOCAL lock_timeout = '300ms'").Error; err != nil {
							return err
						}
						group := "vip"
						_, err := CheckSubscriptionPlanGroupChangeTx(tx, 15, &group)
						return err
					}))
				})
				t.Run("first_binding_invalidates_legacy_purchase_snapshot", func(t *testing.T) {
					legacyPlan := &SubscriptionPlan{Title: "Legacy purchase", TotalAmount: 100, WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "Go Plan"}
					require.NoError(t, db.Create(legacyPlan).Error)
					stale := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
					require.NoError(t, stale.Error)
					defer stale.Rollback()
					var user User
					require.NoError(t, lockForUpdate(stale).Select("id").First(&user, 77).Error)
					require.NoError(t, CreateSubscriptionPlanWithBinding(&SubscriptionPlan{Title: "First binding", SubscriptionGroup: "Go Plan"}))
					_, err := syncSubscriptionPlanEntitlementTx(stale, legacyPlan.Id)
					require.ErrorContains(t, err, "40001", "a stale checkout cannot continue after the first binding commits")
				})
			}
		})
	}
}
