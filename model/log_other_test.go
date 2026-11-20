package model

import (
	"github.com/QuantumNous/new-api/common"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogOtherPersistedBillingSnapshot(t *testing.T) {
	original := NewLogOther()
	original.SetPublic("task_id", "safe-task")
	original.SetAdmin("quota_saturation", map[string]any{"value": "large"})
	original.SetRoot("node", "node-a")
	original.SetAudit("method", "POST")
	raw, err := common.Marshal(original)
	require.NoError(t, err)
	var restored LogOther
	require.NoError(t, common.Unmarshal(raw, &restored))
	require.JSONEq(t, original.JSONString(), restored.JSONString())
	require.NotContains(t, formatLogOtherJSON(restored.JSONString(), logOtherVisibilityUser), "quota_saturation")
}

func TestLogOtherScopesAndMerges(t *testing.T) {
	var other LogOther

	assert.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	other.MergePublic(map[string]any{
		"zero": 0,
	})
	assert.True(t, other.SetAdmin("use_channel", []string{"channel-a"}))
	other.MergeAdmin(map[string]any{
		"rejected": false,
	})
	assert.True(t, other.SetRoot("upstream_request_id", "upstream-private"))
	other.MergeRoot(map[string]any{
		"generation": 0,
	})
	assert.True(t, other.SetAudit("method", "POST"))
	other.MergeAudit(map[string]any{
		"success": false,
	})

	require.JSONEq(t, `{
		"request_path": "/v1/chat/completions",
		"zero": 0,
		"admin_info": {
			"use_channel": ["channel-a"],
			"rejected": false
		},
		"root_info": {
			"upstream_request_id": "upstream-private",
			"generation": 0
		},
		"audit_info": {
			"method": "POST",
			"success": false
		}
	}`, other.JSONString())
}

func TestLogOtherRejectsSensitivePublicFields(t *testing.T) {
	other := NewLogOther()

	for _, key := range []string{
		"admin_info",
		"root_info",
		"audit_info",
		"channel_id",
		"channel_name",
		"channel_type",
		"reject_reason",
	} {
		assert.False(t, other.SetPublic(key, "must-not-leak"), key)
	}
	other.MergePublic(map[string]any{
		"request_path": "/v1/responses",
		"channel_name": "still-must-not-leak",
		"admin_info":   map[string]any{"secret": true},
	})

	require.JSONEq(t, `{"request_path":"/v1/responses"}`, other.JSONString())
	require.JSONEq(t, `{}`, NewLogOther().JSONString())
}

func TestLogOtherJSONStringDoesNotMutateReceiver(t *testing.T) {
	other := NewLogOther()
	require.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	require.True(t, other.SetAdmin("rejected", false))

	before := other.Snapshot()
	first := other.JSONString()
	after := other.Snapshot()
	second := other.JSONString()

	require.Equal(t, before, after)
	require.Equal(t, first, second)
}
