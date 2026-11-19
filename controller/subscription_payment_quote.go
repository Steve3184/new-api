package controller

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v81"
	stripeprice "github.com/stripe/stripe-go/v81/price"
)

type subscriptionCheckoutPrice struct {
	Amount    float64                            `json:"amount"`
	Currency  string                             `json:"currency"`
	ProductID string                             `json:"-"`
	Snapshot  *service.WaffoPancakePriceSnapshot `json:"-"`
}

// Subscription checkout minima are expressed in USD for direct gateways and
// CNY for Epay. Compare in the checkout currency only when a conversion exists.
func validateSubscriptionPaymentMinimum(amount float64, currency, method, gateway string) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return common.NewMessage("Invalid subscription payment amount")
	}
	var minimum decimal.Decimal
	minimumCurrency := "USD"
	config := operation_setting.GetGlobalPayMethod(method)
	switch method {
	case model.PaymentMethodBalance:
		return nil
	case model.PaymentMethodStripe:
		minimum = decimal.NewFromInt(int64(setting.StripeMinTopUp))
	case model.PaymentMethodCreem:
	case model.PaymentMethodWaffoPancake:
		minimum = decimal.NewFromInt(int64(setting.WaffoPancakeMinTopUp))
	case model.PaymentMethodNowPayments:
		minimum = decimal.NewFromInt(int64(setting.NowPaymentsMinTopUp))
	default:
		minimumCurrency = "CNY"
		minimum = decimal.NewFromInt(int64(operation_setting.MinTopUp))
		config = operation_setting.GetPayMethodForGateway(method, gateway)
	}
	if value, err := decimal.NewFromString(strings.TrimSpace(config["min_topup"])); err == nil && value.GreaterThan(minimum) {
		minimum = value
	}
	if !minimum.IsPositive() {
		return nil
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency != minimumCurrency {
		rate := operation_setting.USDExchangeRate
		if minimumCurrency != "USD" || currency != "CNY" || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return common.NewMessage("Cannot compare the {{minimum_currency}} payment minimum with {{currency}} checkout", map[string]any{"minimum_currency": minimumCurrency, "currency": currency})
		}
		minimum = minimum.Mul(decimal.NewFromFloat(rate)).RoundCeil(2)
	}
	if decimal.NewFromFloat(amount).LessThan(minimum) {
		return common.NewMessage("Subscription payment is below the minimum of {{amount}} {{currency}}", map[string]any{"amount": minimum.String(), "currency": currency})
	}
	return nil
}

// Creem's wallet path uses fixed products; only a product with the same price
// and currency can be substituted for a plan without silently changing its cost.
func resolveSubscriptionCreemProduct(plan *model.SubscriptionPlan) (*CreemProduct, error) {
	if strings.TrimSpace(plan.CreemProductId) != "" {
		return &CreemProduct{ProductId: strings.TrimSpace(plan.CreemProductId), Name: plan.Title, Price: plan.PriceAmount, Currency: plan.Currency}, nil
	}
	var products []CreemProduct
	if err := common.UnmarshalJsonStr(setting.CreemProducts, &products); err != nil {
		return nil, err
	}
	for _, product := range products {
		if product.ProductId != "" && strings.EqualFold(product.Currency, plan.Currency) && decimal.NewFromFloat(product.Price).Round(2).Equal(decimal.NewFromFloat(plan.PriceAmount).Round(2)) {
			return &product, nil
		}
	}
	return nil, errors.New("未配置与套餐价格匹配的 Creem 充值商品")
}

func resolveSubscriptionCreemCheckout(ctx context.Context, plan *model.SubscriptionPlan) (*CreemProduct, error) {
	product, err := resolveSubscriptionCreemProduct(plan)
	if err != nil {
		return nil, err
	}
	host := "https://api.creem.io"
	if setting.CreemTestMode {
		host = "https://test-api.creem.io"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/v1/products/"+url.PathEscape(product.ProductId), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("x-api-key", setting.CreemApiKey)
	response, err := (&http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}).Do(request)
	if err != nil {
		return nil, common.NewMessage("Unable to load Creem product price")
	}
	defer response.Body.Close()
	var price struct {
		Price    int64  `json:"price"`
		Currency string `json:"currency"`
	}
	if response.StatusCode != http.StatusOK || common.DecodeJson(io.LimitReader(response.Body, 1<<20), &price) != nil || price.Price <= 0 || strings.TrimSpace(price.Currency) == "" {
		return nil, common.NewMessage("Unable to load Creem product price")
	}
	currency := strings.ToUpper(strings.TrimSpace(price.Currency))
	amount := model.PaidFiatFromMinorUnits(price.Price, currency).Amount
	if strings.TrimSpace(plan.CreemProductId) == "" &&
		(!strings.EqualFold(currency, plan.Currency) || !decimal.NewFromFloat(amount).Equal(decimal.NewFromFloat(plan.PriceAmount))) {
		return nil, common.NewMessage("Creem wallet product no longer matches the subscription price or currency")
	}
	product.Price = amount
	product.Currency = currency
	return product, nil
}

