package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelperKeepsAutoSelectedModelWhenChannelMappingIsEmpty(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	request := &dto.GeneralOpenAIRequest{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "auto/terra",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "openai/gpt-5.6-terra",
		},
	}

	require.NoError(t, ModelMappedHelper(c, info, request))
	require.Equal(t, "openai/gpt-5.6-terra", info.UpstreamModelName)
	require.Equal(t, "openai/gpt-5.6-terra", request.Model)
}

func TestModelMappedHelperUsesCommonJSONDecoder(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_mapping", `{"auto/terra":"openai/gpt-5.6-terra"}`)
	info := &relaycommon.RelayInfo{OriginModelName: "auto/terra"}
	request := &dto.GeneralOpenAIRequest{}

	require.NoError(t, ModelMappedHelper(c, info, request))
	require.Equal(t, "openai/gpt-5.6-terra", request.Model)
}

func TestModelMappedHelperAppliesVirtualMappingAfterAutoSelection(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_mapping", `{"auto/terra":"openai/gpt-5.6-terra"}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "auto/terra",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.6-terra",
		},
	}
	request := &dto.GeneralOpenAIRequest{}

	require.NoError(t, ModelMappedHelper(c, info, request))
	require.Equal(t, "openai/gpt-5.6-terra", info.UpstreamModelName)
	require.Equal(t, "openai/gpt-5.6-terra", request.Model)
}
