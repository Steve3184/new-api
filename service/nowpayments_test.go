package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNowPaymentsServiceTest(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousQuotaPerUnit := common.QuotaPerUnit
	previousPaymentSettings := *operation_setting.GetPaymentSetting()
	previousDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	previousServerAddress := system_setting.ServerAddress
	previousCallbackAddress := operation_setting.CustomCallbackAddress
	previousEnabled := setting.NowPaymentsEnabled
	previousAPIKey := setting.NowPaymentsAPIKey
	previousSecret := setting.NowPaymentsIPNSecret
	previousBaseURL := setting.NowPaymentsAPIBaseURL
	previousCurrencies := setting.NowPaymentsPayCurrencies
	previousMinTopUp := setting.NowPaymentsMinTopUp
	previousRate := setting.NowPaymentsUSDToCurrencyRate
	previousExpiration := setting.NowPaymentsPaymentExpirationMins

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.QuotaPerUnit = 500000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	*operation_setting.GetPaymentSetting() = operation_setting.PaymentSetting{
		ComplianceConfirmed:    true,
		ComplianceTermsVersion: operation_setting.CurrentComplianceTermsVersion,
		AmountDiscount:         map[int]float64{},
	}
	system_setting.ServerAddress = "https://new-api.example"
	operation_setting.CustomCallbackAddress = ""
	setting.NowPaymentsEnabled = true
	setting.NowPaymentsAPIKey = "test-api-key"
	setting.NowPaymentsIPNSecret = "test-ipn-secret"
	setting.NowPaymentsPayCurrencies = "btc,usdtbsc"
	setting.NowPaymentsMinTopUp = 1
	setting.NowPaymentsUSDToCurrencyRate = 1
	setting.NowPaymentsPaymentExpirationMins = 60

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.Redemption{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.NowPaymentsPayment{},
		&model.Log{},
	))
	require.NoError(t, db.Create(&model.User{Id: 992, Username: "nowpayments-user", Group: "default", Status: common.UserStatusEnabled}).Error)

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		common.QuotaPerUnit = previousQuotaPerUnit
		*operation_setting.GetPaymentSetting() = previousPaymentSettings
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousDisplayType
		system_setting.ServerAddress = previousServerAddress
		operation_setting.CustomCallbackAddress = previousCallbackAddress
		setting.NowPaymentsEnabled = previousEnabled
		setting.NowPaymentsAPIKey = previousAPIKey
		setting.NowPaymentsIPNSecret = previousSecret
		setting.NowPaymentsAPIBaseURL = previousBaseURL
		setting.NowPaymentsPayCurrencies = previousCurrencies
		setting.NowPaymentsMinTopUp = previousMinTopUp
		setting.NowPaymentsUSDToCurrencyRate = previousRate
		setting.NowPaymentsPaymentExpirationMins = previousExpiration
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestNowPaymentsCanonicalJSONAndSignature(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	payload := []byte(`{"z":2,"nested":{"z":1,"a":[{"b":2,"a":1},true]},"a":"value"}`)
	canonical, err := canonicalNowPaymentsJSON(payload)
	require.NoError(t, err)
	assert.Equal(t, `{"a":"value","nested":{"a":[{"a":1,"b":2},true],"z":1},"z":2}`, string(canonical))

	mac := hmac.New(sha512.New, []byte(setting.NowPaymentsIPNSecret))
	_, err = mac.Write(canonical)
	require.NoError(t, err)
	signature := hex.EncodeToString(mac.Sum(nil))
	assert.True(t, VerifyNowPaymentsSignature(payload, signature))
	assert.False(t, VerifyNowPaymentsSignature(payload, signature[:len(signature)-2]+"00"))
}

func TestNowPaymentsCanonicalJSONMatchesJSONStringifySemantics(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	payload := []byte(`{"payment_id":18446744073709551615,"whole":10.0,"negative_zero":-0,"small_plain":1e-6,"small_exp":1e-7,"large_plain":1e20,"large_exp":1e21,"trailing":1.2300,"html":"<tag>&","literal":"\\u003c","separator":"\u2028"}`)
	canonical, err := canonicalNowPaymentsJSON(payload)
	require.NoError(t, err)
	expected := `{"html":"<tag>&","large_exp":1e+21,"large_plain":100000000000000000000,"literal":"\\u003c","negative_zero":0,"payment_id":18446744073709552000,"separator":"` + string(rune(0x2028)) + `","small_exp":1e-7,"small_plain":0.000001,"trailing":1.23,"whole":10}`
	assert.Equal(t, expected, string(canonical))
}

