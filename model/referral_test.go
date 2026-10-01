package model

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// The top-up schema shipped before referral rewards, including fork fields.
type preReferralTopUp struct {
	Id               int
	UserId           int `gorm:"index"`
	Amount           int64
	Money            float64
	TradeNo          string `gorm:"unique;type:varchar(255);index"`
	PaymentMethod    string `gorm:"type:varchar(50)"`
	PaymentProvider  string `gorm:"type:varchar(50);default:''"`
	EpayGatewayID    string `gorm:"type:varchar(100);index"`
	CreateTime       int64
	CompleteTime     int64
	Status           string
	OrderType        string `gorm:"type:varchar(32);index"`
	RedemptionQuota  int
	RedemptionCount  int
	RedemptionAmount int64
	RedemptionName   string `gorm:"type:varchar(20)"`
}

func TestReferralRewards(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var dialector gorm.Dialector
			switch engine {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			db, err := gorm.Open(dialector, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: fmt.Sprintf("ref_%d_", time.Now().UnixNano())}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			if engine == "sqlite" {
				sqlDB.SetMaxOpenConns(1)
			}
			oldDB, oldType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(engine))
			payment := operation_setting.GetPaymentSetting()
			oldPayment := *payment
			payment.ComplianceConfirmed = true
			payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
			settings := config.GlobalConfig.Get("referral_setting")
			originalSettings, err := config.ConfigToMap(settings)
			require.NoError(t, err)
			require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"enabled": "true", "percent": "5", "min_paid_cny": "20", "max_orders": "10", "delay_hours": "24", "currency_rates": `{"CNY":1,"USD":5}`}))
			t.Cleanup(func() {
				DB = oldDB
				common.SetMainDatabaseType(oldType)
				*payment = oldPayment
				require.NoError(t, config.UpdateConfigFromMap(settings, originalSettings))
				require.NoError(t, db.Migrator().DropTable(&ReferralReward{}, &TopUp{}, &User{}))
				require.NoError(t, sqlDB.Close())
			})
			// Upgrade a representative pre-referral order table, then migrate twice.
			stmt := &gorm.Statement{DB: db}
			require.NoError(t, stmt.Parse(&TopUp{}))
			require.NoError(t, db.Table(stmt.Schema.Table).AutoMigrate(&preReferralTopUp{}))
			require.NoError(t, db.Table(stmt.Schema.Table).Create(&preReferralTopUp{TradeNo: "legacy-paid", UserId: 999, Money: 100, Status: common.TopUpStatusSuccess, EpayGatewayID: "retained-gateway", OrderType: OrderTypeRedemption, RedemptionQuota: 12345, RedemptionCount: 2}).Error)
			for range 2 {
				require.NoError(t, db.AutoMigrate(&TopUp{}, &ReferralReward{}, &User{}))
			}
			var legacy TopUp
			require.NoError(t, db.Where("trade_no = ?", "legacy-paid").First(&legacy).Error)
			assert.Equal(t, 100.0, legacy.Money)
			assert.Empty(t, legacy.ReferralSnapshot)
			assert.Equal(t, "retained-gateway", legacy.EpayGatewayID)
			assert.Equal(t, 12345, legacy.RedemptionQuota)
			assert.Equal(t, 2, legacy.RedemptionCount)
			assert.True(t, db.Migrator().HasIndex(&ReferralReward{}, "TopUpID"))
			assert.True(t, db.Migrator().HasIndex(&TopUp{}, "TradeNo"))

			inviter := User{Username: "ref-inviter", AffCode: "inviter", Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&inviter).Error)
			invitee := User{Username: "ref-invitee", AffCode: "invitee", InviterId: inviter.Id, Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&invitee).Error)
			newOrder := func(trade string, money float64) *TopUp {
				order := &TopUp{UserId: invitee.Id, Amount: 100, Money: money, PaymentProvider: PaymentProviderEpay, PaymentMethod: "alipay", TradeNo: trade, Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp()}
				require.NoError(t, order.Insert())
				return order
			}
			for i, money := range []float64{19.99, 20} {
				order := newOrder(fmt.Sprintf("below-%d", i), money)
				_, err := RechargeEpay(order.TradeNo, "alipay", "")
				require.NoError(t, err)
			}
			var count int64
			require.NoError(t, db.Model(&ReferralReward{}).Count(&count).Error)
			assert.Zero(t, count)
			// Qualifying slots use the verified paid amount, not the checkout price.
			discounted := newOrder("discounted", 100)
			_, err = RechargeEpay(discounted.TradeNo, "alipay", "", PaidFiat{Amount: 20, Currency: "CNY"})
			require.NoError(t, err)
			manual := newOrder("manual", 100)
			require.NoError(t, ManualCompleteTopUp(manual.TradeNo, ""))
			require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"enabled": "false"}))
			disabled := newOrder("disabled", 100)
			_, err = RechargeEpay(disabled.TradeNo, "alipay", "")
			require.NoError(t, err)
			require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"enabled": "true"}))
			require.NoError(t, db.Model(&ReferralReward{}).Count(&count).Error)
			assert.Zero(t, count)
			first := newOrder("first-eligible", 100)
			// Changing configuration must not alter this already-created order.
			require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"percent": "10", "delay_hours": "48"}))
			_, err = RechargeEpay(first.TradeNo, "alipay", "")
			require.NoError(t, err)
			already, err := RechargeEpay(first.TradeNo, "alipay", "")
			require.NoError(t, err)
			assert.True(t, already)
			var reward ReferralReward
			require.NoError(t, db.Where("top_up_id = ?", first.Id).First(&reward).Error)
			assert.Equal(t, 5.0, reward.Percent)
			assert.Equal(t, int(common.QuotaPerUnit), reward.Quota)
			assert.Equal(t, int64(86400), reward.AvailableAt-reward.CreatedAt)
			require.NoError(t, CreditReferralReward(reward.ID, reward.AvailableAt-1))
			var actual User
			require.NoError(t, db.First(&actual, inviter.Id).Error)
			assert.Zero(t, actual.Quota)
			// A full inviter wallet must leave the reward pending, without partial credit.
			require.NoError(t, db.Model(&User{}).Where("id = ?", inviter.Id).Update("quota", common.MaxWalletQuota).Error)
			assert.ErrorIs(t, CreditReferralReward(reward.ID, reward.AvailableAt), ErrTopUpQuotaLimitExceeded)
			var unchanged ReferralReward
			require.NoError(t, db.First(&unchanged, reward.ID).Error)
			assert.Equal(t, "pending", unchanged.Status)
			require.NoError(t, db.Model(&User{}).Where("id = ?", inviter.Id).Update("quota", 0).Error)
			// Concurrent workers must credit exactly once on all three engines.
			errors := make(chan error, 2)
			var workers sync.WaitGroup
			for range 2 {
				workers.Go(func() { errors <- CreditReferralReward(reward.ID, reward.AvailableAt) })
			}
			workers.Wait()
			close(errors)
			for err := range errors {
				require.NoError(t, err)
			}
			require.NoError(t, db.First(&actual, inviter.Id).Error)
			assert.Equal(t, reward.Quota, actual.Quota)
			require.NoError(t, ReverseReferralReward(reward.ID, inviter.Id, "refunded"))
			require.NoError(t, ReverseReferralReward(reward.ID, inviter.Id, "duplicate"))
			require.NoError(t, db.First(&actual, inviter.Id).Error)
			assert.Zero(t, actual.Quota)
			for i := range 8 {
				order := newOrder(fmt.Sprintf("qualifying-%d", i), 21)
				_, err := RechargeEpay(order.TradeNo, "alipay", "")
				require.NoError(t, err)
			}
			// Two different paid orders race for the last remaining slot.
			orders := []*TopUp{newOrder("slot-ten", 30), newOrder("slot-eleven", 40)}
			errors = make(chan error, 2)
			for _, order := range orders {
				workers.Go(func() { _, err := RechargeEpay(order.TradeNo, "alipay", ""); errors <- err })
			}
			workers.Wait()
			close(errors)
			for err := range errors {
				require.NoError(t, err)
			}
			require.NoError(t, db.Model(&ReferralReward{}).Count(&count).Error)
			assert.Equal(t, int64(10), count)
			var pending ReferralReward
			require.NoError(t, db.Where("status = ?", "pending").First(&pending).Error)
			require.NoError(t, ReverseReferralReward(pending.ID, inviter.Id, "cancel"))
			require.NoError(t, CreditReferralReward(pending.ID, pending.AvailableAt))
			require.NoError(t, db.First(&actual, inviter.Id).Error)
			assert.Zero(t, actual.Quota)
			// A duplicate ledger cannot be inserted, even outside callback code.
			duplicate := reward
			duplicate.ID = 0
			assert.Error(t, db.Create(&duplicate).Error)
			// Fresh schema creation is independently repeatable too.
			require.NoError(t, db.Migrator().DropTable(&ReferralReward{}, &TopUp{}, &User{}))
			for range 2 {
				require.NoError(t, db.AutoMigrate(&ReferralReward{}, &TopUp{}, &User{}))
			}
		})
	}
}

