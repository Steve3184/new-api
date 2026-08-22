package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
