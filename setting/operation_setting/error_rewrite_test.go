package operation_setting

import (
	"testing"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveErrorRewriteSetting(t *testing.T) {
	t.Helper()
	original := GetErrorRewriteSetting()
	t.Cleanup(func() {
		errorRewriteSetting.replace(original)
	})
}

func TestApplyErrorRewriteMatchesStatusAndExpandsTemplate(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{
		Enabled: true,
		Rules: []ErrorRewriteRule{{
			StatusCode: 429,
			Message:    "{model} is temporarily unavailable (upstream {status_code})",
		}},
	})

	apiErr := types.NewOpenAIError(assert.AnError, types.ErrorCodeBadResponseStatusCode, 429)
	apiErr.SetUpstreamStatusCode(429)
	changed := ApplyErrorRewrite(apiErr, "gpt-test")

	require.True(t, changed)
	assert.Equal(t, 429, apiErr.StatusCode)
	assert.Equal(t, "gpt-test is temporarily unavailable (upstream 429)", apiErr.Error())
	assert.Equal(t, apiErr.Error(), apiErr.ToOpenAIError().Message)
}

func TestApplyErrorRewriteDisabledOrUnmatched(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{
		Enabled: false,
		Rules:   []ErrorRewriteRule{{StatusCode: 500, Message: "rewritten"}},
	})

	disabledErr := types.NewOpenAIError(assert.AnError, types.ErrorCodeBadResponseStatusCode, 500)
	disabledErr.SetUpstreamStatusCode(500)
	require.False(t, ApplyErrorRewrite(disabledErr, "model"))
	assert.Equal(t, assert.AnError.Error(), disabledErr.Error())

	require.NoError(t, errorRewriteSetting.UpdateConfigMap(map[string]string{"enabled": "true"}))
	unmatchedErr := types.NewOpenAIError(assert.AnError, types.ErrorCodeBadResponseStatusCode, 404)
	unmatchedErr.SetUpstreamStatusCode(404)
	require.False(t, ApplyErrorRewrite(unmatchedErr, "model"))
	assert.Equal(t, assert.AnError.Error(), unmatchedErr.Error())
}

func TestApplyErrorRewritePreservesClaudeErrorShape(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{
		Enabled: true,
		Rules:   []ErrorRewriteRule{{StatusCode: 503, Message: "{model} unavailable"}},
	})

	apiErr := types.WithClaudeError(types.ClaudeError{
		Type:    "overloaded_error",
		Message: "original upstream message",
	}, 503)
	apiErr.SetUpstreamStatusCode(503)
	require.True(t, ApplyErrorRewrite(apiErr, "claude-test"))

	claudeErr := apiErr.ToClaudeError()
	assert.Equal(t, 503, apiErr.StatusCode)
	assert.Equal(t, "overloaded_error", claudeErr.Type)
	assert.Equal(t, "claude-test unavailable", claudeErr.Message)
}

func TestApplyErrorRewriteMatchesOriginalUpstreamStatusAfterMapping(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{
		Enabled: true,
		Rules: []ErrorRewriteRule{{
			StatusCode: 429,
			Message:    "{model}: upstream {upstream_status_code}, response {status_code}",
		}},
	})

	apiErr := types.NewOpenAIError(assert.AnError, types.ErrorCodeBadResponseStatusCode, 503)
	apiErr.SetUpstreamStatusCode(429)
	require.True(t, ApplyErrorRewrite(apiErr, "gpt-test"))
	assert.Equal(t, 503, apiErr.StatusCode)
	assert.Equal(t, "gpt-test: upstream 429, response 503", apiErr.ToOpenAIError().Message)
}

func TestApplyErrorRewriteIgnoresLocalErrorsWithMatchingStatus(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{
		Enabled: true,
		Rules:   []ErrorRewriteRule{{StatusCode: 400, Message: "rewritten"}},
	})

	apiErr := types.NewError(assert.AnError, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(400))
	require.False(t, ApplyErrorRewrite(apiErr, "gpt-test"))
	assert.Equal(t, assert.AnError.Error(), apiErr.Error())
}

func TestApplyTaskErrorRewriteRequiresUpstreamStatus(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{
		Enabled: true,
		Rules:   []ErrorRewriteRule{{StatusCode: 429, Message: "{model} is busy"}},
	})

	upstreamErr := &taskdto.TaskError{
		Message:            "original",
		StatusCode:         429,
		UpstreamStatusCode: 429,
		Error:              assert.AnError,
	}
	require.True(t, ApplyTaskErrorRewrite(upstreamErr, "video-model"))
	assert.Equal(t, "video-model is busy", upstreamErr.Message)
	assert.EqualError(t, upstreamErr.Error, "video-model is busy")

	localErr := &taskdto.TaskError{Message: "local", StatusCode: 429, Error: assert.AnError}
	require.False(t, ApplyTaskErrorRewrite(localErr, "video-model"))
	assert.Equal(t, "local", localErr.Message)
}

func TestValidateErrorRewriteRulesJSON(t *testing.T) {
	valid := `[{"status_code":429,"message":"retry {model}"},{"status_code":500,"message":"server unavailable"}]`
	require.NoError(t, ValidateErrorRewriteRulesJSON(valid))

	tests := []string{
		`{"status_code":429,"message":"not an array"}`,
		`[{"status_code":99,"message":"bad status"}]`,
		`[{"status_code":429,"message":"first"},{"status_code":429,"message":"duplicate"}]`,
		`[{"status_code":429,"message":"   "}]`,
	}
	for _, value := range tests {
		assert.Error(t, ValidateErrorRewriteRulesJSON(value), value)
	}
}

func TestErrorRewriteConfigExportsOptionKeys(t *testing.T) {
	preserveErrorRewriteSetting(t)
	errorRewriteSetting.replace(ErrorRewriteSetting{Rules: []ErrorRewriteRule{}})

	exported := config.GlobalConfig.ExportAllConfigs()
	assert.Equal(t, "false", exported["error_rewrite.enabled"])
	assert.JSONEq(t, "[]", exported["error_rewrite.rules"])
}

func TestErrorRewriteConfigLoadsThroughConfigManager(t *testing.T) {
	preserveErrorRewriteSetting(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"error_rewrite.enabled": "true",
		"error_rewrite.rules":   `[{"status_code":502,"message":"{model} unavailable"}]`,
	}))

	setting := GetErrorRewriteSetting()
	require.True(t, setting.Enabled)
	require.Equal(t, []ErrorRewriteRule{{StatusCode: 502, Message: "{model} unavailable"}}, setting.Rules)
}
