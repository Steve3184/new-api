package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNovelAIProxyAllowsOnlyReadOnlyTagSuggestions(t *testing.T) {
	for _, test := range []struct {
		method   string
		endpoint string
		want     bool
	}{
		{method: http.MethodGet, endpoint: "generate-image/suggest-tags", want: true},
		{method: http.MethodPost, endpoint: "generate-image/suggest-tags"},
		{method: http.MethodPost, endpoint: "augment-image"},
		{method: http.MethodPost, endpoint: "encode-vibe"},
		{method: http.MethodPost, endpoint: "upscale"},
		{method: http.MethodPost, endpoint: "annotate-image"},
		{method: http.MethodGet, endpoint: "user/subscription"},
		{method: http.MethodGet, endpoint: "../user/subscription"},
	} {
		t.Run(test.method+" /v1/ai/"+test.endpoint, func(t *testing.T) {
			assert.Equal(t, test.want, novelAIProxyRouteAllowed(test.method, test.endpoint))
		})
	}
}

// A transport failure must not hand the caller the administrator's Base URL,
// its query string, or the channel key: the error carries the full upstream
// URL and must not be exposed.
func TestNovelAIProxyHidesUpstreamTargetOnTransportFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	upstream := httptest.NewServer(http.NotFoundHandler())
	baseURL := upstream.URL + "?admin_token=upstream-secret"
	upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/ai/generate-image/suggest-tags?prompt=cat&admin_token=upstream-secret", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeNovelAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, baseURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "fixture-channel-key")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})

	NovelAIProxy(c)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "NovelAI upstream request failed")
	for _, leaked := range []string{upstream.URL, "127.0.0.1", "admin_token", "upstream-secret", "fixture-channel-key", "generate-image/suggest-tags"} {
		assert.NotContains(t, body, leaked)
	}
}
