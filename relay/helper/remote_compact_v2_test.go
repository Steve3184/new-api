package helper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSimulatedRemoteCompactV2RequestTransformsTriggerAndPriorSummary(t *testing.T) {
	encodedSummary, err := dto.EncodeSimulatedRemoteCompactV2("Keep the unresolved migration in scope.")
	require.NoError(t, err)
	input, err := common.Marshal([]map[string]any{
		{"role": "user", "content": "Initial request"},
		{"type": "compaction", "encrypted_content": encodedSummary},
		{"type": "compaction_trigger"},
	})
	require.NoError(t, err)
	stream := true
	request := &dto.OpenAIResponsesRequest{
		Input:             input,
		Stream:            &stream,
		Tools:             json.RawMessage(`[{"type":"function","name":"lookup"}]`),
		ToolChoice:        json.RawMessage(`"required"`),
		ParallelToolCalls: json.RawMessage(`true`),
		Reasoning:         &dto.Reasoning{Effort: "high"},
	}

	prepared, err := PrepareSimulatedRemoteCompactV2Request(request, true)
	require.NoError(t, err)
	assert.True(t, prepared.Modified)
	assert.True(t, prepared.Simulating)
	require.NotNil(t, request.MaxOutputTokens)
	assert.Equal(t, uint(4096), *request.MaxOutputTokens)
	assert.Nil(t, request.Tools)
	assert.Nil(t, request.ToolChoice)
	assert.Nil(t, request.ParallelToolCalls)
	assert.Nil(t, request.Reasoning)

	var transformed []struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	require.NoError(t, common.Unmarshal(request.Input, &transformed))
	require.Len(t, transformed, 3)
	assert.Equal(t, "user", transformed[1].Role)
	assert.Equal(t, "user", transformed[2].Role)
	assert.NotEqual(t, "compaction_trigger", transformed[2].Type)

	var handoffContent []struct {
		Text string `json:"text"`
	}
	require.NoError(t, common.Unmarshal(transformed[1].Content, &handoffContent))
	require.Len(t, handoffContent, 1)
	assert.Contains(t, handoffContent[0].Text, "unresolved migration")

	var compactContent []struct {
		Text string `json:"text"`
	}
	require.NoError(t, common.Unmarshal(transformed[2].Content, &compactContent))
	require.Len(t, compactContent, 1)
	assert.Equal(t, `You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary for another LLM that will resume the task.

Include:
- Current progress and key decisions made
- Important context, constraints, or user preferences
- What remains to be done (clear next steps)
- Any critical data, examples, or references needed to continue

Be concise, structured, and focused on helping the next LLM seamlessly continue the work.
`, compactContent[0].Text)
}

func TestPrepareSimulatedRemoteCompactV2RequestRequiresStreaming(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Input: json.RawMessage(`[{"type":"compaction_trigger"}]`),
	}

	_, err := PrepareSimulatedRemoteCompactV2Request(request, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream=true")
}

func TestSimulatedRemoteCompactV2StreamEmitsCompatibleCompaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "remote-compact-v2-test")
	EnableSimulatedRemoteCompactV2(c)

	require.NoError(t, ResponseChunkData(c, dto.ResponsesStreamResponse{
		Type: "response.created",
	}, `{"type":"response.created","response":{"id":"resp_simulated","model":"upstream-model","created_at":123}}`))
	require.NoError(t, ResponseChunkData(c, dto.ResponsesStreamResponse{
		Type: "response.output_text.delta",
	}, `{"type":"response.output_text.delta","delta":"Preserve the exact migration status."}`))
	require.NoError(t, ResponseChunkData(c, dto.ResponsesStreamResponse{
		Type: "response.completed",
	}, `{"type":"response.completed","response":{"id":"resp_simulated","model":"upstream-model","created_at":123,"usage":{"input_tokens":12,"output_tokens":7,"total_tokens":19}}}`))

	output := recorder.Body.String()
	assert.NotContains(t, output, "response.created")
	assert.NotContains(t, output, "response.output_text.delta")
	assert.Equal(t, 2, strings.Count(output, "event: "))

	payloads := make([]string, 0, 2)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "data: ") {
			payloads = append(payloads, strings.TrimPrefix(line, "data: "))
		}
	}
	require.Len(t, payloads, 2)

	var itemEvent struct {
		Type string `json:"type"`
		Item struct {
			Type             string `json:"type"`
			EncryptedContent string `json:"encrypted_content"`
		} `json:"item"`
	}
	require.NoError(t, common.UnmarshalJsonStr(payloads[0], &itemEvent))
	assert.Equal(t, dto.ResponsesOutputTypeItemDone, itemEvent.Type)
	assert.Equal(t, "compaction", itemEvent.Item.Type)
	summary, simulated, err := dto.DecodeSimulatedRemoteCompactV2(itemEvent.Item.EncryptedContent)
	require.NoError(t, err)
	assert.True(t, simulated)
	assert.Equal(t, "Preserve the exact migration status.", summary)

	var completedEvent struct {
		Type     string `json:"type"`
		Response struct {
			ID     string                `json:"id"`
			Output []dto.ResponsesOutput `json:"output"`
			Usage  *dto.Usage            `json:"usage"`
		} `json:"response"`
	}
	require.NoError(t, common.UnmarshalJsonStr(payloads[1], &completedEvent))
	assert.Equal(t, "response.completed", completedEvent.Type)
	assert.Equal(t, "resp_simulated", completedEvent.Response.ID)
	require.Len(t, completedEvent.Response.Output, 1)
	assert.Equal(t, "compaction", completedEvent.Response.Output[0].Type)
	require.NotNil(t, completedEvent.Response.Usage)
	assert.Equal(t, 19, completedEvent.Response.Usage.TotalTokens)
}

func TestFinalizeSimulatedRemoteCompactV2MarksIncompleteStreamAsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	EnableSimulatedRemoteCompactV2(c)

	require.NoError(t, FinalizeSimulatedRemoteCompactV2(c))
	assert.Contains(t, recorder.Body.String(), "event: response.failed")
	assert.Contains(t, recorder.Body.String(), "upstream stream ended before remote compaction completed")
}
