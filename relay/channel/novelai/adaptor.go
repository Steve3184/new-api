package novelai

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/ugorji/go/codec"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	ChannelName       = "NovelAI"
	maxArchiveSize    = 100 << 20
	maxImageSize      = 50 << 20
	maxTotalImageSize = 50 << 20
)

var ModelList = []string{
	"nai-diffusion-3",
	"nai-diffusion-furry-3",
	"nai-diffusion-4-curated-preview",
	"nai-diffusion-4-full",
	"nai-diffusion-4-5-curated",
	"nai-diffusion-4-5-full",
	"nai-diffusion-5-curated",
	"nai-diffusion-5-full",
}

type Adaptor struct{ openai.Adaptor }

type imagePayload struct {
	Input      string                     `json:"input"`
	Model      string                     `json:"model"`
	Action     string                     `json:"action"`
	Parameters map[string]json.RawMessage `json:"parameters"`
}

type decodedImage struct {
	data     []byte
	mimeType string
}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) GetChannelName() string { return ChannelName }

func (a *Adaptor) GetModelList() []string { return ModelList }

func isStreamModel(model string) bool {
	return strings.HasPrefix(model, "nai-diffusion-4-") || strings.HasPrefix(model, "nai-diffusion-5-")
}

func paramsVersion(model string) int {
	if strings.HasPrefix(model, "nai-diffusion-5-") {
		return 4
	}
	return 3
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("NovelAI adaptor: relay info is nil")
	}
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(constant.ChannelTypeNovelAI)
	}
	if info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return "", fmt.Errorf("unsupported NovelAI relay mode: %d", info.RelayMode)
	}
	if strings.HasSuffix(info.RequestURLPath, "/generate-image-stream") {
		return baseURL + "/ai/generate-image-stream", nil
	}
	if strings.HasSuffix(info.RequestURLPath, "/generate-image") {
		return baseURL + "/ai/generate-image", nil
	}
	if isStreamModel(info.GetUpstreamModelName()) {
		return baseURL + "/ai/generate-image-stream", nil
	}
	return baseURL + "/ai/generate-image", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, headers *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, headers)
	headers.Set("Authorization", "Bearer "+info.ApiKey)
	headers.Set("Accept", "application/zip, application/msgpack, application/octet-stream")
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	if headers.Get("Accept") == "" {
		headers.Set("Accept", "application/zip, application/octet-stream")
	}
	return nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return nil, fmt.Errorf("unsupported NovelAI image relay mode: %d", info.RelayMode)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/ai/") {
		return nativeImagePayload(request, info)
	}
	return openAIImagePayload(request, info)
}

func nativeImagePayload(request dto.ImageRequest, info *relaycommon.RelayInfo) (any, error) {
	raw, ok := request.Extra["novelai_native_request"]
	if !ok {
		return nil, errors.New("NovelAI native image request is missing")
	}
	var payload map[string]json.RawMessage
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode NovelAI image request: %w", err)
	}
	if model := info.GetUpstreamModelName(); model != "" {
		encodedModel, err := common.Marshal(model)
		if err != nil {
			return nil, fmt.Errorf("encode NovelAI upstream model: %w", err)
		}
		payload["model"] = encodedModel
	}
	return payload, nil
}

