package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacySubscriptionRenewalUsesRemainingOrdinaryGroups(t *testing.T) {
	for _, tc := range []struct {
		name, upgrade, excluded string
		amount                  int64
		canRenew                bool
	}{
		{"blacklist_leaves_no_funding_group", "", "default", 100, false},
		{"scoped_identity_does_not_block_ordinary_funding", "vip", "", 100, true},
		{"wallet_only_upgrade_loses_its_only_benefit", "vip", "default,vip", 100, false},
		{"benefits_only_upgrade_still_opens_an_ordinary_group", "vip", "", -1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			oldRatios := ratio_setting.GroupRatio2JSONString()
			oldUsable := setting.UserUsableGroups2JSONString()
			t.Cleanup(func() {
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
			})
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"special":1}`))
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Ordinary","vip":"VIP"}`))
			if tc.amount < 0 {
				groupSettings := ratio_setting.GetGroupRatioSetting()
				oldSpecial := groupSettings.GroupSpecialUsableGroup
				t.Cleanup(func() { groupSettings.GroupSpecialUsableGroup = oldSpecial })
				groupSettings.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
				groupSettings.GroupSpecialUsableGroup.AddAll(map[string]map[string]string{"vip": {"+:special": "Special"}})
			}
			group := "default"
			if tc.upgrade != "" {
				group = tc.upgrade
			}
			initialQuota := 10 * int(common.QuotaPerUnit)
			u := User{Username: "renew-ordinary", AffCode: "renew-ordinary", Group: group, Quota: initialQuota}
			require.NoError(t, DB.Create(&u).Error)
			plan := SubscriptionPlan{Title: "Legacy", Enabled: true, AllowAutoRenew: true, PriceAmount: 1, TotalAmount: tc.amount, UpgradeGroup: tc.upgrade, WalletOnlyGroupsEnabled: tc.excluded != "", WalletOnlyGroupsMode: "blacklist", WalletOnlyGroups: tc.excluded}
			require.NoError(t, DB.Create(&plan).Error)
			require.NoError(t, CreateSubscriptionPlanWithBinding(&SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "vip"}))
			now := GetDBTimestamp()
			sub := UserSubscription{UserId: u.Id, PlanId: plan.Id, StartTime: now - 60, EndTime: now - 1, Status: "active", AutoRenew: true, AmountTotal: tc.amount, UpgradeGroup: tc.upgrade, PrevUserGroup: "default"}
			require.NoError(t, DB.Create(&sub).Error)
			result, err := renewUserSubscription(u.Id, sub.Id, now)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NoError(t, DB.First(&sub, sub.Id).Error)
			require.NoError(t, DB.First(&u, u.Id).Error)
			var orders int64
			require.NoError(t, DB.Model(&SubscriptionOrder{}).Where("user_id = ?", u.Id).Count(&orders).Error)
			if tc.canRenew {
				assert.Greater(t, sub.EndTime, now)
				assert.Equal(t, initialQuota-int(common.QuotaPerUnit), u.Quota)
				assert.EqualValues(t, 1, orders)
				assert.Empty(t, result.stopReason)
			} else {
				assert.Equal(t, now-1, sub.EndTime)
				assert.Equal(t, initialQuota, u.Quota)
				assert.Zero(t, orders)
				assert.Equal(t, "unsupported", result.stopReason)
			}
		})
	}
}