func TestNowPaymentsOptionalExtraIDAndProviderExpiration(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	setting.NowPaymentsPayCurrencies = "btc,usdtbsc,xrp"
	now := common.GetTimestamp()
	providerExpiry := time.Unix(now+600, 0).UTC().Format(time.RFC3339)

	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body nowPaymentsCreateRequest
		require.NoError(t, common.DecodeJson(request.Body, &body))
		response, err := common.Marshal(map[string]any{
			"payment_id":               "memo-payment-1",
			"payment_status":           "waiting",
			"pay_address":              "XRP-address",
			"payin_extra_id":           "987654",
			"price_amount":             10,
			"price_currency":           "usd",
			"pay_amount":               10,
			"pay_currency":             body.PayCurrency,
			"order_id":                 body.OrderID,
			"expiration_estimate_date": providerExpiry,
		})
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(response)
	}))
	defer apiServer.Close()
	setting.NowPaymentsAPIBaseURL = apiServer.URL

	invoice, err := CreateNowPaymentsTopUp(context.Background(), 992, 10, "xrp")
	require.NoError(t, err)
	assert.Equal(t, "987654", invoice.PayinExtraID)
	assert.LessOrEqual(t, invoice.ExpiresAt, now+601)

	payload := []byte(fmt.Sprintf(`{"payment_id":"memo-payment-1","payment_status":"confirming","pay_address":"XRP-address","payin_extra_id":"987654","price_amount":10,"price_currency":"usd","pay_amount":10,"actually_paid":0,"pay_currency":"xrp","order_id":%q}`, invoice.OrderID))
	require.NoError(t, HandleNowPaymentsWebhook(payload, "127.0.0.1"))

	mismatchedPayload := []byte(fmt.Sprintf(`{"payment_id":"memo-payment-1","payment_status":"confirming","pay_address":"XRP-address","payin_extra_id":"654321","price_amount":10,"price_currency":"usd","pay_amount":10,"actually_paid":0,"pay_currency":"xrp","order_id":%q}`, invoice.OrderID))
	assert.ErrorContains(t, HandleNowPaymentsWebhook(mismatchedPayload, "127.0.0.1"), "details do not match")
}

func TestNowPaymentsTopUpCreditsOnlyFinishedPaymentOnce(t *testing.T) {
	setupNowPaymentsServiceTest(t)

	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "test-api-key", request.Header.Get("x-api-key"))
		var body nowPaymentsCreateRequest
		require.NoError(t, common.DecodeJson(request.Body, &body))
		assert.Equal(t, "10.00000000", body.PriceAmount.String())
		assert.Equal(t, "usdtbsc", body.PayCurrency)
		assert.Equal(t, "https://new-api.example/api/nowpayments/webhook", body.IPNCallbackURL)
		response, err := common.Marshal(map[string]any{
			"payment_id":     12345,
			"payment_status": "waiting",
			"pay_address":    "0xtestaddress",
			"price_amount":   10,
			"price_currency": "usd",
			"pay_amount":     10,
			"pay_currency":   "usdtbsc",
			"order_id":       body.OrderID,
		})
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(response)
	}))
	defer apiServer.Close()
	setting.NowPaymentsAPIBaseURL = apiServer.URL

	invoice, err := CreateNowPaymentsTopUp(context.Background(), 992, 10, "usdtbsc")
	require.NoError(t, err)
	assert.Equal(t, "12345", invoice.PaymentID)
	assert.Equal(t, "10", invoice.PayAmount)

	waitingPayload := []byte(fmt.Sprintf(`{"payment_id":12345,"payment_status":"confirming","pay_address":"0xtestaddress","price_amount":10,"price_currency":"usd","pay_amount":10,"actually_paid":10,"pay_currency":"usdtbsc","order_id":%q}`, invoice.OrderID))
	require.NoError(t, HandleNowPaymentsWebhook(waitingPayload, "127.0.0.1"))
	var user model.User
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Zero(t, user.Quota)

	finishedPayload := []byte(fmt.Sprintf(`{"payment_id":12345,"payment_status":"finished","pay_address":"0xtestaddress","price_amount":10,"price_currency":"usd","pay_amount":10,"actually_paid":10,"pay_currency":"usdtbsc","order_id":%q}`, invoice.OrderID))
	require.NoError(t, HandleNowPaymentsWebhook(finishedPayload, "127.0.0.1"))
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 5_000_000, user.Quota)

	require.NoError(t, HandleNowPaymentsWebhook(finishedPayload, "127.0.0.1"))
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 5_000_000, user.Quota)
	var payment model.NowPaymentsPayment
	require.NoError(t, model.DB.Where("payment_id = ?", "12345").First(&payment).Error)
	assert.Equal(t, model.NowPaymentsPaymentStatusSuccess, payment.Status)
	assert.Equal(t, "finished", payment.GatewayStatus)
}

