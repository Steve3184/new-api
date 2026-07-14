package meshy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

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
	var request dto.ThreeDRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" {
		return service.TaskErrorWrapperLocal(errors.New("model is required"), "missing_model", http.StatusBadRequest)
	}
	if !isSupportedModel(request.Model) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported 3D model: %s", request.Model), "invalid_model", http.StatusBadRequest)
	}

	isTexture := strings.HasSuffix(request.Model, "-texture")
	if isTexture {
		if strings.TrimSpace(request.SourceTaskID) == "" {
			return service.TaskErrorWrapperLocal(errors.New("source_task_id is required for texture models"), "missing_source_task_id", http.StatusBadRequest)
		}
		if info.OriginUpstreamTaskID == "" {
			return service.TaskErrorWrapperLocal(errors.New("source task could not be resolved"), "invalid_source_task_id", http.StatusBadRequest)
		}
	} else {
		if request.SourceTaskID != "" {
			return service.TaskErrorWrapperLocal(errors.New("source_task_id is only valid for texture models"), "invalid_source_task_id", http.StatusBadRequest)
		}
		if strings.TrimSpace(request.Prompt) == "" && strings.TrimSpace(request.InputReference) == "" {
			return service.TaskErrorWrapperLocal(errors.New("prompt or input_reference is required"), "invalid_request", http.StatusBadRequest)
		}
	}

	inputReference := strings.ToLower(strings.TrimSpace(request.InputReference))
	if strings.HasPrefix(inputReference, "http://") || strings.HasPrefix(inputReference, "https://") {
		return service.TaskErrorWrapperLocal(errors.New("input_reference does not support HTTP URLs"), "invalid_input_reference", http.StatusBadRequest)
	}

	if request.Metadata != nil && request.Metadata.ArtStyle != "" {
		switch request.Metadata.ArtStyle {
		case "realistic", "cartoon", "sculpture", "pbr":
		default:
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported art_style: %s", request.Metadata.ArtStyle), "invalid_art_style", http.StatusBadRequest)
		}
	}

	info.Action = request.Model
	c.Set("three_d_request", request)
	return nil
}

func isSupportedModel(modelName string) bool {
	for _, model := range ModelList {
		if model == modelName {
			return true
		}
	}
	return false
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/3d", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	value, exists := c.Get("three_d_request")
	if !exists {
		return nil, errors.New("3D request not found in context")
	}
	request, ok := value.(dto.ThreeDRequest)
	if !ok {
		return nil, errors.New("invalid 3D request in context")
	}
	request.Model = info.UpstreamModelName
	if request.SourceTaskID != "" {
		request.SourceTaskID = info.OriginUpstreamTaskID
	}
	body, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var response dto.ThreeDResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusBadGateway)
	}
	if response.ID == "" {
		return "", nil, service.TaskErrorWrapper(errors.New("task id is empty"), "invalid_response", http.StatusBadGateway)
	}

	upstreamTaskID := response.ID
	response.ID = info.PublicTaskID
	response.Object = "3d"
	response.Model = info.OriginModelName
	if response.Data != nil && response.Data.URL != "" {
		response.Data.URL = taskcommon.BuildThreeDProxyURL(info.PublicTaskID)
	}
	c.JSON(http.StatusOK, response)
	return upstreamTaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, errors.New("invalid task_id")
	}

	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/3d/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(request)
}

func (a *TaskAdaptor) ParseTaskResult(responseBody []byte) (*relaycommon.TaskInfo, error) {
	var response dto.ThreeDResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal 3D task result failed")
	}

	result := &relaycommon.TaskInfo{}
	switch response.Status {
	case "queued":
		result.Status = model.TaskStatusQueued
	case "in_progress":
		result.Status = model.TaskStatusInProgress
	case "completed":
		result.Status = model.TaskStatusSuccess
		if response.Data != nil {
			result.Url = response.Data.URL
		}
	case "failed":
		result.Status = model.TaskStatusFailure
		if response.Error != nil {
			result.Reason = response.Error.Message
		} else {
			result.Reason = "3D generation failed"
		}
	}
	if response.Progress > 0 {
		result.Progress = fmt.Sprintf("%d%%", response.Progress)
	}
	return result, nil
}

func (a *TaskAdaptor) ConvertToThreeD(task *model.Task) ([]byte, error) {
	var response dto.ThreeDResponse
	if err := common.Unmarshal(task.Data, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal stored 3D task failed")
	}
	response.ID = task.TaskID
	response.Object = "3d"
	response.Model = task.Properties.OriginModelName
	if response.Status == "completed" {
		if response.Data == nil {
			response.Data = &dto.ThreeDData{Format: "glb"}
		}
		response.Data.URL = taskcommon.BuildThreeDProxyURL(task.TaskID)
	}
	return common.Marshal(response)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}
