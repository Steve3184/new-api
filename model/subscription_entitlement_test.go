package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBenefitsOnlySubscriptionDoesNotFundRequests(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	user := &User{Username: "benefits-only", Password: "password", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{Id: 9711, Title: "Benefits", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: -1}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "test")
		return err
	}))

	hasAny, hasEligible, err := GetActiveSubscriptionAvailability(user.Id)
	require.NoError(t, err)
	assert.True(t, hasAny)
	assert.False(t, hasEligible)
	_, err = PreConsumeUserSubscription("benefits-only-request", user.Id, "gpt-test", 0, 10)
	require.Error(t, err)
}

func TestActiveSubscriptionRateLimitUsesHighestMatchingRPM(t *testing.T) {
	truncateTables(t)
	user := &User{Username: "rpm-entitlement", Password: "password", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, DB.Create(user).Error)
	now := common.GetTimestamp()
	plans := []*SubscriptionPlan{
		{Id: 9721, Title: "RPM 120", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, RateLimitGroups: `[{"group":"default","rpm":120}]`},
		{Id: 9722, Title: "RPM 240", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, RateLimitGroups: `[{"group":"default","rpm":240},{"group":"vip","rpm":480}]`},
	}
	for _, plan := range plans {
		require.NoError(t, DB.Create(plan).Error)
		require.NoError(t, DB.Create(&UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: -1, Status: "active", StartTime: now - 60, EndTime: now + 3600}).Error)
	}
	InvalidateUserSubscriptionRateLimitCache(user.Id)

	rpm, found, err := GetActiveSubscriptionRateLimit(user.Id, "default")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 240, rpm)
	rpm, found, err = GetActiveSubscriptionRateLimit(user.Id, "vip")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 480, rpm)

	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Update("end_time", now-1).Error)
	InvalidateUserSubscriptionRateLimitCache(user.Id)
	_, found, err = GetActiveSubscriptionRateLimit(user.Id, "default")
	require.NoError(t, err)
	assert.False(t, found)
}
