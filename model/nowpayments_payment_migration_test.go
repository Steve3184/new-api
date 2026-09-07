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

func exerciseNowPaymentsPaymentMigration(t *testing.T, db *gorm.DB) {
	t.Helper()
	suffix := time.Now().UnixNano()
	legacyTable := fmt.Sprintf("np_top_%d", suffix)
	paymentTable := fmt.Sprintf("np_pay_%d", suffix)
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(paymentTable)
		_ = db.Migrator().DropTable(legacyTable)
	})

	// A pre-existing top-up row represents the relevant portion of a database
	// created before NOWPayments support was added.
	require.NoError(t, db.Table(legacyTable).AutoMigrate(&TopUp{}))
	legacyTopUp := TopUp{
		UserId:     7,
		Amount:     25,
		Money:      25,
		TradeNo:    fmt.Sprintf("legacy-%d", suffix),
		CreateTime: 1_700_000_000,
		Status:     common.TopUpStatusPending,
	}
	require.NoError(t, db.Table(legacyTable).Create(&legacyTopUp).Error)
	require.False(t, db.Migrator().HasTable(paymentTable))

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
		PayAddress:         "0xmigration-test",
		PayinExtraID:       "246810",
		GatewayStatus:      "waiting",
		Status:             NowPaymentsPaymentStatusPending,
		IPNPayload:         `{"payment_status":"waiting"}`,
		ExpiresAt:          1_700_003_600,
		CreateTime:         1_700_000_000,
	}
	require.NoError(t, db.Table(paymentTable).Create(&payment).Error)

	// Re-running startup migration must be idempotent and retain both the old
	// application row and the newly created payment row.
	require.NoError(t, db.Table(paymentTable).AutoMigrate(&NowPaymentsPayment{}))
	var persistedTopUp TopUp
	require.NoError(t, db.Table(legacyTable).First(&persistedTopUp, legacyTopUp.Id).Error)
	assert.Equal(t, legacyTopUp.TradeNo, persistedTopUp.TradeNo)
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
