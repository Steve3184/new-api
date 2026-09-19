package controller

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateRedemptionPurchaseRejectsDisabledFeatureAndWalletBalance(t *testing.T) {
	settings := operation_setting.GetPaymentSetting()
	original := *settings
	t.Cleanup(func() {
		*settings = original
	})

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("id", 1)

	settings.ComplianceConfirmed = true
	settings.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	settings.RedemptionPurchaseEnabled = false
	_, err := validateRedemptionPurchase(ctx, RedemptionPurchaseRequest{
		UnitAmount:    5,
		Quantity:      1,
		PaymentMethod: "alipay",
	})
	require.EqualError(t, err, "兑换码购买功能未启用")

	settings.RedemptionPurchaseEnabled = true
	_, err = validateRedemptionPurchase(ctx, RedemptionPurchaseRequest{
		UnitAmount:    5,
		Quantity:      1,
		PaymentMethod: model.PaymentMethodBalance,
	})
	assert.EqualError(t, err, "兑换码购买不能使用余额")
}

func TestRedemptionPurchaseMinAmountUsesConfiguredEpayMethodMinimum(t *testing.T) {
	originalMinTopUp := operation_setting.MinTopUp
	originalPayMethods := operation_setting.PayMethods
	originalStripeMinTopUp := setting.StripeMinTopUp
	t.Cleanup(func() {
		operation_setting.MinTopUp = originalMinTopUp
		operation_setting.PayMethods = originalPayMethods
		setting.StripeMinTopUp = originalStripeMinTopUp
	})

	operation_setting.MinTopUp = 5
	setting.StripeMinTopUp = 1
	operation_setting.PayMethods = []map[string]string{
		{"type": "alipay", "min_topup": "20"},
		{"type": "wxpay", "min_topup": "3"},
	}

	assert.Equal(t, int64(20), redemptionPurchaseMinAmount("alipay", ""))
	assert.Equal(t, int64(5), redemptionPurchaseMinAmount("wxpay", ""))
	assert.Equal(t, int64(1), redemptionPurchaseMinAmount(model.PaymentMethodStripe, ""))
}

func TestValidateRedemptionPurchaseAppliesWaffoPancakeFees(t *testing.T) {
	previousDB := model.DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousSettings := *operation_setting.GetPaymentSetting()
	previousGeneral := *operation_setting.GetGeneralSetting()
	previousPayMethods := operation_setting.PayMethods
	previousMerchantID := setting.WaffoPancakeMerchantID
	previousPrivateKey := setting.WaffoPancakePrivateKey
	previousProductID := setting.WaffoPancakeProductID
	previousMinTopUp := setting.WaffoPancakeMinTopUp
	previousUnitPrice := setting.WaffoPancakeUnitPrice
	previousUSDToCurrencyRate := setting.WaffoPancakeUSDToCurrencyRate
	previousTopupGroupRatio := common.TopupGroupRatio2JSONString()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB = database
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.TopUp{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		*operation_setting.GetPaymentSetting() = previousSettings
		*operation_setting.GetGeneralSetting() = previousGeneral
		operation_setting.PayMethods = previousPayMethods
		setting.WaffoPancakeMerchantID = previousMerchantID
		setting.WaffoPancakePrivateKey = previousPrivateKey
		setting.WaffoPancakeProductID = previousProductID
		setting.WaffoPancakeMinTopUp = previousMinTopUp
		setting.WaffoPancakeUnitPrice = previousUnitPrice
		setting.WaffoPancakeUSDToCurrencyRate = previousUSDToCurrencyRate
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(previousTopupGroupRatio))
		sqlDB, dbErr := database.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	settings := operation_setting.GetPaymentSetting()
	settings.ComplianceConfirmed = true
	settings.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	settings.RedemptionPurchaseEnabled = true
	settings.AmountDiscount = map[int]float64{}
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.PayMethods = []map[string]string{{
		"type": model.PaymentMethodWaffoPancake, "min_topup": "1", "fee": "0.30", "fee_rate": "3",
	}}
	setting.WaffoPancakeMerchantID = "merchant"
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	setting.WaffoPancakeMinTopUp = 1
	setting.WaffoPancakeUnitPrice = 1.005
	setting.WaffoPancakeUSDToCurrencyRate = 0
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))

	user := model.User{Id: 1, Username: "redemption-pancake", Password: "unused-password", Group: "default"}
	require.NoError(t, database.Create(&user).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("id", user.Id)

	purchase, err := validateRedemptionPurchase(ctx, RedemptionPurchaseRequest{
		UnitAmount:    10,
		Quantity:      1,
		PaymentMethod: model.PaymentMethodWaffoPancake,
	})

	require.NoError(t, err)
	assert.Equal(t, 10.65, purchase.PayMoney)
	record := newRedemptionTopUp(purchase, "WAFFO-PANCAKE-REDEMPTION", model.PaymentProviderWaffoPancake, purchase.Total, purchase.PayMoney)
	require.NoError(t, record.Insert())

	var stored model.TopUp
	require.NoError(t, database.Where("trade_no = ?", record.TradeNo).First(&stored).Error)
	assert.Equal(t, 10.65, stored.Money)
	assert.Equal(t, model.PaymentMethodWaffoPancake, stored.PaymentMethod)
	assert.Equal(t, model.PaymentProviderWaffoPancake, stored.PaymentProvider)
}
