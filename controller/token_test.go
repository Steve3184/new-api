package controller

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tokenAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenPageResponse struct {
	Items []tokenResponseItem `json:"items"`
}

type tokenResponseItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Key    string `json:"key"`
	Status int    `json:"status"`
}

type tokenKeyResponse struct {
	Key string `json:"key"`
}

type tokenRPMResponse struct {
	RPMs map[string]int `json:"rpms"`
}

type sqliteColumnInfo struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

type legacyToken struct {
	Id                 int    `gorm:"primaryKey"`
	UserId             int    `gorm:"index"`
	Key                string `gorm:"column:key;type:char(48);uniqueIndex"`
	Status             int    `gorm:"default:1"`
	Name               string `gorm:"index"`
	CreatedTime        int64  `gorm:"bigint"`
	AccessedTime       int64  `gorm:"bigint"`
	ExpiredTime        int64  `gorm:"bigint;default:-1"`
	RemainQuota        int    `gorm:"default:0"`
	UnlimitedQuota     bool
	ModelLimitsEnabled bool
	ModelLimits        string  `gorm:"type:text"`
	AllowIps           *string `gorm:"default:''"`
	UsedQuota          int     `gorm:"default:0"`
	Group              string  `gorm:"column:group;default:''"`
	CrossGroupRetry    bool
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (legacyToken) TableName() string {
	return "tokens"
}

func openTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func migrateTokenControllerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}
}

func setupTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := openTokenControllerTestDB(t)
	migrateTokenControllerTestDB(t, db)
	return db
}