func TestReferralEligibility(t *testing.T) {
	for _, tc := range []struct {
		code  string
		minor int64
		major float64
	}{{"usd", 2500, 25}, {"CNY", 2001, 20.01}, {"JPY", 2500, 2500}, {"KWD", 1234, 1.234}, {"ZZZ", 100, 0}, {"USD", -1, 0}} {
		assert.Equal(t, tc.major, PaidFiatFromMinorUnits(tc.minor, tc.code).Amount, tc.code)
	}
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ReferralReward{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM referral_rewards") })
	payment := operation_setting.GetPaymentSetting()
	original := *payment
	payment.ComplianceConfirmed = true
	payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() { *payment = original })
	for _, tc := range []struct {
		provider, method, currency string
		redemption                 bool
		eligible                   bool
	}{
		{PaymentProviderEpay, "alipay", "CNY", false, true},
		{PaymentProviderEpay, "usdt", "CNY", false, false},
		{PaymentProviderNowPayments, "usd", "USD", false, false},
		{PaymentProviderMonero, "monero", "USD", false, false},
		{PaymentProviderBalance, "balance", "USD", false, false},
		{PaymentProviderStripe, "stripe", "USD", true, false},
		{PaymentProviderCreem, "creem", "ZZZ", false, false},
	} {
		order := TopUp{PaymentProvider: tc.provider, PaymentMethod: tc.method, Money: 100}
		if tc.redemption {
			order.OrderType = OrderTypeRedemption
		}
		require.NoError(t, order.SnapshotReferral(tc.currency))
		assert.Equal(t, tc.eligible, order.ReferralSnapshot != "", tc.provider+tc.method)
	}
	order := TopUp{PaymentProvider: PaymentProviderStripe, Money: 100}
	require.NoError(t, order.SnapshotReferral("USD"))
	require.NoError(t, applyReferralPaid(&order, []PaidFiat{{Amount: 4, Currency: "USD"}}))
	var snapshot referralSnapshot
	require.NoError(t, common.UnmarshalJsonStr(order.ReferralSnapshot, &snapshot))
	assert.Equal(t, 4.0, snapshot.Paid)
	require.NoError(t, applyReferralPaid(&order, []PaidFiat{{Amount: 10, Currency: "EUR"}}))
	assert.Empty(t, order.ReferralSnapshot)
	for _, tc := range []struct{ key, value string }{{"percent", "NaN"}, {"percent", "101"}, {"delay_hours", "0"}, {"max_orders", "1.5"}, {"min_paid_cny", "-1"}, {"currency_rates", `{"CNY":2,"USD":7}`}} {
		assert.Error(t, operation_setting.ValidateReferralOption(tc.key, tc.value))
	}
}
