package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldUseResponsesChatCompletionsHonorsChannelSettingForOpenAI(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenAI,
			ChannelSetting: dto.ChannelSettings{ResponsesToChatCompletions: true},
		},
	}

	require.True(t, shouldUseResponsesChatCompletions(info))
}

func TestShouldUseResponsesChatCompletionsIgnoresChannelSettingForOtherTypes(t *testing.T) {
	original := model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy
	model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy = model_setting.ResponsesToChatCompletionsPolicy{}
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy = original
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeAnthropic,
			ChannelSetting: dto.ChannelSettings{ResponsesToChatCompletions: true},
		},
	}

	assert.False(t, shouldUseResponsesChatCompletions(info))
}

func TestShouldUseResponsesChatCompletionsRetainsGlobalPolicy(t *testing.T) {
	original := model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy
	model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy = model_setting.ResponsesToChatCompletionsPolicy{
		Enabled:    true,
		ChannelIDs: []int{42},
	}
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().ResponsesToChatCompletionsPolicy = original
	})

	info := &relaycommon.RelayInfo{
		TokenId: 42,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelId:   42,
		},
	}

	assert.True(t, shouldUseResponsesChatCompletions(info))
}
