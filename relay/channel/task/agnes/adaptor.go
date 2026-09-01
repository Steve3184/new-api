package agnes

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func normalizeAgnesHostURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		baseURL = strings.TrimRight(baseURL[:len(baseURL)-len("/v1")], "/")
	}
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(constant.ChannelTypeAgnes)
	}
	return baseURL
}

func agnesV1URL(baseURL, path string) string {
	return normalizeAgnesHostURL(baseURL) + "/v1" + path
}

func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	id, data, err := a.DoResponse(c, resp, info)
	if err != nil {
		return nil, err
	}
	return &channel.TaskSubmitResponse{UpstreamTaskID: id, TaskData: data}, nil
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		a.baseURL = normalizeAgnesHostURL("")
		return
	}
	a.apiKey = info.ApiKey
	a.baseURL = normalizeAgnesHostURL(info.ChannelBaseUrl)
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionTextToVideo); err != nil {
		return err
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapper(err, "invalid_request", http.StatusBadRequest)
	}
	if !containsModel(req.Model) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported Agnes model: %s", req.Model), "invalid_model", http.StatusBadRequest)
	}
	if strings.Contains(req.Model, "2.5") {
		mode := strings.ToLower(strings.TrimSpace(req.Mode))
		if mode == "" {
			mode = "text"
		}
		if mode != "text" && mode != "keyframe" && mode != "reference" {
			return service.TaskErrorWrapperLocal(fmt.Errorf("invalid Agnes mode: %s", req.Mode), "invalid_mode", http.StatusBadRequest)
		}
		if req.Size != "" && req.Size != "720P" {
			return service.TaskErrorWrapperLocal(fmt.Errorf("Agnes 2.5 size must be 720P"), "invalid_size", http.StatusBadRequest)
		}
		imageCount := len(req.Images)
		if imageCount == 0 && req.Image != "" {
			imageCount = 1
		}
		if (mode == "reference" || mode == "keyframe") && imageCount == 0 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("%s mode requires images", mode), "invalid_images", http.StatusBadRequest)
		}
		if mode == "text" && imageCount > 0 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("text mode does not accept images"), "invalid_images", http.StatusBadRequest)
		}
		if mode == "keyframe" && imageCount > 2 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("keyframe mode supports at most 2 images"), "invalid_images", http.StatusBadRequest)
		}
		if imageCount > 5 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("Agnes 2.5 supports at most 5 reference images"), "invalid_images", http.StatusBadRequest)
		}
		if taskErr := validateAgnes25Seconds(req); taskErr != nil {
			return taskErr
		}
	}
	info.Action = constant.TaskActionTextToVideo
	return nil
}

func validateAgnes25Seconds(req relaycommon.TaskSubmitReq) *taskdto.TaskError {
	seconds := req.Duration
	provided := seconds != 0
	if raw := strings.TrimSpace(req.Seconds); raw != "" {
		provided = true
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("Agnes 2.5 seconds must be an integer between 4 and 12"), "invalid_seconds", http.StatusBadRequest)
		}
		seconds = parsed
	}
	if !provided {
		return nil
	}
	if seconds < 4 || seconds > 12 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("Agnes 2.5 seconds must be between 4 and 12"), "invalid_seconds", http.StatusBadRequest)
	}
	return nil
}

func containsModel(name string) bool {
	for _, modelName := range ModelList {
		if modelName == strings.TrimSpace(name) {
			return true
		}
	}
	return false
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := a.baseURL
	if baseURL == "" && info != nil && info.ChannelMeta != nil {
		baseURL = info.ChannelMeta.ChannelBaseUrl
	}
	return agnesV1URL(baseURL, "/videos"), nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	// Start from the generic request so metadata options remain available to
	// both Agnes generations without exposing internal gateway fields.
	raw, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	var body map[string]interface{}
	if err := common.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	delete(body, "input_reference")
	delete(body, "source_task_id")
	delete(body, "duration")
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = req.Model
	}
	body["model"] = modelName
	if req.Duration > 0 && body["seconds"] == nil {
		body["seconds"] = strconv.Itoa(req.Duration)
	}
	if strings.Contains(modelName, "2.5") {
		mode := strings.ToLower(strings.TrimSpace(req.Mode))
		if mode == "" {
			mode = "text"
		}
		if body["seconds"] == nil || strings.TrimSpace(fmt.Sprint(body["seconds"])) == "" {
			body["seconds"] = "5"
		}
		if req.Size == "" {
			body["size"] = "720P"
		}
		body["mode"] = mode
		if mode == "keyframe" {
			delete(body, "images")
			if len(req.Images) > 0 {
				body["first_frame"] = req.Images[0]
			}
			if len(req.Images) > 1 {
				body["last_frame"] = req.Images[1]
			}
		} else if len(req.Images) > 0 {
			body["images"] = req.Images
		}
	} else if len(req.Images) > 0 {
		body["image"] = req.Images[0]
		if len(req.Images) > 1 {
			body["extra_body"] = map[string]interface{}{"image": req.Images}
		}
	}
	if info.ChannelOtherSettings.AgnesAutoImageURL {
		if err := rewriteImagesForAgnes(c, body); err != nil {
			return nil, err
		}
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func rewriteImagesForAgnes(c *gin.Context, value interface{}) error {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			if err := rewriteImagesForAgnes(c, child); err != nil {
				return err
			}
			if s, ok := child.(string); ok && isImageDataURL(s) {
				converted, err := service.UploadMeshyBase64ImageForChannel(c, s)
				if err != nil {
					return err
				}
				v[key] = converted
			}
		}
	case []interface{}:
		for i, child := range v {
			if err := rewriteImagesForAgnes(c, child); err != nil {
				return err
			}
			if s, ok := child.(string); ok && isImageDataURL(s) {
				converted, err := service.UploadMeshyBase64ImageForChannel(c, s)
				if err != nil {
					return err
				}
				v[i] = converted
			}
		}
	}
	return nil
}

func isImageDataURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:image/")
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

type createResponse struct {
	ID      string `json:"id"`
	TaskID  string `json:"task_id"`
	VideoID string `json:"video_id"`
}

func (a *TaskAdaptor) DoResponse(_ *gin.Context, resp *http.Response, _ *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusBadGateway)
	}
	_ = resp.Body.Close()
	var result createResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "invalid_response", http.StatusBadGateway)
	}
	taskID := result.VideoID
	if taskID == "" {
		taskID = result.TaskID
	}
	if taskID == "" {
		taskID = result.ID
	}
	if taskID == "" {
		return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("Agnes response has no task id"), "invalid_response", http.StatusBadGateway)
	}
	return taskID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	id, ok := body["video_id"].(string)
	if !ok || id == "" {
		id, _ = body["task_id"].(string)
	}
	if id == "" {
		return nil, fmt.Errorf("invalid Agnes task id")
	}
	modelName, _ := body["model"].(string)
	endpoint := agnesV1URL(baseURL, "/agnesapi") + "?video_id=" + url.QueryEscape(id)
	if strings.Contains(modelName, "2.5") {
		endpoint += "&model_name=" + url.QueryEscape(modelName)
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

type statusResponse struct {
	ID          string                 `json:"id"`
	TaskID      string                 `json:"task_id"`
	VideoID     string                 `json:"video_id"`
	Model       string                 `json:"model"`
	Status      string                 `json:"status"`
	Progress    int                    `json:"progress"`
	URL         string                 `json:"url"`
	CompletedAt int64                  `json:"completed_at"`
	Metadata    map[string]interface{} `json:"metadata"`
	Data        map[string]interface{} `json:"data"`
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var result statusResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Status == "" {
		if s, ok := result.Data["status"].(string); ok {
			result.Status = s
		}
	}
	status := &relaycommon.TaskInfo{TaskID: result.VideoID}
	if status.TaskID == "" {
		status.TaskID = result.TaskID
	}
	switch strings.ToLower(result.Status) {
	case "queued", "pending", "submitted", "created":
		status.Status, status.Progress = model.TaskStatusSubmitted, "20%"
	case "in_progress", "processing", "running":
		status.Status, status.Progress = model.TaskStatusInProgress, fmt.Sprintf("%d%%", clampProgress(result.Progress, 30))
	case "completed", "success", "succeeded", "done":
		status.Status, status.Progress = model.TaskStatusSuccess, "100%"
		status.Url = extractAgnesURL(result)
	case "failed", "failure", "error", "cancelled":
		status.Status, status.Progress = model.TaskStatusFailure, "100%"
		if msg, ok := result.Data["error"].(string); ok {
			status.Reason = msg
		}
	default:
		status.Status, status.Progress = model.TaskStatusInProgress, "30%"
	}
	if status.TaskID == "" {
		status.TaskID = result.ID
	}
	return status, nil
}

func clampProgress(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value >= 100 {
		return 99
	}
	return value
}

func extractAgnesURL(result statusResponse) string {
	if result.URL != "" {
		return result.URL
	}
	for _, source := range []map[string]interface{}{result.Metadata, result.Data} {
		if source == nil {
			continue
		}
		if value, ok := source["url"].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }

func (a *TaskAdaptor) GetChannelName() string { return ChannelName }
