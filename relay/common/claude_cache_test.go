package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyClaudeStablePrefixCacheControlOverridesSystemAndMessageSettings(t *testing.T) {
	input := []byte(`{
		"model":"claude-test",
		"cache_control":{"type":"ephemeral","ttl":"1h"},
		"system":[
			{"type":"text","text":"stable instructions","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"more stable instructions"}
		],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"request","cache_control":{"type":"ephemeral","ttl":"1h"}}]}
		]
	}`)

	out, err := ApplyClaudeStablePrefixCacheControl(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"claude-test",
		"system":[
			{"type":"text","text":"stable instructions"},
			{"type":"text","text":"more stable instructions","cache_control":{"type":"ephemeral"}}
		],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"request","cache_control":{"type":"ephemeral"}}]}
		]
	}`, string(out))
}

func TestApplyClaudeStablePrefixCacheControlUsesLastToolWhenPresent(t *testing.T) {
	input := []byte(`{
		"system":[{"type":"text","text":"stable instructions","cache_control":{"type":"old"}}],
		"tools":[
			{"name":"first","cache_control":{"type":"old"}},
			{"name":"last","cache_control":{"type":"old","ttl":"1h"}}
		],
		"messages":[{"role":"user","content":"request"}]
	}`)

	out, err := ApplyClaudeStablePrefixCacheControl(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"system":[{"type":"text","text":"stable instructions"}],
		"tools":[
			{"name":"first"},
			{"name":"last","cache_control":{"type":"ephemeral"}}
		],
		"messages":[{"role":"user","content":[{"type":"text","text":"request","cache_control":{"type":"ephemeral"}}]}]
	}`, string(out))
}

func TestApplyClaudeStablePrefixCacheControlConvertsSystemString(t *testing.T) {
	input := []byte(`{
		"system":"stable instructions",
		"messages":[{"role":"user","content":"request"}]
	}`)

	out, err := ApplyClaudeStablePrefixCacheControl(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"system":[{"type":"text","text":"stable instructions","cache_control":{"type":"ephemeral"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"request","cache_control":{"type":"ephemeral"}}]}]
	}`, string(out))
}

func TestApplyClaudeStablePrefixCacheControlCachesLastUserMessage(t *testing.T) {
	input := []byte(`{
		"messages":[
			{"role":"user","content":"previous request"},
			{"role":"assistant","content":"previous response"},
			{"role":"user","content":"request","cache_control":{"type":"old"}}
		]
	}`)

	out, err := ApplyClaudeStablePrefixCacheControl(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"messages":[
			{"role":"user","content":"previous request"},
			{"role":"assistant","content":"previous response"},
			{"role":"user","content":[{"type":"text","text":"request","cache_control":{"type":"ephemeral"}}]}
		]
	}`, string(out))
}

func TestApplyClaudeStablePrefixCacheControlRemovesNestedToolResultSettings(t *testing.T) {
	input := []byte(`{
		"messages":[{
			"role":"user",
			"cache_control":{"type":"old"},
			"content":[{
				"type":"tool_result",
				"content":[{"type":"text","text":"result","cache_control":{"type":"old"}}],
				"cache_control":{"type":"old"}
			}]
		}]
	}`)

	out, err := ApplyClaudeStablePrefixCacheControl(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"messages":[{
			"role":"user",
			"content":[{
				"type":"tool_result",
				"content":[{"type":"text","text":"result"}],
				"cache_control":{"type":"ephemeral"}
			}]
		}]
	}`, string(out))
}

func TestApplyClaudeStablePrefixCacheControlRejectsInvalidJSON(t *testing.T) {
	_, err := ApplyClaudeStablePrefixCacheControl([]byte(`{"messages":`))
	require.Error(t, err)
}