func TestNowPaymentsPartialPaymentCreditsReceivedAmountProportionally(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	topUp := &model.TopUp{
		UserId:          992,
		Amount:          10,
		Money:           10,
		TradeNo:         "nowpayments-underpaid",
		PaymentMethod:   model.PaymentMethodNowPayments,
		PaymentProvider: model.PaymentProviderNowPayments,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	payment := &model.NowPaymentsPayment{
		PaymentID:     "underpaid-id",
		OrderID:       topUp.TradeNo,
		PayCurrency:   "btc",
		PriceCurrency: "usd",
		PriceAmount:   "10",
		PayAmount:     "0.001",
		CreditedQuota: 5_000_000,
		PayAddress:    "bc1test",
		GatewayStatus: "waiting",
		Status:        model.NowPaymentsPaymentStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	require.NoError(t, model.CreateNowPaymentsTopUp(topUp, payment))

	payload := []byte(`{"payment_id":"underpaid-id","payment_status":"partially_paid","pay_address":"bc1test","price_amount":10,"price_currency":"usd","pay_amount":0.001,"actually_paid":0.0005,"pay_currency":"btc","order_id":"nowpayments-underpaid"}`)
	require.NoError(t, HandleNowPaymentsWebhook(payload, "127.0.0.1"))
	var user model.User
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 2_500_000, user.Quota)
	var settled model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", topUp.TradeNo).First(&settled).Error)
	assert.Equal(t, common.TopUpStatusSuccess, settled.Status)
	assert.Equal(t, 5.0, settled.Money)

	wrongAmountPayload := []byte(`{"payment_id":"underpaid-id","payment_status":"wrong_amount","pay_address":"bc1test","price_amount":10,"price_currency":"usd","pay_amount":0.001,"actually_paid":0.0006,"pay_currency":"btc","order_id":"nowpayments-underpaid"}`)
	require.NoError(t, HandleNowPaymentsWebhook(wrongAmountPayload, "127.0.0.1"))
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 3_000_000, user.Quota)

	finishedPayload := []byte(`{"payment_id":"underpaid-id","payment_status":"finished","pay_address":"bc1test","price_amount":10,"price_currency":"usd","pay_amount":0.001,"actually_paid":0.0008,"pay_currency":"btc","order_id":"nowpayments-underpaid"}`)
	require.NoError(t, HandleNowPaymentsWebhook(finishedPayload, "127.0.0.1"))
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 4_000_000, user.Quota)

	overpaidPayload := []byte(`{"payment_id":"underpaid-id","payment_status":"finished","pay_address":"bc1test","price_amount":10,"price_currency":"usd","pay_amount":0.001,"actually_paid":0.0012,"pay_currency":"btc","order_id":"nowpayments-underpaid"}`)
	require.NoError(t, HandleNowPaymentsWebhook(overpaidPayload, "127.0.0.1"))
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 6_000_000, user.Quota)
	require.NoError(t, HandleNowPaymentsWebhook(overpaidPayload, "127.0.0.1"))
	require.NoError(t, model.DB.First(&user, 992).Error)
	assert.Equal(t, 6_000_000, user.Quota)
}

func TestNowPaymentsRedemptionWaitsForFullPayment(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	topUp := &model.TopUp{
		UserId: 992, Amount: 2, Money: 2, TradeNo: "nowpayments-redemption",
		PaymentMethod: model.PaymentMethodNowPayments, PaymentProvider: model.PaymentProviderNowPayments,
		CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending,
		OrderType: model.OrderTypeRedemption, RedemptionQuota: 500_000,
		RedemptionCount: 2, RedemptionAmount: 1, RedemptionName: "Crypto code",
	}
	payment := &model.NowPaymentsPayment{
		PaymentID: "redemption-payment-id", OrderID: topUp.TradeNo,
		PayCurrency: "btc", PriceCurrency: "usd", PriceAmount: "2", PayAmount: "0.002",
		CreditedQuota: 1, PayAddress: "bc1redemption", GatewayStatus: "waiting",
		Status: model.NowPaymentsPaymentStatusPending, CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, model.CreateNowPaymentsTopUp(topUp, payment))

	partialPayload := []byte(`{"payment_id":"redemption-payment-id","payment_status":"partially_paid","pay_address":"bc1redemption","price_amount":2,"price_currency":"usd","pay_amount":0.002,"actually_paid":0.001,"pay_currency":"btc","order_id":"nowpayments-redemption"}`)
	require.NoError(t, HandleNowPaymentsWebhook(partialPayload, "127.0.0.1"))
	var redemptionCount int64
	require.NoError(t, model.DB.Model(&model.Redemption{}).Count(&redemptionCount).Error)
	assert.Zero(t, redemptionCount)
	var pendingPayment model.NowPaymentsPayment
	require.NoError(t, model.DB.Where("payment_id = ?", payment.PaymentID).First(&pendingPayment).Error)
	assert.Equal(t, model.NowPaymentsPaymentStatusPending, pendingPayment.Status)

	finishedPayload := []byte(`{"payment_id":"redemption-payment-id","payment_status":"finished","pay_address":"bc1redemption","price_amount":2,"price_currency":"usd","pay_amount":0.002,"actually_paid":0.002,"pay_currency":"btc","order_id":"nowpayments-redemption"}`)
	require.NoError(t, HandleNowPaymentsWebhook(finishedPayload, "127.0.0.1"))
	require.NoError(t, model.DB.Model(&model.Redemption{}).Count(&redemptionCount).Error)
	assert.Equal(t, int64(2), redemptionCount)
}

