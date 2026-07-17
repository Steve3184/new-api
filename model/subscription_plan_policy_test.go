package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionPlanWalletOnlyGroupPolicy(t *testing.T) {
	tests := []struct {
		name  string
		plan  SubscriptionPlan
		group string
		want  bool
	}{
		{name: "disabled", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: false, WalletOnlyGroups: "vip"}, group: "vip", want: false},
		{name: "blacklist match", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "blacklist", WalletOnlyGroups: "vip,team"}, group: "vip", want: true},
		{name: "blacklist miss", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "blacklist", WalletOnlyGroups: "vip,team"}, group: "default", want: false},
		{name: "whitelist match", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip,team"}, group: "team", want: false},
		{name: "whitelist miss", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip,team"}, group: "default", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.plan.IsWalletOnlyForGroup(tt.group))
		})
	}
}

func TestDeleteSubscriptionPlanRequiresNoActiveSubscriptions(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{
		Id:            9901,
		Title:         "Deletable",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
	}
	require.NoError(t, DB.Create(plan).Error)
	now := common.GetTimestamp()
	sub := &UserSubscription{
		Id:        9902,
		UserId:    9903,
		PlanId:    plan.Id,
		Status:    "active",
		StartTime: now - 60,
		EndTime:   now + 3600,
	}
	require.NoError(t, DB.Create(sub).Error)

	require.Error(t, DeleteSubscriptionPlan(plan.Id))
	require.NoError(t, DB.Model(sub).Update("end_time", now-1).Error)
	require.NoError(t, DeleteSubscriptionPlan(plan.Id))

	var deleted SubscriptionPlan
	err := DB.First(&deleted, plan.Id).Error
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
