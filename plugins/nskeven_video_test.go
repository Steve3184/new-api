package plugins_test

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const nskevenBaseURL = "https://newapi.nskeven.uk"

func TestNskevenResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "nskeven",
		model:     "even-sd2.5-A",
		requestBody: map[string]any{
			"model": "even-sd2.5-A",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "a cat in the rain"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/ref.png"},
			}}},
			"seconds": 8,
			"size":    "720x1280",
		},
		wantAction: "image_to_video",
		wantRequest: map[string]any{
			"model":        "even-sd2.5-A",
			"prompt":       "a cat in the rain",
			"seconds":      float64(8),
			"resolution":   "720p",
			"aspect_ratio": "9:16",
			"images":       []any{"https://cdn.example/ref.png"},
		},
		wantUsageKeys:       []string{"resolution", "seconds"},
		wantSubmitUsageKeys: []string{"resolution", "seconds"},
		wantVendorName:      "video_provider_2",
	})
}

func newNskevenPlugin(t *testing.T) (*jsplugin.Registry, *jsplugin.LoadedPlugin) {
	t.Helper()
	source, err := builtinplugins.Source("nskeven")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "nskeven"})
	require.NoError(t, err)
	return registry, plugin
}

func nskevenSubmitBody(t *testing.T, plugin *jsplugin.LoadedPlugin, requestBody map[string]any, model string) map[string]any {
	t.Helper()
	value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
		"requestBody":   requestBody,
		"model":         model,
		"upstreamModel": model,
		"baseUrl":       nskevenBaseURL,
		"apiKey":        "sk-test",
		"action":        "text_to_video",
	})
	require.NoError(t, err)
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var descriptor map[string]any
	require.NoError(t, common.Unmarshal(encoded, &descriptor))
	return descriptor
}

// The vendor takes `seconds` as a string and rejects a bare number with 400.
func TestNskevenSubmitsSecondsAsString(t *testing.T) {
	_, plugin := newNskevenPlugin(t)

	descriptor := nskevenSubmitBody(t, plugin, map[string]any{
		"model":   "even-sd2.5-A",
		"prompt":  "a cat",
		"seconds": float64(8),
	}, "even-sd2.5-A")

	assert.Equal(t, nskevenBaseURL+"/v1/video/generations", descriptor["url"])
	body, ok := descriptor["body"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "8", body["seconds"])
}

// Doubao Seedance reads resolution, ratio and materials from `metadata`.
func TestNskevenMovesDoubaoFieldsIntoMetadata(t *testing.T) {
	_, plugin := newNskevenPlugin(t)

	descriptor := nskevenSubmitBody(t, plugin, map[string]any{
		"model":           "doubao-seedance-2-0-fast-260128",
		"prompt":          "a cat",
		"seconds":         float64(6),
		"resolution":      "720p",
		"aspect_ratio":    "9:16",
		"images":          []any{"https://cdn.example/a.png"},
		"videos":          []any{"https://cdn.example/b.mp4"},
		"first_frame_url": "https://cdn.example/first.png",
	}, "doubao-seedance-2-0-fast-260128")

	body, ok := descriptor["body"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, body, "resolution")
	assert.NotContains(t, body, "images")

	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "720p", metadata["resolution"])
	assert.Equal(t, "9:16", metadata["ratio"])

	content, ok := metadata["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 3)
	roles := make([]any, 0, len(content))
	for _, entry := range content {
		item, ok := entry.(map[string]any)
		require.True(t, ok)
		roles = append(roles, item["role"])
	}
	assert.Equal(t, []any{"reference_image", "first_frame", "reference_video"}, roles)
}

// The express line keeps images at the top level but moves reference video and
// audio into `metadata`.
func TestNskevenKeepsExpressLineMediaInMetadata(t *testing.T) {
	_, plugin := newNskevenPlugin(t)

	descriptor := nskevenSubmitBody(t, plugin, map[string]any{
		"model":   "sd-2-E6",
		"prompt":  "a cat",
		"seconds": float64(8),
		"images":  []any{"https://cdn.example/a.png"},
		"videos":  []any{"https://cdn.example/b.mp4"},
		"audios":  []any{"https://cdn.example/c.mp3"},
	}, "sd-2-E6")

	body, ok := descriptor["body"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"https://cdn.example/a.png"}, body["images"])
	assert.NotContains(t, body, "videos")

	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"https://cdn.example/b.mp4"}, metadata["videos"])
	assert.Equal(t, []any{"https://cdn.example/c.mp3"}, metadata["audios"])
}

