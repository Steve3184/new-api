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

func TestGetRequestURLRespectsConfiguredBaseURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		base string
		want string
	}{
		{"default", "https://opencode.ai", "https://opencode.ai/zen/v1/systemone"},
		{"default trailing slash", "https://opencode.ai/", "https://opencode.ai/zen/v1/systemone"},
		{"explicit zen prefix", "https://opencode.ai/zen", "https://opencode.ai/zen/v1/systemone"},
		{"custom prefix", "https://developer.amd.com.cn/radeon/api", "https://developer.amd.com.cn/radeon/api/v1/systemone"},
		{"custom trailing slash", "https://developer.amd.com.cn/radeon/api/", "https://developer.amd.com.cn/radeon/api/v1/systemone"},
		{"versioned prefix", "https://developer.amd.com.cn/radeon/api/v1", "https://developer.amd.com.cn/radeon/api/v1/systemone"},
		{"versioned trailing slash", "https://developer.amd.com.cn/radeon/api/v1/", "https://developer.amd.com.cn/radeon/api/v1/systemone"},
		{"custom host", "https://typesafe.example", "https://typesafe.example/v1/systemone"},
		{"proxy root", "http://localhost:3000", "http://localhost:3000/v1/systemone"},
		{"proxy legacy prefix", "http://localhost:3000/zen", "http://localhost:3000/zen/v1/systemone"},
		{"proxy versioned prefix", "http://localhost:3000/v1/", "http://localhost:3000/v1/systemone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: tc.base},
				RelayFormat: types.RelayFormatSystemOne,
				RelayMode:   relayconstant.RelayModeSystemOne,
			}
			got, err := (&Adaptor{}).GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTypeSafeRegistrationUsesStableChannelType(t *testing.T) {
	assert.Equal(t, 67, constant.ChannelTypeTypeSafe)
}