func TestQuoteNowPaymentsPayMoneyUsesConfiguredCurrencyRate(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	setting.NowPaymentsUSDToCurrencyRate = 5

	quote, err := QuoteNowPaymentsPayMoney(50, "default")
	require.NoError(t, err)
	assert.Equal(t, 10.0, quote)
}

func TestGetNowPaymentsSubscriptionPriceUSDUsesChannelRateForUSDPlans(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalRate := setting.NowPaymentsUSDToCurrencyRate
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		setting.NowPaymentsUSDToCurrencyRate = originalRate
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	setting.NowPaymentsUSDToCurrencyRate = 5
	price, err := GetNowPaymentsSubscriptionPriceUSD(&model.SubscriptionPlan{
		PriceAmount: 10,
		Currency:    "USD",
	})
	require.NoError(t, err)
	assert.True(t, decimal.NewFromInt(2).Equal(price))
}

func TestNowPaymentsSubscriptionCompletesAfterFinishedPayment(t *testing.T) {
	setupNowPaymentsServiceTest(t)
	plan := &model.SubscriptionPlan{
		Id:            707,
		Title:         "Crypto Plan",
		PriceAmount:   12.5,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}
	require.NoError(t, model.DB.Create(plan).Error)

	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body nowPaymentsCreateRequest
		require.NoError(t, common.DecodeJson(request.Body, &body))
		response, err := common.Marshal(map[string]any{
			"payment_id":     "sub-payment-1",
			"payment_status": "waiting",
			"pay_address":    "0xsubscription",
			"price_amount":   12.5,
			"price_currency": "usd",
			"pay_amount":     0.0002,
			"pay_currency":   body.PayCurrency,
			"order_id":       body.OrderID,
		})
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(response)
	}))
	defer apiServer.Close()
	setting.NowPaymentsAPIBaseURL = apiServer.URL

	invoice, err := CreateNowPaymentsSubscription(context.Background(), 992, plan.Id, "btc")
	require.NoError(t, err)
	require.Equal(t, "sub-payment-1", invoice.PaymentID)

	underpaidPayload := []byte(fmt.Sprintf(`{"payment_id":"sub-payment-1","payment_status":"finished","pay_address":"0xsubscription","price_amount":12.5,"price_currency":"usd","pay_amount":0.0002,"actually_paid":0.0001,"pay_currency":"btc","order_id":%q}`, invoice.OrderID))
	require.ErrorContains(t, HandleNowPaymentsWebhook(underpaidPayload, "127.0.0.1"), "below the quoted amount")
	pendingOrder := model.GetSubscriptionOrderByTradeNo(invoice.OrderID)
	require.NotNil(t, pendingOrder)
	assert.Equal(t, common.TopUpStatusPending, pendingOrder.Status)
	var subscriptionCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ? AND plan_id = ?", 992, plan.Id).Count(&subscriptionCount).Error)
	assert.Zero(t, subscriptionCount)

	payload := []byte(fmt.Sprintf(`{"payment_id":"sub-payment-1","payment_status":"finished","pay_address":"0xsubscription","price_amount":12.5,"price_currency":"usd","pay_amount":0.0002,"actually_paid":0.0002,"pay_currency":"btc","order_id":%q}`, invoice.OrderID))
	require.NoError(t, HandleNowPaymentsWebhook(payload, "127.0.0.1"))

	order := model.GetSubscriptionOrderByTradeNo(invoice.OrderID)
	require.NotNil(t, order)
	assert.Equal(t, common.TopUpStatusSuccess, order.Status)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ? AND plan_id = ?", 992, plan.Id).Count(&subscriptionCount).Error)
	assert.Equal(t, int64(1), subscriptionCount)
	var payment model.NowPaymentsPayment
	require.NoError(t, model.DB.Where("payment_id = ?", invoice.PaymentID).First(&payment).Error)
	assert.Equal(t, model.NowPaymentsPaymentStatusSuccess, payment.Status)
}
