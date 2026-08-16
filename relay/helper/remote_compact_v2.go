package helper

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

const (
	simulatedRemoteCompactV2StateKey             = "simulated_remote_compact_v2_state"
	simulatedRemoteCompactV2MaxOutputTokens uint = 4096
	simulatedRemoteCompactV2Prompt               = `You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary for another LLM that will resume the task.

Include:
- Current progress and key decisions made
- Important context, constraints, or user preferences
- What remains to be done (clear next steps)
- Any critical data, examples, or references needed to continue

Be concise, structured, and focused on helping the next LLM seamlessly continue the work.
`
)

type SimulatedRemoteCompactV2Preparation struct {
	Modified   bool
	Simulating bool
}

type simulatedRemoteCompactV2Stream struct {
	text       strings.Builder
	fallback   strings.Builder
	responseID string
	model      string
	createdAt  int
	usage      *dto.Usage
	terminal   bool
}

// PrepareSimulatedRemoteCompactV2Request turns Codex's private V2 trigger
// into a regular summary request that any Responses-capable channel can relay.
func PrepareSimulatedRemoteCompactV2Request(request *dto.OpenAIResponsesRequest, simulate bool) (SimulatedRemoteCompactV2Preparation, error) {
	if request == nil {
		return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("responses request is nil")
	}
	if common.GetJsonType(request.Input) != "array" {
		return SimulatedRemoteCompactV2Preparation{}, nil
	}

	var inputs []json.RawMessage
	if err := common.Unmarshal(request.Input, &inputs); err != nil {
		return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("invalid responses input: %w", err)
	}

	prepared := SimulatedRemoteCompactV2Preparation{}
	updatedInputs := make([]json.RawMessage, 0, len(inputs)+1)
	triggerCount := 0
	for _, rawInput := range inputs {
		if common.GetJsonType(rawInput) != "object" {
			updatedInputs = append(updatedInputs, rawInput)
			continue
		}

		var input struct {
			Type             string `json:"type"`
			EncryptedContent string `json:"encrypted_content"`
		}
		if err := common.Unmarshal(rawInput, &input); err != nil {
			return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("invalid responses input item: %w", err)
		}

		if input.Type == "compaction" {
			summary, simulated, err := dto.DecodeSimulatedRemoteCompactV2(input.EncryptedContent)
			if err != nil {
				return SimulatedRemoteCompactV2Preparation{}, err
			}
			if simulated {
				summaryInput, err := common.Marshal(map[string]any{
					"role": "user",
					"content": []map[string]string{{
						"type": "input_text",
						"text": "A previous context compaction produced this handoff summary. Continue using it as conversation context:\n\n" + summary,
					}},
				})
				if err != nil {
					return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("marshal simulated compaction input: %w", err)
				}
				updatedInputs = append(updatedInputs, summaryInput)
				prepared.Modified = true
				continue
			}
		}

		if simulate && input.Type == "compaction_trigger" {
			triggerCount++
			prepared.Modified = true
			continue
		}

		updatedInputs = append(updatedInputs, rawInput)
	}

	if triggerCount == 0 {
		if prepared.Modified {
			input, err := common.Marshal(updatedInputs)
			if err != nil {
				return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("marshal normalized responses input: %w", err)
			}
			request.Input = input
		}
		return prepared, nil
	}
	if triggerCount > 1 {
		return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("remote compact v2 request contains multiple compaction triggers")
	}
	if request.Stream == nil || !*request.Stream {
		return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("remote compact v2 requires stream=true")
	}

	compactPrompt, err := common.Marshal(map[string]any{
		"role": "user",
		"content": []map[string]string{{
			"type": "input_text",
			"text": simulatedRemoteCompactV2Prompt,
		}},
	})
	if err != nil {
		return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("marshal remote compact v2 prompt: %w", err)
	}
	updatedInputs = append(updatedInputs, compactPrompt)
	input, err := common.Marshal(updatedInputs)
	if err != nil {
		return SimulatedRemoteCompactV2Preparation{}, fmt.Errorf("marshal remote compact v2 input: %w", err)
	}

	request.Input = input
	request.MaxOutputTokens = common.GetPointer(simulatedRemoteCompactV2MaxOutputTokens)
	request.Include = nil
	request.Conversation = nil
	request.ContextManagement = nil
	request.Metadata = nil
	request.Moderation = nil
	request.ParallelToolCalls = nil
	request.FrequencyPenalty = nil
	request.PresencePenalty = nil
	request.PreviousResponseID = ""
	request.Reasoning = nil
	request.ServiceTier = ""
	request.Store = nil
	request.PromptCacheKey = nil
	request.PromptCacheOptions = nil
	request.PromptCacheRetention = nil
	request.SafetyIdentifier = nil
	request.StreamOptions = nil
	request.Temperature = nil
	request.Text = nil
	request.ToolChoice = nil
	request.Tools = nil
	request.TopLogProbs = nil
	request.TopP = nil
	request.Truncation = nil
	request.User = nil
	request.MaxToolCalls = nil
	request.Prompt = nil
	request.ClientMetadata = nil
	request.EnableThinking = nil
	request.ThinkingBudget = nil
	request.Preset = nil

	prepared.Simulating = true
	return prepared, nil
}

