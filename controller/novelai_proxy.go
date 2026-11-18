package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func NovelAIProxy(c *gin.Context) {
	if common.GetContextKeyInt(c, constant.ContextKeyChannelType) != constant.ChannelTypeNovelAI {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "NovelAI route not found", "type": "invalid_request_error"}})
		return
	}
	endpoint := strings.TrimPrefix(c.Request.URL.Path, "/v1/ai/")
	if !novelAIProxyRouteAllowed(c.Request.Method, endpoint) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "NovelAI route not found", "type": "invalid_request_error"}})
		return
	}

	baseURL := strings.TrimRight(common.GetContextKeyString(c, constant.ContextKeyChannelBaseUrl), "/")
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(constant.ChannelTypeNovelAI)
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil || (parsedBase.Scheme != "http" && parsedBase.Scheme != "https") || parsedBase.Host == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "NovelAI channel Base URL is invalid", "type": "server_error"}})
		return
	}
	target := *parsedBase
	target.Path = strings.TrimRight(parsedBase.Path, "/") + "/ai/" + endpoint
	target.RawQuery = c.Request.URL.RawQuery

	var body io.Reader
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read request body", "type": "invalid_request_error"}})
			return
		}
		payload, err := storage.Bytes()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read request body", "type": "invalid_request_error"}})
			return
		}
		body = bytes.NewReader(payload)
	}

	request, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target.String(), body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "Failed to create NovelAI request", "type": "server_error"}})
		return
	}
	request.Header.Set("Authorization", "Bearer "+common.GetContextKeyString(c, constant.ContextKeyChannelKey))
	request.Header.Set("Accept", c.GetHeader("Accept"))
	if contentType := c.GetHeader("Content-Type"); contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	settings, _ := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	client, err := service.GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "Failed to configure NovelAI upstream client", "type": "server_error"}})
		return
	}
	response, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": fmt.Sprintf("NovelAI upstream request failed: %s", err), "type": "upstream_error"}})
		return
	}
	defer response.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Disposition", "Cache-Control", "ETag", "Last-Modified"} {
		if value := response.Header.Get(name); value != "" {
			c.Header(name, value)
		}
	}
	c.Status(response.StatusCode)
	if _, err := io.Copy(c.Writer, response.Body); err != nil {
		return
	}
}

func novelAIProxyRouteAllowed(method, endpoint string) bool {
	return method == http.MethodGet && endpoint == "generate-image/suggest-tags"
}
