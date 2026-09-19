package controller

import (
	"bytes"
	"fmt"
	"maps"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestFormatWaffoPancakeAmount_UsesDisplayPriceString(t *testing.T) {
	testCases := []struct {
		name     string
		amount   float64
		expected string
	}{
		{name: "whole amount", amount: 29, expected: "29.00"},
		{name: "decimal amount", amount: 29.9, expected: "29.90"},
		{name: "round half up to cents", amount: 29.999, expected: "30.00"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, formatWaffoPancakeAmount(tc.amount))
		})
	}
}

func TestGetWaffoPancakePayMoney(t *testing.T) {
	originalUnitPrice := setting.WaffoPancakeUnitPrice
	originalUSDToCurrencyRate := setting.WaffoPancakeUSDToCurrencyRate
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalDiscounts := make(map[int]float64, len(operation_setting.GetPaymentSetting().AmountDiscount))
	maps.Copy(originalDiscounts, operation_setting.GetPaymentSetting().AmountDiscount)
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()

	t.Cleanup(func() {
		setting.WaffoPancakeUnitPrice = originalUnitPrice
		setting.WaffoPancakeUSDToCurrencyRate = originalUSDToCurrencyRate
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		operation_setting.GetPaymentSetting().AmountDiscount = originalDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	setting.WaffoPancakeUnitPrice = 2.5
	setting.WaffoPancakeUSDToCurrencyRate = 0
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{
		10:                           0.8,
		int(common.QuotaPerUnit * 3): 0.5,
		20:                           0,
	}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1,"vip":1.2}`))

	testCases := []struct {
		name             string
		amount           int64
		group            string
		quotaDisplayType string
		expected         float64
	}{
		{
			name:             "currency display applies unit price group ratio and discount",
			amount:           10,
			group:            "vip",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         24,
		},
		{
			name:             "tokens display converts quota to display units before pricing",
			amount:           int64(common.QuotaPerUnit * 3),
			group:            "vip",
			quotaDisplayType: operation_setting.QuotaDisplayTypeTokens,
			expected:         4.5,
		},
		{
			name:             "non-positive discount falls back to no discount",
			amount:           20,
			group:            "default",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			operation_setting.GetGeneralSetting().QuotaDisplayType = tc.quotaDisplayType
			actual := getWaffoPancakePayMoney(tc.amount, tc.group)
			require.InDelta(t, tc.expected, actual, 0.000001)
		})
	}
}

func TestGetWaffoPancakePayMoney_UsesUSDToSystemCurrencyRate(t *testing.T) {
	originalUnitPrice := setting.WaffoPancakeUnitPrice
	originalUSDToCurrencyRate := setting.WaffoPancakeUSDToCurrencyRate
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()

	t.Cleanup(func() {
		setting.WaffoPancakeUnitPrice = originalUnitPrice
		setting.WaffoPancakeUSDToCurrencyRate = originalUSDToCurrencyRate
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	setting.WaffoPancakeUnitPrice = 2.5
	setting.WaffoPancakeUSDToCurrencyRate = 6.55
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCustom
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))

	// 1 USD = 6.55 system currency units, so a 655-unit top-up costs $100.
	actual := getWaffoPancakePayMoney(655, "default")
	require.InDelta(t, 100, actual, 0.000001)
}

func TestGetConfiguredWaffoPancakeProductCheckoutPrice_MultipliesConfiguredUnitPrice(t *testing.T) {
	configuredPrice := &service.WaffoPancakeConfiguredProductPrice{
		Currency:    "CNY",
		Amount:      "1.00",
		TaxCategory: "saas",
	}

	testCases := []struct {
		name            string
		quantity        int64
		expectedAmount  string
		expectedPayment float64
	}{
		{
			name:            "one credit uses one configured product price",
			quantity:        1,
			expectedAmount:  "1.00",
			expectedPayment: 1,
		},
		{
			name:            "two credits use two configured product prices",
			quantity:        2,
			expectedAmount:  "2.00",
			expectedPayment: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			price, err := getConfiguredWaffoPancakeProductCheckoutPrice(configuredPrice, tc.quantity)

			require.NoError(t, err)
			require.InDelta(t, tc.expectedPayment, price.Money, 0.000001)
			require.Equal(t, "CNY", price.Currency)
			require.Equal(t, tc.expectedAmount, price.PriceSnapshot.Amount)
			require.Equal(t, "saas", price.PriceSnapshot.TaxCategory)
		})
	}
}

func TestWaffoPancakeGlobalPaymentMethodControlsPublicMinimum(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMerchantID := setting.WaffoPancakeMerchantID
	originalPrivateKey := setting.WaffoPancakePrivateKey
	originalProductID := setting.WaffoPancakeProductID
	originalMinTopUp := setting.WaffoPancakeMinTopUp
	originalPayMethods := operation_setting.PayMethods
	originalGateways := operation_setting.EpayGateways
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID = originalMerchantID
		setting.WaffoPancakePrivateKey = originalPrivateKey
		setting.WaffoPancakeProductID = originalProductID
		setting.WaffoPancakeMinTopUp = originalMinTopUp
		operation_setting.PayMethods = originalPayMethods
		operation_setting.EpayGateways = originalGateways
	})

	setting.WaffoPancakeMerchantID = "merchant"
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	setting.WaffoPancakeMinTopUp = 1
	operation_setting.PayMethods = []map[string]string{{
		"name":      "Card",
		"type":      model.PaymentMethodWaffoPancake,
		"icon":      "LuCreditCard",
		"min_topup": "25",
		"fee":       "0.30",
		"fee_rate":  "3",
	}}
	operation_setting.EpayGateways = []operation_setting.EpayGateway{{
		ID: "primary", Name: "Primary", Address: "https://pay.example.com",
		MerchantID: "epay-id", Key: "epay-key", Enabled: true,
		PayMethods: []map[string]string{{"name": "Alipay", "type": "alipay"}},
	}}

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	GetTopUpInfo(context)

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			PayMethods           []map[string]string `json:"pay_methods"`
			WaffoPancakeMinTopUp int                 `json:"waffo_pancake_min_topup"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, 25, payload.Data.WaffoPancakeMinTopUp)
	assert.Equal(t, int64(25), redemptionPurchaseMinAmount(model.PaymentMethodWaffoPancake, ""))

	var pancakeMethod map[string]string
	for _, method := range payload.Data.PayMethods {
		if method["type"] == model.PaymentMethodWaffoPancake {
			pancakeMethod = method
			break
		}
	}
	require.NotNil(t, pancakeMethod)
	assert.Equal(t, "Card", pancakeMethod["name"])
	assert.Equal(t, "LuCreditCard", pancakeMethod["icon"])
	assert.Equal(t, "25", pancakeMethod["min_topup"])
	assert.Equal(t, "0.30", pancakeMethod["fee"])
	assert.Equal(t, "3", pancakeMethod["fee_rate"])
	assert.Empty(t, pancakeMethod["gateway"])
}

