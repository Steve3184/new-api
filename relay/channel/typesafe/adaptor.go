package typesafe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayFormat != types.RelayFormatSystemOne || info.RelayMode != relayconstant.RelayModeSystemOne {
		return "", fmt.Errorf("TypeSafe supports only System One requests")
	}
	return info.ChannelBaseUrl + "/zen/v1/systemone", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertSystemOneRequest(info *relaycommon.RelayInfo, request *dto.SystemOneRequest) (*dto.SystemOneRequest, error) {
	if info.RelayFormat != types.RelayFormatSystemOne || info.RelayMode != relayconstant.RelayModeSystemOne {
		return nil, errors.New("TypeSafe supports only System One requests")
	}
	converted := *request
	converted.Model = info.UpstreamModelName
	return &converted, nil
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support OpenAI requests")
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support rerank requests")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support embedding requests")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented: TypeSafe does not support audio requests")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support image requests")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support Responses requests")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support Claude requests")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented: TypeSafe does not support Gemini requests")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("empty TypeSafe response"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	var payload map[string]json.RawMessage
	if jsonErr := json.Unmarshal(body, &payload); jsonErr != nil || payload == nil {
		return nil, types.NewOpenAIError(errors.New("invalid TypeSafe JSON response"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	usageInfo := &dto.Usage{}
	if raw, ok := payload["usage"]; ok {
		var values struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
		if json.Unmarshal(raw, &values) == nil {
			usageInfo.InputTokens = values.InputTokens
			usageInfo.OutputTokens = values.OutputTokens
			usageInfo.PromptTokens = values.InputTokens
			usageInfo.CompletionTokens = values.OutputTokens
			usageInfo.TotalTokens = values.InputTokens + values.OutputTokens
			usageInfo.UsageSource = "upstream"
			usageInfo.UsageSemantic = dto.BillingUsageSemanticOpenAI
		}
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	if _, err := c.Writer.Write(body); err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return usageInfo, nil
}

func (a *Adaptor) GetModelList() []string { return ModelList }

func (a *Adaptor) GetChannelName() string { return ChannelName }
