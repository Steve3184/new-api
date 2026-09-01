package agnes

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResponseDoesNotWriteToClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"task_agnes_1","video_id":"video_agnes_1"}`))),
	}

	parsed, taskErr := (&TaskAdaptor{}).ParseResponse(c, response, &relaycommon.RelayInfo{})
	require.Nil(t, taskErr)
	require.NotNil(t, parsed)
	require.Equal(t, "video_agnes_1", parsed.UpstreamTaskID)
	require.False(t, c.Writer.Written(), "response parsing must not write the client response")
	require.Empty(t, recorder.Body.Bytes())
}

func TestBuildURLs(t *testing.T) {
	a := &TaskAdaptor{baseURL: "https://example.test"}
	for _, modelName := range ModelList {
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
		url, err := a.BuildRequestURL(info)
		require.NoError(t, err)
		require.Equal(t, "https://example.test/v1/videos", url)
	}
	resp, err := a.FetchTask("https://example.test", "key", map[string]any{"video_id": "abc", "model": "agnes-video-2.5-flash"}, "")
	if resp != nil {
		_ = resp.Body.Close()
	}
	// The request fails to connect, but the URL construction is validated by a local transport below.
	require.Error(t, err)
}

func TestBuildURLUsesAgnesDefaultAndNormalizesVersion(t *testing.T) {
	a := &TaskAdaptor{}
	url, err := a.BuildRequestURL(&relaycommon.RelayInfo{})
	require.NoError(t, err)
	require.Equal(t, "https://apihub.agnes-ai.com/v1/videos", url)

	a.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://example.test/v1"}})
	url, err = a.BuildRequestURL(nil)
	require.NoError(t, err)
	require.Equal(t, "https://example.test/v1/videos", url)
}

func TestParseTaskResultProgressAndURL(t *testing.T) {
	a := &TaskAdaptor{}
	result, err := a.ParseTaskResult([]byte(`{"video_id":"v1","status":"processing","progress":100}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusInProgress, result.Status)
	require.Equal(t, "99%", result.Progress)
	result, err = a.ParseTaskResult([]byte(`{"video_id":"v1","status":"completed","metadata":{"url":"https://cdn.test/v.mp4"}}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, "https://cdn.test/v.mp4", result.Url)
}

func TestFetchTaskUsesAgnesAPIPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	a := &TaskAdaptor{}
	resp, err := a.FetchTask(server.URL, "key", map[string]any{"video_id": "abc", "model": "agnes-video-2.5"}, "")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, "/v1/agnesapi?video_id=abc&model_name=agnes-video-2.5", gotPath)
}

func TestValidateAgnes25Seconds(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		wantErr string
	}{
		{name: "default", req: relaycommon.TaskSubmitReq{}},
		{name: "valid string", req: relaycommon.TaskSubmitReq{Seconds: "12"}},
		{name: "valid duration", req: relaycommon.TaskSubmitReq{Duration: 4}},
		{name: "too short", req: relaycommon.TaskSubmitReq{Seconds: "3"}, wantErr: "invalid_seconds"},
		{name: "too long", req: relaycommon.TaskSubmitReq{Duration: 13}, wantErr: "invalid_seconds"},
		{name: "not numeric", req: relaycommon.TaskSubmitReq{Seconds: "five"}, wantErr: "invalid_seconds"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAgnes25Seconds(test.req)
			if test.wantErr == "" {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, test.wantErr, err.Code)
		})
	}
}

func TestBuildRequestBodyDefaultsAgnes25Seconds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("task_request", relaycommon.TaskSubmitReq{Model: "agnes-video-2.5-flash", Prompt: "a calm ocean"})
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "agnes-video-2.5-flash"}}

	bodyReader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	body, err := io.ReadAll(bodyReader)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(body, &payload))
	assert.Equal(t, "5", payload["seconds"])
	assert.NotContains(t, payload, "duration")
	assert.Equal(t, "720P", payload["size"])
}

func TestBuildRequestBodyUsesAgnes25ReferenceFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "agnes-video-2.5-flash",
		Prompt: "animate the reference",
		Mode:   "reference",
		Image:  "data:image/png;base64,abc",
		Images: []string{"https://example.test/reference.png"},
	})
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "agnes-video-2.5-flash"}}

	bodyReader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	body, err := io.ReadAll(bodyReader)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(body, &payload))
	assert.NotContains(t, payload, "image")
	assert.Equal(t, []interface{}{"https://example.test/reference.png"}, payload["images"])
	assert.Equal(t, "reference", payload["mode"])
}

func TestTaskAdaptorRejectsNilRelayInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "a calm ocean", Model: "agnes-video-v2.0"})

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, nil)
	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_relay_info", taskErr.Code)
}

func TestTaskAdaptorInitHandlesMissingChannelMeta(t *testing.T) {
	assert.NotPanics(t, func() {
		(&TaskAdaptor{}).Init(&relaycommon.RelayInfo{})
		(&TaskAdaptor{}).Init(nil)
	})
}

func TestBuildRequestBodyRejectsNilRelayInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "a calm ocean", Model: "agnes-video-v2.0"})

	_, err := (&TaskAdaptor{}).BuildRequestBody(c, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "relay info is missing")
}

func TestDoResponseRejectsMissingResponse(t *testing.T) {
	_, _, taskErr := (&TaskAdaptor{}).DoResponse(nil, nil, nil)
	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_response", taskErr.Code)
}
