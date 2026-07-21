package service

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

type meshyProxyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f meshyProxyRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func meshyProxyJSONResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func configureMeshyImageProxyTest(t *testing.T, resolve func(*http.Request) *http.Response) *atomic.Int32 {
	t.Helper()

	var uploadCount atomic.Int32
	client := &http.Client{Transport: meshyProxyRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "Bearer meshy-key", r.Header.Get("Authorization"))
		assert.Equal(t, "/upload-image", r.URL.Path)
		uploadCount.Add(1)
		file, _, err := r.FormFile("file")
		if !assert.NoError(t, err) {
			return meshyProxyJSONResponse(r, http.StatusBadRequest, `{"error":"missing image"}`), nil
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if !assert.NoError(t, err) || !assert.Equal(t, "image/png", http.DetectContentType(data)) {
			return meshyProxyJSONResponse(r, http.StatusBadRequest, `{"error":"invalid image"}`), nil
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if !assert.NoError(t, err) ||
			!assert.GreaterOrEqual(t, config.Width, meshyMinimumImageDimension) ||
			!assert.GreaterOrEqual(t, config.Height, meshyMinimumImageDimension) {
			return meshyProxyJSONResponse(r, http.StatusBadRequest, `{"error":"image dimensions too small"}`), nil
		}
		if resolve != nil {
			return resolve(r), nil
		}
		return meshyProxyJSONResponse(r, http.StatusOK, `{"image_id":"image-id.png","url":"https://assets.example/original.png?sign=temporary","account_id":1}`), nil
	})}

	previousClient := httpClient
	previousEnabled := system_setting.WorkerMeshyImageProxyEnabled
	previousBaseURL := system_setting.WorkerMeshyImageProxyBaseURL
	previousAPIKey := system_setting.WorkerMeshyImageProxyAPIKey
	httpClient = client
	system_setting.WorkerMeshyImageProxyEnabled = true
	system_setting.WorkerMeshyImageProxyBaseURL = "https://meshy.test"
	system_setting.WorkerMeshyImageProxyAPIKey = "meshy-key"
	t.Cleanup(func() {
		httpClient = previousClient
		system_setting.WorkerMeshyImageProxyEnabled = previousEnabled
		system_setting.WorkerMeshyImageProxyBaseURL = previousBaseURL
		system_setting.WorkerMeshyImageProxyAPIKey = previousAPIKey
	})

	return &uploadCount
}

func newMeshyProxyContext(method, path, body string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestNormalizeMeshyUploadImagePadsOnlyUndersizedEdges(t *testing.T) {
	var undersized bytes.Buffer
	require.NoError(t, png.Encode(&undersized, image.NewRGBA(image.Rect(0, 0, 767, 31))))

	normalized, mimeType, err := normalizeMeshyUploadImage(undersized.Bytes(), "image/png")
	require.NoError(t, err)
	assert.Equal(t, "image/png", mimeType)
	config, _, err := image.DecodeConfig(bytes.NewReader(normalized))
	require.NoError(t, err)
	assert.Equal(t, 767, config.Width)
	assert.Equal(t, meshyMinimumImageDimension, config.Height)

	var valid bytes.Buffer
	require.NoError(t, png.Encode(&valid, image.NewRGBA(image.Rect(0, 0, 767, meshyMinimumImageDimension))))
	preserved, preservedMimeType, err := normalizeMeshyUploadImage(valid.Bytes(), "image/png")
	require.NoError(t, err)
	assert.Equal(t, "image/png", preservedMimeType)
	assert.Equal(t, valid.Bytes(), preserved)
}

func TestRewriteMeshyImageProxyRequestSupportsRelayProtocols(t *testing.T) {
	uploadCount := configureMeshyImageProxyTest(t, nil)
	dataURL := "data:image/png;base64," + testPNGBase64
	proxyURL := "https://assets.example/original.png?sign=temporary"

	tests := []struct {
		name     string
		path     string
		body     string
		expected string
	}{
		{
			name:     "OpenAI image URL",
			path:     "/v1/chat/completions",
			body:     `{"messages":[{"content":[{"type":"image_url","image_url":{"url":"` + dataURL + `"}}]}]}`,
			expected: `{"messages":[{"content":[{"type":"image_url","image_url":{"url":"` + proxyURL + `"}}]}]}`,
		},
		{
			name:     "Claude base64 source",
			path:     "/v1/messages",
			body:     `{"messages":[{"content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + testPNGBase64 + `"}}]}]}`,
			expected: `{"messages":[{"content":[{"type":"image","source":{"type":"url","url":"` + proxyURL + `"}}]}]}`,
		},
		{
			name:     "Gemini inline data",
			path:     "/v1beta/models/gemini-2.5-flash:generateContent",
			body:     `{"contents":[{"parts":[{"inlineData":{"mimeType":"image/png","data":"` + testPNGBase64 + `"}}]}]}`,
			expected: `{"contents":[{"parts":[{"fileData":{"mimeType":"image/png","fileUri":"` + proxyURL + `"}}]}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newMeshyProxyContext(http.MethodPost, test.path, test.body)
			require.NoError(t, RewriteMeshyImageProxyRequest(c))

			storage, err := common.GetBodyStorage(c)
			require.NoError(t, err)
			rewritten, err := storage.Bytes()
			require.NoError(t, err)
			assert.JSONEq(t, test.expected, string(rewritten))
			common.CleanupBodyStorage(c)
		})
	}

	assert.Equal(t, int32(len(tests)), uploadCount.Load())
}

func TestRewriteMeshyImageProxyResponseSupportsFinalImageShapes(t *testing.T) {
	uploadCount := configureMeshyImageProxyTest(t, nil)
	proxyURL := "https://assets.example/original.png?sign=temporary"

	tests := []struct {
		name     string
		path     string
		body     string
		expected string
	}{
		{
			name:     "OpenAI Images",
			path:     "/v1/images/generations",
			body:     `{"data":[{"b64_json":"` + testPNGBase64 + `"}]}`,
			expected: `{"data":[{"url":"` + proxyURL + `"}]}`,
		},
		{
			name:     "OpenAI Responses image call",
			path:     "/v1/responses",
			body:     `{"output":[{"type":"image_generation_call","result":"` + testPNGBase64 + `"}]}`,
			expected: `{"output":[{"type":"image_generation_call","result":"` + proxyURL + `"}]}`,
		},
		{
			name:     "Claude image block",
			path:     "/v1/messages",
			body:     `{"content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + testPNGBase64 + `"}}]}`,
			expected: `{"content":[{"type":"image","source":{"type":"url","url":"` + proxyURL + `"}}]}`,
		},
		{
			name:     "Gemini inline data",
			path:     "/v1beta/models/gemini-2.5-flash:generateContent",
			body:     `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"` + testPNGBase64 + `"}}]}}]}`,
			expected: `{"candidates":[{"content":{"parts":[{"fileData":{"mimeType":"image/png","fileUri":"` + proxyURL + `"}}]}}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newMeshyProxyContext(http.MethodPost, test.path, `{}`)
			rewritten, err := RewriteMeshyImageProxyResponse(c, []byte(test.body))
			require.NoError(t, err)
			assert.JSONEq(t, test.expected, string(rewritten))
		})
	}

	assert.Equal(t, int32(len(tests)), uploadCount.Load())
}

