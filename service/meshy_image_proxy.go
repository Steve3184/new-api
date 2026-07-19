package service

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const (
	meshyImageProxySkipResponseKey = "meshy_image_proxy_skip_response"
	meshyImageProxyCacheKey        = "meshy_image_proxy_upload_cache"
)

type meshyImageUploadResponse struct {
	URL string `json:"url"`
}

func MeshyImageProxyEnabled() bool {
	return system_setting.WorkerMeshyImageProxyEnabled &&
		strings.TrimSpace(system_setting.WorkerMeshyImageProxyBaseURL) != "" &&
		strings.TrimSpace(system_setting.WorkerMeshyImageProxyAPIKey) != ""
}

func isMeshyImageProxyPath(requestPath string) bool {
	requestPath = strings.TrimSpace(requestPath)
	switch requestPath {
	case "/v1/chat/completions", "/v1/responses", "/v1/messages",
		"/v1/images/generations", "/v1/images/edits", "/v1/edits",
		"/pg/chat/completions", "/pg/images/generations", "/pg/images/edits":
		return true
	}
	return (strings.HasPrefix(requestPath, "/v1/models/") || strings.HasPrefix(requestPath, "/v1beta/models/")) &&
		(strings.Contains(requestPath, ":generateContent") || strings.Contains(requestPath, ":streamGenerateContent"))
}

func RewriteMeshyImageProxyRequest(c *gin.Context) error {
	if !MeshyImageProxyEnabled() || c == nil || c.Request == nil || !isMeshyImageProxyPath(c.Request.URL.Path) {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
		return nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	body, err := storage.Bytes()
	if err != nil {
		return err
	}
	var payload any
	if err = common.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if explicitlyRequestsBase64(payload) {
		c.Set(meshyImageProxySkipResponseKey, true)
	}

	changed, err := rewriteMeshyImageValues(c, payload, true)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	rewritten, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	return common.ReplaceRequestBody(c, rewritten)
}

func RewriteMeshyImageProxyResponse(c *gin.Context, body []byte) ([]byte, error) {
	if !MeshyImageProxyEnabled() || c == nil || c.Request == nil || !isMeshyImageProxyPath(c.Request.URL.Path) || c.GetBool(meshyImageProxySkipResponseKey) {
		return body, nil
	}
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return body, nil
	}
	changed, err := rewriteMeshyImageValues(c, payload, false)
	if err != nil {
		return nil, err
	}
	if !changed {
		return body, nil
	}
	return common.Marshal(payload)
}

func RewriteMeshyImageProxyResponseOrOriginal(c *gin.Context, body []byte) []byte {
	rewritten, err := RewriteMeshyImageProxyResponse(c, body)
	if err != nil {
		logger.LogWarn(c, "Meshy image proxy response rewrite failed: "+err.Error())
		return body
	}
	return rewritten
}

func MarkMeshyImageProxyBase64Response(c *gin.Context, request any) {
	if !MeshyImageProxyEnabled() || c == nil || request == nil {
		return
	}
	data, err := common.Marshal(request)
	if err != nil {
		return
	}
	var payload any
	if common.Unmarshal(data, &payload) == nil && explicitlyRequestsBase64(payload) {
		c.Set(meshyImageProxySkipResponseKey, true)
	}
}

func explicitlyRequestsBase64(payload any) bool {
	root, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"response_format", "responseFormat"} {
		value := strings.ToLower(strings.TrimSpace(common.Interface2String(root[key])))
		if value == "b64_json" || value == "base64" {
			return true
		}
	}
	return false
}

