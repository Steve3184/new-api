package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withOrdinaryGroupSettings(t *testing.T, usable, ratios, rules string, special map[string]map[string]string) {
	t.Helper()
	previousUsable := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	console := console_setting.GetConsoleSetting()
	previousRules := console.GroupAccessRules
	groupSettings := ratio_setting.GetGroupRatioSetting()
	previousSpecial := groupSettings.GroupSpecialUsableGroup
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		console.GroupAccessRules = previousRules
		groupSettings.GroupSpecialUsableGroup = previousSpecial
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(usable))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(ratios))
	console.GroupAccessRules = rules
	groupSettings.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
	groupSettings.GroupSpecialUsableGroup.AddAll(special)
}

func TestOrdinaryUsableGroupsConflictingSpecialRulesAreDeterministic(t *testing.T) {
	withOrdinaryGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"default":1,"vip":1,"x":1,"self":1}`, `[]`, map[string]map[string]string{
		"self": {"-:x": "", "+:x": "Plus", "x": "Plain", "-:self": "", "-:default": "", "+:default": "Back"},
	})
	for i := 0; i < 50; i++ {
		groups := model.OrdinaryUserUsableGroups("self")
		assert.NotContains(t, groups, "x", "removal wins over conflicting additions")
		assert.NotContains(t, groups, "default")
		assert.Contains(t, groups, "self", "a user's own group stays usable")
		assert.Equal(t, "VIP", groups["vip"])
	}
	assert.Equal(t, GetUserUsableGroups("vip"), model.OrdinaryUserUsableGroups("vip"))
}

func TestOrdinaryGroupAccessCheckUsesTransactionRatiosAndRules(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.CustomOAuthProvider{}, &model.UserOAuthBinding{}))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM user_oauth_bindings")
		model.DB.Exec("DELETE FROM custom_oauth_providers")
	})
	withOrdinaryGroupSettings(t, `{"default":"Default","vip":"VIP","noratio":"N","gated":"G"}`, `{"default":1,"vip":1,"gated":1}`,
		`[{"group":"gated","conditions":[{"type":"oauth","providers":["acme"]}]},{"group":"default","conditions":[{"type":"balance","min_quota":1000000}]}]`, nil)

	user := &model.User{Username: "ga", Password: "x", Group: "default", Quota: 10}
	require.NoError(t, model.DB.Create(user).Error)
	tx := model.DB.Begin()
	defer tx.Rollback()
	check := model.NewOrdinaryGroupAccessCheck(tx)

	ok, err := check(user.Id, user.Group, "vip")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = check(user.Id, user.Group, "noratio")
	require.NoError(t, err)
	assert.False(t, ok, "missing group ratio denies")
	ok, err = check(user.Id, user.Group, "unlisted")
	require.NoError(t, err)
	assert.False(t, ok)
	ok, _ = check(user.Id, user.Group, "auto")
	assert.False(t, ok)
	ok, err = check(user.Id, user.Group, "default")
	require.NoError(t, err)
	assert.False(t, ok, "access rules also apply to the user's own group")
	ok, err = check(user.Id, user.Group, "gated")
	require.NoError(t, err)
	assert.False(t, ok)

	provider := &model.CustomOAuthProvider{Name: "Acme", Slug: " ACME ", ClientId: "c", AuthorizationEndpoint: "https://a", TokenEndpoint: "https://a", UserInfoEndpoint: "https://a"}
	require.NoError(t, tx.Create(provider).Error)
	require.NoError(t, tx.Create(&model.UserOAuthBinding{UserId: user.Id, ProviderId: provider.Id, ProviderUserId: "1"}).Error)
	check = model.NewOrdinaryGroupAccessCheck(tx)
	ok, err = check(user.Id, user.Group, "gated")
	require.NoError(t, err)
	assert.True(t, ok, "uncommitted binding is visible through the supplied transaction")

	_, err = check(user.Id+999, user.Group, "gated")
	assert.Error(t, err, "missing user is an error for rule-gated groups")
}
