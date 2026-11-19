package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

func TestScopedSubscriptionDoesNotReplaceLegacyUserGroup(t *testing.T) {
	truncateTables(t)
	user := &User{Username: "scope-preserves-group", Password: "password", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	legacy := &SubscriptionPlan{Title: "Legacy", TotalAmount: 100, UpgradeGroup: "vip"}
	scoped := &SubscriptionPlan{Title: "Scoped", TotalAmount: 100, SubscriptionGroup: "svip", UpgradeGroup: "svip", DowngradeGroup: "default"}
	for _, plan := range []*SubscriptionPlan{legacy, scoped} {
		require.NoError(t, DB.Create(plan).Error)
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			_, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "test")
			return err
		}))
	}
	group, err := getUserGroupByIdTx(DB, user.Id)
	require.NoError(t, err)
	assert.Equal(t, "vip", group, "the scoped plan must not change the user's legacy group")
	var sub UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", user.Id, scoped.Id).First(&sub).Error)
	assert.Empty(t, sub.UpgradeGroup)
	assert.Empty(t, sub.DowngradeGroup)
}

func TestSubscriptionGroupEntitlementRequiresExactMatchUnderCaseInsensitiveCollation(t *testing.T) {
	truncateTables(t)
	// SQLite NOCASE gives this fixture the relevant MySQL-style comparison
	// behavior without changing the global dialect or the production schema.
	require.NoError(t, DB.Exec("ALTER TABLE subscription_plans RENAME COLUMN subscription_group TO original_subscription_group").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("ALTER TABLE subscription_plans RENAME COLUMN original_subscription_group TO subscription_group").Error)
	})
	require.NoError(t, DB.Exec("ALTER TABLE subscription_plans ADD COLUMN subscription_group TEXT COLLATE NOCASE NOT NULL DEFAULT ''").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("ALTER TABLE subscription_plans DROP COLUMN subscription_group").Error)
	})
	plan := &SubscriptionPlan{Title: "Case-sensitive entitlements", SubscriptionGroup: "VIP"}
	require.NoError(t, DB.Create(plan).Error)
	sub := &UserSubscription{UserId: 1143, PlanId: plan.Id, Status: "active", EndTime: common.GetTimestamp() + 1000}
	require.NoError(t, DB.Create(sub).Error)
	groups, err := GetActiveUserSubscriptionGroups(sub.UserId, []string{"vip"})
	require.NoError(t, err)
	assert.Empty(t, groups)
	groups, err = GetActiveUserSubscriptionGroups(sub.UserId, []string{"VIP", "vip"})
	require.NoError(t, err)
	assert.Equal(t, []string{"VIP"}, groups)
}

func TestBulkOptionUpdateRejectsSubscriptionGroupWithoutPlan(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	for _, values := range []map[string]string{
		{ratio_setting.SubscriptionGroupsOptionKey: `{"vip":{}}`},
		{ratio_setting.SubscriptionGroupsOptionKey: `{"vip":{}}`, "passkey.rp_id": ""},
	} {
		require.ErrorContains(t, UpdateOptionsBulk(values), "Bind at least one subscription plan")
	}
	var count int64
	require.NoError(t, DB.Model(&Option{}).Where(&Option{Key: ratio_setting.SubscriptionGroupsOptionKey}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestScopedSubscriptionCreatesOneLocalizedKeyUnlessAlreadyCovered(t *testing.T) {
	truncateTables(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, ratio_setting.UpdateSubscriptionGroups(`{"vip":{"auto_create_key":true}}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateSubscriptionGroups(`{}`)) })
	plan := &SubscriptionPlan{Title: "Pro", TotalAmount: 100, SubscriptionGroup: "vip"}
	require.NoError(t, DB.Create(plan).Error)
	for _, kind := range []string{"none", "active", "expired", "disabled"} {
		user := &User{Username: "auto-key-" + kind, AffCode: kind, Password: "password", Group: "default", Status: common.UserStatusEnabled}
		user.SetSetting(dto.UserSetting{Language: "zh-CN"})
		require.NoError(t, DB.Create(user).Error)
		if kind != "none" {
			key, err := common.GenerateKey()
			require.NoError(t, err)
			token := &Token{UserId: user.Id, Name: "Existing", Group: "auto", Key: key, UnlimitedQuota: true, Status: common.TokenStatusEnabled, ExpiredTime: -1}
			if kind == "expired" {
				token.ExpiredTime = common.GetTimestamp() - 1
			}
			if kind == "disabled" {
				token.Status = common.TokenStatusDisabled
			}
			require.NoError(t, token.SetAutoGroups([]string{"default", "vip"}))
			require.NoError(t, DB.Create(token).Error)
		}
		for range 2 {
			require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
				_, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, PaymentMethodBalance)
				return err
			}))
		}
		var keys []Token
		require.NoError(t, DB.Where("user_id = ?", user.Id).Order("id").Find(&keys).Error)
		if kind == "expired" || kind == "disabled" {
			require.Len(t, keys, 2, kind)
			keys = keys[1:]
		}
		require.Len(t, keys, 1)
		if kind == "active" {
			assert.Equal(t, "Existing", keys[0].Name)
		} else {
			assert.Equal(t, "订阅-Pro", keys[0].Name)
			assert.Equal(t, "vip", keys[0].Group)
			assert.True(t, keys[0].UnlimitedQuota)
			assert.False(t, keys[0].ModelLimitsEnabled)
			assert.EqualValues(t, -1, keys[0].ExpiredTime)
		}
	}
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
