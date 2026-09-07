package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type NowPaymentsPayRequest struct {
	Amount      int64  `json:"amount"`
	PayCurrency string `json:"pay_currency"`
}

func RequestNowPaymentsPay(c *gin.Context) {
	var request NowPaymentsPayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "invalid NOWPayments payment request")
		return
	}
	invoice, err := service.CreateNowPaymentsTopUp(c.Request.Context(), c.GetInt("id"), request.Amount, request.PayCurrency)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, invoice)
}

func GetNowPaymentsPaymentStatus(c *gin.Context) {
	paymentID := strings.TrimSpace(c.Query("id"))
	if paymentID == "" {
		common.ApiErrorMsg(c, "NOWPayments payment id is required")
		return
	}
	status, err := service.GetNowPaymentsPaymentStatus(c.GetInt("id"), paymentID)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, status)
}

func NowPaymentsWebhook(c *gin.Context) {
	if !isNowPaymentsWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("NOWPayments webhook rejected reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("NOWPayments webhook body read failed client_ip=%s error=%q", c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	signature := c.GetHeader("x-nowpayments-sig")
	if !service.VerifyNowPaymentsSignature(payload, signature) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("NOWPayments webhook signature verification failed payment_id=%s client_ip=%s", service.ParseNowPaymentsWebhookPaymentID(payload), c.ClientIP()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	paymentID := service.ParseNowPaymentsWebhookPaymentID(payload)
	if err := service.HandleNowPaymentsWebhook(payload, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("NOWPayments webhook processing failed payment_id=%s client_ip=%s error=%q", paymentID, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("NOWPayments webhook processed payment_id=%s client_ip=%s", paymentID, c.ClientIP()))
	c.Status(http.StatusOK)
}
