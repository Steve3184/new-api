package model

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"model_price": 0.004,
		"admin_info": map[string]any{
			"quota_saturation": map[string]any{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

func TestTaskPluginLogVisibilityIsRoleSeparated(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"model_price": 1.25,
		"admin_info": map[string]any{
			"task_plugin": map[string]any{
				"key":     "document-parser",
				"name":    "Document Parser",
				"version": "1.2.3",
			},
		},
		"root_info": map[string]any{
			"upstream_task_id": "upstream-private",
			"task_plugin": map[string]any{
				"generation": 42,
			},
		},
	})

	t.Run("user", func(t *testing.T) {
		logs := []*Log{{Other: other}}
		formatUserLogs(logs, 0)

		parsed, err := common.StrToMap(logs[0].Other)
		require.NoError(t, err)
		assert.NotContains(t, parsed, "admin_info")
		assert.NotContains(t, parsed, "root_info")
		assert.Equal(t, 1.25, parsed["model_price"])
	})

	t.Run("admin", func(t *testing.T) {
		logs := []*Log{{Other: other}}
		FormatAdminLogs(logs)

		parsed, err := common.StrToMap(logs[0].Other)
		require.NoError(t, err)
		assert.Contains(t, parsed, "admin_info")
		assert.NotContains(t, parsed, "root_info")
	})

	t.Run("root", func(t *testing.T) {
		logs := []*Log{{Other: other}}
		FormatRootLogs(logs)

		parsed, err := common.StrToMap(logs[0].Other)
		require.NoError(t, err)
		assert.Contains(t, parsed, "admin_info")
		assert.Contains(t, parsed, "root_info")
	})
}

func TestPrivilegedLogViewsRestoreOriginalRewrittenError(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"status_code": 503,
		"admin_info": map[string]any{
			"original_error":       "status_code=502, upstream failed",
			"original_status_code": 502,
		},
	})

	adminLogs := []*Log{{Type: LogTypeError, Content: "status_code=503, rewritten", Other: other}}
	FormatAdminLogs(adminLogs)
	assert.Equal(t, "status_code=502, upstream failed", adminLogs[0].Content)
	adminOther, err := common.StrToMap(adminLogs[0].Other)
	require.NoError(t, err)
	assert.Equal(t, float64(502), adminOther["status_code"])

	rootLogs := []*Log{{Type: LogTypeError, Content: "status_code=503, rewritten", Other: other}}
	FormatRootLogs(rootLogs)
	assert.Equal(t, "status_code=502, upstream failed", rootLogs[0].Content)

	userLogs := []*Log{{Type: LogTypeError, Content: "status_code=503, rewritten", Other: other}}
	formatUserLogs(userLogs, 0)
	assert.Equal(t, "status_code=503, rewritten", userLogs[0].Content)
}

func TestLegacyLogOtherVisibilityIsRoleSeparated(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"request_path":  "/v1/chat/completions",
		"channel_id":    202,
		"channel_name":  "legacy-secret-channel",
		"channel_type":  1,
		"reject_reason": "legacy-policy-rejection",
		"admin_info": map[string]any{
			"existing_admin_field": "preserved",
		},
		"root_info": map[string]any{
			"upstream_request_id": "upstream-private",
		},
		"audit_info": map[string]any{
			"method": "POST",
		},
	})

	t.Run("user", func(t *testing.T) {
		logs := []*Log{{
			Id:          99,
			ChannelId:   77,
			ChannelName: "resolved-secret-channel",
			Other:       other,
		}}

		formatUserLogs(logs, 10)

		assert.Equal(t, 11, logs[0].Id)
		assert.Equal(t, 77, logs[0].ChannelId)
		assert.Empty(t, logs[0].ChannelName)
		parsed, err := common.StrToMap(logs[0].Other)
		require.NoError(t, err)
		assert.Equal(t, "/v1/chat/completions", parsed["request_path"])
		for _, key := range []string{
			"channel_id",
			"channel_name",
			"channel_type",
			"reject_reason",
			"admin_info",
			"root_info",
			"audit_info",
		} {
			assert.NotContains(t, parsed, key)
		}
	})

	t.Run("admin", func(t *testing.T) {
		logs := []*Log{{Other: other}}

		FormatAdminLogs(logs)

		parsed, err := common.StrToMap(logs[0].Other)
		require.NoError(t, err)
		assert.Equal(t, "legacy-secret-channel", parsed["channel_name"])
		assert.NotContains(t, parsed, "reject_reason")
		assert.NotContains(t, parsed, "root_info")
		assert.Contains(t, parsed, "audit_info")
		adminInfo, ok := parsed["admin_info"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "preserved", adminInfo["existing_admin_field"])
		assert.Equal(t, "legacy-policy-rejection", adminInfo["reject_reason"])
	})

	t.Run("root", func(t *testing.T) {
		logs := []*Log{{Other: other}}

		FormatRootLogs(logs)

		parsed, err := common.StrToMap(logs[0].Other)
		require.NoError(t, err)
		assert.Equal(t, "legacy-secret-channel", parsed["channel_name"])
		assert.NotContains(t, parsed, "reject_reason")
		assert.Contains(t, parsed, "root_info")
		assert.Contains(t, parsed, "audit_info")
		adminInfo, ok := parsed["admin_info"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "preserved", adminInfo["existing_admin_field"])
		assert.Equal(t, "legacy-policy-rejection", adminInfo["reject_reason"])
	})
}

