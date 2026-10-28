package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type legacyTopUpMigrationFixture struct {
	Id              int
	UserId          int
	Amount          int64
	Money           float64
	TradeNo         string `gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `gorm:"type:varchar(50)"`
	PaymentProvider string `gorm:"type:varchar(50);default:''"`
	CreateTime      int64
	Status          string
}

type legacySubscriptionOrderMigrationFixture struct {
	Id              int
	UserId          int
	PlanId          int
	Money           float64
	TradeNo         string `gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `gorm:"type:varchar(50)"`
	PaymentProvider string `gorm:"type:varchar(50);default:''"`
	Status          string
	CreateTime      int64
}

type legacyNowPaymentsPaymentMigrationFixture struct {
	ID                  int
	TopUpID             *int   `gorm:"uniqueIndex"`
	SubscriptionOrderID *int   `gorm:"uniqueIndex"`
	PaymentID           string `gorm:"type:varchar(64);uniqueIndex"`
	OrderID             string `gorm:"type:varchar(255);uniqueIndex"`
	OrderType           string `gorm:"type:varchar(24);index"`
	PayCurrency         string `gorm:"type:varchar(32);index"`
	PriceCurrency       string `gorm:"type:varchar(16)"`
	PriceAmount         string `gorm:"type:varchar(64)"`
	PayAmount           string `gorm:"type:varchar(64)"`
	ActuallyPaidAmount  string `gorm:"type:varchar(64)"`
	CreditedQuota       int
	PayAddress          string `gorm:"type:varchar(255)"`
	GatewayStatus       string `gorm:"type:varchar(32);index"`
	Status              string `gorm:"type:varchar(16);index"`
	CreateTime          int64  `gorm:"index"`
}

