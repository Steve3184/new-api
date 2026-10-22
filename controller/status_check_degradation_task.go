package controller

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

// statusCheckCandyPrompt is a reasoning question whose correct answer is 21.
// Models served below their advertised capability usually answer 29. The
// trailing instruction keeps the visible output to a single number.
const statusCheckCandyPrompt = `在一个黑色的袋子里装有三种口味的糖果，每种糖果有两种不同形状（圆形和五角星形，靠手感可以分辨），数量如下：
苹果味：圆形 7 个，五角星形 7 个
桃子味：圆形 9 个，五角星形 6 个
西瓜味：圆形 8 个，五角星形 4 个
参赛者只能凭形状触感决定摸出的糖果总数，那么，最少取出多少个糖果，才能在最坏情况下必然保证手中同时拥有不同形状的苹果味和桃子味糖果（即拥有‘圆形苹果+星形桃子’或‘圆形桃子+星形苹果’）？
只输出最终答案的阿拉伯数字，不要输出任何解释。`

const statusCheckCandyExpectedAnswer = "21"

type statusCheckDegradationTaskSummary struct {
	SkippedRecent    int `json:"skipped_recent"`
	SkippedNoChannel int `json:"skipped_no_channel"`
	Passed           int `json:"passed"`
	Degraded         int `json:"degraded"`
	Failed           int `json:"failed"`
}

var runStatusCheckDegradationChannelTest = testChannelWithKey

var statusCheckAnswerNumberPattern = regexp.MustCompile(`\d+`)

// runStatusCheckDegradationTask asks each configured group/model/effort the
// candy question at most once per configured interval.
func runStatusCheckDegradationTask(ctx context.Context) (statusCheckDegradationTaskSummary, error) {
	summary := statusCheckDegradationTaskSummary{}
	config := perfmetrics.GetStatusCheckDegradationConfig()
	groups := make([]string, 0)
	for _, group := range statusCheckExplicitProbeGroups() {
		if _, enabled := config.EnabledGroup(group); enabled {
			groups = append(groups, group)
		}
	}
	if len(groups) == 0 {
		return summary, nil
	}
	lastCheckedAt, err := model.GetStatusCheckDegradationLastCheckedAt(groups)
	if err != nil {
		return summary, err
	}

	testUserID := 0
	for _, group := range groups {
		groupConfig, _ := config.EnabledGroup(group)
		for _, entry := range groupConfig.Models {
			if ctx != nil && ctx.Err() != nil {
				return summary, ctx.Err()
			}
			key := model.StatusCheckDegradationKey{Group: group, Model: entry.Model, Effort: entry.Effort}
			if time.Now().Unix()-lastCheckedAt[key] < int64(groupConfig.IntervalMinutes)*60 {
				summary.SkippedRecent++
				continue
			}

			record := model.StatusCheckDegradationResult{
				Group:  group,
				Model:  entry.Model,
				Effort: entry.Effort,
				Status: model.StatusCheckDegradationError,
			}
			channel, err := model.GetRandomSatisfiedChannel(group, entry.Model, 0, nil)
			if err != nil || channel == nil {
				// Recording the miss keeps the interval in force instead of
				// retrying every minute.
				summary.SkippedNoChannel++
				record.CheckedAt = time.Now().Unix()
				if err := model.CreateStatusCheckDegradationResult(&record); err != nil {
					return summary, err
				}
				continue
			}
			if testUserID == 0 {
				testUserID, err = resolveChannelTestUserID(nil)
				if err != nil {
					return summary, err
				}
			}

			// Streaming yields a real time-to-first-token, which fills the
			// status page for hours without real requests.
			endpointType, request := buildStatusCheckCandyRequest(channel, entry.Model, entry.Effort)
			startedAt := time.Now()
			result := runStatusCheckDegradationChannelTest(ctx, channel, testUserID, entry.Model, endpointType, true, -1, request)
			record.LatencyMs = time.Since(startedAt).Milliseconds()
			record.CheckedAt = time.Now().Unix()
			requestSucceeded := result.localErr == nil && result.newAPIError == nil
			perfmetrics.RecordStatusCheckDegradationProbe(group, record.LatencyMs, result.ttftMs, requestSucceeded)
			if requestSucceeded {
				record.Status, record.Answer = judgeStatusCheckCandyAnswer(extractTestResponseText(result.responseBody))
			}
			switch record.Status {
			case model.StatusCheckDegradationPassed:
				summary.Passed++
			case model.StatusCheckDegradationDegraded:
				summary.Degraded++
			default:
				summary.Failed++
			}
			if err := model.CreateStatusCheckDegradationResult(&record); err != nil {
				return summary, err
			}
		}
	}

	if err := model.DeleteStatusCheckDegradationResultsBefore(time.Now().Add(-perfmetrics.StatusCheckDegradationRetention).Unix()); err != nil {
		common.SysError(common.LogText("failed to prune status check degradation results: %v", err))
	}
	return summary, nil
}

