package operation_setting

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
)

// ErrorRewriteRule replaces the client-facing message for one upstream HTTP
// status code. The status code itself is intentionally left unchanged so the
// retry and channel-health decisions continue to use the upstream result.
type ErrorRewriteRule struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

// ErrorRewriteSetting contains the global, operator-configurable error
// rewrites. It is persisted through the generic option/config mechanism under
// the error_rewrite.* keys.
type ErrorRewriteSetting struct {
	Enabled bool               `json:"enabled"`
	Rules   []ErrorRewriteRule `json:"rules"`
}

type errorRewriteConfig struct {
	mu      sync.RWMutex
	setting ErrorRewriteSetting
}

var errorRewriteSetting = errorRewriteConfig{
	setting: ErrorRewriteSetting{Rules: []ErrorRewriteRule{}},
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
	return map[string]string{
		"enabled": strconv.FormatBool(c.setting.Enabled),
		"rules":   string(rules),
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
		seen[rule.StatusCode] = struct{}{}
		if strings.TrimSpace(rule.Message) == "" {
			return fmt.Errorf("error rewrite rule %d message must not be empty", index)
		}
	}
	return nil
}

// ApplyErrorRewrite updates only the error text and protocol payload message.
// The HTTP status and error code are preserved. Unknown placeholders are left
// untouched, allowing operators to use literal braces safely.
func ApplyErrorRewrite(apiErr *types.NewAPIError, modelName string) bool {
	if apiErr == nil {
		return false
	}
	upstreamStatusCode := apiErr.GetUpstreamStatusCode()
	message, ok := errorRewriteMessage(upstreamStatusCode, apiErr.StatusCode, modelName)
	if !ok {
		return false
	}

	apiErr.SetMessage(message)

	// ToOpenAIError/ToClaudeError use RelayError for upstream protocol errors,
	// so keep that payload in sync with Err.
	switch relayError := apiErr.RelayError.(type) {
	case types.OpenAIError:
		relayError.Message = message
		apiErr.RelayError = relayError
	case *types.OpenAIError:
		if relayError != nil {
			relayError.Message = message
		}
	case types.ClaudeError:
		relayError.Message = message
		apiErr.RelayError = relayError
	case *types.ClaudeError:
		if relayError != nil {
			relayError.Message = message
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
	message, ok := errorRewriteMessage(taskErr.UpstreamStatusCode, taskErr.StatusCode, modelName)
	if !ok {
		return false
	}
	taskErr.Message = message
	taskErr.Error = errors.New(message)
	return true
}

func errorRewriteMessage(upstreamStatusCode int, responseStatusCode int, modelName string) (string, bool) {
	if upstreamStatusCode < 100 || upstreamStatusCode > 599 {
		return "", false
	}
	settings := GetErrorRewriteSetting()
	if !settings.Enabled {
		return "", false
	}

	var rule *ErrorRewriteRule
	for index := range settings.Rules {
		if settings.Rules[index].StatusCode == upstreamStatusCode {
			rule = &settings.Rules[index]
			break
		}
	}
	if rule == nil {
		return "", false
	}

	message := strings.NewReplacer(
		"{model}", modelName,
		"{status_code}", strconv.Itoa(responseStatusCode),
		"{upstream_status_code}", strconv.Itoa(upstreamStatusCode),
	).Replace(rule.Message)
	return message, true
}
