package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestMatchesMultiKeyDisableRuleMatchesStatusAndMessage(t *testing.T) {
	channel := &Channel{ChannelInfo: ChannelInfo{
		IsMultiKey: true,
		MultiKeyDisableRules: []MultiKeyDisableRule{{
			StatusCode: 429,
			Message:    "Daily usage limit exceeded",
		}},
	}}

	assert.True(t, channel.MatchesMultiKeyDisableRule(429, "Daily usage limit exceeded: maximum $1 per period"))
	assert.False(t, channel.MatchesMultiKeyDisableRule(500, "Daily usage limit exceeded"))
	assert.False(t, channel.MatchesMultiKeyDisableRule(429, "rate limited"))
}

func TestMultiKeyInventoryReconciliation(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		enabled  bool
		target   int
		statuses map[int]int
		want     map[int]int
	}{
		{name: "initial reserve", enabled: true, target: 2, want: map[int]int{2: MultiKeyStatusStandby, 3: MultiKeyStatusStandby}},
		{name: "retain active keys before consuming inventory", enabled: true, target: 2, statuses: map[int]int{0: MultiKeyStatusStandby, 1: common.ChannelStatusAutoDisabled}, want: map[int]int{0: MultiKeyStatusStandby, 1: common.ChannelStatusAutoDisabled}},
		{name: "refill without reviving disabled keys", enabled: true, target: 3, statuses: map[int]int{0: common.ChannelStatusAutoDisabled, 1: common.ChannelStatusManuallyDisabled, 2: MultiKeyStatusStandby}, want: map[int]int{0: common.ChannelStatusAutoDisabled, 1: common.ChannelStatusManuallyDisabled}},
		{name: "lower target", enabled: true, target: 1, want: map[int]int{1: MultiKeyStatusStandby, 2: MultiKeyStatusStandby, 3: MultiKeyStatusStandby}},
		{name: "disable inventory restores only standby", statuses: map[int]int{0: common.ChannelStatusAutoDisabled, 1: common.ChannelStatusManuallyDisabled, 2: MultiKeyStatusStandby}, want: map[int]int{0: common.ChannelStatusAutoDisabled, 1: common.ChannelStatusManuallyDisabled}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			channel := &Channel{Key: "first\nsecond\nthird\nfourth", ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeyInventoryEnabled: testCase.enabled, MultiKeyInventoryTarget: testCase.target, MultiKeyStatusList: testCase.statuses}}
			channel.ReconcileMultiKeyInventory(channel.GetKeys())
			assert.Equal(t, testCase.want, channel.ChannelInfo.MultiKeyStatusList)
			channel.ReconcileMultiKeyInventory(channel.GetKeys())
			assert.Equal(t, testCase.want, channel.ChannelInfo.MultiKeyStatusList)
		})
	}
}

func TestMatchesMultiKeyDisableRuleSkipsEmptyRulesAndNonMultiKeyChannels(t *testing.T) {
	channel := &Channel{ChannelInfo: ChannelInfo{
		MultiKeyDisableRules: []MultiKeyDisableRule{{}},
	}}
	assert.False(t, channel.MatchesMultiKeyDisableRule(429, "anything"))

	channel.ChannelInfo.IsMultiKey = true
	channel.ChannelInfo.MultiKeyDisableRules = []MultiKeyDisableRule{{StatusCode: 429}}
	assert.True(t, channel.MatchesMultiKeyDisableRule(429, ""))
}
