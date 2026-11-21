package controller

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

func TestStripeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	t.Cleanup(func() {
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
	})

	setting.StripeWebhookSecret = ""
	setting.StripeApiSecret = "sk_test_123"
	setting.StripePriceId = "price_123"
	require.False(t, isStripeWebhookEnabled())

	setting.StripeWebhookSecret = "whsec_test"
	require.True(t, isStripeWebhookEnabled())

	setting.StripePriceId = ""
	require.False(t, isStripeWebhookEnabled())
}

func TestCreemWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPIKey := setting.CreemApiKey
	originalProducts := setting.CreemProducts
	originalWebhookSecret := setting.CreemWebhookSecret
	t.Cleanup(func() {
		setting.CreemApiKey = originalAPIKey
		setting.CreemProducts = originalProducts
		setting.CreemWebhookSecret = originalWebhookSecret
	})

	setting.CreemWebhookSecret = ""
	setting.CreemApiKey = "creem_api_key"
	setting.CreemProducts = `[{"productId":"prod_123"}]`
	require.False(t, isCreemWebhookEnabled())

	setting.CreemWebhookSecret = "creem_secret"
	require.True(t, isCreemWebhookEnabled())

	setting.CreemProducts = "[]"
	require.False(t, isCreemWebhookEnabled())
}

func TestWaffoWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.WaffoEnabled
	originalSandbox := setting.WaffoSandbox
	originalAPIKey := setting.WaffoApiKey
	originalPrivateKey := setting.WaffoPrivateKey
	originalPublicCert := setting.WaffoPublicCert
	originalSandboxAPIKey := setting.WaffoSandboxApiKey
	originalSandboxPrivateKey := setting.WaffoSandboxPrivateKey
	originalSandboxPublicCert := setting.WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.WaffoEnabled = originalEnabled
		setting.WaffoSandbox = originalSandbox
		setting.WaffoApiKey = originalAPIKey
		setting.WaffoPrivateKey = originalPrivateKey
		setting.WaffoPublicCert = originalPublicCert
		setting.WaffoSandboxApiKey = originalSandboxAPIKey
		setting.WaffoSandboxPrivateKey = originalSandboxPrivateKey
		setting.WaffoSandboxPublicCert = originalSandboxPublicCert
	})

	setting.WaffoEnabled = true
	setting.WaffoSandbox = false
	setting.WaffoApiKey = ""
	setting.WaffoPrivateKey = "private"
	setting.WaffoPublicCert = "public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoApiKey = "api"
	require.True(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = false
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = true
	setting.WaffoSandbox = true
	setting.WaffoSandboxApiKey = ""
	setting.WaffoSandboxPrivateKey = "sandbox_private"
	setting.WaffoSandboxPublicCert = "sandbox_public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoSandboxApiKey = "sandbox_api"
	require.True(t, isWaffoWebhookEnabled())
}

func TestWaffoPancakeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMerchantID := setting.WaffoPancakeMerchantID
	originalPrivateKey := setting.WaffoPancakePrivateKey
	originalProductID := setting.WaffoPancakeProductID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID = originalMerchantID
		setting.WaffoPancakePrivateKey = originalPrivateKey
		setting.WaffoPancakeProductID = originalProductID
	})

	// Presence of all three credentials enables the gateway. Webhook public
	// keys are bundled in the SDK and there is no separate Enabled toggle —
	// clear any of the three fields to disable.
	setting.WaffoPancakeMerchantID = ""
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeMerchantID = "merchant"
	require.True(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = ""
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = "product"
	setting.WaffoPancakePrivateKey = ""
	require.False(t, isWaffoPancakeWebhookEnabled())
}

