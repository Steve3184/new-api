package agnes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