func exerciseNowPaymentsPaymentMigration(t *testing.T, db *gorm.DB) {
	t.Helper()
	suffix := time.Now().UnixNano()
	legacyTable := fmt.Sprintf("np_top_%d", suffix)
	subscriptionTable := fmt.Sprintf("np_sub_%d", suffix)
	paymentTable := fmt.Sprintf("np_pay_%d", suffix)
	paymentUpgradeTable := fmt.Sprintf("np_pay_upgrade_%d", suffix)
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(paymentUpgradeTable)
		_ = db.Migrator().DropTable(paymentTable)
		_ = db.Migrator().DropTable(subscriptionTable)
		_ = db.Migrator().DropTable(legacyTable)
	})

	// Existing Epay orders predate the gateway identifier columns. Upgrade the
	// representative tables twice to verify data preservation and idempotency.
	require.NoError(t, db.Table(legacyTable).AutoMigrate(&legacyTopUpMigrationFixture{}))
	legacyTopUp := legacyTopUpMigrationFixture{
		UserId:     7,
		Amount:     25,
		Money:      25,
		TradeNo:    fmt.Sprintf("legacy-%d", suffix),
		CreateTime: 1_700_000_000,
		Status:     common.TopUpStatusPending,
	}
	require.NoError(t, db.Table(legacyTable).Create(&legacyTopUp).Error)
	require.NoError(t, db.Table(legacyTable).AutoMigrate(&TopUp{}))
	require.NoError(t, db.Table(legacyTable).AutoMigrate(&TopUp{}))
	require.True(t, db.Migrator().HasColumn(legacyTable, "epay_gateway_id"))
	require.True(t, db.Migrator().HasColumn(legacyTable, "pay_url"))
	require.True(t, db.Migrator().HasColumn(legacyTable, "pay_url_expires_at"))
	var migratedTopUp TopUp
	require.NoError(t, db.Table(legacyTable).First(&migratedTopUp, legacyTopUp.Id).Error)
	assert.Equal(t, legacyTopUp.TradeNo, migratedTopUp.TradeNo)
	assert.Empty(t, migratedTopUp.EpayGatewayID)

	require.NoError(t, db.Table(subscriptionTable).AutoMigrate(&legacySubscriptionOrderMigrationFixture{}))
	legacySubscription := legacySubscriptionOrderMigrationFixture{
		UserId: 7, PlanId: 9, Money: 12, TradeNo: fmt.Sprintf("legacy-sub-%d", suffix),
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay,
		Status: common.TopUpStatusPending, CreateTime: 1_700_000_000,
	}
	require.NoError(t, db.Table(subscriptionTable).Create(&legacySubscription).Error)
	require.NoError(t, db.Table(subscriptionTable).AutoMigrate(&SubscriptionOrder{}))
	require.NoError(t, db.Table(subscriptionTable).AutoMigrate(&SubscriptionOrder{}))
	require.True(t, db.Migrator().HasColumn(subscriptionTable, "epay_gateway_id"))
	var migratedSubscription SubscriptionOrder
	require.NoError(t, db.Table(subscriptionTable).First(&migratedSubscription, legacySubscription.Id).Error)
	assert.Equal(t, legacySubscription.TradeNo, migratedSubscription.TradeNo)
	assert.Empty(t, migratedSubscription.EpayGatewayID)

	require.False(t, db.Migrator().HasTable(paymentTable))

	// A fresh NOWPayments table must also remain stable across a second startup.
	require.NoError(t, db.Table(paymentTable).AutoMigrate(&NowPaymentsPayment{}))
	require.NoError(t, db.Table(paymentTable).AutoMigrate(&NowPaymentsPayment{}))
	payment := NowPaymentsPayment{
		TopUpID:            &legacyTopUp.Id,
		PaymentID:          fmt.Sprintf("payment-%d", suffix),
		OrderID:            legacyTopUp.TradeNo,
		OrderType:          NowPaymentsOrderTypeTopUp,
		PayCurrency:        "usdtbsc",
		PriceCurrency:      "usd",
		PriceAmount:        "25.00000000",
		PayAmount:          "24.98765432",
		ActuallyPaidAmount: "0",
		CreditedQuota:      12_500_000,
		SettledQuota:       0,
		PayAddress:         "0xmigration-test",
		PayinExtraID:       "246810",
		GatewayStatus:      "waiting",
		Status:             NowPaymentsPaymentStatusPending,
		IPNPayload:         `{"payment_status":"waiting"}`,
		ExpiresAt:          1_700_003_600,
		CreateTime:         1_700_000_000,
	}
	require.NoError(t, db.Table(paymentTable).Create(&payment).Error)

	var persistedPayment NowPaymentsPayment
	require.NoError(t, db.Table(paymentTable).First(&persistedPayment, payment.ID).Error)
	assert.Equal(t, payment, persistedPayment)

	duplicatePaymentID := payment
	duplicatePaymentID.ID = 0
	duplicatePaymentID.TopUpID = nil
	duplicatePaymentID.OrderID = fmt.Sprintf("other-order-%d", suffix)
	require.Error(t, db.Table(paymentTable).Create(&duplicatePaymentID).Error)

	duplicateOrderID := payment
	duplicateOrderID.ID = 0
	duplicateOrderID.TopUpID = nil
	duplicateOrderID.PaymentID = fmt.Sprintf("other-payment-%d", suffix)
	require.Error(t, db.Table(paymentTable).Create(&duplicateOrderID).Error)

	for index := 0; index < 2; index++ {
		unlinked := payment
		unlinked.ID = 0
		unlinked.TopUpID = nil
		unlinked.PaymentID = fmt.Sprintf("unlinked-payment-%d-%d", suffix, index)
		unlinked.OrderID = fmt.Sprintf("unlinked-order-%d-%d", suffix, index)
		require.NoError(t, db.Table(paymentTable).Create(&unlinked).Error)
	}

	// Upgrade a payment table created before incremental partial settlements.
	require.NoError(t, db.Table(paymentUpgradeTable).AutoMigrate(&legacyNowPaymentsPaymentMigrationFixture{}))
	legacyPayment := legacyNowPaymentsPaymentMigrationFixture{
		TopUpID: &legacyTopUp.Id, PaymentID: fmt.Sprintf("legacy-payment-%d", suffix),
		OrderID: legacyTopUp.TradeNo, OrderType: NowPaymentsOrderTypeTopUp,
		PayCurrency: "btc", PriceCurrency: "usd", PriceAmount: "25",
		PayAmount: "0.001", ActuallyPaidAmount: "0.001", CreditedQuota: 12_500_000,
		PayAddress: "bc1legacy", GatewayStatus: "finished",
		Status: NowPaymentsPaymentStatusSuccess, CreateTime: 1_700_000_000,
	}
	require.NoError(t, db.Table(paymentUpgradeTable).Create(&legacyPayment).Error)
	require.NoError(t, db.Table(paymentUpgradeTable).AutoMigrate(&NowPaymentsPayment{}))
	require.NoError(t, db.Table(paymentUpgradeTable).AutoMigrate(&NowPaymentsPayment{}))
	require.True(t, db.Migrator().HasColumn(paymentUpgradeTable, "settled_quota"))
	require.True(t, db.Migrator().HasColumn(paymentUpgradeTable, "parent_payment_id"))
	var migratedPayment NowPaymentsPayment
	require.NoError(t, db.Table(paymentUpgradeTable).First(&migratedPayment, legacyPayment.ID).Error)
	assert.Equal(t, legacyPayment.PaymentID, migratedPayment.PaymentID)
	assert.Zero(t, migratedPayment.SettledQuota)
}

func TestNowPaymentsPaymentMigrationSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	exerciseNowPaymentsPaymentMigration(t, db)
}

func TestNowPaymentsPaymentMigrationConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		dialector func(string) gorm.Dialector
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", dialector: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", env: "TEST_POSTGRES_DSN", dialector: func(dsn string) gorm.Dialector {
			return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.env))
			if dsn == "" {
				t.Skip(test.env + " is not configured")
			}
			db, err := gorm.Open(test.dialector(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			exerciseNowPaymentsPaymentMigration(t, db)
		})
	}
}
