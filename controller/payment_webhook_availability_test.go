package controller

import (
	"bytes"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
	originalPayMethods := operation_setting.PayMethods
	originalEpayGateways := operation_setting.EpayGateways
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		operation_setting.EpayGateways = originalEpayGateways
	})
	operation_setting.EpayGateways = nil

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
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
