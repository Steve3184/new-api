package agnes

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/relay/common"
)

// Adaptor keeps Agnes visible in the common channel registry. Agnes' supported
// generation API is implemented by the task adaptor; embedding the OpenAI
// adaptor preserves harmless model-list and channel-management compatibility
// for installations that expose the channel through generic tooling.
type Adaptor struct{ openai.Adaptor }

func (a *Adaptor) Init(info *common.RelayInfo) {
	if strings.TrimSpace(info.ChannelBaseUrl) == "" {
		info.ChannelBaseUrl = constant.GetChannelBaseURL(constant.ChannelTypeAgnes)
	}
	a.Adaptor.Init(info)
}

func (a *Adaptor) GetModelList() []string { return ModelList }

func (a *Adaptor) GetChannelName() string { return ChannelName }
