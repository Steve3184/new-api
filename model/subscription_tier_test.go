package model

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNormalizeSubscriptionUsageTiers(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "", want: "[]"},
		{name: "sorted and trimmed", raw: `[{"multiplier":5,"price_amount":40,"stripe_price_id":" price_5 "},{"multiplier":2,"price_amount":18}]`, want: `[{"multiplier":2,"price_amount":18},{"multiplier":5,"price_amount":40,"stripe_price_id":"price_5"}]`},
		{name: "1x is the base plan", raw: `[{"multiplier":1,"price_amount":10}]`, wantErr: true},
		{name: "multiplier above bound", raw: `[{"multiplier":101,"price_amount":10}]`, wantErr: true},
		{name: "duplicate multiplier", raw: `[{"multiplier":2,"price_amount":10},{"multiplier":2,"price_amount":12}]`, wantErr: true},
		{name: "negative price", raw: `[{"multiplier":2,"price_amount":-1}]`, wantErr: true},
		{name: "malformed", raw: `{"multiplier":2}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := NormalizeSubscriptionUsageTiers(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSubscriptionUsageTierLimitsSaturate(t *testing.T) {
	plan := SubscriptionPlan{TotalAmount: common.MaxWalletQuota / 2, WeeklyLimit: 100, FiveHourLimit: 0, UsageTiers: `[{"multiplier":3,"price_amount":1}]`}
	assert.False(t, plan.SubscriptionUsageTiersFitQuota())
	view, err := plan.ForUsageTier(3)
	require.NoError(t, err)
	assert.EqualValues(t, common.MaxWalletQuota, view.TotalAmount)
	assert.EqualValues(t, 300, view.WeeklyLimit)
	assert.Zero(t, view.FiveHourLimit)
}

// TestSubscriptionTiersUpgradeAndRenewalDatabaseMatrix upgrades a released
// schema in place and then exercises tier purchase, gateway completion,
// prorated upgrade and balance auto-renewal on every supported database.
func TestSubscriptionTiersUpgradeAndRenewalDatabaseMatrix(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var driver gorm.Dialector
			switch engine {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			prefix := fmt.Sprintf("subtier_%d_", time.Now().UnixNano())
			if engine == "sqlite" {
				prefix = ""
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
			require.NoError(t, err)
			var version string
			versionQuery := "SELECT version()"
			if engine == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("Database version: %s", version)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			if engine == "sqlite" {
				sqlDB.SetMaxOpenConns(1)
			}
			oldDB, oldLogDB, oldType, oldLogType := DB, LOG_DB, common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(common.DatabaseType(engine), common.DatabaseType(engine))
			initCol()
			tables := []any{&SubscriptionOrder{}, &UserSubscription{}, &SubscriptionPlan{}, &User{}, &TopUp{}, &Log{}}
			t.Cleanup(func() {
				DB, LOG_DB = oldDB, oldLogDB
				common.SetDatabaseTypes(oldType, oldLogType)
				initCol()
				require.NoError(t, db.Migrator().DropTable(tables...))
				require.NoError(t, sqlDB.Close())
			})
			migrate := func() {
				for range 2 {
					if engine == "sqlite" {
						require.NoError(t, ensureSubscriptionPlanTableSQLite())
					}
					require.NoError(t, db.AutoMigrate(tables...))
				}
			}
			migrate()

			// Recreate the released schema: the new tier, renewal and upgrade
			// columns are absent and the rows below predate them.
			for _, column := range []string{"usage_tiers", "allow_tier_switch", "upgrade_plan_ids", "allow_auto_renew"} {
				require.NoError(t, db.Migrator().DropColumn(&SubscriptionPlan{}, column))
			}
			for _, column := range []string{"multiplier", "price_amount", "auto_renew", "renew_multiplier"} {
				require.NoError(t, db.Migrator().DropColumn(&UserSubscription{}, column))
			}
			for _, column := range []string{"multiplier", "order_type"} {
				require.NoError(t, db.Migrator().DropColumn(&SubscriptionOrder{}, column))
			}
			newPlanColumns := []string{"UsageTiers", "AllowTierSwitch", "UpgradePlanIds", "AllowAutoRenew"}
			pro := &SubscriptionPlan{Title: "Pro", PriceAmount: 10, TotalAmount: 1000, WeeklyLimit: 300, DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, AllowBalancePay: common.GetPointer(true), QuotaResetPeriod: SubscriptionResetNever}
			require.NoError(t, db.Omit(newPlanColumns...).Create(pro).Error)
			maxPlan := &SubscriptionPlan{Title: "Max", PriceAmount: 30, TotalAmount: 5000, DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, AllowBalancePay: common.GetPointer(true), UpgradeGroup: "vip", QuotaResetPeriod: SubscriptionResetNever}
			require.NoError(t, db.Omit(newPlanColumns...).Create(maxPlan).Error)
			legacyUser := &User{Username: "tier-legacy", AffCode: "tier-legacy", Group: "default", Quota: 100 * int(common.QuotaPerUnit)}
			require.NoError(t, db.Create(legacyUser).Error)
			now := getDBTimestamp(db)
			legacySub := &UserSubscription{UserId: legacyUser.Id, PlanId: pro.Id, Status: "active", Source: "order", AmountTotal: 1000, AmountUsed: 800, StartTime: now - 500000, EndTime: now + 500000}
			require.NoError(t, db.Omit("Multiplier", "PriceAmount", "AutoRenew", "RenewMultiplier").Create(legacySub).Error)
			migrate()

			var migrated UserSubscription
			require.NoError(t, db.First(&migrated, legacySub.Id).Error)
			assert.Equal(t, 1, migrated.Multiplier)
			assert.Nil(t, migrated.PriceAmount)
			assert.False(t, migrated.AutoRenew)

			require.NoError(t, db.Model(pro).Updates(map[string]any{
				"usage_tiers":       `[{"multiplier":5,"price_amount":40}]`,
				"allow_tier_switch": true,
				"allow_auto_renew":  true,
				"upgrade_plan_ids":  fmt.Sprint(maxPlan.Id),
			}).Error)
			InvalidateSubscriptionPlanCache(pro.Id)
			InvalidateSubscriptionPlanCache(maxPlan.Id)
			quotaPerUnit := int(common.QuotaPerUnit)

			// A 5x balance purchase scales every limit and charges the tier price.
			buyer := &User{Username: "tier-buyer", AffCode: "tier-buyer", Group: "default", Quota: 100 * quotaPerUnit}
			require.NoError(t, db.Create(buyer).Error)
			require.NoError(t, PurchaseSubscriptionWithBalance(buyer.Id, pro.Id, 5, true))
			require.Error(t, PurchaseSubscriptionWithBalance(buyer.Id, pro.Id, 3, false))
			var bought UserSubscription
			require.NoError(t, db.Where("user_id = ?", buyer.Id).First(&bought).Error)
			assert.Equal(t, 5, bought.Multiplier)
			assert.EqualValues(t, 5000, bought.AmountTotal)
			assert.EqualValues(t, 1500, bought.WeeklyLimit)
			require.NotNil(t, bought.PriceAmount)
			assert.InDelta(t, 40, *bought.PriceAmount, 1e-9)
			assert.True(t, bought.AutoRenew)
			var purchaseOrder SubscriptionOrder
			require.NoError(t, db.Where("user_id = ? AND order_type = ?", buyer.Id, SubscriptionOrderTypePurchase).First(&purchaseOrder).Error)
			assert.Equal(t, 5, purchaseOrder.Multiplier)
			require.NoError(t, db.First(buyer, buyer.Id).Error)
			assert.Equal(t, 60*quotaPerUnit, buyer.Quota)

			// A paid gateway order completes with the tier it was placed for.
			gatewayUser := &User{Username: "tier-gateway", AffCode: "tier-gateway", Group: "default"}
			require.NoError(t, db.Create(gatewayUser).Error)
			order := &SubscriptionOrder{UserId: gatewayUser.Id, PlanId: pro.Id, Money: 40, TradeNo: "tier-gateway-order", PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusPending, Multiplier: 5}
			require.NoError(t, order.Insert())
			require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "", PaymentProviderStripe, ""))
			var gatewaySub UserSubscription
			require.NoError(t, db.Where("user_id = ?", gatewayUser.Id).First(&gatewaySub).Error)
			assert.Equal(t, 5, gatewaySub.Multiplier)
			assert.EqualValues(t, 5000, gatewaySub.AmountTotal)

			// Lower tiers and unlisted plans cannot be switched to immediately.
			_, err = QuoteSubscriptionUpgrade(buyer.Id, bought.Id, pro.Id, 1)
			require.Error(t, err)
			_, err = QuoteSubscriptionUpgrade(legacyUser.Id, bought.Id, maxPlan.Id, 1)
			require.Error(t, err, "another user's subscription")

			// The legacy subscription is credited at its plan price, capped by
			// its unused quota share (20%) because the quota never resets.
			quote, err := QuoteSubscriptionUpgrade(legacyUser.Id, legacySub.Id, maxPlan.Id, 1)
			require.NoError(t, err)
			assert.Equal(t, 30*quotaPerUnit, quote.PriceQuota)
			assert.Equal(t, 2*quotaPerUnit, quote.CreditQuota)
			assert.Equal(t, 28*quotaPerUnit, quote.ChargeQuota)
			upgraded, err := UpgradeSubscriptionWithBalance(legacyUser.Id, legacySub.Id, maxPlan.Id, 1)
			require.NoError(t, err)
			assert.Equal(t, quote.ChargeQuota, upgraded.ChargeQuota)
			require.NoError(t, db.First(legacyUser, legacyUser.Id).Error)
			assert.Equal(t, 72*quotaPerUnit, legacyUser.Quota)
			assert.Equal(t, "vip", legacyUser.Group)
			var replaced UserSubscription
			require.NoError(t, db.First(&replaced, legacySub.Id).Error)
			assert.Equal(t, SubscriptionStatusUpgraded, replaced.Status)
			assert.LessOrEqual(t, replaced.EndTime, getDBTimestamp(db))
			var upgradedSub UserSubscription
			require.NoError(t, db.Where("user_id = ? AND status = ?", legacyUser.Id, "active").First(&upgradedSub).Error)
			assert.Equal(t, maxPlan.Id, upgradedSub.PlanId)
			assert.Equal(t, SubscriptionSourceUpgrade, upgradedSub.Source)
			assert.EqualValues(t, 5000, upgradedSub.AmountTotal)
			assert.Equal(t, "default", upgradedSub.PrevUserGroup)
			_, err = QuoteSubscriptionUpgrade(legacyUser.Id, upgradedSub.Id, pro.Id, 1)
			require.Error(t, err, "Max lists no upgrade targets")
			require.Error(t, UpdateUserSubscriptionRenewal(legacyUser.Id, upgradedSub.Id, true, 0), "Max does not allow auto-renewal")

			// A same-plan tier switch replaces the subscription and keeps
			// auto-renewal on.
			switcher := &User{Username: "tier-switch", AffCode: "tier-switch", Group: "default", Quota: 100 * quotaPerUnit}
			require.NoError(t, db.Create(switcher).Error)
			require.NoError(t, PurchaseSubscriptionWithBalance(switcher.Id, pro.Id, 1, true))
			var switchFrom UserSubscription
			require.NoError(t, db.Where("user_id = ?", switcher.Id).First(&switchFrom).Error)
			_, err = UpgradeSubscriptionWithBalance(switcher.Id, switchFrom.Id, pro.Id, 5)
			require.NoError(t, err)
			var switched UserSubscription
			require.NoError(t, db.Where("user_id = ? AND status = ?", switcher.Id, "active").First(&switched).Error)
			assert.Equal(t, 5, switched.Multiplier)
			assert.True(t, switched.AutoRenew)
			assert.EqualValues(t, 5000, switched.AmountTotal)

			// Auto-renewal starts the next period at the old end with the tier
			// chosen for renewal and charges its price.
			require.NoError(t, UpdateUserSubscriptionRenewal(buyer.Id, bought.Id, true, 1))
			dueEnd := getDBTimestamp(db) + 30
			require.NoError(t, db.Model(&bought).Update("end_time", dueEnd).Error)
			renewed, err := RenewDueSubscriptions(10, 2*time.Minute)
			require.NoError(t, err)
			assert.Equal(t, 1, renewed)
			var renewedSub UserSubscription
			require.NoError(t, db.First(&renewedSub, bought.Id).Error)
			assert.Equal(t, dueEnd, renewedSub.StartTime)
			assert.Equal(t, time.Unix(dueEnd, 0).AddDate(0, 1, 0).Unix(), renewedSub.EndTime)
			assert.Equal(t, 1, renewedSub.Multiplier)
			assert.Zero(t, renewedSub.RenewMultiplier)
			assert.EqualValues(t, 1000, renewedSub.AmountTotal)
			assert.True(t, renewedSub.AutoRenew)
			require.NoError(t, db.First(buyer, buyer.Id).Error)
			assert.Equal(t, 50*quotaPerUnit, buyer.Quota)
			var renewOrders int64
			require.NoError(t, db.Model(&SubscriptionOrder{}).Where("user_id = ? AND order_type = ?", buyer.Id, SubscriptionOrderTypeRenew).Count(&renewOrders).Error)
			assert.EqualValues(t, 1, renewOrders)

			// A renewal the balance cannot cover turns auto-renewal off and
			// leaves the subscription to expire.
			require.NoError(t, db.Model(buyer).Update("quota", 0).Error)
			require.NoError(t, db.Model(&renewedSub).Update("end_time", dueEnd).Error)
			renewed, err = RenewDueSubscriptions(10, 2*time.Minute)
			require.NoError(t, err)
			assert.Zero(t, renewed)
			var stopped UserSubscription
			require.NoError(t, db.First(&stopped, bought.Id).Error)
			assert.False(t, stopped.AutoRenew)
			assert.Equal(t, dueEnd, stopped.EndTime)
		})
	}
}
