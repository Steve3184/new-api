package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type smtpTestEmailRequest struct {
	Email string `json:"email"`
}

func SendSMTPTestEmail(c *gin.Context) {
	var request smtpTestEmailRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	email := model.NormalizeEmail(request.Email)
	if err := common.Validate.Var(email, "required,email"); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	if err := common.SendSMTPTestEmail(email); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
