package volcengine

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertSeedreamImageEditIncludesUploadedImage(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	imagePart, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("test image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest("POST", "/v1/images/edits", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	result, err := (&Adaptor{}).ConvertImageRequest(context, &relaycommon.RelayInfo{
		RelayMode: constant.RelayModeImagesEdits,
	}, dto.ImageRequest{Model: "seedream-5-0", Prompt: "make it blue"})
	require.NoError(t, err)
	converted, ok := result.(dto.ImageRequest)
	require.True(t, ok)

	var image string
	require.NoError(t, common.Unmarshal(converted.Image, &image))
	assert.Equal(t, "data:application/octet-stream;base64,dGVzdCBpbWFnZQ==", image)
}
