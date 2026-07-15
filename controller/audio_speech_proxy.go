package controller

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
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
	proxyAudioSpeechArtifact(c, false)
}

func AudioSpeechTimestampsProxy(c *gin.Context) {
	proxyAudioSpeechArtifact(c, true)
}

func proxyAudioSpeechArtifact(c *gin.Context, timestamps bool) {
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
	artifactName := "audio"
	artifactURL := task.GetResultURL()
	defaultContentType := "audio/mpeg"
	if timestamps {
		artifactName = "timestamps"
		defaultContentType = "application/json; charset=utf-8"
		var envelope provider.SynthesisTaskEnvelope
		if err := common.Unmarshal(task.Data, &envelope); err != nil {
			audioSpeechProxyError(c, http.StatusBadGateway, "upstream_error", "Speech task has invalid timestamp data")
			return
		}
		artifactURL = provider.FirstURI(envelope.SynthesisTask.TimestampsURI)
	}
	if artifactURL == "" {
		audioSpeechProxyError(c, http.StatusNotFound, "upstream_error", fmt.Sprintf("Speech task has no %s result", artifactName))
		return
	}
	response, err := provider.FetchArtifact(c.Request.Context(), artifactURL, channel.GetSetting().Proxy, defaultContentType)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch speech %s for task %s: %s", artifactName, taskID, err.Error()))
		audioSpeechProxyError(c, http.StatusBadGateway, "upstream_error", fmt.Sprintf("Failed to fetch speech %s", artifactName))
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Encoding", "ETag", "Last-Modified"} {
		if value := response.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}
	if timestamps || c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", defaultContentType)
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, response.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream speech %s for task %s: %s", artifactName, taskID, err.Error()))
	}
}
