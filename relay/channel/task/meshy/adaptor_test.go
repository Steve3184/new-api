package meshy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newThreeDTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodPost, "/v1/3d", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	return context
}

func TestTextureRequestUsesResolvedUpstreamTaskID(t *testing.T) {
	context := newThreeDTestContext(t, `{
		"model":"meshy-6-texture",
		"source_task_id":"task_public_draft",
		"metadata":{"art_style":"pbr"}
	}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "meshy-6-texture",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "meshy-6-texture",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			OriginUpstreamTaskID: "8f57e450-2cad-4b2d-9369-05e6ee891452",
		},
	}
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	body, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	payload, err := io.ReadAll(body)
	require.NoError(t, err)

	var request dto.ThreeDRequest
	require.NoError(t, common.Unmarshal(payload, &request))
	assert.Equal(t, "8f57e450-2cad-4b2d-9369-05e6ee891452", request.SourceTaskID)
	assert.Equal(t, "meshy-6-texture", request.Model)
	assert.Equal(t, "pbr", request.Metadata.ArtStyle)
}

func TestTextureRequestRejectsUnresolvedSourceTask(t *testing.T) {
	context := newThreeDTestContext(t, `{
		"model":"meshy-6-texture",
		"source_task_id":"task_missing"
	}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "meshy-6-texture",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)

	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_source_task_id", taskErr.Code)
}

func TestConvertToThreeDHidesUpstreamTaskAndArtifactURL(t *testing.T) {
	originalServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://gateway.example.com"
	t.Cleanup(func() { system_setting.ServerAddress = originalServerAddress })

	task := &model.Task{
		TaskID: "task_public_result",
		Properties: model.Properties{
			OriginModelName: "meshy-6-draft",
		},
	}
	task.SetData(dto.ThreeDResponse{
		ID:     "8f57e450-2cad-4b2d-9369-05e6ee891452",
		Object: "3d",
		Model:  "meshy-6-draft",
		Status: "completed",
		Data: &dto.ThreeDData{
			Format: "glb",
			URL:    "https://meshy.internal/artifacts/private.glb",
		},
	})

	body, err := (&TaskAdaptor{}).ConvertToThreeD(task)
	require.NoError(t, err)
	var response dto.ThreeDResponse
	require.NoError(t, common.Unmarshal(body, &response))

	assert.Equal(t, "task_public_result", response.ID)
	assert.Equal(t, "https://gateway.example.com/v1/3d/task_public_result/content", response.Data.URL)
	assert.NotContains(t, string(body), "8f57e450-2cad-4b2d-9369-05e6ee891452")
	assert.NotContains(t, string(body), "meshy.internal")
}

func TestSubmitResponseHidesUpstreamTaskAndArtifactURL(t *testing.T) {
	originalServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://gateway.example.com"
	t.Cleanup(func() { system_setting.ServerAddress = originalServerAddress })

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{
			"id":"upstream-task-id",
			"object":"3d",
			"model":"meshy-6-draft",
			"status":"completed",
			"data":{"format":"glb","url":"https://meshy.internal/private.glb"}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "meshy-6-draft",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_submit",
		},
	}

	upstreamTaskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(context, response, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-task-id", upstreamTaskID)
	assert.Contains(t, string(taskData), "meshy.internal")
	assert.NotContains(t, recorder.Body.String(), "upstream-task-id")
	assert.NotContains(t, recorder.Body.String(), "meshy.internal")
	assert.Contains(t, recorder.Body.String(), "https://gateway.example.com/v1/3d/task_public_submit/content")
}

func TestNativeThreeDEndpointPaths(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/v1/3d/upstream-task-id", request.URL.Path)
		assert.Equal(t, "Bearer mk_test", request.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"upstream-task-id","object":"3d","status":"queued"}`))
	}))
	t.Cleanup(server.Close)

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl: server.URL,
		ApiKey:         "mk_test",
	}})
	submitURL, err := adaptor.BuildRequestURL(nil)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1/3d", submitURL)

	response, err := adaptor.FetchTask(server.URL, "mk_test", map[string]any{
		"task_id": "upstream-task-id",
	}, "")
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
}
