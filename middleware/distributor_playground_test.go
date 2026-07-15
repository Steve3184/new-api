package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelRequestPreservesPlaygroundGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
	}{
		{name: "image generation", path: "/pg/images/generations"},
		{name: "speech generation", path: "/pg/audio/speech"},
		{name: "3D generation", path: "/pg/3d"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(
				http.MethodPost,
				test.path,
				bytes.NewBufferString(`{"model":"test-model","group":"image"}`),
			)
			context.Request.Header.Set("Content-Type", gin.MIMEJSON)

			request, shouldSelectChannel, err := getModelRequest(context)

			require.NoError(t, err)
			require.NotNil(t, request)
			assert.True(t, shouldSelectChannel)
			assert.Equal(t, "test-model", request.Model)
			assert.Equal(t, "image", request.Group)
		})
	}
}

func TestGetModelRequestPreservesMultipartPlaygroundGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "test-image-model"))
	require.NoError(t, writer.WriteField("group", "image"))
	imagePart, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("image-data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/pg/images/edits",
		body,
	)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())

	request, shouldSelectChannel, err := getModelRequest(context)

	require.NoError(t, err)
	require.NotNil(t, request)
	assert.True(t, shouldSelectChannel)
	assert.Equal(t, "test-image-model", request.Model)
	assert.Equal(t, "image", request.Group)
}
