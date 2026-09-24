package typesafe

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLIsLimitedToSystemOne(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://opencode.ai"},
		RelayFormat: types.RelayFormatSystemOne,
		RelayMode:   relayconstant.RelayModeSystemOne,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://opencode.ai/zen/v1/systemone", url)

	info.RelayMode = relayconstant.RelayModeChatCompletions
	_, err = adaptor.GetRequestURL(info)
	assert.Error(t, err)

	info.RelayMode = relayconstant.RelayModeSystemOne
	info.RelayFormat = types.RelayFormatOpenAI
	_, err = adaptor.GetRequestURL(info)
	assert.Error(t, err)
}

func TestTypeSafeRegistrationUsesStableChannelType(t *testing.T) {
	assert.Equal(t, 67, constant.ChannelTypeTypeSafe)
}
