package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBillingSessionUsesEffectiveRequestGroupForSubscriptionWhitelist(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username: "subscription-whitelist",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    100,
	}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:                   "VIP subscription",
		DurationUnit:            model.SubscriptionDurationMonth,
		DurationValue:           1,
		TotalAmount:             100,
		WalletOnlyGroupsEnabled: true,
		WalletOnlyGroupsMode:    "whitelist",
		WalletOnlyGroups:        "vip",
	}
	require.NoError(t, model.DB.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	subscription := &model.UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		Status:      "active",
		StartTime:   time.Now().Add(-time.Hour).Unix(),
		EndTime:     time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(subscription).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		RequestId:       "subscription-whitelist-request",
		UserId:          user.Id,
		UserGroup:       user.Group,
		UsingGroup:      "vip",
		OriginModelName: "test-model",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceSubscription, info.BillingSource)
	assert.Equal(t, subscription.Id, info.SubscriptionId)

	var reloadedUser model.User
	require.NoError(t, model.DB.Select("quota").First(&reloadedUser, user.Id).Error)
	assert.Equal(t, 100, reloadedUser.Quota)
	var reloadedSubscription model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").First(&reloadedSubscription, subscription.Id).Error)
	assert.EqualValues(t, 10, reloadedSubscription.AmountUsed)
}