func rewriteMeshyImageValues(c *gin.Context, value any, request bool) (bool, error) {
	switch current := value.(type) {
	case []any:
		changed := false
		for _, item := range current {
			itemChanged, err := rewriteMeshyImageValues(c, item, request)
			if err != nil {
				return false, err
			}
			changed = changed || itemChanged
		}
		return changed, nil
	case map[string]any:
		if !request && strings.Contains(strings.ToLower(common.Interface2String(current["type"])), "partial_image") {
			return false, nil
		}
		if changed, err := rewriteClaudeImageSource(c, current); changed || err != nil {
			return changed, err
		}
		changed, err := rewriteGeminiInlineImage(c, current)
		if err != nil {
			return false, err
		}

		if !request {
			if raw, ok := current["b64_json"].(string); ok && isImageBase64(raw) {
				proxyURL, uploadErr := uploadMeshyBase64Image(c, raw)
				if uploadErr != nil {
					return false, uploadErr
				}
				delete(current, "b64_json")
				current["url"] = proxyURL
				changed = true
			}
			for _, key := range []string{"image_base64", "b64_image"} {
				raw, ok := current[key].(string)
				if !ok || !isImageBase64(raw) {
					continue
				}
				proxyURL, uploadErr := uploadMeshyBase64Image(c, raw)
				if uploadErr != nil {
					return false, uploadErr
				}
				delete(current, key)
				current["image_url"] = proxyURL
				changed = true
			}
			if strings.EqualFold(common.Interface2String(current["type"]), "image_generation_call") {
				if raw, ok := current["result"].(string); ok && isImageBase64(raw) {
					proxyURL, uploadErr := uploadMeshyBase64Image(c, raw)
					if uploadErr != nil {
						return false, uploadErr
					}
					current["result"] = proxyURL
					changed = true
				}
			}
		}

		for key, child := range current {
			if text, ok := child.(string); ok {
				isImageResult := strings.EqualFold(key, "result") &&
					strings.EqualFold(common.Interface2String(current["type"]), "image_generation_call")
				if isImageDataURL(text) && (isRawImageField(key) || isImageResult) {
					proxyURL, uploadErr := uploadMeshyBase64Image(c, text)
					if uploadErr != nil {
						return false, uploadErr
					}
					current[key] = proxyURL
					changed = true
					continue
				}
				if isRawImageField(key) && isImageBase64(text) {
					proxyURL, uploadErr := uploadMeshyBase64Image(c, text)
					if uploadErr != nil {
						return false, uploadErr
					}
					current[key] = proxyURL
					changed = true
				}
				continue
			}
			childChanged, childErr := rewriteMeshyImageValues(c, child, request)
			if childErr != nil {
				return false, childErr
			}
			changed = changed || childChanged
		}
		return changed, nil
	default:
		return false, nil
	}
}

func rewriteClaudeImageSource(c *gin.Context, source map[string]any) (bool, error) {
	if !strings.EqualFold(common.Interface2String(source["type"]), "base64") {
		return false, nil
	}
	mimeType := strings.ToLower(strings.TrimSpace(common.Interface2String(source["media_type"])))
	data, ok := source["data"].(string)
	if !ok || !strings.HasPrefix(mimeType, "image/") || !isImageBase64(data) {
		return false, nil
	}
	proxyURL, err := uploadMeshyBase64Image(c, data)
	if err != nil {
		return false, err
	}
	delete(source, "data")
	delete(source, "media_type")
	source["type"] = "url"
	source["url"] = proxyURL
	return true, nil
}

func rewriteGeminiInlineImage(c *gin.Context, part map[string]any) (bool, error) {
	for _, key := range []string{"inlineData", "inline_data"} {
		inline, ok := part[key].(map[string]any)
		if !ok {
			continue
		}
		mimeKey := "mimeType"
		fileKey := "fileData"
		fileMimeKey := "mimeType"
		fileURIKey := "fileUri"
		if key == "inline_data" {
			mimeKey = "mime_type"
			fileKey = "file_data"
			fileMimeKey = "mime_type"
			fileURIKey = "file_uri"
		}
		mimeType := strings.ToLower(strings.TrimSpace(common.Interface2String(inline[mimeKey])))
		data, dataOK := inline["data"].(string)
		if !dataOK || !strings.HasPrefix(mimeType, "image/") || !isImageBase64(data) {
			continue
		}
		proxyURL, err := uploadMeshyBase64Image(c, data)
		if err != nil {
			return false, err
		}
		delete(part, key)
		part[fileKey] = map[string]any{
			fileMimeKey: mimeType,
			fileURIKey:  proxyURL,
		}
		return true, nil
	}
	return false, nil
}