func TestRewriteMeshyImageProxyResponsePreservesExplicitAndPartialBase64(t *testing.T) {
	uploadCount := configureMeshyImageProxyTest(t, nil)
	body := []byte(`{"type":"image_generation.partial_image","b64_json":"` + testPNGBase64 + `"}`)

	c := newMeshyProxyContext(http.MethodPost, "/v1/images/generations", `{}`)
	rewritten, err := RewriteMeshyImageProxyResponse(c, body)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(body, rewritten))

	c = newMeshyProxyContext(http.MethodPost, "/v1/images/generations", `{}`)
	MarkMeshyImageProxyBase64Response(c, &dto.ImageRequest{ResponseFormat: "b64_json"})
	rewritten, err = RewriteMeshyImageProxyResponse(c, []byte(`{"data":[{"b64_json":"`+testPNGBase64+`"}]}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"data":[{"b64_json":"`+testPNGBase64+`"}]}`, string(rewritten))
	assert.Zero(t, uploadCount.Load())
}

func TestRewriteMeshyImageProxySkipsMeshy2APIChannel(t *testing.T) {
	uploadCount := configureMeshyImageProxyTest(t, nil)
	dataURL := "data:image/png;base64," + testPNGBase64

	c := newMeshyProxyContext(http.MethodPost, "/v1/images/generations", `{"prompt":"test","image":"`+dataURL+`"}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeMeshy2API)
	require.NoError(t, RewriteMeshyImageProxyRequest(c))

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	rewrittenRequest, err := storage.Bytes()
	require.NoError(t, err)
	assert.JSONEq(t, `{"prompt":"test","image":"`+dataURL+`"}`, string(rewrittenRequest))
	common.CleanupBodyStorage(c)

	responseBody := []byte(`{"data":[{"b64_json":"` + testPNGBase64 + `"}]}`)
	rewrittenResponse, err := RewriteMeshyImageProxyResponse(c, responseBody)
	require.NoError(t, err)
	assert.Equal(t, responseBody, rewrittenResponse)
	assert.Zero(t, uploadCount.Load())
}

func TestRewriteMeshyImageProxyResponseRejectsInvalidUploadURL(t *testing.T) {
	configureMeshyImageProxyTest(t, func(r *http.Request) *http.Response {
		return meshyProxyJSONResponse(r, http.StatusOK, `{"image_id":"image-id.png","url":"javascript:alert(1)","account_id":1}`)
	})
	c := newMeshyProxyContext(http.MethodPost, "/v1/images/generations", `{}`)

	_, err := RewriteMeshyImageProxyResponse(c, []byte(`{"data":[{"b64_json":"`+testPNGBase64+`"}]}`))
	require.EqualError(t, err, "Meshy image upload returned an invalid URL")
}
