package model

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

type groupAccessUser struct {
	user        *User
	providers   map[string]struct{}
	customSlugs map[int]string
}

func loadGroupAccessUser(db *gorm.DB, userID int) (*groupAccessUser, error) {
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	result := &groupAccessUser{
		user:        &user,
		providers:   make(map[string]struct{}),
		customSlugs: make(map[int]string),
	}
	if user.GitHubId != "" {
		result.providers["github"] = struct{}{}
	}
	if user.DiscordId != "" {
		result.providers["discord"] = struct{}{}
	}
	if user.OidcId != "" {
		result.providers["oidc"] = struct{}{}
	}
	if user.LinuxDOId != "" {
		result.providers["linuxdo"] = struct{}{}
	}
	if user.TelegramId != "" {
		result.providers["telegram"] = struct{}{}
	}
	if user.WeChatId != "" {
		result.providers["wechat"] = struct{}{}
	}
	var bindings []UserOAuthBinding
	if err := db.Where("user_id = ?", userID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	var customProviders []CustomOAuthProvider
	if len(bindings) > 0 {
		if err := db.Find(&customProviders).Error; err != nil {
			return nil, err
		}
	}
	for _, provider := range customProviders {
		result.customSlugs[provider.Id] = strings.ToLower(strings.TrimSpace(provider.Slug))
	}
	for _, binding := range bindings {
		if slug := result.customSlugs[binding.ProviderId]; slug != "" {
			result.providers[slug] = struct{}{}
		}
	}
	return result, nil
}

func (u *groupAccessUser) hasOAuth(provider string) bool {
	_, ok := u.providers[strings.ToLower(strings.TrimSpace(provider))]
	return ok
}

func (u *groupAccessUser) evaluateCondition(condition console_setting.GroupAccessCondition) bool {
	switch condition.Type {
	case "oauth":
		for _, provider := range condition.Providers {
			if u.hasOAuth(provider) {
				return true
			}
		}
		return false
	case "github_registration_days", "github_days":
		if !u.hasOAuth("github") || u.user.GitHubCreatedAt <= 0 {
			return false
		}
		age := time.Since(time.Unix(u.user.GitHubCreatedAt, 0))
		return age > time.Duration(condition.Days)*24*time.Hour
	case "balance", "min_quota":
		return u.user.Quota >= condition.MinQuota
	case "spend", "min_spend", "user_spend":
		return u.user.UsedQuota >= condition.MinSpend
	default:
		return false
	}
}

func (u *groupAccessUser) evaluateRule(rule console_setting.GroupAccessRule) bool {
	values := make([]bool, 0, len(rule.Conditions)+len(rule.Rules))
	for _, condition := range rule.Conditions {
		values = append(values, u.evaluateCondition(condition))
	}
	for _, child := range rule.Rules {
		values = append(values, u.evaluateRule(child))
	}
	if len(values) == 0 {
		return false
	}
	if strings.EqualFold(rule.Logic, "or") {
		for _, value := range values {
			if value {
				return true
			}
		}
		return false
	}
	for _, value := range values {
		if !value {
			return false
		}
	}
	return true
}

func OrdinaryUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			// Apply plain entries, explicit additions and removals in that
			// order, independent of map iteration.
			for specialGroup, desc := range specialSettings {
				if !strings.HasPrefix(specialGroup, "-:") && !strings.HasPrefix(specialGroup, "+:") {
					groupsCopy[specialGroup] = desc
				}
			}
			for specialGroup, desc := range specialSettings {
				if group, ok := strings.CutPrefix(specialGroup, "+:"); ok {
					groupsCopy[group] = desc
				}
			}
			for specialGroup := range specialSettings {
				if group, ok := strings.CutPrefix(specialGroup, "-:"); ok {
					delete(groupsCopy, group)
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup。
		// 描述是英文源文本，由 web 控制台翻译；管理员配置的描述原样显示。
		if _, ok := groupsCopy[userGroup]; !ok {
			description := console_setting.GetModelSquareVisibleGroupDescriptions()[userGroup]
			if strings.TrimSpace(description) == "" {
				description = "User group"
			}
			groupsCopy[userGroup] = description
		}
	}
	return groupsCopy
}

// NewGroupRuleCheck snapshots the rules and fetches each affected user once.
// A missing user/provider never satisfies a configured rule.
func NewGroupRuleCheck(db *gorm.DB) func(int, string) (bool, error) {
	rules := make(map[string]console_setting.GroupAccessRule)
	for _, rule := range console_setting.GetGroupAccessRules() {
		if rule.Group != "" {
			rules[rule.Group] = rule
		}
	}
	type loadedUser struct {
		access *groupAccessUser
		err    error
	}
	users := make(map[int]loadedUser)
	return func(userID int, group string) (bool, error) {
		rule, ok := rules[group]
		if !ok {
			return true, nil
		}
		entry, cached := users[userID]
		if !cached {
			entry.access, entry.err = loadGroupAccessUser(db, userID)
			users[userID] = entry
		}
		if entry.err != nil {
			return false, entry.err
		}
		return entry.access.evaluateRule(rule), nil
	}
}

func NewOrdinaryGroupAccessCheck(db *gorm.DB) func(int, string, string) (bool, error) {
	checkRule := NewGroupRuleCheck(db)
	ratios := ratio_setting.GetGroupRatioCopy()
	byUserGroup := make(map[string]map[string]string)
	return func(userID int, userGroup, group string) (bool, error) {
		if group == "" || group == "auto" {
			return false, nil
		}
		if _, exists := ratios[group]; !exists {
			return false, nil
		}
		groups, cached := byUserGroup[userGroup]
		if !cached {
			groups = OrdinaryUserUsableGroups(userGroup)
			byUserGroup[userGroup] = groups
		}
		if _, allowed := groups[group]; !allowed {
			return false, nil
		}
		return checkRule(userID, group)
	}
}