func TestEpayWebhookRemainsEnabledForExistingOrders(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	paymentSetting := operation_setting.GetPaymentSetting()
	originalSignType := paymentSetting.EpaySignType
	originalMerchantPrivateKey := paymentSetting.EpayMerchantPrivateKey
	originalPlatformPublicKey := paymentSetting.EpayPlatformPublicKey
	originalPayMethods := operation_setting.PayMethods
	originalEpayGateways := operation_setting.EpayGateways
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		paymentSetting.EpaySignType = originalSignType
		paymentSetting.EpayMerchantPrivateKey = originalMerchantPrivateKey
		paymentSetting.EpayPlatformPublicKey = originalPlatformPublicKey
		operation_setting.PayMethods = originalPayMethods
		operation_setting.EpayGateways = originalEpayGateways
	})
	operation_setting.EpayGateways = nil

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeMD5
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled())

	operation_setting.EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = nil
	require.False(t, isEpayTopUpEnabled())
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	operation_setting.EpayGateways = []operation_setting.EpayGateway{{
		ID: "disabled", Address: "https://disabled.example.com", MerchantID: "id",
		Key: "key", Enabled: false, PayMethods: operation_setting.PayMethods,
	}}
	require.False(t, isEpayTopUpEnabled())
	require.True(t, isEpayWebhookEnabled())
	require.Nil(t, operation_setting.GetEpayGateway(""))
	require.NotNil(t, operation_setting.GetEpayGateway("disabled"))

	operation_setting.EpayGateways = nil
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeRSA
	paymentSetting.EpayMerchantPrivateKey = "merchant_private_key"
	paymentSetting.EpayPlatformPublicKey = ""
	require.False(t, isEpayWebhookEnabled())
	paymentSetting.EpayPlatformPublicKey = "platform_public_key"
	operation_setting.EpayKey = ""
	require.True(t, isEpayWebhookEnabled())
}

func TestLegacyEpaySettingsMigrateToFirstGateway(t *testing.T) {
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	originalGateways := operation_setting.EpayGateways
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		operation_setting.EpayGateways = originalGateways
	})

	operation_setting.PayAddress = "https://legacy-pay.example.com"
	operation_setting.EpayId = "legacy-id"
	operation_setting.EpayKey = "legacy-key"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay", "name": "Alipay"}}
	operation_setting.EpayGateways = nil

	require.True(t, operation_setting.MigrateLegacyEpayGateway())
	require.Len(t, operation_setting.EpayGateways, 1)
	assert.Equal(t, "default", operation_setting.EpayGateways[0].ID)
	assert.Equal(t, operation_setting.PayAddress, operation_setting.EpayGateways[0].Address)
	assert.Equal(t, operation_setting.EpayId, operation_setting.EpayGateways[0].MerchantID)
	assert.Equal(t, operation_setting.EpayKey, operation_setting.EpayGateways[0].Key)
	assert.Equal(t, operation_setting.PayMethods, operation_setting.EpayGateways[0].PayMethods)
	require.False(t, operation_setting.MigrateLegacyEpayGateway())
}

func TestEpayFeeUsesSelectedGateway(t *testing.T) {
	previousGateways := operation_setting.EpayGateways
	t.Cleanup(func() { operation_setting.EpayGateways = previousGateways })
	operation_setting.EpayGateways = []operation_setting.EpayGateway{
		{
			ID: "primary", Address: "https://primary.example.com", MerchantID: "primary-id",
			Key: "primary-key", Enabled: true,
			PayMethods: []map[string]string{{"type": "alipay", "fee": "0", "fee_rate": "0"}},
		},
		{
			ID: "backup", Address: "https://backup.example.com", MerchantID: "backup-id",
			Key: "backup-key", Enabled: true,
			PayMethods: []map[string]string{{"type": "alipay", "gateway": "primary", "fee": "0.5", "fee_rate": "10"}},
		},
	}

	assert.InDelta(t, 10, applyEpayFee(10, "alipay", "primary"), 0.000001)
	assert.InDelta(t, 11.5, applyEpayFee(10, "alipay", "backup"), 0.000001)
	operation_setting.EpayGateways[1].PayMethods[0]["fee"] = "0"
	operation_setting.EpayGateways[1].PayMethods[0]["fee_rate"] = "3"
	assert.Equal(t, 1.03, applyEpayFee(1, "alipay", "backup"))
	method := operation_setting.GetPayMethodForGateway("alipay", "backup")
	require.NotNil(t, method)
	assert.Equal(t, "backup", method["gateway"])
	assert.Nil(t, operation_setting.GetPayMethodForGateway("alipay", ""))
	gateway := epayGatewayForMethod("alipay", "backup")
	require.NotNil(t, gateway)
	assert.Equal(t, "backup", gateway.ID)
	assert.Nil(t, epayGatewayForMethod("alipay", ""))
	operation_setting.EpayGateways[1].PayMethods[0]["type"] = "wxpay"
	method = operation_setting.GetPayMethodForGateway("alipay", "")
	require.NotNil(t, method)
	assert.Equal(t, "primary", method["gateway"])
}

