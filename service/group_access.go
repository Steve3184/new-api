package service

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
)

type groupAccessUser struct {
	user        *model.User
	providers   map[string]struct{}
	customSlugs map[int]string
}

func loadGroupAccessUser(userID int) (*groupAccessUser, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	result := &groupAccessUser{
		user:        user,
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
	bindings, err := model.GetUserOAuthBindingsByUserId(userID)
	if err != nil {
		return nil, err
	}
	customProviders, err := model.GetAllCustomOAuthProviders()
	if err != nil {
		return nil, err
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
