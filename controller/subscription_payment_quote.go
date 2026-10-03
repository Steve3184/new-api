package controller

import (
	"context"
	"errors"
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
	price, err := finalizeWaffoPancakeCheckoutPrice(&waffoPancakeCheckoutPrice{Money: plan.PriceAmount, Currency: "USD", PriceSnapshot: &service.WaffoPancakePriceSnapshot{TaxCategory: "saas"}})
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
		PlanID  int    `json:"plan_id"`
		Method  string `json:"payment_method"`
		Gateway string `json:"epay_gateway"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	plan, err := model.GetSubscriptionPlanById(req.PlanID)
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
		quote.Currency = "USD"
		if plan.StripePriceId != "" {
			stripe.Key = setting.StripeApiSecret
			price, priceErr := stripeprice.Get(plan.StripePriceId, &stripe.PriceParams{Params: stripe.Params{Context: c.Request.Context()}})
			if priceErr != nil {
				common.ApiErrorMsg(c, "获取 Stripe 商品价格失败")
				return
			}
			quote.Amount = model.PaidFiatFromMinorUnits(price.UnitAmount, string(price.Currency)).Amount
			quote.Currency = strings.ToUpper(string(price.Currency))
		}
	case model.PaymentMethodCreem:
		product, productErr := resolveSubscriptionCreemProduct(plan)
		if productErr != nil {
			common.ApiError(c, productErr)
			return
		}
		quote.Amount, quote.Currency = product.Price, product.Currency
		if plan.CreemProductId != "" {
			endpoint := "https://api.creem.io/v1/products/" + url.PathEscape(product.ProductId)
			if setting.CreemTestMode {
				endpoint = "https://test-api.creem.io/v1/products/" + url.PathEscape(product.ProductId)
			}
			request, requestErr := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
			if requestErr != nil {
				common.ApiError(c, requestErr)
				return
			}
			request.Header.Set("x-api-key", setting.CreemApiKey)
			response, requestErr := (&http.Client{Timeout: 15 * time.Second}).Do(request)
			if requestErr != nil {
				common.ApiErrorMsg(c, "获取 Creem 商品价格失败")
				return
			}
			defer response.Body.Close()
			var price struct {
				Price    int64  `json:"price"`
				Currency string `json:"currency"`
			}
			if response.StatusCode != http.StatusOK || common.DecodeJson(response.Body, &price) != nil || price.Price <= 0 {
				common.ApiErrorMsg(c, "获取 Creem 商品价格失败")
				return
			}
			quote.Amount, quote.Currency = float64(price.Price)/100, strings.ToUpper(price.Currency)
		}
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
	common.ApiSuccess(c, quote)
}