func openAIImagePayload(request dto.ImageRequest, info *relaycommon.RelayInfo) (imagePayload, error) {
	width, height := 832, 1216
	if request.Size != "" {
		widthText, heightText, ok := strings.Cut(request.Size, "x")
		if !ok {
			return imagePayload{}, fmt.Errorf("unsupported NovelAI image size %q; expected WIDTHxHEIGHT", request.Size)
		}
		parsedWidth, err := strconv.Atoi(widthText)
		if err != nil || parsedWidth <= 0 {
			return imagePayload{}, fmt.Errorf("invalid NovelAI image width %q", widthText)
		}
		parsedHeight, err := strconv.Atoi(heightText)
		if err != nil || parsedHeight <= 0 {
			return imagePayload{}, fmt.Errorf("invalid NovelAI image height %q", heightText)
		}
		width, height = parsedWidth, parsedHeight
	}

	model := request.Model
	if info.GetUpstreamModelName() != "" {
		model = info.GetUpstreamModelName()
	}
	if model == "" {
		model = "nai-diffusion-3"
	}
	n := uint(1)
	if request.N != nil && *request.N > 0 {
		n = *request.N
	}
	parameters := map[string]json.RawMessage{
		"params_version": rawJSON(paramsVersion(model)),
		"width":          rawJSON(width),
		"height":         rawJSON(height),
		"scale":          rawJSON(6),
		"sampler":        rawJSON("k_euler_ancestral"),
		"steps":          rawJSON(28),
		"n_samples":      rawJSON(n),
	}
	if isStreamModel(model) {
		parameters["stream"] = rawJSON("msgpack")
		parameters["v4_prompt"] = rawJSON(map[string]any{
			"caption":    map[string]any{"base_caption": request.Prompt, "char_captions": []any{}},
			"use_coords": false,
			"use_order":  true,
		})
		negativePrompt := ""
		if raw, ok := request.Extra["negative_prompt"]; ok {
			if err := common.Unmarshal(raw, &negativePrompt); err != nil {
				return imagePayload{}, fmt.Errorf("negative_prompt must be a string: %w", err)
			}
		}
		parameters["v4_negative_prompt"] = rawJSON(map[string]any{
			"caption":    map[string]any{"base_caption": negativePrompt, "char_captions": []any{}},
			"use_coords": false,
			"use_order":  false,
		})
	} else if raw, ok := request.Extra["negative_prompt"]; ok {
		parameters["uc"] = raw
	}
	for _, name := range []string{"steps", "scale", "sampler", "seed", "noise_schedule", "sm", "sm_dyn", "dynamic_thresholding", "cfg_rescale", "qualityToggle", "prefer_brownian", "ucPreset", "v4_prompt", "v4_negative_prompt"} {
		if raw, ok := request.Extra[name]; ok {
			parameters[name] = raw
		}
	}
	if isStreamModel(model) {
		parameters["stream"] = rawJSON("msgpack")
	}
	return imagePayload{Input: request.Prompt, Model: model, Action: "generate", Parameters: parameters}, nil
}

func rawJSON(value any) json.RawMessage {
	data, _ := common.Marshal(value)
	return data
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return a.Adaptor.DoResponse(c, resp, info)
	}
	defer service.CloseResponseBodyGracefully(resp)
	streamResponse := strings.HasSuffix(c.Request.URL.Path, "/generate-image-stream") ||
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "msgpack")
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if int64(len(body)) > maxArchiveSize {
		return nil, types.NewErrorWithStatusCode(errors.New("NovelAI image response exceeds the size limit"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	var images []decodedImage
	if bytes.HasPrefix(body, []byte("PK\x03\x04")) {
		images, err = decodeZipImages(body)
	} else if streamResponse || strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "msgpack") {
		images, err = decodeMessagePackImages(body)
	} else {
		images, err = decodeZipImages(body)
	}
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if len(images) == 0 {
		return nil, types.NewErrorWithStatusCode(errors.New("NovelAI returned no final images"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	if strings.HasPrefix(c.Request.URL.Path, "/v1/ai/") {
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			if streamResponse {
				contentType = "application/msgpack"
			} else {
				contentType = "application/zip"
			}
		}
		c.Header("Content-Type", contentType)
		if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
			c.Header("Content-Disposition", disposition)
		}
		c.Status(resp.StatusCode)
		if _, err := c.Writer.Write(body); err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponse)
		}
		info.UpdateImageCount(int64(len(images)))
		return &dto.Usage{}, nil
	}

	response := dto.ImageResponse{Created: info.StartTime.Unix()}
	imageRequest, _ := info.Request.(*dto.ImageRequest)
	wantsBase64 := imageRequest != nil && strings.EqualFold(imageRequest.ResponseFormat, "b64_json")
	for _, image := range images {
		encoded := base64.StdEncoding.EncodeToString(image.data)
		data := dto.ImageData{}
		if wantsBase64 {
			data.B64Json = encoded
		} else {
			data.Url = "data:" + image.mimeType + ";base64," + encoded
		}
		response.Data = append(response.Data, data)
	}
	info.UpdateImageCount(int64(len(images)))
	c.JSON(http.StatusOK, response)
	return &dto.Usage{}, nil
}