func isRawImageField(key string) bool {
	switch strings.ToLower(key) {
	case "image", "image_url", "image_base64", "b64_json", "url", "input_image", "input_reference":
		return true
	default:
		return false
	}
}

func isImageDataURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "data:image/") && strings.Contains(value, ";base64,")
}

func splitBase64Image(value string) (string, string) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "data:") {
		return value, ""
	}
	comma := strings.IndexByte(value, ',')
	if comma < 0 {
		return value, ""
	}
	header := value[:comma]
	if !strings.Contains(strings.ToLower(header), ";base64") {
		return value, ""
	}
	mimeType := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(header, ";", 2)[0], "data:"))
	return value[comma+1:], mimeType
}

func base64Encoding(value string) *base64.Encoding {
	if len(value)%4 == 0 {
		return base64.StdEncoding
	}
	return base64.RawStdEncoding
}

func detectBase64ImageMime(value string) string {
	value, _ = splitBase64Image(value)
	decoder := base64.NewDecoder(base64Encoding(value), strings.NewReader(value))
	header := make([]byte, 512)
	n, err := io.ReadFull(decoder, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return ""
	}
	if n == 0 {
		return ""
	}
	mimeType := http.DetectContentType(header[:n])
	if !strings.HasPrefix(mimeType, "image/") {
		return ""
	}
	return mimeType
}

func isImageBase64(value string) bool {
	if len(value) < 16 {
		return false
	}
	return detectBase64ImageMime(value) != ""
}

func uploadMeshyBase64Image(c *gin.Context, value string) (string, error) {
	base64Data, _ := splitBase64Image(value)
	mimeType := detectBase64ImageMime(base64Data)
	if mimeType == "" {
		return "", errors.New("invalid base64 image data")
	}
	hash := sha256.Sum256(common.StringToByteSlice(base64Data))
	cacheID := fmt.Sprintf("%x", hash[:])
	if cached, ok := c.Get(meshyImageProxyCacheKey); ok {
		if values, ok := cached.(map[string]string); ok && values[cacheID] != "" {
			return values[cacheID], nil
		}
	}

	extension := ".img"
	if extensions, _ := mime.ExtensionsByType(mimeType); len(extensions) > 0 {
		extension = extensions[0]
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	go func() {
		part, err := multipartWriter.CreateFormFile("file", "image"+extension)
		if err == nil {
			decoder := base64.NewDecoder(base64Encoding(base64Data), strings.NewReader(base64Data))
			_, err = io.Copy(part, decoder)
		}
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		_ = pipeWriter.CloseWithError(err)
	}()

	endpoint := strings.TrimRight(strings.TrimSpace(system_setting.WorkerMeshyImageProxyBaseURL), "/") + "/upload-image"
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(system_setting.WorkerMeshyImageProxyAPIKey))
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	client := GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("Meshy image upload failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return "", fmt.Errorf("read Meshy image upload response: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Meshy image upload returned status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var uploadResponse meshyImageUploadResponse
	if err = common.Unmarshal(responseBody, &uploadResponse); err != nil {
		return "", fmt.Errorf("decode Meshy image upload response: %w", err)
	}
	uploadURL, err := url.Parse(strings.TrimSpace(uploadResponse.URL))
	if err != nil || (uploadURL.Scheme != "http" && uploadURL.Scheme != "https") || uploadURL.Host == "" {
		return "", errors.New("Meshy image upload returned an invalid URL")
	}

	cache, _ := c.Get(meshyImageProxyCacheKey)
	values, _ := cache.(map[string]string)
	if values == nil {
		values = make(map[string]string)
		c.Set(meshyImageProxyCacheKey, values)
	}
	values[cacheID] = uploadURL.String()
	return uploadURL.String(), nil
}
