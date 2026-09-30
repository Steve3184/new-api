package model

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	AuditCategoryLogin       = "login"
	AuditCategorySecurity    = "security"
	AuditCategoryOperation   = "operation"
	AuditCategoryAccessToken = "access_token"
)

// AuditLog is retained independently of usage logs and their cleanup/TTL policy.
// TokenRef identifies a PAT generation without storing its bearer credential.
type AuditLog struct {
	Id         int        `json:"id"`
	EventId    string     `json:"event_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId     int        `json:"user_id" gorm:"index:idx_audit_user_time,priority:1"`
	Username   string     `json:"username" gorm:"type:varchar(64);index"`
	ActorRole  int        `json:"actor_role"` // Immutable role of the actor when the event began, not the log owner.
	CreatedAt  int64      `json:"created_at" gorm:"type:bigint;index:idx_audit_user_time,priority:2;index:idx_audit_token_time,priority:2;index"`
	Category   string     `json:"category" gorm:"type:varchar(24);index"`
	Action     string     `json:"action" gorm:"type:varchar(128)"`
	TokenRef   string     `json:"token_ref" gorm:"type:varchar(64);index:idx_audit_token_time,priority:1"`
	AuthMethod string     `json:"auth_method" gorm:"type:varchar(24)"`
	Ip         string     `json:"ip" gorm:"type:varchar(64)"`
	UserAgent  string     `json:"user_agent" gorm:"type:varchar(512)"`
	Method     string     `json:"method" gorm:"type:varchar(16)"`
	Route      string     `json:"route" gorm:"type:varchar(255)"`
	Status     int        `json:"status"`
	Success    bool       `json:"success"`
	RequestId  string     `json:"request_id" gorm:"type:varchar(64);index"`
	Content    string     `json:"content" gorm:"type:text"`
	Other      AuditOther `json:"other" gorm:"type:json"`
}

type AuditLogFilter struct {
	SelfView        bool // Server-selected metadata projection, independent of the viewer's actual role.
	UserId          int
	Username        string
	Category        string
	TokenRef        string
	ExcludeTokenRef string
	RequestId       string
	StartTimestamp  int64
	EndTimestamp    int64
	Success         *bool
}

func AccessTokenFingerprint(token string) string {
	// PostgreSQL returns CHAR(32) tokens padded with spaces. Normalize that
	// storage padding so persisted tokens and incoming credentials share a ref.
	token = strings.TrimRight(token, " ")
	if token == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", digest)
}

// RecordAuditLog captures safe request metadata only; raw URLs, query strings,
// credentials and response/request bodies must never enter this table.
func RecordAuditLog(c *gin.Context, entry AuditLog) {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
		entry.RequestId = c.GetString(common.RequestIdKey)
		entry.Ip = c.ClientIP()
		entry.UserAgent = c.Request.UserAgent()
		entry.Method = c.Request.Method
		entry.Route = c.FullPath()
		if entry.Status == 0 {
			entry.Status = c.Writer.Status()
		}
		if entry.AuthMethod == "" {
			entry.AuthMethod = "session"
			if c.GetBool("use_access_token") {
				entry.AuthMethod = "access_token"
			}
		}
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = common.GetTimestamp()
	}
	if entry.RequestId == "" {
		entry.RequestId = common.NewRequestId()
	}
	if entry.EventId == "" {
		entry.EventId = common.NewRequestId()
	}
	switch entry.ActorRole {
	case common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser:
	default:
		logger.LogError(ctx, fmt.Sprintf("audit actor role unavailable (request_id=%s, actor_role=%d)", entry.RequestId, entry.ActorRole))
		entry.ActorRole = 0 // Unknown actors remain visible to root only.
	}
	if entry.Username == "" {
		entry.Username, _ = GetUsernameById(entry.UserId, false)
	}
	ua := []rune(entry.UserAgent)
	if len(ua) > 512 {
		entry.UserAgent = string(ua[:512])
	}
	if LOG_DB == nil {
		logger.LogError(ctx, fmt.Sprintf("audit log write failed (request_id=%s): log database unavailable", entry.RequestId))
		return
	}
	var row any = &entry
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		encoded, err := common.Marshal(entry.Other)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("audit log write failed (request_id=%s): %v", entry.RequestId, err))
			return
		}
		// The ClickHouse GORM insert callback passes structs to the native
		// driver without resolving their Valuer. Bind this column's JSON
		// encoding while retaining AuditOther in the domain and API models.
		row = &struct {
			AuditLog     `gorm:"embedded"`
			EncodedOther string `gorm:"column:other;type:json"`
		}{AuditLog: entry, EncodedOther: string(encoded)}
	}
	if err := LOG_DB.Table("audit_logs").Create(row).Error; err != nil {
		logger.LogError(ctx, fmt.Sprintf("audit log write failed (request_id=%s): %v", entry.RequestId, err))
	}
}

func buildAuditLogQuery(filter AuditLogFilter, viewerRole int) *gorm.DB {
	query := LOG_DB.Model(&AuditLog{})
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		// Decode native JSON through database/sql as text for AuditOther.Scan.
		// Preserve numeric metadata instead of returning quoted Int64 values.
		query = query.WithContext(clickhouse.Context(query.Statement.Context, clickhouse.WithSettings(clickhouse.Settings{
			"output_format_native_write_json_as_string": 1,
			"output_format_json_quote_64bit_integers":   0,
		})))
	}
	if viewerRole < common.RoleRootUser {
		query = query.Where("actor_role IN ?", []int{common.RoleCommonUser, common.RoleAdminUser})
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.Username != "" {
		query = query.Where("username = ?", filter.Username)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.TokenRef != "" {
		query = query.Where("token_ref = ?", filter.TokenRef)
	}
	if filter.ExcludeTokenRef != "" {
		query = query.Where("token_ref <> ?", filter.ExcludeTokenRef)
	}
	if filter.RequestId != "" {
		query = query.Where("request_id = ?", filter.RequestId)
	}
	if filter.StartTimestamp > 0 {
		query = query.Where("created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp > 0 {
		query = query.Where("created_at <= ?", filter.EndTimestamp)
	}
	if filter.Success != nil {
		query = query.Where("success = ?", *filter.Success)
	}
	return query
}

func auditLogVisibility(filter AuditLogFilter, viewerRole int) logOtherVisibility {
	visibility := logOtherVisibilityUser
	if !filter.SelfView && viewerRole >= common.RoleRootUser {
		return logOtherVisibilityRoot
	}
	if !filter.SelfView && viewerRole >= common.RoleAdminUser {
		return logOtherVisibilityAdmin
	}
	return visibility
}

func projectAuditLogOther(entry *AuditLog, visibility logOtherVisibility) {
	if visibility != logOtherVisibilityRoot {
		entry.Other.RootInfo = nil
	}
	if visibility == logOtherVisibilityUser {
		entry.Other.AdminInfo = nil
		entry.Other.AuditInfo = nil
		if entry.Other.Op != nil {
			entry.Other.Op.Params = redactAuditFields(entry.Other.Op.Params)
		}
	}
}

func isPrivateAuditField(key string) bool {
	return isUserHiddenLogOtherKey(key)
}

func redactAuditFields(fields AuditFields) AuditFields {
	redacted, changed := redactAuditFieldsValue(fields)
	if !changed {
		return fields
	}
	return redacted
}

func redactAuditFieldsValue(fields AuditFields) (AuditFields, bool) {
	if len(fields) == 0 {
		return fields, false
	}
	redacted := make(AuditFields, len(fields))
	changed := false
	for key, value := range fields {
		if isPrivateAuditField(key) {
			changed = true
			continue
		}
		if next, valueChanged := redactAuditValue(value); valueChanged {
			redacted[key] = next
			changed = true
		} else {
			redacted[key] = value
		}
	}
	return redacted, changed
}

func redactAuditValue(value any) (any, bool) {
	switch typed := value.(type) {
	case AuditFields:
		return redactAuditFieldsValue(typed)
	case map[string]any:
		redacted, changed := redactAuditFieldsValue(AuditFields(typed))
		if !changed {
			return typed, false
		}
		return map[string]any(redacted), true
	case []any:
		redacted := make([]any, len(typed))
		changed := false
		for index, item := range typed {
			if next, itemChanged := redactAuditValue(item); itemChanged {
				redacted[index] = next
				changed = true
			} else {
				redacted[index] = item
			}
		}
		return redacted, changed
	case json.RawMessage:
		return redactAuditJSON(typed)
	default:
		return value, false
	}
}

func redactAuditJSON(value json.RawMessage) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return value, false
	}
	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := common.Unmarshal(trimmed, &object); err != nil {
			return value, false
		}
		changed := false
		for key, item := range object {
			if isPrivateAuditField(key) {
				delete(object, key)
				changed = true
				continue
			}
			if next, itemChanged := redactAuditJSON(item); itemChanged {
				object[key] = next
				changed = true
			}
		}
		if !changed {
			return value, false
		}
		encoded, err := common.Marshal(object)
		if err != nil {
			return value, false
		}
		return encoded, true
	case '[':
		var array []json.RawMessage
		if err := common.Unmarshal(trimmed, &array); err != nil {
			return value, false
		}
		changed := false
		for index, item := range array {
			if next, itemChanged := redactAuditJSON(item); itemChanged {
				array[index] = next
				changed = true
			}
		}
		if !changed {
			return value, false
		}
		encoded, err := common.Marshal(array)
		if err != nil {
			return value, false
		}
		return encoded, true
	default:
		return value, false
	}
}

func GetAuditLogs(filter AuditLogFilter, start, limit, viewerRole int) ([]*AuditLog, int64, error) {
	query := buildAuditLogQuery(filter, viewerRole)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]*AuditLog, 0)
	if err := query.Order("created_at DESC").Order("event_id DESC").Offset(start).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	visibility := auditLogVisibility(filter, viewerRole)
	for _, entry := range logs {
		projectAuditLogOther(entry, visibility)
	}
	return logs, total, nil
}

var auditCSVHeader = []string{
	"event_id", "user_id", "username", "actor_role", "created_at", "category", "action",
	"token_ref", "auth_method", "ip", "user_agent", "method", "route", "status", "success",
	"request_id", "content", "other",
}

// WriteAuditLogsCSV streams matching audit events with the same visibility
// projection as the paginated API.
func WriteAuditLogsCSV(writer io.Writer, filter AuditLogFilter, viewerRole int) error {
	query := buildAuditLogQuery(filter, viewerRole)
	rows, err := query.Order("created_at DESC").Order("event_id DESC").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write(auditCSVHeader); err != nil {
		return err
	}
	visibility := auditLogVisibility(filter, viewerRole)
	for rows.Next() {
		var entry AuditLog
		if err := query.ScanRows(rows, &entry); err != nil {
			return err
		}
		projectAuditLogOther(&entry, visibility)
		other, err := common.Marshal(entry.Other)
		if err != nil {
			return err
		}
		if err := csvWriter.Write([]string{
			entry.EventId, strconv.Itoa(entry.UserId), entry.Username, strconv.Itoa(entry.ActorRole),
			strconv.FormatInt(entry.CreatedAt, 10), entry.Category, entry.Action, entry.TokenRef,
			entry.AuthMethod, entry.Ip, entry.UserAgent, entry.Method, entry.Route, strconv.Itoa(entry.Status),
			strconv.FormatBool(entry.Success), entry.RequestId, entry.Content, string(other),
		}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

// UserAccessTokenStatus describes the legacy users.access_token credential.
//
// Deprecated: 旧版面板访问令牌，仅在升级后的过渡期内使用；删除 users.access_token 列时一并移除。
type UserAccessTokenStatus struct {
	Exists     bool   `json:"exists"`
	TokenHint  string `json:"token_hint"`
	TokenRef   string `json:"token_ref"`
	CreatedAt  *int64 `json:"created_at"`
	LastUsedAt *int64 `json:"last_used_at"`
	LastUsedIp string `json:"last_used_ip"`
}

// GetUserAccessTokenStatus reads last use of the legacy token from the access audit.
//
// Deprecated: 旧版面板访问令牌，仅在升级后的过渡期内使用；删除 users.access_token 列时一并移除。
func GetUserAccessTokenStatus(userId int) (*UserAccessTokenStatus, error) {
	var user User
	if err := DB.Select("id", "role", "access_token", "access_token_created_at").First(&user, userId).Error; err != nil {
		return nil, err
	}
	status := &UserAccessTokenStatus{Exists: user.GetAccessToken() != ""}
	if !status.Exists {
		return status, nil
	}
	status.TokenRef = AccessTokenFingerprint(user.GetAccessToken())
	status.TokenHint = AccessTokenHint(user.GetAccessToken())
	status.CreatedAt = user.AccessTokenCreatedAt
	var latest AuditLog
	query := LOG_DB.Select("created_at", "ip").Where("user_id = ? AND token_ref = ? AND category = ?", userId, status.TokenRef, AuditCategoryAccessToken)
	if user.Role < common.RoleRootUser {
		query = query.Where("actor_role IN ?", []int{common.RoleCommonUser, common.RoleAdminUser})
	}
	err := query.Order("created_at DESC").Order("event_id DESC").Take(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status, nil
	}
	if err != nil {
		return nil, err
	}
	status.LastUsedAt = &latest.CreatedAt
	status.LastUsedIp = latest.Ip
	return status, nil
}

// MigrateAuditLogs also supports independently configured ClickHouse log stores.
// No TTL clause or usage-log cleanup integration is intentional.
func MigrateAuditLogs() error {
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return LOG_DB.AutoMigrate(&AuditLog{})
	}
	return LOG_DB.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id Int64 DEFAULT 0, event_id String, user_id Int64, username String, actor_role Int32,
		created_at Int64, category String, action String, token_ref String,
		auth_method String, ip String, user_agent String, method String, route String,
		status Int32, success UInt8, request_id String, content String, other JSON
	) ENGINE = MergeTree()
	PARTITION BY toYYYYMM(toDateTime(created_at))
	ORDER BY (created_at, event_id)`).Error
}

func ValidAuditCategory(category string) bool {
	return category == "" || category == AuditCategoryLogin || category == AuditCategorySecurity || category == AuditCategoryOperation || category == AuditCategoryAccessToken
}

func ValidTokenFingerprint(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 64 {
		return false
	}
	return strings.Trim(value, "0123456789abcdef") == ""
}
