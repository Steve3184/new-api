package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type SubscriptionNowPaymentsPayRequest struct {
	PlanID      int    `json:"plan_id"`
	PayCurrency string `json:"pay_currency"`
}

func SubscriptionRequestNowPaymentsPay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var request SubscriptionNowPaymentsPayRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.PlanID <= 0 {
		common.ApiErrorMsg(c, "invalid NOWPayments subscription request")
		return
	}
	invoice, err := service.CreateNowPaymentsSubscription(c.Request.Context(), c.GetInt("id"), request.PlanID, request.PayCurrency)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, invoice)
}
