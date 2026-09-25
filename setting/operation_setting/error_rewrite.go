package operation_setting

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
)

// ErrorRewriteRule replaces the client-facing message for one upstream HTTP
// status code. RewriteStatusCode is optional; retry and channel-health
// decisions always use the original upstream result before this is applied.
type ErrorRewriteRule struct {
	StatusCode        int    `json:"status_code"`
	RewriteStatusCode *int   `json:"rewrite_status_code,omitempty"`
	Message           string `json:"message"`
}

// ErrorRewriteSetting contains the global, operator-configurable error
// rewrites. It is persisted through the generic option/config mechanism under
// the error_rewrite.* keys.
type ErrorRewriteSetting struct {
	Enabled                   bool               `json:"enabled"`
	AffectUsageLogs           bool               `json:"affect_usage_logs"`
	BodyKeywordTriggerEnabled bool               `json:"body_keyword_trigger_enabled"`
	BodyKeywordTriggers       []string           `json:"body_keyword_triggers"`
	Rules                     []ErrorRewriteRule `json:"rules"`
}

type errorRewriteConfig struct {
	mu      sync.RWMutex
	setting ErrorRewriteSetting
}

var errorRewriteSetting = errorRewriteConfig{
	setting: ErrorRewriteSetting{
		BodyKeywordTriggers: []string{},
		Rules:               []ErrorRewriteRule{},
	},
}

func init() {
	config.GlobalConfig.Register("error_rewrite", &errorRewriteSetting)
}

func (c *errorRewriteConfig) ExportConfigMap() (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rules, err := common.Marshal(c.setting.Rules)
	if err != nil {
		return nil, err
	}
	bodyKeywords, err := common.Marshal(c.setting.BodyKeywordTriggers)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"enabled":                      strconv.FormatBool(c.setting.Enabled),
		"affect_usage_logs":            strconv.FormatBool(c.setting.AffectUsageLogs),
		"body_keyword_trigger_enabled": strconv.FormatBool(c.setting.BodyKeywordTriggerEnabled),
		"body_keyword_triggers":        string(bodyKeywords),
		"rules":                        string(rules),
	}, nil
}

func (c *errorRewriteConfig) UpdateConfigMap(values map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.setting
	next.Rules = append([]ErrorRewriteRule{}, c.setting.Rules...)
	if value, ok := values["enabled"]; ok {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("error rewrite enabled must be a boolean: %w", err)
		}
		next.Enabled = enabled
	}
	if value, ok := values["affect_usage_logs"]; ok {
		affectLogs, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("error rewrite affect usage logs must be a boolean: %w", err)
		}
		next.AffectUsageLogs = affectLogs
	}
	if value, ok := values["body_keyword_trigger_enabled"]; ok {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("error rewrite body keyword trigger enabled must be a boolean: %w", err)
		}
		next.BodyKeywordTriggerEnabled = enabled
	}
	if value, ok := values["body_keyword_triggers"]; ok {
		if err := ValidateErrorRewriteBodyKeywordsJSON(value); err != nil {
			return err
		}
		if err := common.UnmarshalJsonStr(value, &next.BodyKeywordTriggers); err != nil {
			return err
		}
		next.BodyKeywordTriggers = normalizeErrorRewriteBodyKeywords(next.BodyKeywordTriggers)
	}
	if value, ok := values["rules"]; ok {
		if err := ValidateErrorRewriteRulesJSON(value); err != nil {
			return err
		}
		if err := common.UnmarshalJsonStr(value, &next.Rules); err != nil {
			return err
		}
	}
	c.setting = next
	return nil
}

func (c *errorRewriteConfig) replace(setting ErrorRewriteSetting) {
	c.mu.Lock()
	defer c.mu.Unlock()
	setting.Rules = append([]ErrorRewriteRule{}, setting.Rules...)
	setting.BodyKeywordTriggers = normalizeErrorRewriteBodyKeywords(setting.BodyKeywordTriggers)
	c.setting = setting
}

// GetErrorRewriteSetting returns a snapshot suitable for read-path use. The
// rules slice is copied so callers cannot mutate the config managed by the
// option loader.
func GetErrorRewriteSetting() ErrorRewriteSetting {
	errorRewriteSetting.mu.RLock()
	defer errorRewriteSetting.mu.RUnlock()

	snapshot := errorRewriteSetting.setting
	snapshot.Rules = append([]ErrorRewriteRule{}, errorRewriteSetting.setting.Rules...)
	snapshot.BodyKeywordTriggers = append([]string{}, errorRewriteSetting.setting.BodyKeywordTriggers...)
	return snapshot
}

// ValidateErrorRewriteRulesJSON validates the JSON persisted by the option
// API. Keeping validation at the option boundary prevents malformed rules
// from disabling otherwise valid configuration during a reload.
func ValidateErrorRewriteRulesJSON(value string) error {
	var rules []ErrorRewriteRule
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return fmt.Errorf("error rewrite rules must be a JSON array: %w", err)
	}
	if rules == nil {
		return fmt.Errorf("error rewrite rules must be a JSON array")
	}

	seen := make(map[int]struct{}, len(rules))
	for index, rule := range rules {
		if rule.StatusCode < 100 || rule.StatusCode > 599 {
			return fmt.Errorf("error rewrite rule %d has invalid HTTP status code %d", index, rule.StatusCode)
		}
		if _, exists := seen[rule.StatusCode]; exists {
			return fmt.Errorf("error rewrite rules contain duplicate HTTP status code %d", rule.StatusCode)
		}
		if rule.RewriteStatusCode != nil && (*rule.RewriteStatusCode < 100 || *rule.RewriteStatusCode > 599) {
			return fmt.Errorf("error rewrite rule %d has invalid replacement HTTP status code %d", index, *rule.RewriteStatusCode)
		}
		seen[rule.StatusCode] = struct{}{}
		if strings.TrimSpace(rule.Message) == "" {
			return fmt.Errorf("error rewrite rule %d message must not be empty", index)
		}
	}
	return nil
}

