package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionSimultaneousLimitValidation(t *testing.T) {
	for _, limit := range []int{-1, 2147483648} {
		require.Error(t, normalizeSubscriptionPlanInput(&model.SubscriptionPlan{MaxActivePerUser: limit}))
	}
	for _, limit := range []int{0, 1, 10} {
		plan := &model.SubscriptionPlan{MaxActivePerUser: limit, MaxPurchasePerUser: 20}
		require.NoError(t, normalizeSubscriptionPlanInput(plan))
		assert.Equal(t, limit, plan.MaxActivePerUser)
		assert.Equal(t, 20, plan.MaxPurchasePerUser)
	}
	require.Error(t, normalizeSubscriptionPlanInput(nil))
}

func TestSubscriptionCreemProductPriorityAndWalletFallback(t *testing.T) {
	original := setting.CreemProducts
	t.Cleanup(func() { setting.CreemProducts = original })
	setting.CreemProducts = `[{"productId":"wallet-usd","name":"Wallet","price":12.5,"currency":"USD","quota":6250000}]`
	plan := &model.SubscriptionPlan{PriceAmount: 12.5, Currency: "USD", CreemProductId: "plan-specific"}
	product, err := resolveSubscriptionCreemProduct(plan)
	require.NoError(t, err)
	assert.Equal(t, "plan-specific", product.ProductId)
	plan.CreemProductId = ""
	product, err = resolveSubscriptionCreemProduct(plan)
	require.NoError(t, err)
	assert.Equal(t, "wallet-usd", product.ProductId)
	plan.PriceAmount = 20
	_, err = resolveSubscriptionCreemProduct(plan)
	require.Error(t, err)
	plan.PriceAmount = 12.5
	plan.Currency = "CNY"
	_, err = resolveSubscriptionCreemProduct(plan)
	require.Error(t, err)
}

func TestSubscriptionPancakeFallbackUsesWalletProductAndFees(t *testing.T) {
	originalProduct, originalMethods := setting.WaffoPancakeProductID, operation_setting.PayMethods
	t.Cleanup(func() {
		setting.WaffoPancakeProductID = originalProduct
		operation_setting.PayMethods = originalMethods
	})
	setting.WaffoPancakeProductID = "wallet-product"
	operation_setting.PayMethods = []map[string]string{{"type": model.PaymentMethodWaffoPancake, "fee": "0.30", "fee_rate": "3"}}
	price, err := resolveSubscriptionPancakePrice(context.Background(), &model.SubscriptionPlan{PriceAmount: 10, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "wallet-product", price.ProductID)
	assert.Equal(t, 10.60, price.Amount)
	assert.Equal(t, "USD", price.Currency)
	require.NotNil(t, price.Snapshot)
	assert.Equal(t, "10.60", price.Snapshot.Amount)
	assert.Equal(t, "saas", price.Snapshot.TaxCategory)
	setting.WaffoPancakeProductID = ""
	_, err = resolveSubscriptionPancakePrice(context.Background(), &model.SubscriptionPlan{PriceAmount: 10})
	require.Error(t, err)
}

func TestSubscriptionPancakeFallbackUsesChannelRateForUSDPlans(t *testing.T) {
	originalProduct := setting.WaffoPancakeProductID
	originalRate := setting.WaffoPancakeUSDToCurrencyRate
	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		setting.WaffoPancakeProductID = originalProduct
		setting.WaffoPancakeUSDToCurrencyRate = originalRate
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		operation_setting.PayMethods = originalMethods
	})

	setting.WaffoPancakeProductID = "wallet-product"
	setting.WaffoPancakeUSDToCurrencyRate = 5
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.PayMethods = []map[string]string{{"type": model.PaymentMethodWaffoPancake}}

	price, err := resolveSubscriptionPancakePrice(context.Background(), &model.SubscriptionPlan{
		PriceAmount: 10,
		Currency:    "USD",
	})
	require.NoError(t, err)
	assert.Equal(t, 2.0, price.Amount)
	assert.Equal(t, "2.00", price.Snapshot.Amount)
}

func TestNormalizeSubscriptionPlanInputNormalizesBenefitsOnlyQuota(t *testing.T) {
	plan := &model.SubscriptionPlan{
		TotalAmount:   -42,
		FiveHourLimit: 100,
		WeeklyLimit:   200,
		MonthlyLimit:  300,
	}

	require.NoError(t, normalizeSubscriptionPlanInput(plan))
	assert.EqualValues(t, -1, plan.TotalAmount)
	assert.Zero(t, plan.FiveHourLimit)
	assert.Zero(t, plan.WeeklyLimit)
	assert.Zero(t, plan.MonthlyLimit)
}

func TestNormalizeSubscriptionPlanInputRejectsInvalidUsageLimit(t *testing.T) {
	plan := &model.SubscriptionPlan{FiveHourLimit: int64(common.MaxQuota) + 1}

	require.Error(t, normalizeSubscriptionPlanInput(plan))
}

func TestNormalizeSubscriptionPlanInputRejectsInvalidRPMEntitlement(t *testing.T) {
	plan := &model.SubscriptionPlan{RateLimitGroups: `[{"group":"","rpm":120}]`}

	require.Error(t, normalizeSubscriptionPlanInput(plan))
}
