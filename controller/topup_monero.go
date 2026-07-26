package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type MoneroPayRequest struct {
	Amount int64 `json:"amount"`
}

// RequestMoneroPay creates a unique wallet-RPC subaddress and freezes both the
// XMR/USD quote and USD/quota conversion for this invoice. The background
// monitor credits the account after the configured on-chain confirmations.
func RequestMoneroPay(c *gin.Context) {
	var req MoneroPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	invoice, err := service.CreateMoneroInvoice(c.Request.Context(), c.GetInt("id"), req.Amount)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, invoice)
}
