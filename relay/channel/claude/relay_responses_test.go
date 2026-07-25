package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptorConvertsOpenAIResponsesRequestToAnthropicMessages(t *testing.T) {
	info := newClaudeResponsesRelayInfo(false)
	info.RequestConversionChain = []types.RelayFormat{types.RelayFormatOpenAIResponses}
	maxOutputTokens := uint(512)

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:           "claude-test",
		Instructions:    mustClaudeResponsesRawMessage(t, "system rules"),
		MaxOutputTokens: &maxOutputTokens,
		Input: mustClaudeResponsesRawMessage(t, []map[string]any{
			{"role": "user", "content": "question"},
			{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": map[string]any{"q": "x"}},
			{"type": "function_call_output", "call_id": "call_1", "output": map[string]any{"ok": true}},
		}),
		Tools: mustClaudeResponsesRawMessage(t, []map[string]any{
			{"type": "function", "name": "lookup", "parameters": map[string]any{"type": "object"}},
		}),
	})
	require.NoError(t, err)

	request, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	assert.Equal(t, "claude-test", request.Model)
	assert.Equal(t, maxOutputTokens, *request.MaxTokens)
	system := request.ParseSystem()
	require.Len(t, system, 1)
	assert.Equal(t, "system rules", system[0].GetText())
	require.Len(t, request.Messages, 3)

	assistantContent, err := request.Messages[1].ParseContent()
	require.NoError(t, err)
	require.Len(t, assistantContent, 1)
	assert.Equal(t, "tool_use", assistantContent[0].Type)
	assert.Equal(t, "call_1", assistantContent[0].Id)
	assert.Equal(t, "lookup", assistantContent[0].Name)
	assert.Equal(t, map[string]any{"q": "x"}, assistantContent[0].Input)

	toolResultContent, err := request.Messages[2].ParseContent()
	require.NoError(t, err)
	require.Len(t, toolResultContent, 1)
	assert.Equal(t, "tool_result", toolResultContent[0].Type)
	assert.Equal(t, "call_1", toolResultContent[0].ToolUseId)
	assert.Equal(t, map[string]any{"ok": true}, toolResultContent[0].Content)
	assert.Equal(t, []types.RelayFormat{types.RelayFormatOpenAIResponses, types.RelayFormatClaude}, info.RequestConversionChain)
}

func TestAdaptorConvertsAnthropicStreamToOpenAIResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "claude-responses-stream-test")

	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	text := "hello"
	stopReason := "end_turn"
	index := 0
	events := []dto.ClaudeResponse{
		{
			Type: "message_start",
			Message: &dto.ClaudeMediaMessage{
				Id:    "msg_1",
				Model: "claude-test",
				Role:  "assistant",
				Usage: &dto.ClaudeUsage{
					InputTokens:              2,
					CacheReadInputTokens:     1,
					CacheCreationInputTokens: 1,
				},
			},
		},
		{Type: "content_block_start", Index: &index, ContentBlock: &dto.ClaudeMediaMessage{Type: "text"}},
		{Type: "content_block_delta", Index: &index, Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: &text}},
		{Type: "content_block_stop", Index: &index},
		{
			Type:  "message_delta",
			Delta: &dto.ClaudeMediaMessage{Type: "message_delta", StopReason: &stopReason},
			Usage: &dto.ClaudeUsage{OutputTokens: 3},
		},
		{Type: "message_stop"},
	}
	streamLines := make([]string, 0, len(events)*2)
	for _, event := range events {
		data, err := common.Marshal(event)
		require.NoError(t, err)
		streamLines = append(streamLines, "data: "+string(data), "")
	}

	info := newClaudeResponsesRelayInfo(true)
	usage, newAPIError := (&Adaptor{}).DoResponse(c, &http.Response{
		Body: io.NopCloser(strings.NewReader(strings.Join(streamLines, "\n"))),
	}, info)
	require.Nil(t, newAPIError)
	responsesUsage, ok := usage.(*dto.Usage)
	require.True(t, ok)
	assert.Equal(t, 2, responsesUsage.PromptTokens)
	assert.Equal(t, 3, responsesUsage.CompletionTokens)
	assert.Equal(t, 5, responsesUsage.TotalTokens)

	got := recorder.Body.String()
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Contains(t, got, `event: response.created`)
	assert.Contains(t, got, `event: response.output_text.delta`)
	assert.Contains(t, got, `"delta":"hello"`)
	assert.Contains(t, got, `event: response.completed`)
	assert.Contains(t, got, `"input_tokens":4`)
	assert.Contains(t, got, `"output_tokens":3`)
	assert.NotContains(t, got, `"type":"message_start"`)
	requireOrderedClaudeResponsesSubstrings(t, got,
		`event: response.created`,
		`event: response.output_item.added`,
		`event: response.output_text.delta`,
		`event: response.output_text.done`,
		`event: response.completed`,
	)
}

func newClaudeResponsesRelayInfo(isStream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream:        isStream,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		DisablePing:     true,
		OriginModelName: "claude-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-test",
		},
	}
}

func mustClaudeResponsesRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}

func requireOrderedClaudeResponsesSubstrings(t *testing.T, value string, parts ...string) {
	t.Helper()
	offset := 0
	for _, part := range parts {
		index := strings.Index(value[offset:], part)
		require.NotEqualf(t, -1, index, "missing %q after byte offset %d", part, offset)
		offset += index + len(part)
	}
}
