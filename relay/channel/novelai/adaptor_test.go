package novelai

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/ugorji/go/codec"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImageRequestUsesV5ParamsVersion(t *testing.T) {
	request := dto.ImageRequest{Model: "nai-diffusion-5-full", Prompt: "test"}
	got, err := openAIImagePayload(request, &relaycommon.RelayInfo{})
	require.NoError(t, err)
	assert.JSONEq(t, "4", string(got.Parameters["params_version"]))
}

func TestDoResponseStreamEndpointAcceptsZipImageResponse(t *testing.T) {
	archive := novelAIArchive(t, map[string][]byte{"image.png": []byte("png-image")})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/ai/generate-image-stream", nil)
	info := &relaycommon.RelayInfo{
		Request:   &dto.ImageRequest{ResponseFormat: "b64_json"},
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1700000000, 0),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "nai-diffusion-4-5-full",
		},
	}
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/zip"}}, Body: io.NopCloser(bytes.NewReader(archive))}

	_, apiErr := (&Adaptor{}).DoResponse(c, response, info)
	require.Nil(t, apiErr)
	assert.Equal(t, archive, recorder.Body.Bytes())
}

func TestNativeImageRequestPreservesTopLevelFields(t *testing.T) {
	request := dto.ImageRequest{Extra: map[string]json.RawMessage{
		"novelai_native_request": json.RawMessage(`{"input":"a blue cup","model":"nai-diffusion-3","action":"generate","parameters":{"steps":20},"url":"https://image.novelai.net/reference.png"}`),
	}}
	context := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/ai/generate-image", nil)
	got, err := (&Adaptor{}).ConvertImageRequest(context, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "nai-diffusion-3",
		},
	}, request)
	require.NoError(t, err)
	body, err := common.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"input":"a blue cup","model":"nai-diffusion-3","action":"generate","parameters":{"steps":20},"url":"https://image.novelai.net/reference.png"}`, string(body))
}

func TestDecodeZipImagesRejectsExcessiveAggregateSize(t *testing.T) {
	largeImage := bytes.Repeat([]byte{'x'}, 26<<20)
	archive := novelAIArchive(t, map[string][]byte{"one.png": largeImage, "two.png": largeImage})
	_, err := decodeZipImages(archive)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total image size")
}

func TestGetRequestURLSelectsZipOrStreamByModel(t *testing.T) {
	adaptor := &Adaptor{}
	for _, test := range []struct {
		model string
		want  string
	}{
		{model: "nai-diffusion-3", want: "https://image.novelai.net/ai/generate-image"},
		{model: "nai-diffusion-4-5-full", want: "https://image.novelai.net/ai/generate-image-stream"},
		{model: "nai-diffusion-5-full", want: "https://image.novelai.net/ai/generate-image-stream"},
	} {
		t.Run(test.model, func(t *testing.T) {
			got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeImagesGenerations,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: test.model,
				},
			})
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestDecodeMessagePackFinalImages(t *testing.T) {
	var handle codec.MsgpackHandle
	handle.MapType = reflect.TypeFor[map[string]any]()
	var encoded bytes.Buffer
	event := map[string]any{"event_type": "final", "samp_ix": 0, "image": []byte("png-image")}
	require.NoError(t, codec.NewEncoder(&encoded, &handle).Encode(event))
	frame := make([]byte, 4+encoded.Len())
	binary.BigEndian.PutUint32(frame[:4], uint32(encoded.Len()))
	copy(frame[4:], encoded.Bytes())

	images, err := decodeMessagePackImages(frame)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, []byte("png-image"), images[0].data)
	assert.Equal(t, "image/png", images[0].mimeType)
}

func TestNativeGenerateImageKeepsRequestedV4Endpoint(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/ai/generate-image",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "nai-diffusion-4-5-full",
		},
	}
	got, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://image.novelai.net/ai/generate-image", got)
}

func TestConvertImageRequestMapsOpenAIFieldsToNovelAI(t *testing.T) {
	steps := json.RawMessage(`24`)
	request := dto.ImageRequest{
		Model:          "nai-diffusion-4-5-full",
		Prompt:         "moonlit city",
		Size:           "832x1216",
		ResponseFormat: "b64_json",
		N:              uintPointer(2),
		Extra:          map[string]json.RawMessage{"steps": steps, "negative_prompt": json.RawMessage(`"blurry"`)},
	}

	context := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	got, err := (&Adaptor{}).ConvertImageRequest(context, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "nai-diffusion-4-5-full",
		},
	}, request)
	require.NoError(t, err)
	body, err := common.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"input":"moonlit city","model":"nai-diffusion-4-5-full","action":"generate","parameters":{"params_version":3,"width":832,"height":1216,"n_samples":2,"scale":6,"sampler":"k_euler_ancestral","steps":24,"stream":"msgpack","v4_prompt":{"caption":{"base_caption":"moonlit city","char_captions":[]},"use_coords":false,"use_order":true},"v4_negative_prompt":{"caption":{"base_caption":"blurry","char_captions":[]},"use_coords":false,"use_order":false}}}`, string(body))
}

func TestDoResponseConvertsNovelAIZipToOpenAIImage(t *testing.T) {
	archive := novelAIArchive(t, map[string][]byte{"image.png": []byte("png-image")})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		Request:   &dto.ImageRequest{ResponseFormat: "b64_json"},
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1700000000, 0),
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	response := &http.Response{StatusCode: http.StatusCreated, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(archive))}

	usage, apiErr := (&Adaptor{}).DoResponse(c, response, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Contains(t, recorder.Body.String(), base64.StdEncoding.EncodeToString([]byte("png-image")))
	assert.Contains(t, recorder.Body.String(), `"created":1700000000`)
	assert.Contains(t, recorder.Body.String(), `"b64_json"`)
	assert.NotContains(t, recorder.Body.String(), `"url":"data:`)
}

func TestValidateMaxStepsOnlyRejectsValuesAbove28WhenEnabled(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		enabled bool
		wantErr bool
	}{
		{name: "28 is allowed", raw: `{"steps":28}`, enabled: true},
		{name: "nested 28 is allowed", raw: `{"parameters":{"steps":28}}`, enabled: true},
		{name: "nested 29 is rejected", raw: `{"parameters":{"steps":29}}`, enabled: true, wantErr: true},
		{name: "nested value cannot be masked by top-level value", raw: `{"steps":28,"parameters":{"steps":29}}`, enabled: true, wantErr: true},
		{name: "fractional steps are rejected", raw: `{"steps":28.5}`, enabled: true, wantErr: true},
		{name: "negative steps are rejected", raw: `{"steps":-1}`, enabled: true, wantErr: true},
		{name: "29 is allowed when disabled", raw: `{"steps":29}`},
		{name: "missing steps is allowed", raw: `{}`, enabled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error
			if test.enabled {
				err = relaycommon.ValidateNovelAISteps([]byte(test.raw), true)
			}
			if test.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "28")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDoResponseRejectsMalformedZip(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("not a zip"))}

	_, apiErr := (&Adaptor{}).DoResponse(c, response, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
}

func TestNovelAIChannelUsesStableChannelType(t *testing.T) {
	assert.Equal(t, 68, constant.ChannelTypeNovelAI)
}

func novelAIArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		require.NoError(t, err)
		_, err = file.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func uintPointer(value uint) *uint { return &value }
