package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUpdateOptionRejectsDuplicateNowPaymentsCurrencies(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"NowPaymentsPayCurrencies","value":"btc, BTC"}`),
	)

	UpdateOption(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.False(t, payload.Success)
	assert.Equal(t, "NOWPayments currency codes must not be duplicated", payload.Message)
}

func TestUpdateOptionRejectsInvalidNowPaymentsExpiration(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"NowPaymentsPaymentExpirationMins","value":2}`),
	)

	UpdateOption(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Equal(t, "NOWPayments payment expiration must be between 5 and 1440 minutes", payload.Message)
}

// DSNs must point to disposable databases reserved for this test.
func TestNowPaymentsGlobalMethodDatabaseMatrix(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMethods, originalGateways := operation_setting.PayMethods, operation_setting.EpayGateways
	originalEnabled, originalAPIKey, originalSecret := setting.NowPaymentsEnabled, setting.NowPaymentsAPIKey, setting.NowPaymentsIPNSecret
	originalCurrencies, originalMinimum, originalRate := setting.NowPaymentsPayCurrencies, setting.NowPaymentsMinTopUp, setting.NowPaymentsUSDToCurrencyRate
	originalURL, originalServer, originalCallback := setting.NowPaymentsAPIBaseURL, system_setting.ServerAddress, operation_setting.CustomCallbackAddress
	originalDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalDiscounts := operation_setting.GetPaymentSetting().AmountDiscount
	originalGroupRatios := common.TopupGroupRatio2JSONString()
	originalRedis := common.RedisEnabled
	t.Cleanup(func() {
		operation_setting.PayMethods, operation_setting.EpayGateways = originalMethods, originalGateways
		setting.NowPaymentsEnabled, setting.NowPaymentsAPIKey, setting.NowPaymentsIPNSecret = originalEnabled, originalAPIKey, originalSecret
		setting.NowPaymentsPayCurrencies, setting.NowPaymentsMinTopUp, setting.NowPaymentsUSDToCurrencyRate = originalCurrencies, originalMinimum, originalRate
		setting.NowPaymentsAPIBaseURL, system_setting.ServerAddress, operation_setting.CustomCallbackAddress = originalURL, originalServer, originalCallback
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplay
		operation_setting.GetPaymentSetting().AmountDiscount = originalDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalGroupRatios))
		common.RedisEnabled = originalRedis
	})
	common.RedisEnabled = false
	setting.NowPaymentsEnabled, setting.NowPaymentsAPIKey, setting.NowPaymentsIPNSecret = true, "test-key", "test-secret"
	setting.NowPaymentsPayCurrencies, setting.NowPaymentsMinTopUp, setting.NowPaymentsUSDToCurrencyRate = "btc", 1, 5
	system_setting.ServerAddress, operation_setting.CustomCallbackAddress = "https://new-api.example", ""
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{50: 0.8}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))
	operation_setting.PayMethods = []map[string]string{{"type": "nowpayments", "name": "Crypto checkout", "icon": "LuWallet", "fee": "0.50", "fee_rate": "3", "gateway": "ignored"}}
	operation_setting.EpayGateways = []operation_setting.EpayGateway{{ID: "primary", Enabled: true, Address: "https://pay.example", MerchantID: "test", Key: "test", PayMethods: []map[string]string{{"type": "nowpayments", "name": "Wrong gateway name"}, {"type": "alipay"}}}}

	for _, enabled := range []bool{true, false} {
		setting.NowPaymentsEnabled = enabled
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		GetTopUpInfo(ctx)
		var payload struct {
			Data struct {
				PayMethods []map[string]string `json:"pay_methods"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
		var methods []map[string]string
		for _, method := range payload.Data.PayMethods {
			if method["type"] == "nowpayments" {
				methods = append(methods, method)
			}
		}
		if enabled {
			require.Len(t, methods, 1)
			assert.Equal(t, "Crypto checkout", methods[0]["name"])
			assert.Equal(t, "0.50", methods[0]["fee"])
			assert.Equal(t, "3", methods[0]["fee_rate"])
			assert.Empty(t, methods[0]["gateway"])
		} else {
			assert.Empty(t, methods)
		}
	}
	setting.NowPaymentsEnabled = true
	quote, err := service.QuoteNowPaymentsPayMoney(50, "default")
	require.NoError(t, err)
	assert.Equal(t, 8.74, quote) // 50 / 5 * 0.8 * 1.03 + 0.50
	subscriptionPrice, err := service.GetNowPaymentsSubscriptionPriceUSD(&model.SubscriptionPlan{PriceAmount: 50, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "10.8", subscriptionPrice.String())
	configuredMethods := operation_setting.PayMethods
	operation_setting.PayMethods = nil
	quoteWithoutFees, err := service.QuoteNowPaymentsPayMoney(50, "default")
	require.NoError(t, err)
	assert.Equal(t, 8.0, quoteWithoutFees)
	operation_setting.PayMethods = configuredMethods
	for _, invalid := range []string{"-1", "NaN", "+Inf", "1e999", "bad"} {
		for _, field := range []string{"fee", "fee_rate"} {
			previous := operation_setting.PayMethods[0][field]
			operation_setting.PayMethods[0][field] = invalid
			_, err = service.QuoteNowPaymentsPayMoney(50, "default")
			require.Error(t, err)
			operation_setting.PayMethods[0][field] = previous
		}
	}

	for _, dialect := range []struct {
		name common.DatabaseType
		env  string
	}{
		{common.DatabaseTypeSQLite, ""}, {common.DatabaseTypeMySQL, "TEST_NOWPAYMENTS_MYSQL_DSN"}, {common.DatabaseTypePostgreSQL, "TEST_NOWPAYMENTS_POSTGRES_DSN"},
	} {
		t.Run(string(dialect.name), func(t *testing.T) {
			var driver gorm.Dialector = sqlite.Open(":memory:")
			if dialect.env != "" {
				dsn := os.Getenv(dialect.env)
				if dsn == "" {
					t.Skip(dialect.env + " is not configured")
				}
				if dialect.name == common.DatabaseTypeMySQL {
					driver = mysql.Open(dsn)
				} else {
					driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
				}
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			oldDB, oldLogDB := model.DB, model.LOG_DB
			oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
			model.DB, model.LOG_DB = db, db
			common.SetDatabaseTypes(dialect.name, dialect.name)
			t.Cleanup(func() { model.DB, model.LOG_DB = oldDB, oldLogDB; common.SetDatabaseTypes(oldMainType, oldLogType) })
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.NowPaymentsPayment{}, &model.Redemption{}, &model.Log{}))
			versionSQL := "select version()"
			if dialect.name == common.DatabaseTypeSQLite {
				versionSQL = "select sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("database: %s", version)
			user := &model.User{Username: "nowpayments-global-fees", Group: "default", Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(user).Error)
			t.Cleanup(func() { require.NoError(t, db.Unscoped().Delete(user).Error) })
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					PriceAmount json.Number `json:"price_amount"`
					OrderID     string      `json:"order_id"`
				}
				require.NoError(t, common.DecodeJson(r.Body, &body))
				assert.Equal(t, "8.74000000", body.PriceAmount.String())
				response, err := common.Marshal(map[string]any{"payment_id": body.OrderID, "payment_status": "waiting", "pay_address": "test-address", "price_amount": 8.74, "price_currency": "usd", "pay_amount": 0.001, "pay_currency": "btc", "order_id": body.OrderID})
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write(response)
				require.NoError(t, err)
			}))
			defer apiServer.Close()
			setting.NowPaymentsAPIBaseURL = apiServer.URL
			invoice, err := service.CreateNowPaymentsTopUp(context.Background(), user.Id, 50, "btc")
			require.NoError(t, err)
			assert.Equal(t, "8.74", invoice.PriceAmount)
			var payment model.NowPaymentsPayment
			require.NoError(t, db.Where("payment_id = ?", invoice.PaymentID).First(&payment).Error)
			assert.Equal(t, 50*int(common.QuotaPerUnit), payment.CreditedQuota)
			t.Cleanup(func() {
				require.NoError(t, db.Delete(&payment).Error)
				require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&model.TopUp{}).Error)
				require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&model.Log{}).Error)
			})
			payload := []byte(fmt.Sprintf(`{"payment_id":%q,"payment_status":"finished","pay_address":"test-address","price_amount":8.74,"price_currency":"usd","pay_amount":0.001,"actually_paid":0.001,"pay_currency":"btc","order_id":%q}`, invoice.PaymentID, invoice.OrderID))
			require.NoError(t, service.HandleNowPaymentsWebhook(payload, "127.0.0.1"))
			require.NoError(t, service.HandleNowPaymentsWebhook(payload, "127.0.0.1"))
			require.NoError(t, db.First(user, user.Id).Error)
			assert.Equal(t, 50*int(common.QuotaPerUnit), user.Quota)
			var topup model.TopUp
			require.NoError(t, db.Where("trade_no = ?", invoice.OrderID).First(&topup).Error)
			assert.Equal(t, 8.74, topup.Money)
			assert.Equal(t, common.TopUpStatusSuccess, topup.Status)
			redemptionInvoice, err := service.CreateNowPaymentsRedemptionInvoice(context.Background(), user.Id, 25, 2, 25*int(common.QuotaPerUnit), quote, "btc")
			require.NoError(t, err)
			assert.Equal(t, "8.74", redemptionInvoice.PriceAmount)
			t.Cleanup(func() {
				require.NoError(t, db.Where("payment_id = ?", redemptionInvoice.PaymentID).Delete(&model.NowPaymentsPayment{}).Error)
			})
		})
	}
}