func EnableSimulatedRemoteCompactV2(c *gin.Context) {
	if c != nil {
		c.Set(simulatedRemoteCompactV2StateKey, &simulatedRemoteCompactV2Stream{})
	}
}

func interceptSimulatedRemoteCompactV2Stream(c *gin.Context, response dto.ResponsesStreamResponse, data string) (bool, error) {
	if c == nil {
		return false, nil
	}
	value, ok := c.Get(simulatedRemoteCompactV2StateKey)
	if !ok {
		return false, nil
	}
	state, ok := value.(*simulatedRemoteCompactV2Stream)
	if !ok || state == nil {
		return false, nil
	}
	if state.terminal {
		return true, nil
	}
	if response.Response == nil && response.Item == nil && response.Delta == "" && response.Text == "" && data != "" {
		var parsed dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &parsed); err == nil {
			if parsed.Type == "" {
				parsed.Type = response.Type
			}
			response = parsed
		}
	}

	if response.Response != nil {
		if response.Response.ID != "" {
			state.responseID = response.Response.ID
		}
		if response.Response.Model != "" {
			state.model = response.Response.Model
		}
		if response.Response.CreatedAt != 0 {
			state.createdAt = response.Response.CreatedAt
		}
		if response.Response.Usage != nil {
			state.usage = response.Response.Usage
		}
	}

	switch response.Type {
	case "response.output_text.delta":
		state.text.WriteString(response.Delta)
		return true, nil
	case "response.output_text.done":
		if state.fallback.Len() == 0 {
			state.fallback.WriteString(response.Text)
		}
		return true, nil
	case dto.ResponsesOutputTypeItemDone:
		if response.Item != nil && state.fallback.Len() == 0 {
			for _, content := range response.Item.Content {
				state.fallback.WriteString(content.Text)
			}
		}
		return true, nil
	case "response.completed", "response.done":
		if response.Response != nil && state.fallback.Len() == 0 {
			for _, output := range response.Response.Output {
				for _, content := range output.Content {
					state.fallback.WriteString(content.Text)
				}
			}
		}
		state.terminal = true
		summary := state.text.String()
		if summary == "" {
			summary = state.fallback.String()
		}
		if strings.TrimSpace(summary) == "" {
			return true, writeSimulatedRemoteCompactV2Failure(c, state, "upstream response did not contain a compaction summary")
		}

		encryptedContent, err := dto.EncodeSimulatedRemoteCompactV2(summary)
		if err != nil {
			return true, writeSimulatedRemoteCompactV2Failure(c, state, err.Error())
		}
		responseID := state.responseID
		if responseID == "" {
			responseID = GetResponseID(c)
		}
		compactionItem := map[string]any{
			"type":              "compaction",
			"encrypted_content": encryptedContent,
		}
		itemData, err := common.Marshal(map[string]any{
			"type":         dto.ResponsesOutputTypeItemDone,
			"output_index": 0,
			"item":         compactionItem,
		})
		if err != nil {
			return true, err
		}
		if err := writeResponsesChunkData(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone}, string(itemData)); err != nil {
			return true, err
		}

		completedResponse := map[string]any{
			"id":     responseID,
			"object": "response",
			"status": "completed",
			"output": []any{compactionItem},
		}
		if state.model != "" {
			completedResponse["model"] = state.model
		}
		if state.createdAt != 0 {
			completedResponse["created_at"] = state.createdAt
		}
		if state.usage != nil {
			completedResponse["usage"] = state.usage
		}
		completedData, err := common.Marshal(map[string]any{
			"type":     "response.completed",
			"response": completedResponse,
		})
		if err != nil {
			return true, err
		}
		return true, writeResponsesChunkData(c, dto.ResponsesStreamResponse{Type: "response.completed"}, string(completedData))
	case "response.failed", "response.incomplete", "response.cancelled", "response.canceled", "error":
		state.terminal = true
		return false, nil
	default:
		return true, nil
	}
}

// FinalizeSimulatedRemoteCompactV2 emits a protocol error when an upstream
// stream ends without a terminal Responses event.
func FinalizeSimulatedRemoteCompactV2(c *gin.Context) error {
	if c == nil {
		return nil
	}
	value, ok := c.Get(simulatedRemoteCompactV2StateKey)
	if !ok {
		return nil
	}
	state, ok := value.(*simulatedRemoteCompactV2Stream)
	if !ok || state == nil || state.terminal {
		return nil
	}
	state.terminal = true
	return writeSimulatedRemoteCompactV2Failure(c, state, "upstream stream ended before remote compaction completed")
}

func writeSimulatedRemoteCompactV2Failure(c *gin.Context, state *simulatedRemoteCompactV2Stream, message string) error {
	responseID := state.responseID
	if responseID == "" {
		responseID = GetResponseID(c)
	}
	response := map[string]any{
		"id":     responseID,
		"object": "response",
		"status": "failed",
		"error": map[string]string{
			"type":    "invalid_response_error",
			"message": message,
		},
	}
	if state.model != "" {
		response["model"] = state.model
	}
	if state.usage != nil {
		response["usage"] = state.usage
	}
	data, err := common.Marshal(map[string]any{
		"type":     "response.failed",
		"response": response,
	})
	if err != nil {
		return err
	}
	return writeResponsesChunkData(c, dto.ResponsesStreamResponse{Type: "response.failed"}, string(data))
}
