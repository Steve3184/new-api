package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func threeDProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func ThreeDProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		threeDProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query 3D task %s: %s", taskID, err.Error()))
		threeDProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		threeDProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}
	if task.Status != model.TaskStatusSuccess {
		threeDProxyError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		threeDProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	if channel.Type != constant.ChannelTypeMeshy2API {
		threeDProxyError(c, http.StatusBadRequest, "invalid_request_error", "Task is not a Meshy2API 3D task")
		return
	}
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		threeDProxyError(c, http.StatusInternalServerError, "server_error", "Meshy2API base URL is not configured")
		return
	}

	client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		threeDProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy client")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	upstreamURL := fmt.Sprintf("%s/v1/3d/%s/content", baseURL, url.PathEscape(task.GetUpstreamTaskID()))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		threeDProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}
	apiKey := task.PrivateData.Key
	if apiKey == "" {
		apiKey = channel.Key
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := client.Do(request)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch 3D content for task %s: %s", taskID, err.Error()))
		threeDProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch 3D content")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		threeDProxyError(c, http.StatusBadGateway, "upstream_error", fmt.Sprintf("Upstream service returned status %d", response.StatusCode))
		return
	}

	for _, header := range []string{"Content-Type", "Content-Length", "Content-Disposition", "ETag", "Last-Modified"} {
		if value := response.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "model/gltf-binary")
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, response.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream 3D content for task %s: %s", taskID, err.Error()))
	}
}
