package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
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

func TestSubscriptionPlanInputUsageTiersAndAutoRenew(t *testing.T) {
	plan := &model.SubscriptionPlan{TotalAmount: 1000, UsageTiers: `[{"multiplier":5,"price_amount":40},{"multiplier":2,"price_amount":18}]`, AllowBalancePay: common.GetPointer(true), AllowAutoRenew: true}
	require.NoError(t, normalizeSubscriptionPlanInput(plan))
	assert.JSONEq(t, `[{"multiplier":2,"price_amount":18},{"multiplier":5,"price_amount":40}]`, plan.UsageTiers)
	assert.True(t, plan.AllowAutoRenew)

	// Auto-renewal charges the wallet, so it is dropped without balance payment.
	plan.AllowBalancePay = common.GetPointer(false)
	require.NoError(t, normalizeSubscriptionPlanInput(plan))
	assert.False(t, plan.AllowAutoRenew)

	require.Error(t, normalizeSubscriptionPlanInput(&model.SubscriptionPlan{UsageTiers: `[{"multiplier":0,"price_amount":1}]`}))
	require.Error(t, normalizeSubscriptionPlanInput(&model.SubscriptionPlan{TotalAmount: common.MaxWalletQuota / 2, UsageTiers: `[{"multiplier":3,"price_amount":1}]`}))
}

func TestSubscriptionGroupBindingIsOptIn(t *testing.T) {
	legacy := &model.SubscriptionPlan{UpgradeGroup: "vip"}
	require.NoError(t, normalizeSubscriptionPlanInput(legacy))
	assert.Empty(t, legacy.SubscriptionGroup)
	for _, group := range []string{"auto", "missing-subscription-group"} {
		plan := &model.SubscriptionPlan{SubscriptionGroup: group}
		require.ErrorContains(t, normalizeSubscriptionPlanInput(plan), "Subscription group does not exist")
	}
	plan := &model.SubscriptionPlan{SubscriptionGroup: " vip "}
	require.NoError(t, normalizeSubscriptionPlanInput(plan))
	assert.Equal(t, "vip", plan.SubscriptionGroup)
}

func TestLegacyPlanEditorDoesNotClearSubscriptionGroup(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionOrder{}, &model.SystemTask{}))
	settings := operation_setting.GetPaymentSetting()
	original := *settings
	t.Cleanup(func() { *settings = original })
	settings.ComplianceConfirmed = true
	settings.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	plan := model.SubscriptionPlan{Title: "Scoped plan", SubscriptionGroup: "vip"}
	require.NoError(t, db.Create(&plan).Error)
	router := gin.New()
	router.PUT("/plans/:id", AdminUpdateSubscriptionPlan)
	path := "/plans/" + strconv.Itoa(plan.Id)
	for _, tc := range []struct {
		body, wantGroup string
		wantKey         bool
	}{
		{`{"plan":{"title":"Enabled key","auto_create_key":true}}`, "vip", true},
		{`{"plan":{"title":"Edited by legacy client"}}`, "vip", true},
		{`{"plan":{"title":"Explicit legacy mode","subscription_group":""}}`, "", false},
		{`{"plan":{"title":"Scoped again","subscription_group":" vip "}}`, "vip", false},
		{`{"plan":{"title":"Bound and enabled","subscription_group":"vip","auto_create_key":true}}`, "vip", true},
		{`{"plan":{"title":"Disabled key","auto_create_key":false}}`, "vip", false},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		var result struct{ Success bool }
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
		require.True(t, result.Success, response.Body.String())
		var saved model.SubscriptionPlan
		require.NoError(t, db.First(&saved, plan.Id).Error)
		assert.Equal(t, tc.wantGroup, saved.SubscriptionGroup)
		payload, err := common.Marshal(saved)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, common.Unmarshal(payload, &fields))
		assert.Equal(t, tc.wantKey, fields["auto_create_key"])
	}
}

