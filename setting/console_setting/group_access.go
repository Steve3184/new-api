package console_setting

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	maxGroupAccessDays  = 36500 // 100 years; also keeps duration arithmetic bounded.
	maxGroupAccessQuota = math.MaxInt32
)

// GroupAccessCondition is one leaf in a group access expression.
// OAuth providers are matched by their stable slug (github, linuxdo, custom
// provider slug, and so on). Balance is expressed in quota units.
type GroupAccessCondition struct {
	Type      string   `json:"type"`
	Providers []string `json:"providers,omitempty"`
	Days      int      `json:"days,omitempty"`
	MinQuota  int      `json:"min_quota,omitempty"`
}

// GroupAccessRule supports nested boolean expressions. A rule may contain
// conditions, child rules, or both; Logic controls how those children combine.
type GroupAccessRule struct {
	Group      string                 `json:"group,omitempty"`
	Logic      string                 `json:"logic,omitempty"`
	Conditions []GroupAccessCondition `json:"conditions,omitempty"`
	Rules      []GroupAccessRule      `json:"rules,omitempty"`
}

func normalizeLogic(logic string) (string, error) {
	logic = strings.ToLower(strings.TrimSpace(logic))
	if logic == "" {
		return "and", nil
	}
	if logic != "and" && logic != "or" {
		return "", fmt.Errorf("group access logic must be and or or")
	}
	return logic, nil
}

func normalizeCondition(condition GroupAccessCondition) (GroupAccessCondition, error) {
	condition.Type = strings.ToLower(strings.TrimSpace(condition.Type))
	switch condition.Type {
	case "oauth":
		seen := make(map[string]struct{}, len(condition.Providers))
		providers := make([]string, 0, len(condition.Providers))
		for _, provider := range condition.Providers {
			provider = strings.ToLower(strings.TrimSpace(provider))
			if provider == "" {
				continue
			}
			if _, exists := seen[provider]; exists {
				continue
			}
			seen[provider] = struct{}{}
			providers = append(providers, provider)
		}
		if len(providers) == 0 {
			return GroupAccessCondition{}, errors.New("oauth condition needs at least one provider")
		}
		condition.Providers = providers
	case "github_days", "github_registration_days":
		condition.Type = "github_registration_days"
		if condition.Days < 0 || condition.Days > maxGroupAccessDays {
			return GroupAccessCondition{}, fmt.Errorf("github registration days must be between 0 and %d", maxGroupAccessDays)
		}
	case "balance", "min_quota":
		condition.Type = "balance"
		if condition.MinQuota < 0 || condition.MinQuota > maxGroupAccessQuota {
			return GroupAccessCondition{}, fmt.Errorf("minimum balance must be between 0 and %d", maxGroupAccessQuota)
		}
	default:
		return GroupAccessCondition{}, fmt.Errorf("unsupported group access condition %q", condition.Type)
	}
	return condition, nil
}

func normalizeRule(rule GroupAccessRule, validGroups map[string]float64, isRoot bool) (GroupAccessRule, bool, error) {
	logic, err := normalizeLogic(rule.Logic)
	if err != nil {
		return GroupAccessRule{}, false, err
	}
	rule.Logic = logic
	rule.Group = strings.TrimSpace(rule.Group)
	if isRoot {
		if rule.Group == "" {
			return GroupAccessRule{}, false, errors.New("group access rule needs a group")
		}
		if _, exists := validGroups[rule.Group]; !exists {
			return GroupAccessRule{}, false, nil
		}
	}

	conditions := make([]GroupAccessCondition, 0, len(rule.Conditions))
	for _, condition := range rule.Conditions {
		normalized, conditionErr := normalizeCondition(condition)
		if conditionErr != nil {
			return GroupAccessRule{}, false, conditionErr
		}
		conditions = append(conditions, normalized)
	}
	rule.Conditions = conditions

	rules := make([]GroupAccessRule, 0, len(rule.Rules))
	for _, child := range rule.Rules {
		normalized, keep, childErr := normalizeRule(child, validGroups, false)
		if childErr != nil {
			return GroupAccessRule{}, false, childErr
		}
		if keep {
			rules = append(rules, normalized)
		}
	}
	rule.Rules = rules
	return rule, len(rule.Conditions) > 0 || len(rule.Rules) > 0, nil
}

// NormalizeGroupAccessRules validates the expression and drops rules for
// groups that no longer exist in GroupRatio before the option is persisted.
func NormalizeGroupAccessRules(value string) (string, error) {
	var rules []GroupAccessRule
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return "", err
	}
	validGroups := ratio_setting.GetGroupRatioCopy()
	normalized := make([]GroupAccessRule, 0, len(rules))
	for _, rule := range rules {
		clean, keep, err := normalizeRule(rule, validGroups, true)
		if err != nil {
			return "", err
		}
		if keep {
			normalized = append(normalized, clean)
		}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func GetGroupAccessRules() []GroupAccessRule {
	var rules []GroupAccessRule
	if err := common.UnmarshalJsonStr(consoleSetting.GroupAccessRules, &rules); err != nil {
		common.SysLog("failed to parse group access rules: " + err.Error())
		return nil
	}
	return rules
}

// GetModelSquareVisibleGroups returns only configured groups that still have
// a pricing ratio. This keeps deleted groups from leaking into the public plaza.
func GetModelSquareVisibleGroups() []string {
	var groups []string
	if err := common.UnmarshalJsonStr(consoleSetting.ModelSquareVisibleGroups, &groups); err != nil {
		common.SysLog("failed to parse model square visible groups: " + err.Error())
		return nil
	}
	valid := ratio_setting.GetGroupRatioCopy()
	seen := make(map[string]struct{}, len(groups))
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, exists := valid[group]; !exists {
			continue
		}
		if _, exists := seen[group]; exists {
			continue
		}
		seen[group] = struct{}{}
		result = append(result, group)
	}
	return result
}

func NormalizeModelSquareVisibleGroups(value string) (string, error) {
	var groups []string
	if err := common.UnmarshalJsonStr(value, &groups); err != nil {
		return "", err
	}
	valid := ratio_setting.GetGroupRatioCopy()
	seen := make(map[string]struct{}, len(groups))
	filtered := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, exists := valid[group]; !exists {
			continue
		}
		if _, exists := seen[group]; exists {
			continue
		}
		seen[group] = struct{}{}
		filtered = append(filtered, group)
	}
	data, err := common.Marshal(filtered)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
