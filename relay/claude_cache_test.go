package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
)

func TestShouldApplyClaudeCacheControlIsScopedAndDisabledByDefault(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAnthropic,
		},
	}

	assert.False(t, shouldApplyClaudeCacheControl(info))

	info.ChannelOtherSettings.ClaudeCacheControl = true
	assert.True(t, shouldApplyClaudeCacheControl(info))

	info.ChannelType = constant.ChannelTypeOpenAI
	assert.False(t, shouldApplyClaudeCacheControl(info))
}