func ValidateErrorRewriteBodyKeywordsJSON(value string) error {
	var keywords []string
	if err := common.UnmarshalJsonStr(value, &keywords); err != nil {
		return fmt.Errorf("error rewrite body keywords must be a JSON array: %w", err)
	}
	if keywords == nil {
		return fmt.Errorf("error rewrite body keywords must be a JSON array")
	}
	if len(keywords) > 100 {
		return fmt.Errorf("error rewrite body keywords must not exceed 100 entries")
	}
	seen := make(map[string]struct{}, len(keywords))
	for index, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			return fmt.Errorf("error rewrite body keyword %d must not be empty", index)
		}
		if len(keyword) > 256 {
			return fmt.Errorf("error rewrite body keyword %d must not exceed 256 characters", index)
		}
		key := strings.ToLower(keyword)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("error rewrite body keywords contain duplicate keyword %q", keyword)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func normalizeErrorRewriteBodyKeywords(keywords []string) []string {
	normalized := make([]string, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		key := strings.ToLower(keyword)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, keyword)
	}
	return normalized
}

// ErrorRewriteResult contains the client-facing values after a matching rule.
// It is also used to project a rewritten message into user-visible logs while
// keeping the original error object unchanged for retry and channel health.
type ErrorRewriteResult struct {
	Message            string
	StatusCode         int
	UpstreamStatusCode int
}

func GetErrorRewriteResult(apiErr *types.NewAPIError, modelName string) (ErrorRewriteResult, bool) {
	if apiErr == nil {
		return ErrorRewriteResult{}, false
	}
	return getErrorRewriteResult(apiErr.GetUpstreamStatusCode(), apiErr.StatusCode, apiErr.GetUpstreamResponseBody(), modelName)
}

func GetTaskErrorRewriteResult(taskErr *taskdto.TaskError, modelName string) (ErrorRewriteResult, bool) {
	if taskErr == nil {
		return ErrorRewriteResult{}, false
	}
	return getErrorRewriteResult(taskErr.UpstreamStatusCode, taskErr.StatusCode, taskErr.UpstreamResponseBody, modelName)
}

// ApplyErrorRewrite updates the client-facing error text, status, and protocol
// payload message. Retry and channel-health decisions use the original result
// before this response-only projection is applied.
func ApplyErrorRewrite(apiErr *types.NewAPIError, modelName string) bool {
	if apiErr == nil {
		return false
	}
	result, ok := GetErrorRewriteResult(apiErr, modelName)
	if !ok {
		return false
	}

	apiErr.SetMessage(result.Message)
	apiErr.StatusCode = result.StatusCode

	// ToOpenAIError/ToClaudeError use RelayError for upstream protocol errors,
	// so keep that payload in sync with Err.
	switch relayError := apiErr.RelayError.(type) {
	case types.OpenAIError:
		relayError.Message = result.Message
		apiErr.RelayError = relayError
	case *types.OpenAIError:
		if relayError != nil {
			relayError.Message = result.Message
		}
	case types.ClaudeError:
		relayError.Message = result.Message
		apiErr.RelayError = relayError
	case *types.ClaudeError:
		if relayError != nil {
			relayError.Message = result.Message
		}
	}
	return true
}

// ApplyTaskErrorRewrite applies the same global rule to asynchronous task
// endpoints while preserving their response schema and client-facing status.
func ApplyTaskErrorRewrite(taskErr *taskdto.TaskError, modelName string) bool {
	if taskErr == nil {
		return false
	}
	result, ok := GetTaskErrorRewriteResult(taskErr, modelName)
	if !ok {
		return false
	}
	taskErr.Message = result.Message
	taskErr.StatusCode = result.StatusCode
	taskErr.Error = errors.New(result.Message)
	return true
}

func getErrorRewriteResult(upstreamStatusCode int, responseStatusCode int, responseBody string, modelName string) (ErrorRewriteResult, bool) {
	if upstreamStatusCode < 100 || upstreamStatusCode > 599 || upstreamStatusCode == http.StatusOK {
		return ErrorRewriteResult{}, false
	}
	settings := GetErrorRewriteSetting()
	if !settings.Enabled {
		return ErrorRewriteResult{}, false
	}

	var rule *ErrorRewriteRule
	for index := range settings.Rules {
		if settings.Rules[index].StatusCode == upstreamStatusCode {
			rule = &settings.Rules[index]
			break
		}
	}
	if rule == nil {
		return ErrorRewriteResult{}, false
	}
	if settings.BodyKeywordTriggerEnabled && !errorRewriteBodyContainsKeyword(responseBody, settings.BodyKeywordTriggers) {
		return ErrorRewriteResult{}, false
	}

	message := strings.NewReplacer(
		"{model}", modelName,
		"{status_code}", strconv.Itoa(responseStatusCode),
		"{upstream_status_code}", strconv.Itoa(upstreamStatusCode),
	).Replace(rule.Message)
	statusCode := responseStatusCode
	if rule.RewriteStatusCode != nil {
		statusCode = *rule.RewriteStatusCode
	}
	return ErrorRewriteResult{Message: message, StatusCode: statusCode, UpstreamStatusCode: upstreamStatusCode}, true
}

func errorRewriteBodyContainsKeyword(body string, keywords []string) bool {
	body = strings.ToLower(body)
	for _, keyword := range keywords {
		if strings.Contains(body, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
