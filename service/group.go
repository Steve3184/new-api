package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type UserGroupAccess struct {
	UsableGroups map[string]string
	AutoGroups   []string
}

func (access UserGroupAccess) Allows(group string) bool {
	if group == "auto" {
		return len(access.AutoGroups) > 0
	}
	_, ok := access.UsableGroups[group]
	return ok
}

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

// GetUserUsableGroupsForUser applies the optional per-group access expressions
// after the existing user-group and special-group rules have been resolved.
func GetUserUsableGroupsForUser(userID int, userGroup string) map[string]string {
	groups := GetUserUsableGroups(userGroup)
	rules := GetGroupAccessRulesByGroup()
	if len(rules) == 0 {
		return groups
	}
	accessUser, err := loadGroupAccessUser(userID)
	if err != nil {
		for group := range rules {
			delete(groups, group)
		}
		return groups
	}
	for group, rule := range rules {
		if !accessUser.evaluateRule(rule) {
			delete(groups, group)
		}
	}
	return groups
}

func GetGroupAccessRulesByGroup() map[string]console_setting.GroupAccessRule {
	rules := make(map[string]console_setting.GroupAccessRule)
	for _, rule := range console_setting.GetGroupAccessRules() {
		if rule.Group != "" {
			rules[rule.Group] = rule
		}
	}
	return rules
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func ResolveUserGroupAccess(userGroup string) UserGroupAccess {
	usableGroups := GetUserUsableGroups(userGroup)
	autoGroups := filterAutoGroups(setting.GetAutoGroups(), usableGroups)
	return UserGroupAccess{
		UsableGroups: usableGroups,
		AutoGroups:   autoGroups,
	}
}

func ResolveUserGroupAccessForUser(userID int, userGroup string) UserGroupAccess {
	usableGroups := GetUserUsableGroupsForUser(userID, userGroup)
	autoGroups := filterAutoGroups(setting.GetAutoGroups(), usableGroups)
	return UserGroupAccess{UsableGroups: usableGroups, AutoGroups: autoGroups}
}

func filterAutoGroups(configuredGroups []string, usableGroups map[string]string) []string {
	autoGroups := make([]string, 0, len(configuredGroups))
	seen := make(map[string]struct{}, len(configuredGroups))
	for _, group := range configuredGroups {
		if _, duplicate := seen[group]; duplicate {
			continue
		}
		if _, ok := usableGroups[group]; !ok {
			continue
		}
		seen[group] = struct{}{}
		autoGroups = append(autoGroups, group)
	}
	return autoGroups
}

func IsUserSelectableGroup(userGroup, groupName string) bool {
	if groupName == "" || groupName == "auto" {
		return false
	}
	return GroupInUserUsableGroups(userGroup, groupName) && ratio_setting.ContainsGroupRatio(groupName)
}

func IsUserSelectableGroupForUser(userID int, userGroup, groupName string) bool {
	if groupName == "" || groupName == "auto" {
		return false
	}
	_, ok := GetUserUsableGroupsForUser(userID, userGroup)[groupName]
	return ok && ratio_setting.ContainsGroupRatio(groupName)
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	autoGroups := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range setting.GetAutoGroups() {
		if !IsUserSelectableGroup(userGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		autoGroups = append(autoGroups, group)
	}
	return autoGroups
}

func GetUserAutoGroupForUser(userID int, userGroup string) []string {
	usableGroups := GetUserUsableGroupsForUser(userID, userGroup)
	autoGroups := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range setting.GetAutoGroups() {
		if _, ok := usableGroups[group]; !ok || !ratio_setting.ContainsGroupRatio(group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		autoGroups = append(autoGroups, group)
	}
	return autoGroups
}

func GetRequestUserGroupAccess(c *gin.Context) UserGroupAccess {
	if cached, ok := common.GetContextKey(c, constant.ContextKeyUserGroupAccess); ok {
		if access, valid := cached.(UserGroupAccess); valid {
			return access
		}
	}
	access := ResolveUserGroupAccessForUser(c.GetInt("id"), common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	common.SetContextKey(c, constant.ContextKeyUserGroupAccess, access)
	return access
}

// FilterUserTokenAutoGroups applies current permissions before the current
// per-token limit. It intentionally does not fall back to the global Auto list.
func FilterUserTokenAutoGroups(userGroup string, groups []string) []string {
	return FilterUserTokenAutoGroupsForUser(0, userGroup, groups)
}

func FilterUserTokenAutoGroupsForUser(userID int, userGroup string, groups []string) []string {
	maxCount := setting.GetMaxTokenAutoGroups()
	filtered := make([]string, 0, min(len(groups), maxCount))
	seen := make(map[string]struct{})
	for _, group := range groups {
		if !IsUserSelectableGroupForUser(userID, userGroup, group) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		filtered = append(filtered, group)
		if len(filtered) == maxCount {
			break
		}
	}
	return filtered
}

// GetRequestAutoGroups resolves the ordered Auto groups for the current token.
// The absence of the context value means that the token inherits the complete
// global Auto list; a present (even empty) value is an explicit token snapshot.
func GetRequestAutoGroups(c *gin.Context, userGroup string) []string {
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenAutoGroups)
	if !ok {
		return GetUserAutoGroupForUser(c.GetInt("id"), userGroup)
	}
	groups, ok := value.([]string)
	if !ok {
		return []string{}
	}
	return FilterUserTokenAutoGroupsForUser(c.GetInt("id"), userGroup, groups)
}

// GetRequestAutoRoute returns the token-scoped candidate chain for a virtual
// model. A missing route is intentionally distinguishable from an empty route;
// the latter is treated as invalid and never falls back to a recursive route.
func GetRequestAutoRoute(c *gin.Context, modelName string) ([]string, bool) {
	if !strings.HasPrefix(modelName, "auto/") {
		return nil, false
	}
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenAutoRoutes)
	if !ok {
		return nil, false
	}
	routes, ok := value.(map[string][]string)
	if !ok {
		return nil, false
	}
	chain, ok := routes[modelName]
	if !ok || len(chain) == 0 {
		return nil, false
	}
	return chain, true
}

// GetGroupsEnabledModels 按 groups 顺序获取各分组启用的模型并去重
func GetGroupsEnabledModels(groups []string) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if _, ok := seen[modelName]; !ok {
				seen[modelName] = struct{}{}
				models = append(models, modelName)
			}
		}
	}
	return models
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}