func openTokenControllerExternalDB(t *testing.T, dialect string, dsn string) (*gorm.DB, *bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false

	var (
		db     *gorm.DB
		dbType common.DatabaseType
		err    error
	)
	switch dialect {
	case "mysql":
		dbType = common.DatabaseTypeMySQL
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		dbType = common.DatabaseTypePostgreSQL
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", dialect)
	}
	common.SetDatabaseTypes(dbType, dbType)
	if err != nil {
		t.Fatalf("failed to open %s db: %v", dialect, err)
	}

	model.DB = db
	model.LOG_DB = db

	if db.Migrator().HasTable("tokens") {
		t.Skipf("refusing to run %s migration compatibility test against external database because tokens table already exists", dialect)
	}

	managedTokensTable := new(bool)

	t.Cleanup(func() {
		if *managedTokensTable && db.Migrator().HasTable("tokens") {
			_ = db.Migrator().DropTable("tokens")
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db, managedTokensTable
}

func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func newAuthenticatedContext(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func getSQLiteColumnType(t *testing.T, db *gorm.DB, tableName string, columnName string) string {
	t.Helper()

	var columns []sqliteColumnInfo
	if err := db.Raw("PRAGMA table_info(" + tableName + ")").Scan(&columns).Error; err != nil {
		t.Fatalf("failed to inspect %s schema: %v", tableName, err)
	}

	for _, column := range columns {
		if column.Name == columnName {
			return strings.ToLower(column.Type)
		}
	}

	t.Fatalf("column %s not found in %s schema", columnName, tableName)
	return ""
}

func getTokenKeyColumnType(t *testing.T, db *gorm.DB, dialect string) string {
	t.Helper()

	switch dialect {
	case "sqlite":
		return getSQLiteColumnType(t, db, "tokens", "key")
	case "mysql":
		var columnType string
		if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Scan(&columnType).Error; err != nil {
			t.Fatalf("failed to inspect mysql token key column: %v", err)
		}
		return strings.ToLower(columnType)
	case "postgres":
		var dataType string
		var maxLength sql.NullInt64
		if err := db.Raw(`SELECT data_type, character_maximum_length
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Row().Scan(&dataType, &maxLength); err != nil {
			t.Fatalf("failed to inspect postgres token key column: %v", err)
		}
		switch strings.ToLower(dataType) {
		case "character varying":
			return fmt.Sprintf("varchar(%d)", maxLength.Int64)
		case "character":
			return fmt.Sprintf("char(%d)", maxLength.Int64)
		default:
			if maxLength.Valid {
				return fmt.Sprintf("%s(%d)", strings.ToLower(dataType), maxLength.Int64)
			}
			return strings.ToLower(dataType)
		}
	default:
		t.Fatalf("unsupported dialect %q", dialect)
		return ""
	}
}

func getTokenAutoGroupsColumnType(t *testing.T, db *gorm.DB, dialect string) string {
	t.Helper()

	switch dialect {
	case "sqlite":
		return getSQLiteColumnType(t, db, "tokens", "auto_groups")
	case "mysql":
		var columnType string
		if err := db.Raw(`SELECT DATA_TYPE FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			"tokens", "auto_groups").Scan(&columnType).Error; err != nil {
			t.Fatalf("failed to inspect mysql token auto_groups column: %v", err)
		}
		return strings.ToLower(columnType)
	case "postgres":
		var dataType string
		if err := db.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			"tokens", "auto_groups").Scan(&dataType).Error; err != nil {
			t.Fatalf("failed to inspect postgres token auto_groups column: %v", err)
		}
		return strings.ToLower(dataType)
	default:
		t.Fatalf("unsupported dialect %q", dialect)
		return ""
	}
}

func runTokenMigrationCompatibilityTest(t *testing.T, db *gorm.DB, dialect string, managedTokensTable *bool) {
	t.Helper()

	legacyKey := strings.Repeat("a", 48)
	longKey := strings.Repeat("b", 64)

	if err := db.AutoMigrate(&legacyToken{}); err != nil {
		t.Fatalf("failed to create legacy token schema: %v", err)
	}
	if managedTokensTable != nil {
		*managedTokensTable = true
	}
	if err := db.Create(&legacyToken{
		UserId:             7,
		Key:                legacyKey,
		Status:             common.TokenStatusEnabled,
		Name:               "legacy-token",
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        100,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}).Error; err != nil {
		t.Fatalf("failed to seed legacy token row: %v", err)
	}

	if got := getTokenKeyColumnType(t, db, dialect); got != "char(48)" {
		t.Fatalf("expected legacy key column type char(48), got %q", got)
	}

	migrateTokenControllerTestDB(t, db)

	if got := getTokenKeyColumnType(t, db, dialect); got != "varchar(128)" {
		t.Fatalf("expected migrated key column type varchar(128), got %q", got)
	}
	if !db.Migrator().HasColumn(&model.Token{}, "auto_groups") {
		t.Fatal("expected migration to add auto_groups column")
	}
	if got := getTokenAutoGroupsColumnType(t, db, dialect); got != "text" {
		t.Fatalf("expected migrated auto_groups column type text, got %q", got)
	}

	var migratedToken model.Token
	if err := db.First(&migratedToken, "name = ?", "legacy-token").Error; err != nil {
		t.Fatalf("failed to load migrated token row: %v", err)
	}
	if migratedToken.Key != legacyKey {
		t.Fatalf("expected migrated token key %q, got %q", legacyKey, migratedToken.Key)
	}
	if migratedToken.Name != "legacy-token" {
		t.Fatalf("expected migrated token name to be preserved, got %q", migratedToken.Name)
	}
	if migratedToken.AutoGroups != "" {
		t.Fatalf("expected legacy token to inherit global Auto groups, got %q", migratedToken.AutoGroups)
	}

	inserted := model.Token{
		UserId:             8,
		Name:               "long-token",
		Key:                longKey,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        200,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}
	if err := db.Create(&inserted).Error; err != nil {
		t.Fatalf("failed to insert long token after migration: %v", err)
	}

	var fetched model.Token
	if err := db.First(&fetched, "id = ?", inserted.Id).Error; err != nil {
		t.Fatalf("failed to fetch long token after migration: %v", err)
	}
	if fetched.Key != longKey {
		t.Fatalf("expected long token key %q, got %q", longKey, fetched.Key)
	}
}

func TestTokenAutoMigrateUsesVarchar128KeyColumn(t *testing.T) {
	db := setupTokenControllerTestDB(t)

	if got := getTokenKeyColumnType(t, db, "sqlite"); got != "varchar(128)" {
		t.Fatalf("expected key column type varchar(128), got %q", got)
	}
	if got := getSQLiteColumnType(t, db, "tokens", "auto_groups"); got != "text" {
		t.Fatalf("expected auto_groups column type text, got %q", got)
	}
}

func TestTokenMigrationFromChar48ToVarchar128(t *testing.T) {
	db := openTokenControllerTestDB(t)
	runTokenMigrationCompatibilityTest(t, db, "sqlite", nil)
}

func TestTokenMigrationFromChar48ToVarchar128MySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "mysql", dsn)
	runTokenMigrationCompatibilityTest(t, db, "mysql", managedTokensTable)
}

func TestTokenMigrationFromChar48ToVarchar128Postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run postgres migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "postgres", dsn)
	runTokenMigrationCompatibilityTest(t, db, "postgres", managedTokensTable)
}

func TestGetAllTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "list-token", "abcd1234efgh5678")
	seedToken(t, db, 2, "other-user-token", "zzzz1234yyyy5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("list response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestSearchTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "searchable-token", "ijkl1234mnop5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=searchable-token&p=1&size=10", nil, 1)
	SearchTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one search result, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked search key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("search response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenRPMOnlyReturnsOwnedTokenStats(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	ownedToken := seedToken(t, db, 1, "owned-rpm-token", "owned1234efgh5678")
	otherToken := seedToken(t, db, 2, "other-rpm-token", "other1234efgh5678")
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.Log{
		CreatedAt: now,
		Type:      model.LogTypeConsume,
		TokenId:   ownedToken.Id,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		CreatedAt: now,
		Type:      model.LogTypeConsume,
		TokenId:   otherToken.Id,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/rpm", nil, 1)
	ctx.Request.URL.RawQuery = "ids=" + strconv.Itoa(ownedToken.Id) + "&ids=" + strconv.Itoa(otherToken.Id)
	GetTokenRPM(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, "expected success response, got message: %s", response.Message)
	var payload tokenRPMResponse
	require.NoError(t, common.Unmarshal(response.Data, &payload))
	assert.Equal(t, 1, payload.RPMs[strconv.Itoa(ownedToken.Id)])
	assert.NotContains(t, payload.RPMs, strconv.Itoa(otherToken.Id))
}

func TestGetTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "detail-token", "qrst1234uvwx5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked detail key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("detail response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "editable-token", "yzab1234cdef5678")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked update key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenKeyRequiresOwnershipAndReturnsFullKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(authorizedCtx)

	authorizedResponse := decodeAPIResponse(t, authorizedRecorder)
	if !authorizedResponse.Success {
		t.Fatalf("expected authorized key fetch to succeed, got message: %s", authorizedResponse.Message)
	}

	var keyData tokenKeyResponse
	if err := common.Unmarshal(authorizedResponse.Data, &keyData); err != nil {
		t.Fatalf("failed to decode token key response: %v", err)
	}
	if keyData.Key != token.GetFullKey() {
		t.Fatalf("expected full key %q, got %q", token.GetFullKey(), keyData.Key)
	}

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(unauthorizedCtx)

	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	if unauthorizedResponse.Success {
		t.Fatalf("expected unauthorized key fetch to fail")
	}
	if strings.Contains(unauthorizedRecorder.Body.String(), token.Key) {
		t.Fatalf("unauthorized key response leaked raw token key: %s", unauthorizedRecorder.Body.String())
	}
}

