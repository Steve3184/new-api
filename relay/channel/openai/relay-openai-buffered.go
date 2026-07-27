package openai

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type bufferedChatCompletionChoice struct {
	role         string
	content      strings.Builder
	reasoning    strings.Builder
	finishReason string
	toolCalls    map[int]*dto.ToolCallResponse
}

// OaiBufferedStreamHandler turns an upstream Chat Completions SSE response into
// one normal Chat Completions response for a non-streaming client request.
func OaiBufferedStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if info == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid relay info"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	choices := make(map[int]*bufferedChatCompletionChoice)
	var (
		responseID        string
		model             string
		created           int64
		usage             *dto.Usage
		responseText      strings.Builder
		toolCount         int
		lastStreamPayload []byte
		streamErr         *types.NewAPIError
	)
	model = info.UpstreamModelName

	scanner := helper.NewStreamScanner(resp.Body)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var envelope struct {
			Error any `json:"error"`
		}
		if err := common.UnmarshalJsonStr(data, &envelope); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			break
		}
		if openAIError := dto.GetOpenAIError(envelope.Error); openAIError != nil && openAIError.Type != "" {
			streamErr = types.WithOpenAIError(*openAIError, http.StatusInternalServerError)
			break
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			break
		}
		lastStreamPayload = common.StringToByteSlice(data)
		if chunk.Id != "" {
			responseID = chunk.Id
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Created != 0 {
			created = chunk.Created
		}
		if service.ValidUsage(chunk.Usage) {
			usage = chunk.Usage
		}

		_ = ProcessStreamResponse(chunk, &responseText, &toolCount)
		for _, chunkChoice := range chunk.Choices {
			choice := choices[chunkChoice.Index]
			if choice == nil {
				choice = &bufferedChatCompletionChoice{toolCalls: make(map[int]*dto.ToolCallResponse)}
				choices[chunkChoice.Index] = choice
			}
			if chunkChoice.Delta.Role != "" {
				choice.role = chunkChoice.Delta.Role
			}
			choice.content.WriteString(chunkChoice.Delta.GetContentString())
			choice.reasoning.WriteString(chunkChoice.Delta.GetReasoningContent())
			if chunkChoice.FinishReason != nil && *chunkChoice.FinishReason != "" {
				choice.finishReason = *chunkChoice.FinishReason
			}
			for position, toolCall := range chunkChoice.Delta.ToolCalls {
				toolIndex := position
				if toolCall.Index != nil {
					toolIndex = *toolCall.Index
				}
				accumulated := choice.toolCalls[toolIndex]
				if accumulated == nil {
					accumulated = &dto.ToolCallResponse{}
					choice.toolCalls[toolIndex] = accumulated
				}
				if toolCall.ID != "" {
					accumulated.ID = toolCall.ID
				}
				if toolCall.Type != nil {
					accumulated.Type = toolCall.Type
				}
				if toolCall.Function.Name != "" {
					accumulated.Function.Name += toolCall.Function.Name
				}
				if toolCall.Function.Arguments != "" {
					accumulated.Function.Arguments += toolCall.Function.Arguments
				}
			}
		}
	}

	if streamErr != nil {
		return nil, streamErr
	}
	if err := scanner.Err(); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if responseID == "" {
		responseID = helper.GetResponseID(c)
	}
	if created == 0 {
		created = time.Now().Unix()
	}
	if usage == nil {
		usage = service.ResponseText2Usage(c, responseText.String(), model, info.GetEstimatePromptTokens())
		usage.CompletionTokens += toolCount * 7
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	applyUsagePostProcessing(info, usage, lastStreamPayload)

	choiceIndices := make([]int, 0, len(choices))
	for index := range choices {
		choiceIndices = append(choiceIndices, index)
	}
	sort.Ints(choiceIndices)
	responseChoices := make([]dto.OpenAITextResponseChoice, 0, len(choiceIndices))
	for _, index := range choiceIndices {
		choice := choices[index]
		role := choice.role
		if role == "" {
			role = "assistant"
		}
		message := dto.Message{Role: role, Content: choice.content.String()}
		if choice.reasoning.Len() > 0 {
			message.ReasoningContent = common.GetPointer(choice.reasoning.String())
		}
		if len(choice.toolCalls) > 0 {
			toolIndices := make([]int, 0, len(choice.toolCalls))
			for toolIndex := range choice.toolCalls {
				toolIndices = append(toolIndices, toolIndex)
			}
			sort.Ints(toolIndices)
			toolCalls := make([]dto.ToolCallResponse, 0, len(toolIndices))
			for _, toolIndex := range toolIndices {
				toolCalls = append(toolCalls, *choice.toolCalls[toolIndex])
			}
			toolCallsJSON, err := common.Marshal(toolCalls)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			}
			message.ToolCalls = toolCallsJSON
		}
		finishReason := choice.finishReason
		if finishReason == "" {
			finishReason = "stop"
		}
		responseChoices = append(responseChoices, dto.OpenAITextResponseChoice{
			Index:        index,
			Message:      message,
			FinishReason: finishReason,
		})
	}

	chatResponse := &dto.OpenAITextResponse{
		Id:      responseID,
		Model:   model,
		Object:  "chat.completion",
		Created: created,
		Choices: responseChoices,
		Usage:   *usage,
	}
	responseValue := any(chatResponse)
	if info.RelayFormat != types.RelayFormatOpenAI {
		result, err := relayconvert.ConvertResponse(c, info, info.RelayFormat, chatResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		responseValue = result.Value
	}

	responseBody, err := common.Marshal(responseValue)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	resp.Header.Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}
