package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// serveTaskPluginSpeechProtocol answers an OpenAI Speech API request from a
// task plugin. The upstream task remains durable, while this request waits for
// completion and streams the plugin's audio artifact without exposing its URL.
func serveTaskPluginSpeechProtocol(c *gin.Context, pinned pluginruntime.PinnedEndpoint, deps pluginProtocolBridgeDeps) {
	deps = deps.withDefaults()
	requestValue, exists := c.Get(pluginruntime.ContextKeyProtocolRequest)
	protocolRequest, ok := requestValue.(pluginruntime.ProtocolRequestContext)
	if !exists || !ok || protocolRequest.Protocol != pinned.Protocol || pinned.Plugin == nil {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}

	clientRequest := c.Request
	var relayInfo *relaycommon.RelayInfo
	var relayInfoErr error
	var outcome *taskSubmissionOutcome
	var taskErr *dto.TaskError
	func() {
		submissionContext, cancelSubmission := context.WithTimeout(context.WithoutCancel(clientRequest.Context()), deps.submissionTimeout)
		c.Request = clientRequest.Clone(submissionContext)
		defer func() {
			c.Request = clientRequest
			cancelSubmission()
		}()

		relayInfo, relayInfoErr = relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
		if relayInfoErr != nil {
			return
		}
		relayInfo.IsStream = false
		relayInfo.OriginModelName = c.GetString("resolved_task_model")
		if action := c.GetString("task_action"); action != "" {
			relayInfo.Action = action
		}
		if taskErr = relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
			return
		}
		if taskErr = relay.ApplyOriginTaskAffinity(c, relayInfo); taskErr != nil {
			return
		}
		outcome, taskErr = deps.submit(c, relayInfo)
	}()
	if clientRequest.Context().Err() != nil {
		return
	}
	if relayInfoErr != nil {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}
	if taskErr != nil {
		respondTaskPluginImageError(c, taskErr)
		return
	}
	if outcome == nil || outcome.Task == nil || outcome.Task.Platform != constant.TaskPlatform(pinned.Plugin.Meta.Key) {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}
	task := outcome.Task
	if task.Status != model.TaskStatusSuccess && task.Status != model.TaskStatusFailure {
		if taskErr = waitTaskPluginImageTask(c, task, deps); taskErr != nil {
			if c.Request.Context().Err() == nil {
				respondTaskPluginImageError(c, taskErr)
			}
			return
		}
	}
	if task.Status == model.TaskStatusFailure {
		reason := strings.TrimSpace(task.FailReason)
		if reason == "" {
			reason = "speech generation failed"
		}
		respondTaskPluginImageError(c, service.TaskErrorWrapperLocal(errors.New(reason), "speech_generation_failed", http.StatusBadRequest))
		return
	}

	artifacts, err := projectTaskArtifacts(task)
	if err != nil {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}
	artifactKey := ""
	for _, artifact := range artifacts {
		if artifact.Type == "audio" {
			artifactKey = artifact.Key
			break
		}
	}
	if artifactKey == "" {
		respondTaskPluginImageError(c, service.TaskErrorWrapperLocal(errors.New("Task completed without a WAV audio result"), "audio_result_missing", http.StatusBadGateway))
		return
	}

	adaptor, err := initTaskArtifactAdaptor(task)
	if err != nil {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}
	provider, ok := adaptor.(channel.TaskContentRequestProvider)
	if !ok {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}
	descriptor, err := provider.BuildContentRequest(task, artifactKey, channel.TaskArtifactClientRequest{Method: http.MethodGet})
	if err != nil || descriptor == nil {
		respondPluginProtocolError(c, http.StatusInternalServerError, "task_protocol_error", "Task protocol request failed")
		return
	}
	if err = proxyTaskMedia(c, task, descriptor); err != nil {
		writeTaskMediaProxyError(c, err)
	}
}
