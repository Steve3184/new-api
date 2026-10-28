package controller

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ResumeTopUpPayment returns what the owner needs to continue paying a
// pending order: a re-signed Epay form, the stored crypto invoice, or the
// stored hosted checkout link of a redirect gateway.
func ResumeTopUpPayment(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Query("trade_no"))
	if tradeNo == "" {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	topUp, err := model.GetUserTopUpByTradeNo(c.GetInt("id"), tradeNo)
	if err != nil || topUp.Status != common.TopUpStatusPending {
		common.ApiErrorT(c, "Order is not pending payment")
		return
	}
	now := common.GetTimestamp()
	switch topUp.PaymentProvider {
	case model.PaymentProviderEpay:
		if topUp.CreateTime+model.TopUpPayURLLifetime <= now {
			common.ApiErrorT(c, "This payment has expired")
			return
		}
		gatewayID := topUp.EpayGatewayID
		if gatewayID == "" {
			gatewayID = "default"
		}
		gateway := operation_setting.GetEpayGateway(gatewayID)
		if gateway == nil {
			common.ApiErrorT(c, "Payment information has not been configured by administrator")
			return
		}
		client := getEpayClientForGateway(gateway.ID)
		if client == nil {
			common.ApiErrorT(c, "Payment information has not been configured by administrator")
			return
		}
		returnPath, name := "/usage-logs", fmt.Sprintf("TUC%d", topUp.Amount)
		if topUp.IsRedemptionPurchase() {
			returnPath, name = "/wallet", fmt.Sprintf("RC%dX%d", topUp.RedemptionAmount, topUp.RedemptionCount)
		}
		notifyURL, _ := url.Parse(service.GetCallbackAddress() + "/api/user/epay/notify")
		returnURL, _ := url.Parse(paymentReturnPath(returnPath))
		uri, params, err := client.Purchase(&epay.PurchaseArgs{
			Type:           topUp.PaymentMethod,
			ServiceTradeNo: topUp.TradeNo,
			Name:           name,
			Money:          strconv.FormatFloat(topUp.Money, 'f', 2, 64),
			Device:         epay.PC,
			NotifyUrl:      notifyURL,
			ReturnUrl:      returnURL,
		})
		if err != nil {
			common.ApiErrorT(c, "Failed to start payment")
			return
		}
		common.ApiSuccess(c, gin.H{"type": "form", "url": uri, "params": params})
	case model.PaymentProviderNowPayments:
		payment, err := model.GetNowPaymentsPaymentByTopUpID(topUp.Id)
		if err != nil || payment.Status != model.NowPaymentsPaymentStatusPending || payment.ExpiresAt <= now {
			common.ApiErrorT(c, "This payment has expired")
			return
		}
		common.ApiSuccess(c, gin.H{"type": "nowpayments", "invoice": service.NowPaymentsInvoice{
			PaymentID:     payment.PaymentID,
			OrderID:       payment.OrderID,
			PayAddress:    payment.PayAddress,
			PayinExtraID:  payment.PayinExtraID,
			PayAmount:     payment.PayAmount,
			PayCurrency:   payment.PayCurrency,
			PriceAmount:   payment.PriceAmount,
			PriceCurrency: payment.PriceCurrency,
			Status:        payment.GatewayStatus,
			ExpiresAt:     payment.ExpiresAt,
		}})
	case model.PaymentProviderMonero:
		payment, err := model.GetMoneroPaymentByTopUpID(topUp.Id)
		if err != nil || payment.Status != model.MoneroPaymentStatusPending || payment.ExpiresAt <= now {
			common.ApiErrorT(c, "This payment has expired")
			return
		}
		expected, err := decimal.NewFromString(payment.ExpectedAtomic)
		if err != nil {
			common.ApiErrorT(c, "Failed to start payment")
			return
		}
		common.ApiSuccess(c, gin.H{"type": "monero", "invoice": service.MoneroInvoice{
			Address:       payment.Address,
			QuotaAmount:   topUp.Amount,
			AmountXMR:     expected.Shift(-12).StringFixed(12),
			AmountAtomic:  payment.ExpectedAtomic,
			QuoteUSD:      payment.QuoteUSD,
			USDPerXMR:     payment.USDPerXMR,
			Network:       payment.Network,
			Confirmations: setting.MoneroConfirmations,
			ExpiresAt:     payment.ExpiresAt,
		}})
	default:
		if topUp.PayURL == "" || topUp.PayURLExpiresAt <= now {
			common.ApiErrorT(c, "This payment has expired")
			return
		}
		common.ApiSuccess(c, gin.H{"type": "redirect", "url": topUp.PayURL})
	}
}