func TestMigrateTokenGroupMovesOnlyActiveSourceTokens(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	sourceOne := seedToken(t, db, 1, "source-one", "migration-source-one")
	sourceTwo := seedToken(t, db, 2, "source-two", "migration-source-two")
	other := seedToken(t, db, 3, "other", "migration-other")
	deleted := seedToken(t, db, 4, "deleted", "migration-deleted")

	require.NoError(t, db.Model(&model.Token{}).
		Where("id IN ?", []int{sourceOne.Id, sourceTwo.Id, deleted.Id}).
		Update("group", "ClaudeA").Error)
	require.NoError(t, db.Model(&model.Token{}).
		Where("id = ?", other.Id).
		Update("group", "Other").Error)
	require.NoError(t, db.Delete(deleted).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/group/migrate", map[string]any{
		"source_group": " ClaudeA ",
		"target_group": "vip",
	}, 1)
	MigrateTokenGroup(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var result struct {
		SourceGroup string `json:"source_group"`
		TargetGroup string `json:"target_group"`
		Migrated    int    `json:"migrated"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &result))
	assert.Equal(t, "ClaudeA", result.SourceGroup)
	assert.Equal(t, "vip", result.TargetGroup)
	assert.Equal(t, 2, result.Migrated)

	for _, tokenID := range []int{sourceOne.Id, sourceTwo.Id} {
		var token model.Token
		require.NoError(t, db.First(&token, tokenID).Error)
		assert.Equal(t, "vip", token.Group)
	}
	var otherToken model.Token
	require.NoError(t, db.First(&otherToken, other.Id).Error)
	assert.Equal(t, "Other", otherToken.Group)
	var deletedToken model.Token
	require.NoError(t, db.Unscoped().First(&deletedToken, deleted.Id).Error)
	assert.Equal(t, "ClaudeA", deletedToken.Group)
}

func TestMigrateTokenGroupRejectsInvalidTarget(t *testing.T) {
	setupTokenControllerTestDB(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/group/migrate", map[string]any{
		"source_group": "ClaudeA",
		"target_group": "missing-group",
	}, 1)

	MigrateTokenGroup(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "target_group")
}

func TestTokenAllowUserQuotaQueryOnCreateAndUpdate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		stored *bool // nil creates the token through the API
		field  any   // nil omits allow_user_quota_query from the request
		want   bool
	}{
		{name: "create without the field turns it on", target: "/api/token/", want: true},
		{name: "create with false keeps it off", target: "/api/token/", field: false},
		{name: "update without the field keeps it on", target: "/api/token/", stored: common.GetPointer(true), want: true},
		{name: "update without the field keeps it off", target: "/api/token/", stored: common.GetPointer(false)},
		{name: "update turns it off", target: "/api/token/", stored: common.GetPointer(true), field: false},
		{name: "update turns it on", target: "/api/token/", stored: common.GetPointer(false), field: true, want: true},
		{name: "status update leaves it alone", target: "/api/token/?status_only=true", stored: common.GetPointer(true), field: false, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTokenControllerTestDB(t)
			body := map[string]any{"name": "quota-query", "expired_time": -1, "unlimited_quota": true, "status": common.TokenStatusEnabled}
			if tc.field != nil {
				body["allow_user_quota_query"] = tc.field
			}
			var recorder *httptest.ResponseRecorder
			if tc.stored == nil {
				var ctx *gin.Context
				ctx, recorder = newAuthenticatedContext(t, http.MethodPost, tc.target, body, 1)
				AddToken(ctx)
			} else {
				token := seedToken(t, db, 1, "quota-query", "quota-query-key")
				require.NoError(t, db.Model(token).Update("allow_user_quota_query", *tc.stored).Error)
				body["id"] = token.Id
				var ctx *gin.Context
				ctx, recorder = newAuthenticatedContext(t, http.MethodPut, tc.target, body, 1)
				UpdateToken(ctx)
			}
			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)

			var saved model.Token
			require.NoError(t, db.Where("name = ?", "quota-query").First(&saved).Error)
			assert.Equal(t, tc.want, saved.AllowUserQuotaQuery)
			if tc.stored != nil {
				var returned struct {
					AllowUserQuotaQuery bool `json:"allow_user_quota_query"`
				}
				require.NoError(t, common.Unmarshal(response.Data, &returned))
				assert.Equal(t, tc.want, returned.AllowUserQuotaQuery)
			}
		})
	}
}

func TestGetTokenUsageFollowsTheUserQuotaSwitchAndIPWhitelist(t *testing.T) {
	for _, tc := range []struct {
		name          string
		allow         bool
		expired       bool
		allowIps      string
		remoteAddr    string
		wantStatus    int
		wantUserQuota bool
	}{
		{name: "switch off", remoteAddr: "203.0.113.7:4321", wantStatus: http.StatusOK},
		{name: "switch on without IP whitelist", allow: true, remoteAddr: "203.0.113.7:4321", wantStatus: http.StatusOK, wantUserQuota: true},
		{name: "switch on with matching IP whitelist", allow: true, allowIps: "198.51.100.1\n203.0.113.0/24", remoteAddr: "203.0.113.7:4321", wantStatus: http.StatusOK, wantUserQuota: true},
		{name: "expired key with the switch on", allow: true, expired: true, remoteAddr: "203.0.113.7:4321", wantStatus: http.StatusOK, wantUserQuota: true},
		{name: "switch on from an IP outside the whitelist", allow: true, allowIps: "198.51.100.1", remoteAddr: "203.0.113.7:4321", wantStatus: http.StatusForbidden},
		{name: "switch off from an IP outside the whitelist", allowIps: "198.51.100.1", remoteAddr: "203.0.113.7:4321", wantStatus: http.StatusForbidden},
		{name: "unresolved client IP with a whitelist", allow: true, allowIps: "203.0.113.0/24", remoteAddr: "unresolved", wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTokenControllerTestDB(t)
			require.NoError(t, db.AutoMigrate(&model.User{}))
			// Initialize production column quoting for the token key lookup.
			t.Setenv("LOG_SQL_DSN", "")
			require.NoError(t, model.InitLogDB())
			user := &model.User{Username: "quota-owner", Password: "placeholder", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", Quota: 12345, UsedQuota: 678, AuthVersion: 1, AffCode: "quota-owner"}
			require.NoError(t, db.Create(user).Error)
			token := seedToken(t, db, user.Id, "usage-token", "usagekey1234")
			updates := map[string]any{"allow_user_quota_query": tc.allow, "allow_ips": tc.allowIps, "used_quota": 30}
			if tc.expired {
				updates["status"], updates["expired_time"] = common.TokenStatusExpired, 1
			}
			require.NoError(t, db.Model(token).Updates(updates).Error)

			router := gin.New()
			router.GET("/api/usage/token/", middleware.TokenAuthReadOnly(), GetTokenUsage)
			request := httptest.NewRequest(http.MethodGet, "/api/usage/token/", nil)
			request.Header.Set("Authorization", "Bearer sk-usagekey1234")
			request.RemoteAddr = tc.remoteAddr
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			if tc.wantStatus != http.StatusOK {
				assert.NotContains(t, recorder.Body.String(), "total_available")
				return
			}
			var response struct {
				Code bool                       `json:"code"`
				Data map[string]json.RawMessage `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Code, recorder.Body.String())
			fields := []string{"expires_at", "model_limits", "model_limits_enabled", "name", "object", "total_available", "total_granted", "total_used", "unlimited_quota"}
			if tc.wantUserQuota {
				fields = append(fields, "user_quota")
			}
			assert.Equal(t, fields, slices.Sorted(maps.Keys(response.Data)))
			assert.JSONEq(t, `100`, string(response.Data["total_available"]))
			assert.JSONEq(t, `30`, string(response.Data["total_used"]))
			if tc.wantUserQuota {
				assert.JSONEq(t, `{"total_available":12345,"total_used":678}`, string(response.Data["user_quota"]))
			}
		})
	}
}

