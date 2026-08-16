package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareSubscriptionUsageLimitTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	require.NoError(t, DB.Exec("DELETE FROM subscription_pre_consume_records").Error)
}

func TestSubscriptionPlanNormalizeDefaultsClearsBenefitsOnlyUsageLimits(t *testing.T) {
	plan := SubscriptionPlan{
		TotalAmount:   -42,
		FiveHourLimit: 100,
		WeeklyLimit:   200,
		MonthlyLimit:  300,
	}

	plan.NormalizeDefaults()

	require.EqualValues(t, -1, plan.TotalAmount)
	require.Zero(t, plan.FiveHourLimit)
	require.Zero(t, plan.WeeklyLimit)
	require.Zero(t, plan.MonthlyLimit)
}

func createSubscriptionUsageLimitFixture(t *testing.T, plan *SubscriptionPlan) (*User, *UserSubscription) {
	t.Helper()
	user := &User{
		Username: "subscription-usage-limit-" + common.GetRandomString(8),
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, DB.Create(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	var subscription *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		subscription, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "test")
		return err
	}))
	return user, subscription
}

func TestSubscriptionUsageLimitsAreEnforcedAtomically(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{
		Title:         "Usage caps",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		FiveHourLimit: 100,
		WeeklyLimit:   150,
		MonthlyLimit:  200,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)

	_, err := PreConsumeUserSubscription("subscription-usage-first", user.Id, "test", 0, 60)
	require.NoError(t, err)

	var afterFirst UserSubscription
	require.NoError(t, DB.First(&afterFirst, subscription.Id).Error)
	assert.EqualValues(t, 60, afterFirst.AmountUsed)
	assert.EqualValues(t, 60, afterFirst.FiveHourUsed)
	assert.EqualValues(t, 60, afterFirst.WeeklyUsed)
	assert.EqualValues(t, 60, afterFirst.MonthlyUsed)

	_, err = PreConsumeUserSubscription("subscription-usage-over-five-hours", user.Id, "test", 0, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription quota insufficient")

	var afterRejected UserSubscription
	require.NoError(t, DB.First(&afterRejected, subscription.Id).Error)
	assert.EqualValues(t, 60, afterRejected.AmountUsed)
	assert.EqualValues(t, 60, afterRejected.FiveHourUsed)
	assert.EqualValues(t, 60, afterRejected.WeeklyUsed)
	assert.EqualValues(t, 60, afterRejected.MonthlyUsed)

	require.NoError(t, PostConsumeUserSubscriptionDelta(subscription.Id, 40))
	assert.Error(t, PostConsumeUserSubscriptionDelta(subscription.Id, 1))

	var afterSettlement UserSubscription
	require.NoError(t, DB.First(&afterSettlement, subscription.Id).Error)
	assert.EqualValues(t, 100, afterSettlement.AmountUsed)
	assert.EqualValues(t, 100, afterSettlement.FiveHourUsed)
	assert.EqualValues(t, 100, afterSettlement.WeeklyUsed)
	assert.EqualValues(t, 100, afterSettlement.MonthlyUsed)
}

func TestSubscriptionUsageLimitPeriodsAreIndependentlyEnforced(t *testing.T) {
	tests := []struct {
		name string
		plan SubscriptionPlan
		used func(UserSubscription) int64
	}{
		{
			name: "five-hour",
			plan: SubscriptionPlan{FiveHourLimit: 10},
			used: func(sub UserSubscription) int64 { return sub.FiveHourUsed },
		},
		{
			name: "weekly",
			plan: SubscriptionPlan{WeeklyLimit: 10},
			used: func(sub UserSubscription) int64 { return sub.WeeklyUsed },
		},
		{
			name: "monthly",
			plan: SubscriptionPlan{MonthlyLimit: 10},
			used: func(sub UserSubscription) int64 { return sub.MonthlyUsed },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepareSubscriptionUsageLimitTest(t)
			tt.plan.Title = "Independent " + tt.name
			tt.plan.DurationUnit = SubscriptionDurationMonth
			tt.plan.DurationValue = 1
			user, subscription := createSubscriptionUsageLimitFixture(t, &tt.plan)

			_, err := PreConsumeUserSubscription("subscription-usage-"+tt.name+"-first", user.Id, "test", 0, 10)
			require.NoError(t, err)
			_, err = PreConsumeUserSubscription("subscription-usage-"+tt.name+"-over", user.Id, "test", 0, 1)
			require.Error(t, err)

			var reloaded UserSubscription
			require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
			assert.EqualValues(t, 10, tt.used(reloaded))
		})
	}
}

func TestSubscriptionUsageLimitsResetDuringPreConsume(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{
		Title:         "Resettable usage caps",
		DurationUnit:  SubscriptionDurationYear,
		DurationValue: 1,
		FiveHourLimit: 10,
		WeeklyLimit:   10,
		MonthlyLimit:  10,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	now := GetDBTimestamp()
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Updates(map[string]interface{}{
		"five_hour_used":            7,
		"five_hour_last_reset_time": now - int64((5 * time.Hour).Seconds()),
		"five_hour_next_reset_time": now - 1,
		"weekly_used":               7,
		"weekly_last_reset_time":    now - 8*24*3600,
		"weekly_next_reset_time":    now - 1,
		"monthly_used":              7,
		"monthly_last_reset_time":   now - 32*24*3600,
		"monthly_next_reset_time":   now - 1,
	}).Error)

	_, err := PreConsumeUserSubscription("subscription-usage-reset", user.Id, "test", 0, 1)
	require.NoError(t, err)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
	assert.EqualValues(t, 1, reloaded.FiveHourUsed)
	assert.EqualValues(t, 1, reloaded.WeeklyUsed)
	assert.EqualValues(t, 1, reloaded.MonthlyUsed)
	assert.Greater(t, reloaded.FiveHourNextResetTime, now)
	assert.Greater(t, reloaded.WeeklyNextResetTime, now)
	assert.Greater(t, reloaded.MonthlyNextResetTime, now)
}

func TestSubscriptionUsageLimitKeepsTerminalWindowUsage(t *testing.T) {
	now := int64(1_700_000_000)
	used := int64(7)
	lastResetTime := now - 60
	nextResetTime := int64(0)

	changed := maybeResetSubscriptionUsageLimit(
		&used,
		&lastResetTime,
		&nextResetTime,
		10,
		SubscriptionResetFiveHour,
		now-60,
		now+60,
		now,
	)

	assert.False(t, changed)
	assert.EqualValues(t, 7, used)
	assert.Equal(t, now-60, lastResetTime)
	assert.Zero(t, nextResetTime)
}

func TestResetDueSubscriptionsResetsIndependentUsageLimits(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Title:         "Scheduled usage caps",
		DurationUnit:  SubscriptionDurationYear,
		DurationValue: 1,
		FiveHourLimit: 10,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Updates(map[string]interface{}{
		"five_hour_used":            7,
		"five_hour_last_reset_time": now - int64((5 * time.Hour).Seconds()),
		"five_hour_next_reset_time": now - 1,
	}).Error)

	resetCount, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, resetCount)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
	assert.Equal(t, user.Id, reloaded.UserId)
	assert.Zero(t, reloaded.FiveHourUsed)
	assert.Greater(t, reloaded.FiveHourNextResetTime, now)
}