func resolveSubscriptionStripeCheckout(ctx context.Context, plan *model.SubscriptionPlan) (*subscriptionCheckoutPrice, error) {
	quote := &subscriptionCheckoutPrice{Amount: math.Round(plan.PriceAmount*100) / 100, Currency: "USD"}
	if plan.StripePriceId == "" {
		return quote, nil
	}
	client := stripeprice.Client{B: stripe.GetBackend(stripe.APIBackend), Key: setting.StripeApiSecret}
	price, err := client.Get(plan.StripePriceId, &stripe.PriceParams{Params: stripe.Params{Context: ctx}})
	if err != nil {
		return nil, common.NewMessage("Unable to load Stripe product price")
	}
	quote.Amount = model.PaidFiatFromMinorUnits(price.UnitAmount, string(price.Currency)).Amount
	quote.Currency = strings.ToUpper(string(price.Currency))
	return quote, nil
}

func resolveSubscriptionPancakePrice(ctx context.Context, plan *model.SubscriptionPlan) (*subscriptionCheckoutPrice, error) {
	productID := strings.TrimSpace(plan.WaffoPancakeProductId)
	if productID != "" {
		configured, err := service.GetWaffoPancakeConfiguredProductPrice(ctx, productID)
		if err != nil {
			return nil, err
		}
		amount, err := decimal.NewFromString(configured.Amount)
		if err != nil || !amount.IsPositive() {
			return nil, errors.New("商品价格无效")
		}
		return &subscriptionCheckoutPrice{Amount: amount.InexactFloat64(), Currency: configured.Currency, ProductID: productID}, nil
	}
	productID = strings.TrimSpace(setting.WaffoPancakeProductID)
	if productID == "" {
		return nil, errors.New("Waffo Pancake 未配置商品")
	}
	amount := decimal.NewFromFloat(plan.PriceAmount)
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeUSD &&
		operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		rate := setting.WaffoPancakeUSDToCurrencyRate
		if rate > 0 && !math.IsNaN(rate) && !math.IsInf(rate, 0) {
			amount = amount.Div(decimal.NewFromFloat(rate))
		}
	}
	price, err := finalizeWaffoPancakeCheckoutPrice(&waffoPancakeCheckoutPrice{Money: amount.InexactFloat64(), Currency: "USD", PriceSnapshot: &service.WaffoPancakePriceSnapshot{TaxCategory: "saas"}})
	if err != nil {
		return nil, err
	}
	return &subscriptionCheckoutPrice{Amount: price.Money, Currency: price.Currency, ProductID: productID, Snapshot: price.PriceSnapshot}, nil
}

func SubscriptionRequestPaymentQuote(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req struct {
		PlanID     int    `json:"plan_id"`
		Multiplier int    `json:"multiplier"`
		Method     string `json:"payment_method"`
		Gateway    string `json:"epay_gateway"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	plan, err := model.GetSubscriptionPlanTier(req.PlanID, req.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if err := model.CheckSubscriptionPurchaseCapacity(nil, c.GetInt("id"), plan); err != nil {
		common.ApiError(c, err)
		return
	}
	quote := &subscriptionCheckoutPrice{Amount: plan.PriceAmount, Currency: plan.Currency}
	switch req.Method {
	case model.PaymentMethodBalance:
	case model.PaymentMethodNowPayments:
		price, priceErr := service.GetNowPaymentsSubscriptionPriceUSD(plan)
		if priceErr != nil {
			common.ApiError(c, priceErr)
			return
		}
		quote.Amount, quote.Currency = price.InexactFloat64(), "USD"
	case model.PaymentMethodStripe:
		quote, err = resolveSubscriptionStripeCheckout(c.Request.Context(), plan)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	case model.PaymentMethodCreem:
		product, productErr := resolveSubscriptionCreemCheckout(c.Request.Context(), plan)
		if productErr != nil {
			common.ApiError(c, productErr)
			return
		}
		quote.Amount, quote.Currency = product.Price, product.Currency
	case model.PaymentMethodWaffoPancake:
		quote, err = resolveSubscriptionPancakePrice(c.Request.Context(), plan)
		if err != nil {
			common.ApiErrorMsg(c, "获取 Waffo Pancake 商品价格失败")
			return
		}
	default:
		if operation_setting.GetPayMethodForGateway(req.Method, req.Gateway) == nil {
			common.ApiErrorMsg(c, "不支持的支付方式")
			return
		}
		quote.Amount = applyEpayFee(plan.PriceAmount, req.Method, req.Gateway)
		quote.Currency = "CNY"
	}
	if quote.Amount < 0 || math.IsNaN(quote.Amount) || math.IsInf(quote.Amount, 0) {
		common.ApiErrorMsg(c, "套餐支付金额无效")
		return
	}
	if err := validateSubscriptionPaymentMinimum(quote.Amount, quote.Currency, req.Method, req.Gateway); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, quote)
}