func TestLegacyRejectReasonDoesNotOverrideScopedValue(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"reject_reason": "legacy-value",
		"admin_info": map[string]any{
			"reject_reason": "scoped-value",
		},
	})
	logs := []*Log{{Other: other}}

	FormatRootLogs(logs)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "reject_reason")
	adminInfo, ok := parsed["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "scoped-value", adminInfo["reject_reason"])
}

func TestUserLogOtherVisibilityRedactsNestedSensitiveFields(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"public": map[string]any{
			"channel_id":          12,
			"upstream_request_id": "upstream-secret",
			"keep":                "value",
		},
		"items": []any{
			map[string]any{
				"channel":          "provider-secret",
				"upstream_task_id": "task-secret",
				"keep":             true,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	var parsed map[string]any
	require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &parsed))
	public, ok := parsed["public"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, public, "channel_id")
	assert.NotContains(t, public, "upstream_request_id")
	assert.Equal(t, "value", public["keep"])
	items, ok := parsed["items"].([]any)
	require.True(t, ok)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, item, "channel")
	assert.NotContains(t, item, "upstream_task_id")
	assert.Equal(t, true, item["keep"])
}

func TestUserExportJSONRedactsSensitiveNestedFields(t *testing.T) {
	redacted := sanitizeUserExportJSON([]byte(`{
		"channelID": 12,
		"upstreamRequestID": "upstream-secret",
		"keep": "value",
		"items": [{"channel-name": "provider-secret", "keep": true}]
	}`))

	var parsed map[string]any
	require.NoError(t, common.UnmarshalJsonStr(redacted, &parsed))
	assert.NotContains(t, parsed, "channelID")
	assert.NotContains(t, parsed, "upstreamRequestID")
	assert.Equal(t, "value", parsed["keep"])
	items, ok := parsed["items"].([]any)
	require.True(t, ok)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, item, "channel-name")
	assert.Equal(t, true, item["keep"])
}

func TestUserExportJSONOmitsInvalidHistoricalPayload(t *testing.T) {
	assert.Empty(t, sanitizeUserExportJSON([]byte("provider response with upstream_request_id")))
}

func TestLegacyRejectReasonHandlesNullAdminInfo(t *testing.T) {
	logs := []*Log{{Other: `{"reject_reason":"legacy-value","admin_info":null}`}}

	FormatAdminLogs(logs)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "reject_reason")
	adminInfo, ok := parsed["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "legacy-value", adminInfo["reject_reason"])
}

