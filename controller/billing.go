package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// ownerBalance reads the remaining and used quota of a user. An unlimited key
// has no balance of its own: it spends from the account that owns it, so both
// billing endpoints report that account instead of a fixed placeholder.
func ownerBalance(userId int) (remainQuota int, usedQuota int, err error) {
	remainQuota, err = model.GetUserQuota(userId, false)
	if err != nil {
		return 0, 0, err
	}
	usedQuota, err = model.GetUserUsedQuota(userId)
	if err != nil {
		return 0, 0, err
	}
	return remainQuota, usedQuota, nil
}

// quotaDisplayAmount converts a quota value to the site's display unit. The
// OpenAI compatible *_USD fields keep those semantics: USD divides by
// QuotaPerUnit, CNY converts and applies the exchange rate, TOKENS keeps the
// raw token count.
func quotaDisplayAmount(quota int) float64 {
	amount := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		return amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		return amount
	default:
		return amount / common.QuotaPerUnit
	}
}

func writeOpenAIError(c *gin.Context, errorType string, err error) {
	c.JSON(200, gin.H{
		"error": types.OpenAIError{
			Message: err.Error(),
			Type:    errorType,
		},
	})
}

func GetSubscription(c *gin.Context) {
	var remainQuota int
	var usedQuota int
	var err error
	var expiredTime int64
	userId := c.GetInt("id")
	if common.DisplayTokenStatEnabled {
		token, tokenErr := model.GetTokenById(c.GetInt("token_id"))
		if tokenErr != nil {
			writeOpenAIError(c, "upstream_error", tokenErr)
			return
		}
		expiredTime = token.ExpiredTime
		remainQuota = token.RemainQuota
		usedQuota = token.UsedQuota
		if token.UnlimitedQuota {
			remainQuota, usedQuota, err = ownerBalance(userId)
		}
	} else {
		remainQuota, usedQuota, err = ownerBalance(userId)
	}
	if err != nil {
		writeOpenAIError(c, "upstream_error", err)
		return
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	amount := quotaDisplayAmount(remainQuota + usedQuota)
	subscription := OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
	return
}

func GetUsage(c *gin.Context) {
	var quota int
	var err error
	if common.DisplayTokenStatEnabled {
		token, tokenErr := model.GetTokenById(c.GetInt("token_id"))
		if tokenErr != nil {
			writeOpenAIError(c, "new_api_error", tokenErr)
			return
		}
		quota = token.UsedQuota
		if token.UnlimitedQuota {
			_, quota, err = ownerBalance(c.GetInt("id"))
		}
	} else {
		quota, err = model.GetUserUsedQuota(c.GetInt("id"))
	}
	if err != nil {
		writeOpenAIError(c, "new_api_error", err)
		return
	}
	usage := OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: quotaDisplayAmount(quota) * 100,
	}
	c.JSON(200, usage)
	return
}
