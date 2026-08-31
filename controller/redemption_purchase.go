package controller

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
	"github.com/waffo-com/waffo-go/types/order"
)

const maxRedemptionPurchaseInt64 = int64(^uint64(0) >> 1)

type RedemptionPurchaseRequest struct {
	UnitAmount     int64  `json:"unit_amount"`
	Amount         int64  `json:"amount"` // compatibility alias for clients using the top-up shape
	Quantity       int    `json:"quantity"`
	PaymentMethod  string `json:"payment_method"`
	PayMethodIndex *int   `json:"pay_method_index"`
}

type redemptionPurchaseContext struct {
	Request   RedemptionPurchaseRequest
	User      *model.User
	Group     string
	Total     int64
	UnitQuota int
	PayMoney  float64
}

func redemptionPurchaseEnabled() bool {
	return operation_setting.IsPaymentComplianceConfirmed() &&
		operation_setting.GetPaymentSetting().RedemptionPurchaseEnabled
}

func redemptionPurchasePaymentMethods() []string {
	if !redemptionPurchaseEnabled() {
		return []string{}
	}
	methods := make([]string, 0, len(operation_setting.PayMethods)+4)
	seen := make(map[string]struct{})
	add := func(method string) {
		if method == "" || method == model.PaymentMethodBalance {
			return
		}
		if _, ok := seen[method]; ok {
			return
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	if isEpayTopUpEnabled() {
		for _, method := range operation_setting.PayMethods {
			add(method["type"])
		}
	}
	if isStripeTopUpEnabled() {
		add(model.PaymentMethodStripe)
	}
	if isWaffoTopUpEnabled() {
		add(model.PaymentMethodWaffo)
	}
	if isWaffoPancakeTopUpEnabled() {
		add(model.PaymentMethodWaffoPancake)
	}
	if service.IsMoneroTopUpEnabled() {
		add(model.PaymentMethodMonero)
	}
	return methods
}

func isRedemptionPurchasePaymentMethodAvailable(method string) bool {
	for _, available := range redemptionPurchasePaymentMethods() {
		if available == method {
			return true
		}
	}
	return false
}

func redemptionPurchaseMinAmount(method string) int64 {
	minAmount := getMinTopup()
	switch method {
	case model.PaymentMethodStripe:
		return getStripeMinTopup()
	case model.PaymentMethodWaffo:
		return int64(setting.WaffoMinTopUp)
	case model.PaymentMethodWaffoPancake:
		return int64(setting.WaffoPancakeMinTopUp)
	case model.PaymentMethodMonero:
		return int64(operation_setting.MinTopUp)
	}

	for _, configuredMethod := range operation_setting.PayMethods {
		if configuredMethod["type"] != method {
			continue
		}
		configuredMin, err := strconv.ParseInt(configuredMethod["min_topup"], 10, 64)
		if err == nil && configuredMin > minAmount {
			minAmount = configuredMin
		}
		break
	}

	return minAmount
}

func normalizeRedemptionPurchaseAmount(req RedemptionPurchaseRequest) (int64, error) {
	amount := req.UnitAmount
	if amount <= 0 {
		amount = req.Amount
	}
	if amount <= 0 {
		return 0, errors.New("兑换码面值必须大于 0")
	}
	if req.Quantity <= 0 || req.Quantity > model.MaxRedemptionPurchaseCount {
		return 0, fmt.Errorf("兑换码数量必须在 1 到 %d 之间", model.MaxRedemptionPurchaseCount)
	}
	if amount > maxRedemptionPurchaseInt64/int64(req.Quantity) {
		return 0, errors.New("兑换码购买总额超出范围")
	}
	return amount, nil
}

func validateRedemptionPurchase(c *gin.Context, req RedemptionPurchaseRequest) (*redemptionPurchaseContext, error) {
	if !redemptionPurchaseEnabled() {
		return nil, errors.New("兑换码购买功能未启用")
	}
	if req.PaymentMethod == model.PaymentMethodBalance || req.PaymentMethod == model.PaymentProviderBalance {
		return nil, errors.New("兑换码购买不能使用余额")
	}
	if !isRedemptionPurchasePaymentMethodAvailable(req.PaymentMethod) {
		return nil, errors.New("支付方式未配置或不可用于购买兑换码")
	}
	unitAmount, err := normalizeRedemptionPurchaseAmount(req)
	if err != nil {
		return nil, err
	}
	total := unitAmount * int64(req.Quantity)
	if minAmount := redemptionPurchaseMinAmount(req.PaymentMethod); minAmount > 0 && total < minAmount {
		return nil, fmt.Errorf("兑换码购买总额不能小于 %d", minAmount)
	}
	if maxAmount := getMaxTopUpAmount(); maxAmount > 0 && total > maxAmount {
		return nil, fmt.Errorf("兑换码购买总额不能大于 %d", maxAmount)
	}
	unitQuota, err := getTopUpQuota(unitAmount)
	if err != nil || unitQuota <= 0 {
		return nil, errors.New("兑换码面值无效")
	}
	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil || user == nil {
		return nil, errors.New("用户不存在")
	}
	group := user.Group
	ctx := &redemptionPurchaseContext{
		Request:   req,
		User:      user,
		Group:     group,
		Total:     total,
		UnitQuota: unitQuota,
	}
	ctx.Request.UnitAmount = unitAmount
	if req.PaymentMethod == model.PaymentMethodMonero {
		return ctx, nil
	}
	var payMoney float64
	switch req.PaymentMethod {
	case model.PaymentMethodStripe:
		payMoney = getStripePayMoney(float64(total), group)
	case model.PaymentMethodWaffo:
		payMoney = getWaffoPayMoney(float64(total), group)
	case model.PaymentMethodWaffoPancake:
		// Purchase orders always use the configured amount discount. A
		// configured Pancake product price is a wallet-top-up option and is
		// intentionally not reused for code purchases.
		payMoney = getWaffoPancakePayMoney(total, group)
	default:
		payMoney = getPayMoney(total, group)
	}
	if payMoney < 0.01 || math.IsNaN(payMoney) || math.IsInf(payMoney, 0) {
		return nil, errors.New("支付金额过低或无效")
	}
	ctx.PayMoney = payMoney
	return ctx, nil
}

func redemptionPurchaseName(unitAmount int64, userID int) string {
	name := fmt.Sprintf("Code %d U%d", unitAmount, userID)
	if len(name) > 20 {
		return "Purchased code"
	}
	return name
}

func newRedemptionTopUp(ctx *redemptionPurchaseContext, tradeNo string, provider string, amount int64, money float64) *model.TopUp {
	return &model.TopUp{
		UserId:           ctx.User.Id,
		Amount:           amount,
		Money:            money,
		TradeNo:          tradeNo,
		PaymentMethod:    ctx.Request.PaymentMethod,
		PaymentProvider:  provider,
		CreateTime:       time.Now().Unix(),
		Status:           common.TopUpStatusPending,
		OrderType:        model.OrderTypeRedemption,
		RedemptionQuota:  ctx.UnitQuota,
		RedemptionCount:  ctx.Request.Quantity,
		RedemptionAmount: ctx.Request.UnitAmount,
		RedemptionName:   redemptionPurchaseName(ctx.Request.UnitAmount, ctx.User.Id),
	}
}

func RequestRedemptionPurchaseAmount(c *gin.Context) {
	var req RedemptionPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	ctx, err := validateRedemptionPurchase(c, req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	if req.PaymentMethod == model.PaymentMethodMonero {
		c.JSON(http.StatusOK, gin.H{"message": "success", "data": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(ctx.PayMoney, 'f', 2, 64)})
}

func RequestRedemptionPurchase(c *gin.Context) {
	var req RedemptionPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	ctx, err := validateRedemptionPurchase(c, req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	switch req.PaymentMethod {
	case model.PaymentMethodStripe:
		requestRedemptionStripePurchase(c, ctx)
	case model.PaymentMethodWaffo:
		requestRedemptionWaffoPurchase(c, ctx)
	case model.PaymentMethodWaffoPancake:
		requestRedemptionWaffoPancakePurchase(c, ctx)
	case model.PaymentMethodMonero:
		invoice, invoiceErr := service.CreateMoneroRedemptionInvoice(c.Request.Context(), ctx.User.Id, req.UnitAmount, req.Quantity)
		if invoiceErr != nil {
			common.ApiErrorMsg(c, invoiceErr.Error())
			return
		}
		common.ApiSuccess(c, invoice)
	default:
		requestRedemptionEpayPurchase(c, ctx)
	}
}

func requestRedemptionEpayPurchase(c *gin.Context, ctx *redemptionPurchaseContext) {
	client := GetEpayClient()
	if client == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "当前管理员未配置支付信息"})
		return
	}
	tradeNo := fmt.Sprintf("USR%dRC%s", ctx.User.Id, common.GetRandomString(18))
	callbackAddress := service.GetCallbackAddress()
	returnURL, _ := url.Parse(paymentReturnPath("/wallet"))
	notifyURL, _ := url.Parse(callbackAddress + "/api/user/epay/notify")
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           ctx.Request.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("RC%dX%d", ctx.Request.UnitAmount, ctx.Request.Quantity),
		Money:          strconv.FormatFloat(ctx.PayMoney, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyURL,
		ReturnUrl:      returnURL,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("创建兑换码支付失败 user_id=%d trade_no=%s error=%q", ctx.User.Id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if err := newRedemptionTopUp(ctx, tradeNo, model.PaymentProviderEpay, ctx.Total, ctx.PayMoney).Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("创建兑换码订单失败 user_id=%d trade_no=%s error=%q", ctx.User.Id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

func requestRedemptionStripePurchase(c *gin.Context, ctx *redemptionPurchaseContext) {
	reference := fmt.Sprintf("redemption-ref-%d-%d-%s", ctx.User.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceID := "ref_" + common.Sha1([]byte(reference))
	payLink, err := genStripeAmountLink(
		referenceID,
		ctx.User.StripeCustomer,
		ctx.User.Email,
		ctx.PayMoney,
		redemptionPurchaseName(ctx.Request.UnitAmount, ctx.User.Id),
		"",
		"",
	)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("创建兑换码 Stripe 支付失败 user_id=%d trade_no=%s error=%q", ctx.User.Id, referenceID, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if err := newRedemptionTopUp(ctx, referenceID, model.PaymentProviderStripe, ctx.Total, ctx.PayMoney).Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"pay_link": payLink}})
}

func requestRedemptionWaffoPurchase(c *gin.Context, ctx *redemptionPurchaseContext) {
	var req WaffoPayRequest
	req.Amount = ctx.Total
	req.PayMethodIndex = ctx.Request.PayMethodIndex
	methods := setting.GetWaffoPayMethods()
	var payMethodType, payMethodName string
	if req.PayMethodIndex != nil {
		idx := *req.PayMethodIndex
		if idx < 0 || idx >= len(methods) {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付方式"})
			return
		}
		payMethodType = methods[idx].PayMethodType
		payMethodName = methods[idx].PayMethodName
	}
	tradeNo := fmt.Sprintf("WAFFO-RC-%d-%d-%s", ctx.User.Id, time.Now().UnixMilli(), randstr.String(6))
	topUp := newRedemptionTopUp(ctx, tradeNo, model.PaymentProviderWaffo, ctx.Total, ctx.PayMoney)
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	sdk, err := getWaffoSDK()
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}
	notifyURL := service.GetCallbackAddress() + "/api/waffo/webhook"
	if setting.WaffoNotifyUrl != "" {
		notifyURL = setting.WaffoNotifyUrl
	}
	returnURL := paymentReturnPath("/wallet")
	if setting.WaffoReturnUrl != "" {
		returnURL = setting.WaffoReturnUrl
	}
	currency := getWaffoCurrency()
	goodsInfo := buildWaffoTopUpGoodsInfo(ctx.Total)
	resp, err := sdk.Order().Create(c.Request.Context(), &order.CreateOrderParams{
		PaymentRequestID:   tradeNo,
		MerchantOrderID:    tradeNo,
		OrderAmount:        formatWaffoAmount(ctx.PayMoney, currency),
		OrderCurrency:      currency,
		OrderDescription:   goodsInfo.GoodsName,
		OrderRequestedAt:   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		NotifyURL:          notifyURL,
		MerchantInfo:       &order.MerchantInfo{MerchantID: setting.WaffoMerchantId},
		UserInfo:           &order.UserInfo{UserID: strconv.Itoa(ctx.User.Id), UserEmail: getWaffoUserEmail(ctx.User), UserTerminal: "WEB"},
		PaymentInfo:        &order.PaymentInfo{ProductName: "ONE_TIME_PAYMENT", PayMethodType: payMethodType, PayMethodName: payMethodName},
		GoodsInfo:          goodsInfo,
		SuccessRedirectURL: returnURL,
		FailedRedirectURL:  returnURL,
	}, nil)
	if err != nil || !resp.IsSuccess() {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	orderData := resp.GetData()
	paymentURL := orderData.FetchRedirectURL()
	if paymentURL == "" {
		paymentURL = orderData.OrderAction
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"payment_url": paymentURL, "order_id": tradeNo}})
}

func requestRedemptionWaffoPancakePurchase(c *gin.Context, ctx *redemptionPurchaseContext) {
	tradeNo := fmt.Sprintf("WAFFO_PANCAKE-RC-%d-%d-%s", ctx.User.Id, time.Now().UnixMilli(), randstr.String(6))
	price := &waffoPancakeCheckoutPrice{
		Money:         ctx.PayMoney,
		Currency:      "USD",
		PriceSnapshot: &service.WaffoPancakePriceSnapshot{Amount: formatWaffoPancakeAmount(ctx.PayMoney), TaxCategory: "saas"},
	}
	topUp := newRedemptionTopUp(ctx, tradeNo, model.PaymentProviderWaffoPancake, ctx.Total, ctx.PayMoney)
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	expiresInSeconds := 45 * 60
	session, err := service.CreateWaffoPancakeCheckoutSession(c.Request.Context(), &service.WaffoPancakeCreateSessionParams{
		ProductID:               setting.WaffoPancakeProductID,
		Currency:                price.Currency,
		BuyerIdentity:           getWaffoPancakeBuyerIdentity(ctx.User),
		PriceSnapshot:           price.PriceSnapshot,
		BuyerEmail:              getWaffoPancakeBuyerEmail(ctx.User),
		ExpiresInSeconds:        &expiresInSeconds,
		OrderMerchantExternalID: tradeNo,
	})
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{
		"checkout_url":     session.CheckoutURL,
		"session_id":       session.SessionID,
		"expires_at":       session.ExpiresAt,
		"order_id":         tradeNo,
		"token":            session.Token,
		"token_expires_at": session.TokenExpiresAt,
	}})
}
