package unrealspeech

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTaskContext(body string) *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech/tasks", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context
}

func TestTaskRequestUsesCharacterTokenBilling(t *testing.T) {
	context := newTaskContext(`{"model":"unreal-speech-v8","Text":"你好 world","VoiceId":"Sierra","Speed":0}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.ValidateRequestAndSetAction(context, info))
	meta := adaptor.GetTokenCountMeta(context, info)
	assert.Equal(t, "你好 world", meta.CombineText)
	assert.Equal(t, "text_number", string(meta.TokenType))
	originalCountToken := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = originalCountToken })
	tokens, err := service.EstimateRequestToken(context, meta, info)
	require.NoError(t, err)
	assert.Equal(t, 8, tokens)
}

func TestParseTaskResultMapsLiveStatuses(t *testing.T) {
	adaptor := &TaskAdaptor{}
	tests := []struct {
		status   string
		expected model.TaskStatus
	}{
		{status: "scheduled", expected: model.TaskStatusQueued},
		{status: "inProgress", expected: model.TaskStatusInProgress},
		{status: "completed", expected: model.TaskStatusSuccess},
		{status: "failed", expected: model.TaskStatusFailure},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			body := `{"SynthesisTask":{"TaskId":"upstream-id","TaskStatus":"` + test.status + `","OutputUri":"https://audio.example.com/result.mp3","StatusDetails":"failed upstream"}}`
			result, err := adaptor.ParseTaskResult([]byte(body))
			require.NoError(t, err)
			assert.Equal(t, test.expected, model.TaskStatus(result.Status))
			if test.expected == model.TaskStatusSuccess {
				assert.Equal(t, "https://audio.example.com/result.mp3", result.Url)
			}
		})
	}
}

func TestPublicTaskResponseHidesUpstreamIdentifiersAndURL(t *testing.T) {
	originalAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://gateway.example.com"
	t.Cleanup(func() { system_setting.ServerAddress = originalAddress })

	task := &model.Task{
		TaskID:    "task_public",
		CreatedAt: 123,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		Properties: model.Properties{
			OriginModelName: "unreal-speech-v8",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-private",
			ResultURL:      "https://audio.example.com/private.mp3",
		},
	}
	task.SetData(map[string]any{
		"SynthesisTask": map[string]any{
			"TaskId":        "upstream-private",
			"OutputUri":     "https://audio.example.com/private.mp3",
			"TimestampsUri": "https://audio.example.com/private-timestamps.json",
		},
	})

	body, err := (&TaskAdaptor{}).ConvertToOpenAIAudioTask(task)
	require.NoError(t, err)
	assert.Contains(t, string(body), "https://gateway.example.com/v1/audio/speech/tasks/task_public/content")
	assert.Contains(t, string(body), "https://gateway.example.com/v1/audio/speech/tasks/task_public/timestamps")
	assert.NotContains(t, string(body), "upstream-private")
	assert.NotContains(t, string(body), "audio.example.com")

	var response map[string]any
	require.NoError(t, common.Unmarshal(body, &response))
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, "https://gateway.example.com/v1/audio/speech/tasks/task_public/timestamps", response["timestamps_url"])
}
