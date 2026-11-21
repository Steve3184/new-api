package controller

import (
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
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type SubscriptionEpayPayRequest struct {
	Multiplier    int    `json:"multiplier"`
	PlanId        int    `json:"plan_id"`
	PaymentMethod string `json:"payment_method"`
	EpayGatewayID string `json:"epay_gateway"`
}

func SubscriptionRequestEpay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req SubscriptionEpayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	plan, err := model.GetSubscriptionPlanTier(req.PlanId, req.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorT(c, "Subscription plan is not enabled")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorT(c, "Plan amount is too low")
		return
	}
	if operation_setting.GetPayMethodForGateway(req.PaymentMethod, req.EpayGatewayID) == nil {
		common.ApiErrorT(c, "Payment method does not exist")
		return
	}

	userId := c.GetInt("id")
	if err := model.CheckSubscriptionPurchaseCapacity(nil, userId, plan); err != nil {
		common.ApiError(c, err)
		return
	}

	callBackAddress := service.GetCallbackAddress()
	returnUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/return")
	if err != nil {
		common.ApiErrorT(c, "Callback URL configuration error")
		return
	}
	notifyUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/notify")
	if err != nil {
		common.ApiErrorT(c, "Callback URL configuration error")
		return
	}

	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("SUBUSR%dNO%s", userId, tradeNo)

	gateway := epayGatewayForMethod(req.PaymentMethod, req.EpayGatewayID)
	if gateway == nil {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}
	client := getEpayClientForGateway(gateway.ID)
	if client == nil {
		common.ApiErrorT(c, "Payment information has not been configured by administrator")
		return
	}

	payMoney := applyEpayFee(plan.PriceAmount, req.PaymentMethod, req.EpayGatewayID)
	if payMoney < 0.01 || math.IsNaN(payMoney) || math.IsInf(payMoney, 0) {
		common.ApiErrorMsg(c, "套餐支付金额无效")
		return
	}
	if err := validateSubscriptionPaymentMinimum(payMoney, "CNY", req.PaymentMethod, gateway.ID); err != nil {
		common.ApiError(c, err)
		return
	}
	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		Multiplier:      plan.UsageMultiplier,
		Money:           payMoney,
		PaymentCurrency: "CNY",
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: model.PaymentProviderEpay,
		EpayGatewayID:   gateway.ID,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		common.ApiErrorT(c, "Failed to create order")
		return
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("SUB:%s", plan.Title),
		Money:          strconv.FormatFloat(payMoney, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentProviderEpay)
		common.ApiErrorT(c, "Failed to start payment")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

func SubscriptionEpayNotify(c *gin.Context) {
	var params map[string]string

	if c.Request.Method == "POST" {
		// POST 请求：从 POST body 解析参数
		if err := c.Request.ParseForm(); err != nil {
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET 请求：从 URL Query 解析参数
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription webhook params are empty client_ip=%s", c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	gateway := epayGatewayForTradeNo(params["out_trade_no"])
	if gateway == nil {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription webhook order or gateway not found trade_no=%s client_ip=%s", params["out_trade_no"], c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	client := getEpayClientForGateway(gateway.ID)
	if client == nil {
		logger.LogError(c.Request.Context(), common.LogText("Epay client is not initialized path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	verifyInfo, err := verifyEpayCallback(client, gateway, params)
	if err != nil {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay webhook signature verification failed path=%q client_ip=%s verify_error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	if verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		logger.LogInfo(c.Request.Context(), common.LogText("Epay webhook ignored event trade_no=%s callback_type=%s trade_status=%s client_ip=%s verify_info=%q", verifyInfo.ServiceTradeNo, verifyInfo.Type, verifyInfo.TradeStatus, c.ClientIP(), common.GetJsonString(verifyInfo)))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	order := model.GetSubscriptionOrderByTradeNo(verifyInfo.ServiceTradeNo)
	if order == nil || order.PaymentProvider != model.PaymentProviderEpay {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription webhook order or gateway not found trade_no=%s client_ip=%s", verifyInfo.ServiceTradeNo, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if err := validateEpayCallbackOrder(gateway, verifyInfo, order.TradeNo, order.PaymentMethod, order.Money, order.EpayGatewayID); err != nil {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription callback does not match the order trade_no=%s callback_type=%s money=%s client_ip=%s error=%q", verifyInfo.ServiceTradeNo, verifyInfo.Type, verifyInfo.Money, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)

	if err := model.CompleteSubscriptionOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Epay subscription order processing failed trade_no=%s client_ip=%s error=%q", verifyInfo.ServiceTradeNo, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	logger.LogInfo(c.Request.Context(), common.LogText("Epay subscription order processed trade_no=%s callback_type=%s client_ip=%s", verifyInfo.ServiceTradeNo, verifyInfo.Type, c.ClientIP()))
	_, _ = c.Writer.Write([]byte("success"))
}

// SubscriptionEpayReturn handles browser return after payment.
// It verifies the payload and completes the order, then redirects to console.
func SubscriptionEpayReturn(c *gin.Context) {
	var params map[string]string

	if c.Request.Method == "POST" {
		// POST 请求：从 POST body 解析参数
		if err := c.Request.ParseForm(); err != nil {
			c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET 请求：从 URL Query 解析参数
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription webhook params are empty client_ip=%s", c.ClientIP()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
		return
	}

	gateway := epayGatewayForTradeNo(params["out_trade_no"])
	if gateway == nil {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription webhook order or gateway not found trade_no=%s client_ip=%s", params["out_trade_no"], c.ClientIP()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
		return
	}
	client := getEpayClientForGateway(gateway.ID)
	if client == nil {
		logger.LogError(c.Request.Context(), common.LogText("Epay client is not initialized path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
		return
	}
	verifyInfo, err := verifyEpayCallback(client, gateway, params)
	if err != nil {
		logger.LogWarn(c.Request.Context(), common.LogText("Epay webhook signature verification failed path=%q client_ip=%s verify_error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
		return
	}
	if verifyInfo.TradeStatus == epay.StatusTradeSuccess {
		order := model.GetSubscriptionOrderByTradeNo(verifyInfo.ServiceTradeNo)
		if order == nil || order.PaymentProvider != model.PaymentProviderEpay {
			logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription webhook order or gateway not found trade_no=%s client_ip=%s", verifyInfo.ServiceTradeNo, c.ClientIP()))
			c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
			return
		}
		if err := validateEpayCallbackOrder(gateway, verifyInfo, order.TradeNo, order.PaymentMethod, order.Money, order.EpayGatewayID); err != nil {
			logger.LogWarn(c.Request.Context(), common.LogText("Epay subscription callback does not match the order trade_no=%s callback_type=%s money=%s client_ip=%s error=%q", verifyInfo.ServiceTradeNo, verifyInfo.Type, verifyInfo.Money, c.ClientIP(), err.Error()))
			c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
			return
		}
		LockOrder(verifyInfo.ServiceTradeNo)
		defer UnlockOrder(verifyInfo.ServiceTradeNo)
		if err := model.CompleteSubscriptionOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type); err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Epay subscription order processing failed trade_no=%s client_ip=%s error=%q", verifyInfo.ServiceTradeNo, c.ClientIP(), err.Error()))
			c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=fail"))
			return
		}
		logger.LogInfo(c.Request.Context(), common.LogText("Epay subscription order processed trade_no=%s callback_type=%s client_ip=%s", verifyInfo.ServiceTradeNo, verifyInfo.Type, c.ClientIP()))
		c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=success"))
		return
	}
	c.Redirect(http.StatusFound, paymentReturnPath("/wallet?pay=pending"))
}