// Only `data.status` decides the outcome; a missing or unknown status stays
// UNKNOWN so the poller keeps waiting instead of inventing progress.
func TestNskevenParsesTaskStatusFromTheStableEnvelope(t *testing.T) {
	_, plugin := newNskevenPlugin(t)

	cases := []struct {
		name   string
		body   map[string]any
		status string
		reason string
	}{
		{"progress", map[string]any{"code": "success", "data": map[string]any{"status": "IN_PROGRESS", "progress": "45%"}}, "IN_PROGRESS", ""},
		{"success", map[string]any{"code": "success", "data": map[string]any{"status": "SUCCESS", "progress": "100%"}}, "SUCCESS", ""},
		{"failure", map[string]any{"code": "success", "data": map[string]any{"status": "FAILURE", "fail_reason": "moderation blocked"}}, "FAILURE", "moderation blocked"},
		{"unknown status", map[string]any{"code": "success", "data": map[string]any{"status": "SUCCEEDED"}}, "UNKNOWN", "unrecognized status: SUCCEEDED"},
		{"error code", map[string]any{"code": "error", "message": "task not found", "data": map[string]any{"status": "FAILURE"}}, "UNKNOWN", "task not found"},
		{"no data object", map[string]any{"code": "success"}, "UNKNOWN", "response has no data object"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{
				"taskId": "task_1", "data": nil,
			}, testCase.body, map[string]any{"status": http.StatusOK})
			require.NoError(t, err)
			encoded, err := common.Marshal(value)
			require.NoError(t, err)
			var result map[string]any
			require.NoError(t, common.Unmarshal(encoded, &result))
			assert.Equal(t, testCase.status, result["status"])
			if testCase.reason == "" {
				assert.NotContains(t, result, "reason")
			} else {
				assert.Equal(t, testCase.reason, result["reason"])
			}
		})
	}
}

// The finished video requires the channel key, so it is always fetched through
// the gateway content route instead of the vendor's signed result_url.
func TestNskevenDownloadsContentWithTheChannelKey(t *testing.T) {
	registry, plugin := newNskevenPlugin(t)

	// Both vendor routes are served by this built-in plugin at the vendor's own
	// path, so a client already integrated with the vendor keeps its URLs.
	submit, found := registry.Generation().LookupDeclaredRoute(http.MethodPost, "/v1/video/generations")
	require.True(t, found)
	assert.Equal(t, jsplugin.RouteTypeSubmit, submit.Route.Type)

	binding, found := registry.Generation().LookupDeclaredRoute(http.MethodGet, "/v1/video/generations/:task_id")
	require.True(t, found)
	assert.Equal(t, jsplugin.RouteTypeQuery, binding.Route.Type)

	value, err := plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
		"artifactKey":    "video",
		"upstreamTaskId": "task_385412",
		"baseUrl":        nskevenBaseURL,
		"apiKey":         "sk-test",
		"clientRequest":  map[string]any{"method": "GET", "headers": map[string]any{}},
	})
	require.NoError(t, err)
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var descriptor map[string]any
	require.NoError(t, common.Unmarshal(encoded, &descriptor))
	assert.Equal(t, nskevenBaseURL+"/v1/videos/task_385412/content", descriptor["url"])
	headers, ok := descriptor["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bearer sk-test", headers["Authorization"])
}

// Every client-visible field answers with the gateway's signed artifact URL.
// The vendor's own result_url needs the channel key, so exposing it would both
// break the client and leak the upstream address.
func TestNskevenNeverLeaksTheVendorURL(t *testing.T) {
	_, plugin := newNskevenPlugin(t)

	const gatewayURL = "https://gateway.example/v1/tasks/task_public/artifacts/video/content?access=signed"
	const vendorURL = "https://newapi.nskeven.uk/v1/videos/task_385412/content"
	finishedTask := map[string]any{
		"task_id":    "task_public",
		"status":     "SUCCESS",
		"progress":   "100%",
		"created_at": 10,
		"updated_at": 20,
		"data": map[string]any{
			"code":    "success",
			"message": "",
			"data": map[string]any{
				"task_id":    "task_385412",
				"status":     "SUCCESS",
				"progress":   "100%",
				"result_url": vendorURL,
			},
		},
	}
	queryContext := map[string]any{
		"artifacts": map[string]any{
			"video": map[string]any{"key": "video", "type": "video", "mimeType": "video/mp4", "url": gatewayURL},
		},
	}

	callJSON := func(root string, path []string, ctx map[string]any, task map[string]any) string {
		t.Helper()
		value, err := plugin.Engine.CallPath(t.Context(), root, path, ctx, task)
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		return string(encoded)
	}

	t.Run("native query", func(t *testing.T) {
		encoded := callJSON("native", []string{"videoStatus"}, queryContext, finishedTask)
		assert.Contains(t, encoded, "gateway.example")
		assert.NotContains(t, encoded, "newapi.nskeven.uk")
	})

	t.Run("openai video retrieve", func(t *testing.T) {
		encoded := callJSON("protocols", []string{"openai_video", "render"}, queryContext, finishedTask)
		assert.NotContains(t, encoded, "newapi.nskeven.uk")
		assert.NotContains(t, encoded, "\"url\"")
	})

	t.Run("responses final", func(t *testing.T) {
		encoded := callJSON("protocols", []string{"openai_responses", "renderFinal"}, queryContext, finishedTask)
		assert.Contains(t, encoded, "gateway.example")
		assert.NotContains(t, encoded, "newapi.nskeven.uk")
	})

	t.Run("responses stream", func(t *testing.T) {
		encoded := callJSON("protocols", []string{"openai_responses", "renderEvents"}, queryContext, finishedTask)
		assert.Contains(t, encoded, "gateway.example")
		assert.NotContains(t, encoded, "newapi.nskeven.uk")
	})
}