func TestSubscriptionPlanUnbindProtectsActiveHolders(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionOrder{}, &model.SystemTask{}, &model.Option{}))
	settings := operation_setting.GetPaymentSetting()
	original := *settings
	t.Cleanup(func() { *settings = original })
	settings.ComplianceConfirmed = true
	settings.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	plan := model.SubscriptionPlan{Title: "Scoped plan", SubscriptionGroup: "vip"}
	require.NoError(t, db.Create(&plan).Error)
	router := gin.New()
	router.PUT("/plans/:id", AdminUpdateSubscriptionPlan)
	path := "/plans/" + strconv.Itoa(plan.Id)
	update := func() bool {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, path, strings.NewReader(`{"plan":{"title":"Edited","subscription_group":""}}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		var result struct{ Success bool }
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
		return result.Success
	}
	sub := model.UserSubscription{PlanId: plan.Id, UserId: 1, Status: "active", EndTime: common.GetTimestamp() + 1000}
	require.NoError(t, db.Create(&sub).Error)
	assert.False(t, update(), "active holders must keep their access")
	require.Error(t, model.DeleteSubscriptionPlan(plan.Id))
	require.NoError(t, db.Model(&sub).Update("status", "cancelled").Error)
	order := model.SubscriptionOrder{PlanId: plan.Id, UserId: 1, TradeNo: "pending-bound-order", Status: common.TopUpStatusPending}
	require.NoError(t, db.Create(&order).Error)
	assert.False(t, update(), "an unsettled checkout must keep its plan binding")
	require.Error(t, model.DeleteSubscriptionPlan(plan.Id))
	require.NoError(t, db.Model(&order).Update("status", common.TopUpStatusExpired).Error)
	require.True(t, update(), "removing the last binding is allowed without active holders")
	var saved model.SubscriptionPlan
	require.NoError(t, db.First(&saved, plan.Id).Error)
	assert.Empty(t, saved.SubscriptionGroup)
}

func TestSubscriptionEpayQuoteHonorsPerGatewayMinimum(t *testing.T) {
	original := operation_setting.MinTopUp
	t.Cleanup(func() { operation_setting.MinTopUp = original })
	operation_setting.MinTopUp = 5
	require.ErrorContains(t, validateSubscriptionPaymentMinimum(4.99, "CNY", "alipay", ""), "below the minimum")
	require.NoError(t, validateSubscriptionPaymentMinimum(5, "CNY", "alipay", ""))
	require.NoError(t, validateSubscriptionPaymentMinimum(0, "USD", model.PaymentMethodBalance, ""))
}

func TestSubscriptionMinimumUsesGatewayCurrency(t *testing.T) {
	oldMin, oldRate := setting.StripeMinTopUp, operation_setting.USDExchangeRate
	t.Cleanup(func() { setting.StripeMinTopUp, operation_setting.USDExchangeRate = oldMin, oldRate })
	setting.StripeMinTopUp, operation_setting.USDExchangeRate = 5, 7
	require.ErrorContains(t, validateSubscriptionPaymentMinimum(34.99, "CNY", model.PaymentMethodStripe, ""), "below the minimum")
	require.NoError(t, validateSubscriptionPaymentMinimum(35, "CNY", model.PaymentMethodStripe, ""))
	require.ErrorContains(t, validateSubscriptionPaymentMinimum(50, "EUR", model.PaymentMethodStripe, ""), "Cannot compare")
	setting.StripeMinTopUp = 0
	require.NoError(t, validateSubscriptionPaymentMinimum(50, "EUR", model.PaymentMethodStripe, ""))
}

func TestSubscriptionStripeCheckoutDoesNotSetGlobalKey(t *testing.T) {
	backend, global, configured := stripe.GetBackend(stripe.APIBackend), stripe.Key, setting.StripeApiSecret
	t.Cleanup(func() {
		stripe.SetBackend(stripe.APIBackend, backend)
		stripe.Key, setting.StripeApiSecret = global, configured
	})
	stripe.Key, setting.StripeApiSecret = "fixture-global", "fixture-checkout"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "Bearer fixture-checkout", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/prices/price_fixture":
			_, _ = io.WriteString(w, `{"currency":"eur","unit_amount":1200}`)
		case "/v1/checkout/sessions":
			_, _ = io.WriteString(w, `{"url":"https://example.test/checkout"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(server.URL)}))
	price, err := resolveSubscriptionStripeCheckout(context.Background(), &model.SubscriptionPlan{StripePriceId: "price_fixture"})
	require.NoError(t, err)
	assert.Equal(t, 12.0, price.Amount)
	assert.Equal(t, "EUR", price.Currency)
	assert.Equal(t, "fixture-global", stripe.Key)
	link, err := genStripeSubscriptionLink("ref_fixture", "cus_fixture", "", "price_fixture")
	require.NoError(t, err)
	assert.Equal(t, "https://example.test/checkout", link)
	assert.Equal(t, "fixture-global", stripe.Key)
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

func TestSubscriptionCreemFallbackVerifiesProviderPrice(t *testing.T) {
	oldProducts, oldTransport := setting.CreemProducts, http.DefaultTransport
	t.Cleanup(func() { setting.CreemProducts, http.DefaultTransport = oldProducts, oldTransport })
	setting.CreemProducts = `[{"productId":"fixture-wallet","name":"Wallet","price":12.5,"currency":"USD"}]`
	plan := &model.SubscriptionPlan{PriceAmount: 12.5, Currency: "USD"}
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"price":1250,"currency":"USD"}`, true},
		{`{"price":1300,"currency":"USD"}`, false},
		{`{"price":1250,"currency":"EUR"}`, false},
	} {
		calls := 0
		http.DefaultTransport = emailHTTPTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			assert.Equal(t, http.MethodGet, req.Method)
			assert.Equal(t, "/v1/products/fixture-wallet", req.URL.Path)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
		})
		product, err := resolveSubscriptionCreemCheckout(context.Background(), plan)
		assert.Equal(t, 1, calls)
		if tc.ok {
			require.NoError(t, err)
			assert.Equal(t, 12.5, product.Price)
		} else {
			require.Error(t, err)
		}
	}
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
