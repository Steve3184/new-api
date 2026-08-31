package agnes

import (
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/relay/common"
)

// Adaptor keeps Agnes visible in the common channel registry. Agnes' supported
// generation API is implemented by the task adaptor; embedding the OpenAI
// adaptor preserves harmless model-list and channel-management compatibility
// for installations that expose the channel through generic tooling.
type Adaptor struct{ openai.Adaptor }

func (a *Adaptor) Init(info *common.RelayInfo) { a.Adaptor.Init(info) }

func (a *Adaptor) GetModelList() []string { return ModelList }

func (a *Adaptor) GetChannelName() string { return ChannelName }