func configureChatPresetKeyTest(t *testing.T, chats string, presetSettings string) {
	t.Helper()
	originalChats := setting.Chats2JsonString()
	originalSettings := setting.ChatPresetSettings2JsonString()
	require.NoError(t, setting.UpdateChatsByJsonString(chats))
	require.NoError(t, setting.UpdateChatPresetSettingsByJsonString(presetSettings))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateChatsByJsonString(originalChats))
		require.NoError(t, setting.UpdateChatPresetSettingsByJsonString(originalSettings))
	})
}

// requestChatPresetKey calls the endpoint as a browser session, or as a scoped
// access token when scopes are given.
func requestChatPresetKey(t *testing.T, preset string, userID int, scopes ...string) (tokenAPIResponse, string) {
	t.Helper()
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/chat-preset-key", map[string]any{"preset": preset}, userID)
	if scopes != nil {
		ctx.Set("access_token_id", 7)
		ctx.Set("access_token_scopes", scopes)
	}
	GetChatPresetKey(ctx)
	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		return response, ""
	}
	var keyData tokenKeyResponse
	require.NoError(t, common.Unmarshal(response.Data, &keyData))
	return response, keyData.Key
}

func TestGetChatPresetKeyCreatesTheTemplateKeyOnceAndReusesIt(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	longName := strings.Repeat("聊", 20)
	configureChatPresetKeyTest(t,
		`[{"Lobe":"https://lobe.example/?key={key}"},{"`+longName+`":"https://long.example/?key={key}"}]`,
		`{"Lobe":{"auto_key":{"enabled":true,"remain_quota":500,"unlimited_quota":false,"model_limits":"gpt-4o,gpt-4o-mini","allow_ips":"10.0.0.0/8","allow_user_quota_query":false,"group":"default","auto_groups":["vip"],"cross_group_retry":true}},
		 "`+longName+`":{"auto_key":{"enabled":true,"unlimited_quota":true,"group":"default"}}}`)
	seedToken(t, db, 2, "app-Lobe", "other-user-chat-key")

	response, firstKey := requestChatPresetKey(t, "Lobe", 1)
	require.True(t, response.Success, response.Message)
	_, secondKey := requestChatPresetKey(t, "Lobe", 1)
	assert.Equal(t, firstKey, secondKey)

	var created []model.Token
	require.NoError(t, db.Where("user_id = ? AND name = ?", 1, "app-Lobe").Find(&created).Error)
	require.Len(t, created, 1)
	assert.Equal(t, firstKey, created[0].Key)
	assert.Equal(t, 500, created[0].RemainQuota)
	assert.False(t, created[0].UnlimitedQuota)
	assert.True(t, created[0].ModelLimitsEnabled)
	assert.Equal(t, "gpt-4o,gpt-4o-mini", created[0].ModelLimits)
	require.NotNil(t, created[0].AllowIps)
	assert.Equal(t, "10.0.0.0/8", *created[0].AllowIps)
	assert.False(t, created[0].AllowUserQuotaQuery)
	assert.Equal(t, "default", created[0].Group)
	assert.False(t, created[0].CrossGroupRetry, "non-auto groups drop Auto-only options")
	assert.Equal(t, int64(-1), created[0].ExpiredTime)

	response, _ = requestChatPresetKey(t, longName, 1)
	require.True(t, response.Success, response.Message)
	var longToken model.Token
	require.NoError(t, db.Where("user_id = ? AND name LIKE ?", 1, "app-%").Where("name <> ?", "app-Lobe").First(&longToken).Error)
	assert.LessOrEqual(t, len(longToken.Name), 50)
	assert.True(t, strings.HasPrefix("app-"+longName, longToken.Name), "the default name is app-<preset> cut at a character boundary")
}

