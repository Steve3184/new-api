/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package router

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	taskplugin "github.com/QuantumNous/new-api/relay/channel/task/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gin panics at registration time when routes sharing a path position use
// different wildcard names, which unit tests that build their own routers
// never catch. Registering against a real engine is the only guard.
func TestSetTaskRouterRegistersWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NotPanics(t, func() { SetTaskRouter(engine) })

	routes := engine.Routes()
	require.Len(t, routes, 7)
	actual := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	assert.Contains(t, actual, http.MethodPost+" /v1/tasks/:key")
	assert.Contains(t, actual, http.MethodGet+" /v1/tasks/:key")
	assert.Contains(t, actual, http.MethodGet+" /v1/tasks/:key/artifacts")
	assert.Contains(t, actual, http.MethodGet+" /v1/tasks/:key/artifacts/:artifact_key/content")
	assert.Contains(t, actual, http.MethodHead+" /v1/tasks/:key/artifacts/:artifact_key/content")
	for route := range actual {
		assert.NotContains(t, route, "/native/")
	}
}

func TestSignedInputMediaSubmissionAndDownload(t *testing.T) {
	previousSecret, previousAddress := common.CryptoSecret, system_setting.TaskPublicAddress
	common.CryptoSecret, system_setting.TaskPublicAddress = "input-route-test-secret", "https://media.example"
	t.Cleanup(func() { common.CryptoSecret, system_setting.TaskPublicAddress = previousSecret, previousAddress })
	source := `export const meta = {apiVersion:1,key:"signed-input-test",name:"Input",version:"1.0.0",author:{name:"Test"},models:["m"],fetchMode:"per_task",requiredCapabilities:["signed-input-media@1"]};
export function buildSubmitRequest(ctx) {return {url:ctx.baseUrl+"/submit",body:{image:{__mediaData:ctx.requestBody.ref,kind:"image"},nested:[{__mediaData:ctx.requestBody.ref,kind:"image"}],url:"https://cdn.example/unchanged.png"}};}
export function parseSubmitResponse(){return {taskId:"one"};}
export function buildQueryRequest(){return {};}
export function parseTaskResult(){return {status:"SUCCESS"};}`
	plugin, err := pluginruntime.NewRegistry().Register(source, pluginruntime.Options{})
	require.NoError(t, err)
	payload := []byte("\x89PNG\r\n\x1a\nimage-payload")
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(payload)
	normalized := map[string]any{"ref": dataURI}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", normalized)
	info := &relaycommon.RelayInfo{UserId: 1, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://provider.example"}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	adaptor := taskplugin.New(plugin)
	adaptor.Init(info)
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	var request map[string]any
	require.NoError(t, common.Unmarshal(encoded, &request))
	mediaURL := request["image"].(string)
	assert.True(t, strings.HasPrefix(mediaURL, "https://media.example/v1/task-inputs/"))
	parsedMedia, parseErr := url.Parse(mediaURL)
	require.NoError(t, parseErr)
	assert.True(t, strings.HasSuffix(parsedMedia.Path, ".png"), "upstream media validators need a file extension")
	assert.Equal(t, "https://cdn.example/unchanged.png", request["url"])
	assert.Equal(t, dataURI, normalized["ref"], "normalized facts must not be rewritten")
	assert.NotContains(t, string(encoded), "base64")
	assert.NotContains(t, string(encoded), common.CryptoSecret)
	assert.Equal(t, mediaURL, request["nested"].([]any)[0], "identical inputs can share one URL")
	engine := gin.New()
	SetTaskRouter(engine)
	parsed, err := url.Parse(mediaURL)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, method, target, byteRange string
		status                          int
	}{
		{"GET", http.MethodGet, parsed.RequestURI(), "", http.StatusOK},
		{"HEAD", http.MethodHead, parsed.RequestURI(), "", http.StatusOK},
		{"Range", http.MethodGet, parsed.RequestURI(), "bytes=0-7", http.StatusPartialContent},
		{"missing", http.MethodGet, parsed.Path, "", http.StatusUnauthorized},
		{"tampered", http.MethodGet, parsed.RequestURI() + "x", "", http.StatusNotFound},
		{"duplicate", http.MethodGet, parsed.RequestURI() + "&access=x", "", http.StatusNotFound},
		{"other object", http.MethodGet, strings.Replace(parsed.RequestURI(), "/input-", "x/input-", 1), "", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.target, nil)
			if tc.byteRange != "" {
				req.Header.Set("Range", tc.byteRange)
			}
			engine.ServeHTTP(recorder, req)
			assert.Equal(t, tc.status, recorder.Code)
			assert.NotContains(t, req.RequestURI, "access=", "capabilities must be removed before request logging")
			if tc.status == http.StatusOK || tc.status == http.StatusPartialContent {
				assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
				assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
				switch {
				case tc.method == http.MethodHead:
					assert.Empty(t, recorder.Body.Bytes())
				case tc.byteRange != "":
					assert.Equal(t, payload[:8], recorder.Body.Bytes())
				default:
					assert.Equal(t, payload, recorder.Body.Bytes())
				}
			}
		})
	}
}
