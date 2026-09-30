package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserUsableGroupsAssignedGroupDescription(t *testing.T) {
	previousUsable := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	console := console_setting.GetConsoleSetting()
	previousVisible := console.ModelSquareVisibleGroups
	groupSettings := ratio_setting.GetGroupRatioSetting()
	previousSpecial := groupSettings.GroupSpecialUsableGroup
	groupSettings.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		console.ModelSquareVisibleGroups = previousVisible
		groupSettings.GroupSpecialUsableGroup = previousSpecial
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"assigned":1,"other":1}`))
	for _, tc := range []struct {
		name, usable, visible, special, want string
	}{
		{"assigned group description", `{"default":"Default"}`, `{"assigned":"Assigned plan\nFor existing users","other":"Public only"}`, "", "Assigned plan\nFor existing users"},
		{"blank description fallback", `{"default":"Default"}`, `{"assigned":"  "}`, "", "用户分组"},
		{"legacy visible array fallback", `{"default":"Default"}`, `["assigned","other"]`, "", "用户分组"},
		{"selectable description wins", `{"default":"Default","assigned":"Selectable plan"}`, `{"assigned":"Public plan"}`, "", "Selectable plan"},
		{"special description wins", `{"default":"Default"}`, `{"assigned":"Public plan"}`, "Special plan", "Special plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(tc.usable))
			console.ModelSquareVisibleGroups = tc.visible
			groupSettings.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
			if tc.special != "" {
				groupSettings.GroupSpecialUsableGroup.Set("assigned", map[string]string{"+:assigned": tc.special})
			}
			groups := GetUserUsableGroups("assigned")
			assert.Equal(t, tc.want, groups["assigned"])
			assert.Equal(t, "Default", groups["default"])
			assert.NotContains(t, groups, "other", "display visibility must not grant access")
		})
	}
}

func TestUserGroupAccessAllowsAutoWhenCandidatesRemain(t *testing.T) {
	access := UserGroupAccess{
		UsableGroups: map[string]string{"default": "Default"},
		AutoGroups:   []string{"default"},
	}

	assert.True(t, access.Allows("auto"))
	assert.True(t, access.Allows("default"))
	assert.False(t, access.Allows("vip"))
}

func TestUserGroupAccessRejectsAutoWithoutUsableCandidates(t *testing.T) {
	access := UserGroupAccess{
		UsableGroups: map[string]string{"default": "Default"},
	}

	assert.False(t, access.Allows("auto"))
}

func TestFilterAutoGroupsPreservesOrderAndRemovesUnavailableDuplicates(t *testing.T) {
	usableGroups := map[string]string{
		"default": "Default",
		"vip":     "VIP",
	}

	groups := filterAutoGroups(
		[]string{"vip", "private", "default", "vip"},
		usableGroups,
	)

	assert.Equal(t, []string{"vip", "default"}, groups)
}

func TestGetRequestUserGroupAccessReusesCachedResolution(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	cached := UserGroupAccess{
		UsableGroups: map[string]string{"vip": "VIP"},
		AutoGroups:   []string{"vip"},
	}
	common.SetContextKey(c, constant.ContextKeyUserGroupAccess, cached)

	actual := GetRequestUserGroupAccess(c)

	assert.Equal(t, cached, actual)
}