func TestGetChatPresetKeyRejectsPresetsWithoutAnUsableTemplateKey(t *testing.T) {
	for _, tc := range []struct {
		name     string
		preset   string
		settings string
		seed     func(t *testing.T, db *gorm.DB)
	}{
		{name: "preset without automatic keys", preset: "Plain", settings: `{"Plain":{"standalone_menu":true}}`},
		{name: "automatic keys turned off", preset: "Lobe", settings: `{"Lobe":{"auto_key":{"enabled":false,"unlimited_quota":true}}}`},
		{name: "settings left for a removed preset", preset: "Removed", settings: `{"Removed":{"auto_key":{"enabled":true,"unlimited_quota":true}}}`},
		{
			name:     "existing key with the template name is disabled",
			preset:   "Lobe",
			settings: `{"Lobe":{"auto_key":{"enabled":true,"name":"lobe-key","unlimited_quota":true}}}`,
			seed: func(t *testing.T, db *gorm.DB) {
				token := seedToken(t, db, 1, "lobe-key", "disabled-chat-key")
				require.NoError(t, db.Model(token).Update("status", common.TokenStatusDisabled).Error)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTokenControllerTestDB(t)
			configureChatPresetKeyTest(t, `[{"Lobe":"https://lobe.example/?key={key}"},{"Plain":"https://plain.example"}]`, tc.settings)
			if tc.seed != nil {
				tc.seed(t, db)
			}
			var before int64
			require.NoError(t, db.Model(&model.Token{}).Count(&before).Error)

			response, key := requestChatPresetKey(t, tc.preset, 1)

			assert.False(t, response.Success)
			assert.Empty(t, key)
			assert.NotContains(t, response.Message, "disabled-chat-key")
			var after int64
			require.NoError(t, db.Model(&model.Token{}).Count(&after).Error)
			assert.Equal(t, before, after)
		})
	}
}

func TestGetChatPresetKeyNeedsWriteScopeOnlyToCreateTheKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	configureChatPresetKeyTest(t, `[{"Lobe":"https://lobe.example/?key={key}"}]`,
		`{"Lobe":{"auto_key":{"enabled":true,"unlimited_quota":true,"group":"default"}}}`)

	response, key := requestChatPresetKey(t, "Lobe", 1, "api_key:reveal")
	assert.False(t, response.Success)
	assert.Empty(t, key)
	var count int64
	require.NoError(t, db.Model(&model.Token{}).Count(&count).Error)
	assert.Zero(t, count)

	response, created := requestChatPresetKey(t, "Lobe", 1, "api_key:reveal", "api_key:write")
	require.True(t, response.Success, response.Message)
	_, reused := requestChatPresetKey(t, "Lobe", 1, "api_key:reveal")
	assert.Equal(t, created, reused)
}

func TestChatPresetSettingsValidateTheTemplateKeyName(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings string
		wantErr  bool
	}{
		{name: "default name", settings: `{"Lobe":{"auto_key":{"enabled":true}}}`},
		{name: "custom name over 50 bytes", settings: `{"Lobe":{"auto_key":{"enabled":true,"name":"` + strings.Repeat("k", 51) + `"}}}`, wantErr: true},
		{name: "name with a LIKE wildcard", settings: `{"50% off":{"auto_key":{"enabled":true}}}`, wantErr: true},
		{name: "negative quota", settings: `{"Lobe":{"auto_key":{"enabled":true,"remain_quota":-1}}}`, wantErr: true},
		{name: "blank preset name", settings: `{" ":{"standalone_menu":true}}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := setting.ParseChatPresetSettings(tc.settings)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