// buildStatusCheckCandyRequest builds the smallest streamed request that
// carries the candy question and the requested reasoning effort. Responses-only channels
// receive a Responses request; all others use chat completions so adaptors
// convert the effort to their native thinking controls.
func buildStatusCheckCandyRequest(channel *model.Channel, modelName string, effort string) (string, dto.Request) {
	endpointType := normalizeChannelTestEndpoint(channel, "")
	if endpointType == string(constant.EndpointTypeOpenAIResponse) || strings.Contains(strings.ToLower(modelName), "codex") {
		input, _ := common.Marshal([]map[string]string{{"role": "user", "content": statusCheckCandyPrompt}})
		request := &dto.OpenAIResponsesRequest{
			Model:  modelName,
			Input:  json.RawMessage(input),
			Stream: lo.ToPtr(true),
		}
		if effort != "" {
			request.Reasoning = &dto.Reasoning{Effort: effort}
		}
		return string(constant.EndpointTypeOpenAIResponse), request
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           modelName,
		Stream:          lo.ToPtr(true),
		Messages:        []dto.Message{{Role: "user", Content: statusCheckCandyPrompt}},
		ReasoningEffort: effort,
		StreamOptions:   &dto.StreamOptions{IncludeUsage: true},
	}
	return string(constant.EndpointTypeOpenAI), request
}

// extractTestResponseText returns the visible answer text of a chat
// completions or Responses body, streamed or not. Reasoning text is ignored.
func extractTestResponseText(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	payloads := []string{trimmed}
	if !strings.HasPrefix(trimmed, "{") {
		payloads = payloads[:0]
		for line := range strings.SplitSeq(trimmed, "\n") {
			payload, ok := strings.CutPrefix(strings.TrimSpace(line), "data:")
			payload = strings.TrimSpace(payload)
			if !ok || payload == "" || payload == "[DONE]" {
				continue
			}
			payloads = append(payloads, payload)
		}
	}

	var text strings.Builder
	completedText := ""
	for _, payload := range payloads {
		parsed := gjson.Parse(payload)
		for _, choice := range parsed.Get("choices").Array() {
			text.WriteString(choice.Get("message.content").String())
			text.WriteString(choice.Get("delta.content").String())
		}
		switch parsed.Get("type").String() {
		case "response.output_text.delta":
			text.WriteString(parsed.Get("delta").String())
		case "response.completed":
			completedText = responsesOutputText(parsed.Get("response.output"))
		}
		if parsed.Get("object").String() == "response" {
			text.WriteString(responsesOutputText(parsed.Get("output")))
		}
	}
	if text.Len() == 0 {
		return completedText
	}
	return text.String()
}

func responsesOutputText(output gjson.Result) string {
	var text strings.Builder
	for _, item := range output.Array() {
		if item.Get("type").String() != "message" {
			continue
		}
		for _, content := range item.Get("content").Array() {
			if content.Get("type").String() == "output_text" {
				text.WriteString(content.Get("text").String())
			}
		}
	}
	return text.String()
}

// judgeStatusCheckCandyAnswer compares the last number of the visible answer
// with the expected answer. An empty answer is a failed check, not a
// degradation, because the model may have spent its output on reasoning.
func judgeStatusCheckCandyAnswer(text string) (string, string) {
	if index := strings.LastIndex(text, "</think>"); index >= 0 {
		text = text[index+len("</think>"):]
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return model.StatusCheckDegradationError, ""
	}
	numbers := statusCheckAnswerNumberPattern.FindAllString(text, -1)
	if len(numbers) == 0 {
		answer := []rune(text)
		return model.StatusCheckDegradationDegraded, string(answer[:min(len(answer), 16)])
	}
	answer := numbers[len(numbers)-1]
	if answer == statusCheckCandyExpectedAnswer {
		return model.StatusCheckDegradationPassed, answer
	}
	return model.StatusCheckDegradationDegraded, answer
}