func TestGetTopUpInfoKeepsDuplicateEpayTypesDistinct(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	previousGateways := operation_setting.EpayGateways
	t.Cleanup(func() { operation_setting.EpayGateways = previousGateways })
	operation_setting.EpayGateways = []operation_setting.EpayGateway{
		{
			ID: "primary", Address: "https://primary.example.com", MerchantID: "primary-id",
			Key: "primary-key", Enabled: true,
			PayMethods: []map[string]string{{"type": "alipay"}},
		},
		{
			ID: "backup", Address: "https://backup.example.com", MerchantID: "backup-id",
			Key: "backup-key", Enabled: true,
			PayMethods: []map[string]string{{"type": "alipay", "name": "Backup Alipay"}},
		},
	}

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	GetTopUpInfo(context)

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			PayMethods []map[string]string `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	epayMethods := make([]map[string]string, 0, 2)
	for _, method := range payload.Data.PayMethods {
		if method["gateway"] != "" {
			epayMethods = append(epayMethods, method)
		}
	}
	require.Len(t, epayMethods, 2)
	assert.Equal(t, "primary", epayMethods[0]["gateway"])
	assert.Equal(t, "alipay", epayMethods[0]["type"])
	assert.Equal(t, "alipay", epayMethods[0]["name"])
	assert.Equal(t, "backup", epayMethods[1]["gateway"])
	assert.Equal(t, "alipay", epayMethods[1]["type"])
	assert.Equal(t, "Backup Alipay", epayMethods[1]["name"])
}

func TestEpayCallbackRejectsTamperingAndOrderMismatch(t *testing.T) {
	gateway := &operation_setting.EpayGateway{
		ID: "secure", Address: "https://secure-pay.example.com",
		MerchantID: "merchant-123", Key: "callback-secret", Enabled: true,
	}
	client, err := epay.NewClient(&epay.Config{
		PartnerID: gateway.MerchantID,
		Key:       gateway.Key,
	}, gateway.Address)
	require.NoError(t, err)

	validParams := epay.GenerateParams(map[string]string{
		"pid": gateway.MerchantID, "trade_no": "provider-1",
		"out_trade_no": "order-1", "type": "alipay", "name": "Top up",
		"money": "10.00", "trade_status": epay.StatusTradeSuccess,
	}, gateway.Key)
	verifyInfo, err := verifyEpayCallback(client, gateway, validParams)
	require.NoError(t, err)
	require.NoError(t, validateEpayCallbackOrder(gateway, verifyInfo, "order-1", "alipay", 10, "secure"))

	tamperedParams := make(map[string]string, len(validParams))
	maps.Copy(tamperedParams, validParams)
	tamperedParams["money"] = "0.01"
	_, err = verifyEpayCallback(client, gateway, tamperedParams)
	require.ErrorContains(t, err, "signature")

	underpaidParams := epay.GenerateParams(map[string]string{
		"pid": gateway.MerchantID, "trade_no": "provider-2",
		"out_trade_no": "order-1", "type": "alipay", "name": "Top up",
		"money": "0.01", "trade_status": epay.StatusTradeSuccess,
	}, gateway.Key)
	underpaidInfo, err := verifyEpayCallback(client, gateway, underpaidParams)
	require.NoError(t, err)
	require.ErrorContains(t, validateEpayCallbackOrder(gateway, underpaidInfo, "order-1", "alipay", 10, "secure"), "amount does not match")

	wrongMerchantParams := epay.GenerateParams(map[string]string{
		"pid": "merchant-other", "trade_no": "provider-3",
		"out_trade_no": "order-1", "type": "alipay", "name": "Top up",
		"money": "10.00", "trade_status": epay.StatusTradeSuccess,
	}, gateway.Key)
	_, err = verifyEpayCallback(client, gateway, wrongMerchantParams)
	require.ErrorContains(t, err, "merchant")

	unsupportedSignatureType := maps.Clone(validParams)
	unsupportedSignatureType["sign_type"] = "SHA256"
	_, err = verifyEpayCallback(client, gateway, unsupportedSignatureType)
	require.ErrorContains(t, err, "signature type")

	missingProviderTradeNo := epay.GenerateParams(map[string]string{
		"pid": gateway.MerchantID, "out_trade_no": "order-1",
		"type": "alipay", "name": "Top up", "money": "10.00",
		"trade_status": epay.StatusTradeSuccess,
	}, gateway.Key)
	missingTradeNoInfo, err := verifyEpayCallback(client, gateway, missingProviderTradeNo)
	require.NoError(t, err)
	require.ErrorContains(t, validateEpayCallbackOrder(gateway, missingTradeNoInfo, "order-1", "alipay", 10, "secure"), "provider trade number")

	require.ErrorContains(t, validateEpayCallbackOrder(gateway, verifyInfo, "order-2", "alipay", 10, "secure"), "order does not match")
	require.ErrorContains(t, validateEpayCallbackOrder(gateway, verifyInfo, "order-1", "wxpay", 10, "secure"), "payment method")
	require.ErrorContains(t, validateEpayCallbackOrder(gateway, verifyInfo, "order-1", "alipay", 10, "other"), "gateway")
}

func TestUpdateOptionRejectsMaskedKeyForUnknownEpayGateway(t *testing.T) {
	previousGateways := operation_setting.EpayGateways
	t.Cleanup(func() { operation_setting.EpayGateways = previousGateways })
	operation_setting.EpayGateways = []operation_setting.EpayGateway{{
		ID: "existing", Name: "Existing", Address: "https://pay.example.com",
		MerchantID: "merchant", Key: "secret", Enabled: true,
	}}
	gateways, err := common.Marshal([]operation_setting.EpayGateway{{
		ID: "new", Name: "New", Address: "https://new-pay.example.com",
		MerchantID: "merchant", Key: common.SensitiveOptionPlaceholder, Enabled: true,
	}})
	require.NoError(t, err)
	requestBody, err := common.Marshal(map[string]string{
		"key":   "EpayGateways",
		"value": string(gateways),
	})
	require.NoError(t, err)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		bytes.NewReader(requestBody),
	)

	UpdateOption(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Equal(t, "masked Epay gateway key does not match an existing gateway", payload.Message)
}

func TestNowPaymentsWebhookEnabledRequiresCompleteConfiguration(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.NowPaymentsEnabled
	originalAPIKey := setting.NowPaymentsAPIKey
	originalSecret := setting.NowPaymentsIPNSecret
	originalCurrencies := setting.NowPaymentsPayCurrencies
	t.Cleanup(func() {
		setting.NowPaymentsEnabled = originalEnabled
		setting.NowPaymentsAPIKey = originalAPIKey
		setting.NowPaymentsIPNSecret = originalSecret
		setting.NowPaymentsPayCurrencies = originalCurrencies
	})

	setting.NowPaymentsEnabled = true
	setting.NowPaymentsAPIKey = "api-key"
	setting.NowPaymentsIPNSecret = ""
	setting.NowPaymentsPayCurrencies = "btc"
	require.False(t, isNowPaymentsWebhookEnabled())

	setting.NowPaymentsIPNSecret = "ipn-secret"
	require.True(t, isNowPaymentsWebhookEnabled())

	setting.NowPaymentsPayCurrencies = ""
	require.False(t, isNowPaymentsWebhookEnabled())
}

// epayCallbackCase is one callback sent to every Epay callback endpoint.
type epayCallbackCase struct {
	name         string
	params       map[string]string
	validPending bool
}

// setupEpayCallbackTest persists pending orders so rejected callbacks cannot
// settle them, then restores the process-wide database pointers at cleanup.
func setupEpayCallbackTest(t *testing.T) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	paymentSetting := operation_setting.GetPaymentSetting()
	originalSignType := paymentSetting.EpaySignType
	originalMerchantPrivateKey := paymentSetting.EpayMerchantPrivateKey
	originalPlatformPublicKey := paymentSetting.EpayPlatformPublicKey
	originalPayMethods := operation_setting.PayMethods
	originalServerAddress := system_setting.ServerAddress
	originalDB, originalLogDB := model.DB, model.LOG_DB
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.SubscriptionOrder{}))
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		paymentSetting.EpaySignType = originalSignType
		paymentSetting.EpayMerchantPrivateKey = originalMerchantPrivateKey
		paymentSetting.EpayPlatformPublicKey = originalPlatformPublicKey
		operation_setting.PayMethods = originalPayMethods
		system_setting.ServerAddress = originalServerAddress
		model.DB, model.LOG_DB = originalDB, originalLogDB
	})
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "test-key"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	system_setting.ServerAddress = "https://dashboard.example.com"
	// Gateway selection reads the order before verification. Persist pending orders
	// and verify rejected or pending callbacks cannot settle them.
	for _, tradeNo := range []string{"order", "other&out_trade_no=order"} {
		require.NoError(t, db.Create(&model.TopUp{TradeNo: tradeNo, PaymentProvider: model.PaymentProviderEpay, PaymentMethod: "alipay", Money: 1, Status: common.TopUpStatusPending}).Error)
	}
	require.True(t, isEpayWebhookEnabled())
}

// assertEpayCallbacksVerifyBeforeSettlement sends each callback to the top-up
// and subscription endpoints by GET and POST. Only a valid pending callback is
// accepted; every other callback is rejected.
func assertEpayCallbacksVerifyBeforeSettlement(t *testing.T, cases []epayCallbackCase) {
	t.Helper()
	endpoints := []struct {
		name             string
		path             string
		handler          gin.HandlerFunc
		status           int
		rejectedBody     string
		pendingBody      string
		rejectedLocation string
		pendingLocation  string
	}{
		{
			name: "topup notify", path: "/api/user/epay/notify", handler: EpayNotify,
			status: http.StatusOK, rejectedBody: "fail", pendingBody: "success",
		},
		{
			name: "subscription notify", path: "/api/subscription/epay/notify", handler: SubscriptionEpayNotify,
			status: http.StatusOK, rejectedBody: "fail", pendingBody: "fail",
		},
		{
			name: "subscription return", path: "/api/subscription/epay/return", handler: SubscriptionEpayReturn,
			status:           http.StatusFound,
			rejectedLocation: "https://dashboard.example.com/wallet?pay=fail",
			pendingLocation:  "https://dashboard.example.com/wallet?pay=pending",
		},
	}
	router := gin.New()
	for _, endpoint := range endpoints {
		router.Any(endpoint.path, endpoint.handler)
	}
	for _, endpoint := range endpoints {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			for _, tc := range cases {
				t.Run(endpoint.name+"/"+method+"/"+tc.name, func(t *testing.T) {
					params := url.Values{}
					for key, value := range tc.params {
						params.Set(key, value)
					}
					request := httptest.NewRequest(method, endpoint.path+"?"+params.Encode(), nil)
					if method == http.MethodPost {
						request = httptest.NewRequest(method, endpoint.path, strings.NewReader(params.Encode()))
						request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					}
					recorder := httptest.NewRecorder()
					require.NotPanics(t, func() { router.ServeHTTP(recorder, request) })
					assert.Equal(t, endpoint.status, recorder.Code)
					wantBody, wantLocation := endpoint.rejectedBody, endpoint.rejectedLocation
					if tc.validPending {
						wantBody, wantLocation = endpoint.pendingBody, endpoint.pendingLocation
					}
					if wantBody != "" {
						assert.Equal(t, wantBody, recorder.Body.String())
					}
					assert.Equal(t, wantLocation, recorder.Header().Get("Location"))
					var orders []model.TopUp
					require.NoError(t, model.DB.Find(&orders).Error)
					require.Len(t, orders, 2)
					for _, order := range orders {
						assert.Equal(t, common.TopUpStatusPending, order.Status)
					}
				})
			}
		}
	}
}

// newEpayRSAKeyPairForTest returns a throwaway key pair as single-line Base64
// text, the format Epay merchant consoles show.
func newEpayRSAKeyPairForTest(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(privateDER), base64.StdEncoding.EncodeToString(publicDER)
}

func TestEpayCallbacksVerifyBeforeSettlement(t *testing.T) {
	setupEpayCallbackTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeMD5
	operation_setting.EpayKey = "test-key"
	require.True(t, isEpayWebhookEnabled())

	// Both ambiguous maps below had this same valid signing string in v0.0.4.
	// This must fail because of parameter ambiguity, not a mismatched signature.
	ambiguousSign := epay.MD5String("money=1.00&name=item&out_trade_no=other&out_trade_no=order&pid=epay_id&trade_status=TRADE_SUCCESS&type=alipay", "test-key")
	pendingSign := epay.MD5String("money=1.00&name=item%26other&out_trade_no=order&pid=epay_id&trade_status=WAIT_BUYER_PAY&type=alipay", "test-key")
	rows := []struct {
		name         string
		productName  string
		tradeNo      string
		status       string
		sign         string
		validPending bool
	}{
		{"separator in product name", "item&out_trade_no=other", "order", epay.StatusTradeSuccess, ambiguousSign, false},
		{"separator in order number", "item", "other&out_trade_no=order", epay.StatusTradeSuccess, ambiguousSign, false},
		{"invalid signature", "item", "order", epay.StatusTradeSuccess, "invalid", false},
		{"missing signature", "item", "order", epay.StatusTradeSuccess, "", false},
		{"valid pending with literal percent escape", "item%26other", "order", "WAIT_BUYER_PAY", pendingSign, true},
	}
	cases := make([]epayCallbackCase, 0, len(rows))
	for _, row := range rows {
		params := map[string]string{
			"pid": "epay_id", "type": "alipay", "money": "1.00",
			"name": row.productName, "out_trade_no": row.tradeNo,
			"trade_status": row.status, "sign_type": "MD5",
		}
		if row.sign != "" {
			params["sign"] = row.sign
		}
		cases = append(cases, epayCallbackCase{name: row.name, params: params, validPending: row.validPending})
	}
	assertEpayCallbacksVerifyBeforeSettlement(t, cases)
}

func TestEpayRSAModeSignsOrdersAndVerifiesCallbacks(t *testing.T) {
	setupEpayCallbackTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	merchantPrivateKey, merchantPublicKey := newEpayRSAKeyPairForTest(t)
	platformPrivateKey, platformPublicKey := newEpayRSAKeyPairForTest(t)
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeRSA
	// A leftover MD5 key must not be accepted once RSA is selected.
	operation_setting.EpayKey = "test-key"
	paymentSetting.EpayMerchantPrivateKey = merchantPrivateKey
	paymentSetting.EpayPlatformPublicKey = platformPublicKey
	require.True(t, isEpayWebhookEnabled())

	client := GetEpayClient()
	require.NotNil(t, client)
	notifyURL, err := url.Parse("https://dashboard.example.com/api/user/epay/notify")
	require.NoError(t, err)
	returnURL, err := url.Parse("https://dashboard.example.com/usage-logs")
	require.NoError(t, err)
	paymentURL, order, err := client.Purchase(&epay.PurchaseArgs{
		Type: "alipay", ServiceTradeNo: "order", Name: "TUC1", Money: "1.00",
		Device: epay.PC, NotifyUrl: notifyURL, ReturnUrl: returnURL,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://pay.example.com/api/pay/submit", paymentURL)
	assert.Equal(t, epay.SignTypeRSA, order["sign_type"])

	// The platform side holds the merchant public key and signs callbacks with its own private key.
	platform, err := epay.NewClient(&epay.Config{
		PartnerID: "epay_id", SignType: epay.SignTypeRSA,
		MerchantPrivateKey: platformPrivateKey, PlatformPublicKey: merchantPublicKey,
	}, "https://pay.example.com")
	require.NoError(t, err)
	verifiedOrder, err := platform.Verify(order)
	require.NoError(t, err)
	assert.True(t, verifiedOrder.VerifyStatus)

	pending, err := platform.SignParams(map[string]string{
		"type": "alipay", "money": "1.00", "name": "item", "out_trade_no": "order", "trade_status": "WAIT_BUYER_PAY",
	})
	require.NoError(t, err)
	paidFields := map[string]string{
		"type": "alipay", "money": "1.00", "name": "item", "out_trade_no": "order", "trade_status": epay.StatusTradeSuccess,
	}
	paid, err := platform.SignParams(paidFields)
	require.NoError(t, err)
	tampered := maps.Clone(paid)
	tampered["money"] = "100.00"
	merchantSigned, err := client.SignParams(paidFields)
	require.NoError(t, err)
	md5Client, err := epay.NewClient(&epay.Config{PartnerID: "epay_id", Key: "test-key"}, "https://pay.example.com")
	require.NoError(t, err)
	md5Signed, err := md5Client.SignParams(paidFields)
	require.NoError(t, err)
	unsigned := maps.Clone(paid)
	delete(unsigned, "sign")

	assertEpayCallbacksVerifyBeforeSettlement(t, []epayCallbackCase{
		{name: "valid pending signed by platform", params: pending, validPending: true},
		{name: "tampered amount", params: tampered},
		{name: "signed with merchant key", params: merchantSigned},
		{name: "MD5 signature in RSA mode", params: md5Signed},
		{name: "missing signature", params: unsigned},
	})
}

// TestEpayTopUpNotifyCompletesSubscriptionOrder covers a gateway that ignores
// the per-order notify_url and reports a paid subscription order to the
// top-up callback. A subscription order has no top-up row until completion.
func TestEpayTopUpNotifyCompletesSubscriptionOrder(t *testing.T) {
	setupEpayCallbackTest(t)
	operation_setting.GetPaymentSetting().EpaySignType = operation_setting.EpaySignTypeMD5
	operation_setting.EpayKey = "test-key"
	require.True(t, isEpayWebhookEnabled())

	const userID, tradeNo = 9001, "SUBUSR9001NOnotify"
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "epay_subscription_user", Status: common.UserStatusEnabled}).Error)
	plan := &model.SubscriptionPlan{Id: 9001, Title: "EpayPlan", PriceAmount: 1, Currency: "CNY", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 1000}
	require.NoError(t, model.DB.Create(plan).Error)
	require.NoError(t, (&model.SubscriptionOrder{
		UserId: userID, PlanId: plan.Id, Money: 1, TradeNo: tradeNo, PaymentMethod: "alipay",
		PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}).Insert())

	signer, err := epay.NewClient(&epay.Config{PartnerID: "epay_id", Key: "test-key"}, "https://pay.example.com")
	require.NoError(t, err)
	router := gin.New()
	router.GET("/api/user/epay/notify", EpayNotify)
	notify := func(overrides map[string]string) string {
		t.Helper()
		fields := map[string]string{
			"type": "alipay", "money": "1.00", "name": "SUB:EpayPlan", "out_trade_no": tradeNo,
			"trade_no": "provider-trade-1", "trade_status": epay.StatusTradeSuccess,
		}
		maps.Copy(fields, overrides)
		signed, err := signer.SignParams(fields)
		require.NoError(t, err)
		query := url.Values{}
		for key, value := range signed {
			query.Set(key, value)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/epay/notify?"+query.Encode(), nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		return recorder.Body.String()
	}
	assertState := func(wantStatus string, wantSubscriptions int64) {
		t.Helper()
		order := model.GetSubscriptionOrderByTradeNo(tradeNo)
		require.NotNil(t, order)
		assert.Equal(t, wantStatus, order.Status)
		var granted int64
		require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", userID).Count(&granted).Error)
		assert.Equal(t, wantSubscriptions, granted)
	}

	assert.Equal(t, "success", notify(map[string]string{"trade_status": "WAIT_BUYER_PAY"}))
	assertState(common.TopUpStatusPending, 0)
	assert.Equal(t, "fail", notify(map[string]string{"money": "0.01"}))
	assertState(common.TopUpStatusPending, 0)
	assert.Nil(t, model.GetTopUpByTradeNo(tradeNo))

	assert.Equal(t, "success", notify(nil))
	assertState(common.TopUpStatusSuccess, 1)
	topUp := model.GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.Equal(t, model.OrderTypeSubscription, topUp.OrderType)

	// A repeated callback is acknowledged without granting a second subscription.
	assert.Equal(t, "success", notify(nil))
	assertState(common.TopUpStatusSuccess, 1)
}

func TestUpdateOptionValidatesEpaySignSettings(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	originalOptionMap := common.OptionMap
	paymentSetting := operation_setting.GetPaymentSetting()
	originalSignType := paymentSetting.EpaySignType
	originalMerchantPrivateKey := paymentSetting.EpayMerchantPrivateKey
	originalPlatformPublicKey := paymentSetting.EpayPlatformPublicKey
	t.Cleanup(func() {
		common.OptionMap = originalOptionMap
		paymentSetting.EpaySignType = originalSignType
		paymentSetting.EpayMerchantPrivateKey = originalMerchantPrivateKey
		paymentSetting.EpayPlatformPublicKey = originalPlatformPublicKey
	})
	// A site upgraded from an MD5-only release has no saved RSA settings.
	common.OptionMap = map[string]string{
		operation_setting.EpaySignTypeOptionKey:           operation_setting.EpaySignTypeMD5,
		operation_setting.EpayMerchantPrivateKeyOptionKey: "",
		operation_setting.EpayPlatformPublicKeyOptionKey:  "",
	}
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeMD5
	paymentSetting.EpayMerchantPrivateKey = ""
	paymentSetting.EpayPlatformPublicKey = ""
	signTypeKey := operation_setting.EpaySignTypeOptionKey
	privateKeyKey := operation_setting.EpayMerchantPrivateKeyOptionKey
	publicKeyKey := operation_setting.EpayPlatformPublicKeyOptionKey
	merchantPrivateKey, _ := newEpayRSAKeyPairForTest(t)
	_, platformPublicKey := newEpayRSAKeyPairForTest(t)
	const privateKeyError = "The Epay merchant private key is missing or invalid"
	const publicKeyError = "The Epay platform public key is missing or invalid"

	steps := []struct {
		name      string
		key       string
		value     string
		wantError string
	}{
		{"unknown signature type", signTypeKey, "SHA1", "Epay signature type must be MD5 or RSA"},
		{"RSA before keys are saved", signTypeKey, "RSA", privateKeyError},
		{"malformed private key", privateKeyKey, "not a key", privateKeyError},
		{"public key in the private key field", privateKeyKey, platformPublicKey, privateKeyError},
		{"save private key", privateKeyKey, merchantPrivateKey, ""},
		{"RSA before the public key is saved", signTypeKey, "RSA", publicKeyError},
		{"private key in the public key field", publicKeyKey, merchantPrivateKey, publicKeyError},
		{"save public key", publicKeyKey, platformPublicKey, ""},
		{"switch to RSA", signTypeKey, "RSA", ""},
		{"clear private key while RSA is active", privateKeyKey, "", privateKeyError},
		{"switch back to MD5", signTypeKey, "MD5", ""},
	}
	for _, step := range steps {
		err := model.UpdateOption(step.key, step.value)
		if step.wantError == "" {
			require.NoError(t, err, step.name)
			continue
		}
		require.Error(t, err, step.name)
		assert.Equal(t, step.wantError, err.Error(), step.name)
	}
	assert.Equal(t, operation_setting.EpaySignTypeMD5, paymentSetting.EpaySignType)
	assert.Equal(t, merchantPrivateKey, paymentSetting.EpayMerchantPrivateKey)
	assert.Equal(t, platformPublicKey, paymentSetting.EpayPlatformPublicKey)

	// The private key is never sent back to the settings page; the public key may be.
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)
	GetOptions(context)
	var response struct {
		Success bool           `json:"success"`
		Data    []model.Option `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	returned := make(map[string]string, len(response.Data))
	for _, option := range response.Data {
		returned[option.Key] = option.Value
	}
	assert.Equal(t, operation_setting.EpaySignTypeMD5, returned[signTypeKey])
	assert.Equal(t, platformPublicKey, returned[publicKeyKey])
	assert.Equal(t, common.SensitiveOptionPlaceholder, returned[privateKeyKey])
}
