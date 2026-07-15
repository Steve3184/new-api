package unrealspeech

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const contextKeyMode = "unrealspeech_mode"

type Adaptor struct{}

func (a *Adaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeUnrealSpeech]
	}
	if info.RelayMode == relayconstant.RelayModeAudioSpeechWebSocket {
		websocketScheme := "wss://"
		if strings.HasPrefix(baseURL, "http://") {
			websocketScheme = "ws://"
		}
		baseURL = strings.TrimPrefix(baseURL, "https://")
		baseURL = strings.TrimPrefix(baseURL, "http://")
		return websocketScheme + baseURL + "/streamWithTimestamps", nil
	}
	mode := ModeSpeech
	if value, ok := info.Request.(*dto.AudioRequest); ok {
		resolved, err := ResolveMode(*value)
		if err != nil {
			return "", err
		}
		mode = resolved
	}
	return baseURL + "/" + mode, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}
	mode, err := ResolveMode(request)
	if err != nil {
		return nil, err
	}
	upstream, err := BuildRequest(request, mode)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(upstream)
	if err != nil {
		return nil, err
	}
	c.Set(contextKeyMode, mode)
	return bytes.NewReader(body), nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeAudioSpeechWebSocket {
		return channel.DoWssRequest(a, c, info, body)
	}
	return channel.DoApiRequest(a, c, info, body)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.RelayMode == relayconstant.RelayModeAudioSpeechWebSocket {
		return handleWebSocketResponse(c, info)
	}
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, types.NewError(errors.New("unsupported response relay mode"), types.ErrorCodeBadResponse)
	}
	if c.GetString(contextKeyMode) == ModeStream {
		return writeAudioResponse(c, resp, info), nil
	}
	return handleSpeechResponse(c, resp, info)
}

func handleSpeechResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	_ = resp.Body.Close()

	var result SynthesisTask
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("invalid UnrealSpeech response: %w", err), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if result.TaskStatus != "completed" {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("UnrealSpeech task status is %s", result.TaskStatus), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	audioURL := FirstURI(result.OutputURI)
	if audioURL == "" {
		return nil, types.NewErrorWithStatusCode(errors.New("UnrealSpeech response has no audio URL"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	audioResponse, err := FetchAudio(c.Request.Context(), audioURL, info.ChannelSetting.Proxy)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("fetch UnrealSpeech audio: %w", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	return writeAudioResponse(c, audioResponse, info), nil
}

func writeAudioResponse(c *gin.Context, response *http.Response, info *relaycommon.RelayInfo) *dto.Usage {
	defer service.CloseResponseBodyGracefully(response)
	for key, values := range response.Header {
		if !service.ShouldCopyUpstreamHeader(c, key, values) || len(values) == 0 {
			continue
		}
		c.Writer.Header().Set(key, values[0])
	}
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Writer.Header().Set("Content-Type", "audio/mpeg")
	}
	c.Writer.WriteHeader(response.StatusCode)
	buffer := make([]byte, 32*1024)
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			if _, writeErr := c.Writer.Write(buffer[:read]); writeErr != nil {
				logger.LogError(c, "failed to write UnrealSpeech audio: "+writeErr.Error())
				break
			}
			c.Writer.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF {
				logger.LogError(c, "failed to read UnrealSpeech audio: "+readErr.Error())
			}
			break
		}
	}
	tokens := info.GetEstimatePromptTokens()
	return &dto.Usage{
		PromptTokens: tokens,
		TotalTokens:  tokens,
		PromptTokensDetails: dto.InputTokenDetails{
			TextTokens: tokens,
		},
	}
}

func FetchAudio(ctx context.Context, audioURL, proxy string) (*http.Response, error) {
	return FetchArtifact(ctx, audioURL, proxy, "audio/mpeg")
}

func FetchArtifact(ctx context.Context, artifactURL, proxy, defaultContentType string) (*http.Response, error) {
	parsed, err := url.Parse(artifactURL)
	if err != nil {
		return nil, err
	}
	trustedS3URL := isTrustedS3ArtifactURL(parsed)

	var client *http.Client
	if trustedS3URL {
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			return nil, err
		}
	} else if proxy == "" {
		if err := service.ValidateSSRFProtectedFetchURL(artifactURL); err != nil {
			return nil, err
		}
		client = service.GetSSRFProtectedHTTPClient()
	} else {
		fetchSetting := system_setting.GetFetchSetting()
		if err := common.ValidateURLWithFetchSetting(artifactURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
			return nil, err
		}
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			return nil, err
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		cancel()
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		cancel()
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		cancel()
		return nil, fmt.Errorf("artifact host returned status %d", response.StatusCode)
	}
	response.Body = &cancelReadCloser{ReadCloser: response.Body, cancel: cancel}
	if response.Header.Get("Content-Type") == "" && defaultContentType != "" {
		response.Header.Set("Content-Type", defaultContentType)
	}
	return response, nil
}

func isTrustedS3ArtifactURL(parsed *url.URL) bool {
	if parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil {
		return false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "s3.amazonaws.com" || strings.HasSuffix(host, ".s3.amazonaws.com") {
		return true
	}
	return strings.HasSuffix(host, ".amazonaws.com") &&
		(strings.Contains(host, ".s3-") || strings.Contains(host, ".s3."))
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func handleWebSocketResponse(c *gin.Context, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.ClientWs == nil || info.TargetWs == nil {
		return nil, types.NewError(errors.New("invalid UnrealSpeech websocket connection"), types.ErrorCodeBadResponse)
	}

	done := make(chan error, 2)
	var usageMutex sync.Mutex
	var workers sync.WaitGroup
	workers.Add(2)
	usage := &dto.RealtimeUsage{}

	gopool.Go(func() {
		defer workers.Done()
		for {
			messageType, message, err := info.ClientWs.ReadMessage()
			if err != nil {
				done <- err
				return
			}
			if messageType == websocket.TextMessage {
				var payload map[string]any
				if common.Unmarshal(message, &payload) == nil {
					text := common.Interface2String(payload["Text"])
					if text == "" {
						text = common.Interface2String(payload["text"])
					}
					if text != "" {
						tokens := utf8.RuneCountInString(text)
						usageMutex.Lock()
						usage.TotalTokens += tokens
						usage.InputTokens += tokens
						usage.InputTokenDetails.TextTokens += tokens
						usageMutex.Unlock()
					}
				}
			}
			if err := info.TargetWs.WriteMessage(messageType, message); err != nil {
				done <- err
				return
			}
		}
	})

	gopool.Go(func() {
		defer workers.Done()
		for {
			messageType, message, err := info.TargetWs.ReadMessage()
			if err != nil {
				done <- err
				return
			}
			info.SetFirstResponseTime()
			if err := info.ClientWs.WriteMessage(messageType, message); err != nil {
				done <- err
				return
			}
		}
	})

	err := <-done
	_ = info.ClientWs.Close()
	_ = info.TargetWs.Close()
	workers.Wait()
	if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		logger.LogError(c, "UnrealSpeech websocket relay error: "+err.Error())
	}
	return usage, nil
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("UnrealSpeech only supports audio speech requests")
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errors.New("UnrealSpeech does not support rerank requests")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("UnrealSpeech does not support embedding requests")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("UnrealSpeech does not support image requests")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("UnrealSpeech does not support responses requests")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("UnrealSpeech does not support Claude requests")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("UnrealSpeech does not support Gemini requests")
}

func (a *Adaptor) GetModelList() []string { return ModelList }

func (a *Adaptor) GetChannelName() string { return ChannelName }
