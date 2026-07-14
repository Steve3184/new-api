package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertNativeGeminiImageRequestSupportsEditingAndResolution(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	request := dto.ImageRequest{
		Prompt:  "add a blue sky",
		Size:    "2048x2048",
		Quality: "2K",
		Image:   []byte(`"data:image/png;base64,aGVsbG8="`),
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(ctx, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "nano-banana-pro-preview"},
	}, request)
	require.NoError(t, err)
	geminiRequest, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok)
	require.Len(t, geminiRequest.Contents, 1)
	require.Len(t, geminiRequest.Contents[0].Parts, 2)
	assert.Equal(t, "image/png", geminiRequest.Contents[0].Parts[1].InlineData.MimeType)
	assert.Equal(t, "aGVsbG8=", geminiRequest.Contents[0].Parts[1].InlineData.Data)
	assert.JSONEq(t, `{"aspectRatio":"1:1","imageSize":"2K"}`, string(geminiRequest.GenerationConfig.ImageConfig))
}

func TestGeminiNativeImageHandlerReturnsOpenAIImageResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	response := `{
		"candidates":[{"content":{"parts":[{"text":"refined prompt"},{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}],
		"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":20,"totalTokenCount":32}
	}`
	httpResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(response)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "nano-banana-pro-preview",
		},
	}

	usage, apiErr := GeminiNativeImageHandler(ctx, info, httpResponse)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 12, usage.PromptTokens)
	assert.Equal(t, 20, usage.CompletionTokens)

	var imageResponse dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResponse))
	require.Len(t, imageResponse.Data, 1)
	assert.Equal(t, "aW1hZ2U=", imageResponse.Data[0].B64Json)
	assert.Equal(t, "refined prompt", imageResponse.Data[0].RevisedPrompt)
}
