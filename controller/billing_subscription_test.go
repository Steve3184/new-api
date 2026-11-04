package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	billingOwnerQuota     = 900_000
	billingOwnerUsedQuota = 300_000
	billingTokenQuota     = 700_000
	billingTokenUsedQuota = 100_000
)

// setupBillingTestContext creates one owner account with an unlimited key and a
// limited key, so both billing endpoints can be compared against the same
// balances.
func setupBillingTestContext(t *testing.T) (userId int, unlimitedTokenId int, limitedTokenId int) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))

	user := &model.User{
		Id:        41,
		Username:  "billing-owner",
		Quota:     billingOwnerQuota,
		UsedQuota: billingOwnerUsedQuota,
		Status:    common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             411,
		UserId:         user.Id,
		Key:            "sk-billing-unlimited",
		Name:           "unlimited",
		UnlimitedQuota: true,
		Status:         common.TokenStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          412,
		UserId:      user.Id,
		Key:         "sk-billing-limited",
		Name:        "limited",
		RemainQuota: billingTokenQuota,
		UsedQuota:   billingTokenUsedQuota,
		Status:      common.TokenStatusEnabled,
	}).Error)

	previousDisplayTokens := common.DisplayTokenStatEnabled
	previousDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	common.DisplayTokenStatEnabled = true
	// Tokens keep raw quota values, which makes the expected amounts exact.
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	t.Cleanup(func() {
		common.DisplayTokenStatEnabled = previousDisplayTokens
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousDisplayType
	})

	return user.Id, 411, 412
}

func billingRecorder(t *testing.T, userId int, tokenId int, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", userId)
	ctx.Set("token_id", tokenId)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/billing/subscription", nil)
	handler(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder
}

func callSubscription(t *testing.T, userId int, tokenId int) OpenAISubscriptionResponse {
	t.Helper()
	recorder := billingRecorder(t, userId, tokenId, GetSubscription)
	var subscription OpenAISubscriptionResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &subscription))
	return subscription
}

func callUsage(t *testing.T, userId int, tokenId int) OpenAIUsageResponse {
	t.Helper()
	recorder := billingRecorder(t, userId, tokenId, GetUsage)
	var usage OpenAIUsageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &usage))
	return usage
}

func TestUnlimitedTokenReportsOwnerBalance(t *testing.T) {
	userId, unlimitedTokenId, _ := setupBillingTestContext(t)

	subscription := callSubscription(t, userId, unlimitedTokenId)
	usage := callUsage(t, userId, unlimitedTokenId)

	require.Equal(t, float64(billingOwnerQuota+billingOwnerUsedQuota), subscription.HardLimitUSD)
	require.Equal(t, float64(billingOwnerUsedQuota)*100, usage.TotalUsage)
	// A client subtracting usage from the limit must land on the real balance,
	// not on a placeholder budget.
	require.Equal(t, float64(billingOwnerQuota), subscription.HardLimitUSD-usage.TotalUsage/100)
}

func TestLimitedTokenKeepsItsOwnBalance(t *testing.T) {
	userId, _, limitedTokenId := setupBillingTestContext(t)

	subscription := callSubscription(t, userId, limitedTokenId)
	usage := callUsage(t, userId, limitedTokenId)

	require.Equal(t, float64(billingTokenQuota+billingTokenUsedQuota), subscription.HardLimitUSD)
	require.Equal(t, float64(billingTokenUsedQuota)*100, usage.TotalUsage)
}

func TestSubscriptionFallsBackToUserBalanceWithoutTokenStats(t *testing.T) {
	userId, _, _ := setupBillingTestContext(t)
	common.DisplayTokenStatEnabled = false

	subscription := callSubscription(t, userId, 0)
	usage := callUsage(t, userId, 0)

	require.Equal(t, float64(billingOwnerQuota+billingOwnerUsedQuota), subscription.HardLimitUSD)
	require.Equal(t, float64(billingOwnerUsedQuota)*100, usage.TotalUsage)
}
