package unrealspeech

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	provider "github.com/QuantumNous/new-api/relay/channel/unrealspeech"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const contextKeyRequest = "unrealspeech_task_request"

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var request dto.AudioRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	request.NormalizeUnrealSpeechAliases()
	if strings.TrimSpace(request.Model) == "" {
		return service.TaskErrorWrapperLocal(errors.New("model is required"), "missing_model", http.StatusBadRequest)
	}
	if utf8.RuneCountInString(request.Input) > 500000 {
		return service.TaskErrorWrapperLocal(errors.New("input must not exceed 500000 characters"), "invalid_input", http.StatusBadRequest)
	}
	request.Stream = nil
	request.Speech = nil
	upstream, err := provider.BuildRequest(request, provider.ModeTask)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	info.Action = "speech"
	c.Set(contextKeyRequest, upstream)
	return nil
}

func (a *TaskAdaptor) GetTokenCountMeta(c *gin.Context, _ *relaycommon.RelayInfo) *types.TokenCountMeta {
	value, exists := c.Get(contextKeyRequest)
	if !exists {
		return &types.TokenCountMeta{TokenType: types.TokenTypeTextNumber}
	}
	request, ok := value.(provider.Request)
	if !ok {
		return &types.TokenCountMeta{TokenType: types.TokenTypeTextNumber}
	}
	return &types.TokenCountMeta{CombineText: request.Text, TokenType: types.TokenTypeTextNumber}
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", errors.New("UnrealSpeech base URL is empty")
	}
	return a.baseURL + "/synthesisTasks", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, request *http.Request, _ *relaycommon.RelayInfo) error {
	request.Header.Set("Authorization", "Bearer "+a.apiKey)
	request.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, _ *relaycommon.RelayInfo) (io.Reader, error) {
	value, exists := c.Get(contextKeyRequest)
	if !exists {
		return nil, errors.New("UnrealSpeech task request not found in context")
	}
	request, ok := value.(provider.Request)
	if !ok {
		return nil, errors.New("invalid UnrealSpeech task request")
	}
	body, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, response *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusBadGateway)
	}
	_ = response.Body.Close()

	var envelope provider.SynthesisTaskEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("invalid UnrealSpeech task response: %w", err), "invalid_response", http.StatusBadGateway)
	}
	if envelope.SynthesisTask.TaskID == "" {
		return "", nil, service.TaskErrorWrapper(errors.New("UnrealSpeech task id is empty"), "invalid_response", http.StatusBadGateway)
	}

	public := newPublicResponse(info.PublicTaskID, info.OriginModelName, time.Now().Unix(), model.TaskStatusQueued, "0%", "")
	if envelope.SynthesisTask.TaskStatus == "inProgress" {
		public.Status = "in_progress"
		public.Progress = 30
	}
	c.JSON(http.StatusOK, public)
	return envelope.SynthesisTask.TaskID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, errors.New("invalid task_id")
	}
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/synthesisTasks/"+url.PathEscape(taskID), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var envelope provider.SynthesisTaskEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	task := envelope.SynthesisTask
	result := &relaycommon.TaskInfo{TaskID: task.TaskID}
	switch task.TaskStatus {
	case "scheduled":
		result.Status = model.TaskStatusQueued
		result.Progress = taskcommon.ProgressQueued
	case "inProgress":
		result.Status = model.TaskStatusInProgress
		result.Progress = taskcommon.ProgressInProgress
	case "completed":
		result.Status = model.TaskStatusSuccess
		result.Progress = taskcommon.ProgressComplete
		result.Url = provider.FirstURI(task.OutputURI)
	case "failed":
		result.Status = model.TaskStatusFailure
		result.Progress = taskcommon.ProgressComplete
		result.Reason = strings.TrimSpace(task.StatusDetails)
		if result.Reason == "" {
			result.Reason = "UnrealSpeech generation failed"
		}
	default:
		return nil, fmt.Errorf("unknown UnrealSpeech task status: %s", task.TaskStatus)
	}
	return result, nil
}

func (a *TaskAdaptor) ConvertToOpenAIAudioTask(task *model.Task) ([]byte, error) {
	createdAt := task.CreatedAt
	if createdAt == 0 {
		createdAt = task.SubmitTime
	}
	response := newPublicResponse(task.TaskID, task.Properties.OriginModelName, createdAt, task.Status, task.Progress, task.FailReason)
	if task.Status == model.TaskStatusSuccess {
		var envelope provider.SynthesisTaskEnvelope
		if common.Unmarshal(task.Data, &envelope) == nil && provider.FirstURI(envelope.SynthesisTask.TimestampsURI) != "" {
			response.TimestampsURL = taskcommon.BuildAudioSpeechTimestampsProxyURL(task.TaskID)
		}
	}
	return common.Marshal(response)
}

func newPublicResponse(taskID, modelName string, createdAt int64, status model.TaskStatus, progress, failReason string) dto.AudioSpeechTaskResponse {
	response := dto.AudioSpeechTaskResponse{
		ID:        taskID,
		Object:    "audio.speech",
		CreatedAt: createdAt,
		Status:    "queued",
		Model:     modelName,
	}
	if value, err := strconv.Atoi(strings.TrimSuffix(progress, "%")); err == nil {
		response.Progress = value
	}
	switch status {
	case model.TaskStatusInProgress:
		response.Status = "in_progress"
	case model.TaskStatusSuccess:
		response.Status = "completed"
		response.Progress = 100
		response.ContentURL = taskcommon.BuildAudioSpeechProxyURL(taskID)
	case model.TaskStatusFailure:
		response.Status = "failed"
		response.Progress = 100
		response.Error = &dto.AudioSpeechTaskError{Code: "generation_failed", Message: failReason}
	}
	return response
}

func (a *TaskAdaptor) GetModelList() []string { return provider.ModelList }

func (a *TaskAdaptor) GetChannelName() string { return provider.ChannelName }
