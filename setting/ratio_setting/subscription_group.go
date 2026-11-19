package ratio_setting

import (
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const SubscriptionGroupsOptionKey = "SubscriptionGroups"

type SubscriptionGroupConfig struct {
	AutoCreateKey bool `json:"auto_create_key"`
}

var subscriptionGroups atomic.Pointer[map[string]SubscriptionGroupConfig]

func ParseSubscriptionGroups(value string) (map[string]SubscriptionGroupConfig, error) {
	var groups map[string]SubscriptionGroupConfig
	if err := common.UnmarshalJsonStr(value, &groups); err != nil {
		return nil, err
	}
	if groups == nil {
		return nil, common.NewMessage("Subscription groups must be a JSON object")
	}
	for name := range groups {
		if name == "" || name == "auto" || name != strings.TrimSpace(name) {
			return nil, common.NewMessage("Invalid subscription group: {{group}}", map[string]any{"group": name})
		}
	}
	return groups, nil
}

func UpdateSubscriptionGroups(value string) error {
	groups, err := ParseSubscriptionGroups(value)
	if err != nil {
		return err
	}
	subscriptionGroups.Store(&groups)
	return nil
}

func GetSubscriptionGroup(name string) (SubscriptionGroupConfig, bool) {
	if groups := subscriptionGroups.Load(); groups != nil {
		config, ok := (*groups)[name]
		return config, ok
	}
	return SubscriptionGroupConfig{}, false
}

func SubscriptionGroupNames() []string {
	var names []string
	if groups := subscriptionGroups.Load(); groups != nil {
		for name := range *groups {
			if ContainsGroupRatio(name) {
				names = append(names, name)
			}
		}
	}
	return names
}
