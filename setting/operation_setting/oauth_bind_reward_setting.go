package operation_setting

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const OAuthBindRewardsOptionKey = "oauth_bind_reward.rewards"

// OAuthBindRewardBuiltinProviders lists the reward keys of built-in login
// providers. Custom OAuth providers use "custom:<provider id>" because their
// slug can be edited.
var OAuthBindRewardBuiltinProviders = []string{"github", "discord", "oidc", "linuxdo", "telegram", "wechat"}

// OAuthBindRewardSetting holds the one-time quota paid the first time an
// external account of a provider is linked to a user.
type OAuthBindRewardSetting struct {
	Rewards map[string]int `json:"rewards"`
}

var oauthBindRewardSetting = OAuthBindRewardSetting{
	Rewards: map[string]int{},
}

func init() {
	config.GlobalConfig.Register("oauth_bind_reward", &oauthBindRewardSetting)
}

// OAuthBindRewardCustomKey returns the reward key of a custom OAuth provider.
func OAuthBindRewardCustomKey(providerId int) string {
	return "custom:" + strconv.Itoa(providerId)
}

// GetOAuthBindReward returns the configured reward for a provider key, or 0.
func GetOAuthBindReward(key string) int {
	return max(oauthBindRewardSetting.Rewards[key], 0)
}

// GetOAuthBindRewards returns a copy of the positive rewards.
func GetOAuthBindRewards() map[string]int {
	rewards := make(map[string]int, len(oauthBindRewardSetting.Rewards))
	for key, quota := range oauthBindRewardSetting.Rewards {
		if quota > 0 {
			rewards[key] = quota
		}
	}
	return rewards
}

// ValidateOAuthBindRewards validates the JSON persisted by the option API.
func ValidateOAuthBindRewards(value string) error {
	var rewards map[string]int
	if err := common.UnmarshalJsonStr(value, &rewards); err != nil {
		return fmt.Errorf("OAuth binding rewards must be a JSON map of provider to quota: %w", err)
	}
	if rewards == nil {
		return fmt.Errorf("OAuth binding rewards must be a JSON map of provider to quota")
	}
	for key, quota := range rewards {
		if !slices.Contains(OAuthBindRewardBuiltinProviders, key) {
			idText, ok := strings.CutPrefix(key, "custom:")
			id, err := strconv.Atoi(idText)
			if !ok || err != nil || id <= 0 || strconv.Itoa(id) != idText {
				return fmt.Errorf("unknown OAuth binding reward provider %q", key)
			}
		}
		if quota < 0 || quota > common.MaxWalletQuota {
			return fmt.Errorf("OAuth binding reward for %q must be between 0 and %d", key, common.MaxWalletQuota)
		}
	}
	return nil
}