func TestUserCSVExportsOmitProviderIdentifiers(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Log{}, &Task{}, &Midjourney{}, &AuditLog{}))
	const userID = 70101

	logEntry := &Log{
		UserId:            userID,
		CreatedAt:         1,
		Type:              LogTypeConsume,
		ModelName:         "csv-model",
		ChannelId:         17,
		UpstreamRequestId: "upstream-request-secret",
		Other: common.MapToJsonStr(map[string]any{
			"channel_id":          17,
			"upstream_request_id": "upstream-request-secret",
			"keep":                "log-value",
		}),
	}
	errorLogEntry := &Log{
		UserId:    userID,
		CreatedAt: 2,
		Type:      LogTypeError,
		Content:   "status_code=503, rewritten",
		Other: common.MapToJsonStr(map[string]any{
			"status_code": 503,
			"admin_info": map[string]any{
				"original_error":       "status_code=502, upstream failed",
				"original_status_code": 502,
			},
		}),
	}
	taskEntry := &Task{
		TaskID:    "task_csv_public",
		UserId:    userID,
		CreatedAt: 1,
		UpdatedAt: 1,
		Data:      []byte(`{"channel_id":17,"upstream_task_id":"upstream-task-secret","keep":"task-value"}`),
		PrivateData: TaskPrivateData{
			UpstreamTaskID: "upstream-task-secret",
			NodeName:       "private-node",
		},
	}
	midjourneyEntry := &Midjourney{
		UserId:     userID,
		SubmitTime: 1,
		ChannelId:  17,
		Properties: `{"channel_id":17,"upstream_request_id":"upstream-request-secret","keep":"drawing-value"}`,
	}
	auditEntry := &AuditLog{
		EventId:   "csv-audit-event",
		UserId:    userID,
		ActorRole: 1,
		CreatedAt: 1,
		RequestId: "local-request-id",
		Other: AuditOther{Op: &AuditOperation{Params: AuditFields{
			"channel_id": 17,
			"keep":       "audit-value",
		}}},
	}
	require.NoError(t, DB.Create(logEntry).Error)
	require.NoError(t, DB.Create(errorLogEntry).Error)
	require.NoError(t, DB.Create(taskEntry).Error)
	require.NoError(t, DB.Create(midjourneyEntry).Error)
	require.NoError(t, LOG_DB.Create(auditEntry).Error)
	t.Cleanup(func() {
		DB.Delete(&Log{}, logEntry.Id)
		DB.Delete(&Log{}, errorLogEntry.Id)
		DB.Delete(&Task{}, taskEntry.ID)
		DB.Delete(&Midjourney{}, midjourneyEntry.Id)
		LOG_DB.Delete(&AuditLog{}, auditEntry.Id)
	})

	readCSV := func(write func(*bytes.Buffer) error) []string {
		var buffer bytes.Buffer
		require.NoError(t, write(&buffer))
		records, err := csv.NewReader(&buffer).ReadAll()
		require.NoError(t, err)
		require.NotEmpty(t, records)
		return records[0]
	}

	logHeader := readCSV(func(buffer *bytes.Buffer) error {
		return WriteLogsCSV(buffer, userID, 0, 0, 0, "", "", "", 0, "", "", "", 0, true)
	})
	assert.NotContains(t, logHeader, "channel_id")
	assert.NotContains(t, logHeader, "upstream_request_id")
	var logCSV bytes.Buffer
	require.NoError(t, WriteLogsCSV(&logCSV, userID, 0, 0, 0, "", "", "", 0, "", "", "", 0, true))
	assert.NotContains(t, logCSV.String(), "upstream-request-secret")
	assert.Contains(t, logCSV.String(), "status_code=503, rewritten")
	assert.NotContains(t, logCSV.String(), "status_code=502, upstream failed")

	var adminLogCSV bytes.Buffer
	require.NoError(t, WriteLogsCSV(&adminLogCSV, 0, 0, 0, 0, "", "", "", 0, "", "", "", common.RoleAdminUser, false))
	assert.Contains(t, adminLogCSV.String(), "status_code=502, upstream failed")
	assert.NotContains(t, adminLogCSV.String(), "status_code=503, rewritten")

	taskHeader := readCSV(func(buffer *bytes.Buffer) error {
		return WriteTaskCSV(buffer, userID, SyncTaskQueryParams{}, 0, true)
	})
	assert.NotContains(t, taskHeader, "channel_id")
	assert.NotContains(t, taskHeader, "upstream_task_id")
	assert.NotContains(t, taskHeader, "node_name")
	var taskCSV bytes.Buffer
	require.NoError(t, WriteTaskCSV(&taskCSV, userID, SyncTaskQueryParams{}, 0, true))
	assert.NotContains(t, taskCSV.String(), "upstream-task-secret")

	midjourneyHeader := readCSV(func(buffer *bytes.Buffer) error {
		return WriteMidjourneyCSV(buffer, userID, TaskQueryParams{})
	})
	assert.NotContains(t, midjourneyHeader, "channel_id")
	var midjourneyCSV bytes.Buffer
	require.NoError(t, WriteMidjourneyCSV(&midjourneyCSV, userID, TaskQueryParams{}))
	assert.NotContains(t, midjourneyCSV.String(), "upstream-request-secret")

	var auditCSV bytes.Buffer
	require.NoError(t, WriteAuditLogsCSV(&auditCSV, AuditLogFilter{SelfView: true, UserId: userID}, 1))
	auditRecords, err := csv.NewReader(&auditCSV).ReadAll()
	require.NoError(t, err)
	require.Len(t, auditRecords, 2)
	assert.Contains(t, auditRecords[0], "request_id")
	assert.Contains(t, auditRecords[1], "local-request-id")
	assert.NotContains(t, auditCSV.String(), "channel_id")
	assert.NotContains(t, auditCSV.String(), "upstream-request-secret")
}

func TestLogFormattingPreservesLargeIntegerLexemes(t *testing.T) {
	const other = `{"public_id":9007199254740993,"admin_info":{"admin_id":9007199254740995},"root_info":{"generation":18446744073709551615}}`

	t.Run("user", func(t *testing.T) {
		logs := []*Log{{Other: other}}

		formatUserLogs(logs, 0)

		assert.Contains(t, logs[0].Other, `"public_id":9007199254740993`)
		assert.NotContains(t, logs[0].Other, "admin_id")
		assert.NotContains(t, logs[0].Other, "generation")
	})

	t.Run("admin", func(t *testing.T) {
		logs := []*Log{{Other: other}}

		FormatAdminLogs(logs)

		assert.Contains(t, logs[0].Other, `"public_id":9007199254740993`)
		assert.Contains(t, logs[0].Other, `"admin_id":9007199254740995`)
		assert.NotContains(t, logs[0].Other, "generation")
	})

	t.Run("root", func(t *testing.T) {
		logs := []*Log{{Other: other}}

		FormatRootLogs(logs)

		assert.Equal(t, other, logs[0].Other)
	})

	t.Run("unprivileged", func(t *testing.T) {
		const unprivileged = `{"public_id":9007199254740993,"model_price":0.004}`

		userLogs := []*Log{{Other: unprivileged}}
		formatUserLogs(userLogs, 0)
		assert.Equal(t, unprivileged, userLogs[0].Other)

		adminLogs := []*Log{{Other: unprivileged}}
		FormatAdminLogs(adminLogs)
		assert.Equal(t, unprivileged, adminLogs[0].Other)
	})
}
