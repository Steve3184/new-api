package controller

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	provider "github.com/QuantumNous/new-api/relay/channel/unrealspeech"

	"github.com/gin-gonic/gin"
)

func audioSpeechProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func AudioSpeechProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		audioSpeechProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query speech task %s: %s", taskID, err.Error()))
		audioSpeechProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		audioSpeechProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}
	if task.Status != model.TaskStatusSuccess {
		audioSpeechProxyError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		audioSpeechProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	if channel.Type != constant.ChannelTypeUnrealSpeech {
		audioSpeechProxyError(c, http.StatusBadRequest, "invalid_request_error", "Task is not an UnrealSpeech task")
		return
	}
	audioURL := task.GetResultURL()
	if audioURL == "" {
		audioSpeechProxyError(c, http.StatusBadGateway, "upstream_error", "Speech task has no audio result")
		return
	}
	response, err := provider.FetchAudio(c.Request.Context(), audioURL, channel.GetSetting().Proxy)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch speech content for task %s: %s", taskID, err.Error()))
		audioSpeechProxyError(c, http.StatusBadGateway, "upstream_error", "Failed to fetch speech content")
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Length", "ETag", "Last-Modified"} {
		if value := response.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "audio/mpeg")
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, response.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream speech content for task %s: %s", taskID, err.Error()))
	}
}