func countZipImages(body []byte) int {
	images, err := decodeZipImages(body)
	if err != nil {
		return 0
	}
	return len(images)
}

func decodeZipImages(body []byte) ([]decodedImage, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("NovelAI returned an invalid ZIP archive: %w", err)
	}
	images := make([]decodedImage, 0, len(reader.File))
	var totalImageSize uint64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		switch strings.ToLower(path.Ext(file.Name)) {
		case ".png", ".jpg", ".jpeg", ".webp":
		default:
			continue
		}
		if len(images) >= dto.MaxImageN {
			return nil, fmt.Errorf("NovelAI archive contains more than %d images", dto.MaxImageN)
		}
		if file.UncompressedSize64 > maxImageSize {
			return nil, errors.New("NovelAI image exceeds the size limit")
		}
		if file.UncompressedSize64 > maxTotalImageSize-totalImageSize {
			return nil, errors.New("NovelAI archive exceeds the total image size limit")
		}
		totalImageSize += file.UncompressedSize64
		imageFile, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open NovelAI image archive entry: %w", err)
		}
		image, readErr := io.ReadAll(io.LimitReader(imageFile, maxImageSize+1))
		closeErr := imageFile.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read NovelAI image archive entry: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close NovelAI image archive entry: %w", closeErr)
		}
		if int64(len(image)) > maxImageSize {
			return nil, errors.New("NovelAI image exceeds the size limit")
		}
		mimeType := "image/png"
		switch strings.ToLower(path.Ext(file.Name)) {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".webp":
			mimeType = "image/webp"
		}
		images = append(images, decodedImage{data: image, mimeType: mimeType})
	}
	return images, nil
}

func decodeMessagePackImages(body []byte) ([]decodedImage, error) {
	var handle codec.MsgpackHandle
	handle.MapType = reflect.TypeFor[map[string]any]()
	images := make([]decodedImage, 0)
	var totalImageSize uint64
	for len(body) > 0 {
		if len(body) < 4 {
			return nil, errors.New("NovelAI returned a truncated MessagePack frame header")
		}
		frameSize := binary.BigEndian.Uint32(body[:4])
		body = body[4:]
		if frameSize == 0 || frameSize > maxImageSize || uint64(frameSize) > uint64(len(body)) {
			return nil, errors.New("NovelAI returned an invalid MessagePack frame size")
		}
		var event struct {
			EventType string `codec:"event_type"`
			Message   string `codec:"message"`
			Image     []byte `codec:"image"`
		}
		if err := codec.NewDecoderBytes(body[:frameSize], &handle).Decode(&event); err != nil {
			return nil, fmt.Errorf("decode NovelAI MessagePack event: %w", err)
		}
		body = body[frameSize:]
		switch event.EventType {
		case "error":
			message := event.Message
			if message == "" {
				message = "NovelAI image generation stream failed"
			}
			return nil, errors.New(message)
		case "final":
			image := event.Image
			if len(image) == 0 {
				return nil, errors.New("NovelAI returned a final event without image bytes")
			}
			if uint64(len(image)) > maxTotalImageSize-totalImageSize {
				return nil, errors.New("NovelAI stream exceeds the total image size limit")
			}
			totalImageSize += uint64(len(image))
			if len(images) >= dto.MaxImageN {
				return nil, fmt.Errorf("NovelAI stream contains more than %d final images", dto.MaxImageN)
			}
			mimeType := "image/png"
			if bytes.HasPrefix(image, []byte("\xff\xd8\xff")) {
				mimeType = "image/jpeg"
			} else if len(image) >= 12 && string(image[:4]) == "RIFF" && string(image[8:12]) == "WEBP" {
				mimeType = "image/webp"
			}
			images = append(images, decodedImage{data: image, mimeType: mimeType})
		}
	}
	return images, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return a.Adaptor.ConvertOpenAIRequest(c, info, request)
}