func TestGetWaffoPancakeCheckoutPriceAppliesGlobalFeeAndMatchesSnapshot(t *testing.T) {
	originalUseConfiguredPrice := setting.WaffoPancakeUseConfiguredProductPrice
	originalUnitPrice := setting.WaffoPancakeUnitPrice
	originalUSDToCurrencyRate := setting.WaffoPancakeUSDToCurrencyRate
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalPayMethods := operation_setting.PayMethods
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()
	t.Cleanup(func() {
		setting.WaffoPancakeUseConfiguredProductPrice = originalUseConfiguredPrice
		setting.WaffoPancakeUnitPrice = originalUnitPrice
		setting.WaffoPancakeUSDToCurrencyRate = originalUSDToCurrencyRate
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		operation_setting.PayMethods = originalPayMethods
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	setting.WaffoPancakeUseConfiguredProductPrice = false
	setting.WaffoPancakeUnitPrice = 1.005
	setting.WaffoPancakeUSDToCurrencyRate = 0
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.PayMethods = []map[string]string{{
		"type": model.PaymentMethodWaffoPancake, "fee": "0.30", "fee_rate": "3",
	}}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))

	price, err := getWaffoPancakeCheckoutPrice(t.Context(), 10, "default")

	require.NoError(t, err)
	assert.Equal(t, "10.65", price.PriceSnapshot.Amount)
	assert.Equal(t, 10.65, price.Money)
}

func TestRequestWaffoPancakeAmountRejectsGlobalMinimum(t *testing.T) {
	originalMinTopUp := setting.WaffoPancakeMinTopUp
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		setting.WaffoPancakeMinTopUp = originalMinTopUp
		operation_setting.PayMethods = originalPayMethods
	})

	setting.WaffoPancakeMinTopUp = 1
	operation_setting.PayMethods = []map[string]string{{
		"type": model.PaymentMethodWaffoPancake, "min_topup": "25",
	}}

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest("POST", "/api/user/waffo-pancake/amount", bytes.NewBufferString(`{"amount":24}`))
	context.Request.Header.Set("Content-Type", "application/json")
	RequestWaffoPancakeAmount(context)

	var payload struct {
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, "error", payload.Message)
	assert.Equal(t, "充值数量不能小于 25", payload.Data)
}

func TestWaffoPancakePaymentRecordCompatibility(t *testing.T) {
	testCases := []struct {
		name      string
		dsn       func() string
		dialector func(string) gorm.Dialector
	}{
		{
			name: "sqlite",
			dsn: func() string {
				return fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
			},
			dialector: sqlite.Open,
		},
		{name: "mysql", dsn: func() string { return os.Getenv("TEST_MYSQL_DSN") }, dialector: mysql.Open},
		{name: "postgres", dsn: func() string { return os.Getenv("TEST_POSTGRES_DSN") }, dialector: postgres.Open},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := tc.dsn()
			if dsn == "" {
				t.Skip("database DSN is not configured")
			}
			database, err := gorm.Open(tc.dialector(dsn), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, database.AutoMigrate(&model.TopUp{}))

			previousDB := model.DB
			model.DB = database
			defer func() { model.DB = previousDB }()

			tradeNo := "WAFFO-PANCAKE-COMPAT-" + tc.name
			require.NoError(t, database.Where("trade_no = ?", tradeNo).Delete(&model.TopUp{}).Error)
			record := &model.TopUp{
				UserId:          1001,
				Amount:          10,
				Money:           10.65,
				TradeNo:         tradeNo,
				PaymentMethod:   model.PaymentMethodWaffoPancake,
				PaymentProvider: model.PaymentProviderWaffoPancake,
				CreateTime:      1_700_000_000,
				Status:          common.TopUpStatusPending,
			}
			require.NoError(t, record.Insert())

			var stored model.TopUp
			require.NoError(t, database.Where("trade_no = ?", record.TradeNo).First(&stored).Error)
			assert.Equal(t, 10.65, stored.Money)
			assert.Equal(t, model.PaymentMethodWaffoPancake, stored.PaymentMethod)
			assert.Equal(t, model.PaymentProviderWaffoPancake, stored.PaymentProvider)
		})
	}
}
