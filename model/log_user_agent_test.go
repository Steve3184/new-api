package model

import (
	"bytes"
	"encoding/csv"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// These tests require disposable databases; they create and drop the logs table.
func TestLogUserAgentDatabaseMatrix(t *testing.T) {
	primary, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, primary.AutoMigrate(&User{}))
	require.NoError(t, primary.Create(&User{Id: 11, Username: "ua-owner"}).Error)
	oldDB, oldRedis, oldConsume, oldExport := DB, common.RedisEnabled, common.LogConsumeEnabled, common.DataExportEnabled
	DB, common.RedisEnabled, common.LogConsumeEnabled, common.DataExportEnabled = primary, false, true, false
	t.Cleanup(func() {
		DB, common.RedisEnabled, common.LogConsumeEnabled, common.DataExportEnabled = oldDB, oldRedis, oldConsume, oldExport
		sqlDB, err := primary.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	for _, engine := range []string{"sqlite", "mysql", "postgres", "clickhouse"} {
		t.Run(engine, func(t *testing.T) {
			var driver gorm.Dialector
			switch engine {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				// The fixture recreates a released schema on the same connection.
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			case "clickhouse":
				dsn := os.Getenv("TEST_CLICKHOUSE_DSN")
				if dsn == "" {
					t.Skip("TEST_CLICKHOUSE_DSN not configured")
				}
				driver = clickhouse.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			oldLogDB, oldType := LOG_DB, common.LogDatabaseType()
			LOG_DB = db
			common.SetLogDatabaseType(common.DatabaseType(engine))
			initCol()
			t.Cleanup(func() {
				LOG_DB = oldLogDB
				common.SetLogDatabaseType(oldType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			var version string
			query := "SELECT version()"
			if engine == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("Database version: %s", version)
			for _, upgrade := range []bool{false, true} {
				t.Run(map[bool]string{false: "fresh", true: "upgrade"}[upgrade], func(t *testing.T) {
					require.NoError(t, db.Migrator().DropTable(&Log{}))
					if upgrade {
						// The released schema is identical except for the new column. Build
						// that schema and seed a historical row before the production migration.
						if engine == "clickhouse" {
							require.NoError(t, db.Exec(strings.Replace(clickHouseLogCreateTableSQL(0), "\n\tuser_agent String DEFAULT '',", "", 1)).Error)
						} else {
							require.NoError(t, db.AutoMigrate(&logBeforeUserAgent{}))
						}
						require.NoError(t, db.Table("logs").Create(map[string]any{"user_id": 11, "request_id": "historical", "created_at": time.Now().Unix(), "type": LogTypeConsume, "quota": 17, "content": "retained"}).Error)
					}
					for range 2 {
						if engine == "clickhouse" {
							require.NoError(t, migrateClickHouseLogDB())
						} else {
							require.NoError(t, db.AutoMigrate(&Log{}))
						}
					}
					if upgrade {
						var old Log
						require.NoError(t, db.Where("request_id = ?", "historical").Take(&old).Error)
						assert.Equal(t, "", old.UserAgent)
						assert.Equal(t, "retained", old.Content)
						assert.Equal(t, 17, old.Quota)
						if engine != "clickhouse" {
							assert.True(t, db.Migrator().HasIndex(&Log{}, "idx_logs_request_id"))
							assert.True(t, db.Migrator().HasIndex(&Log{}, "idx_user_id_id"))
						}
					}
					ua := "Claude_Code/2.0 50%! O'Reilly"
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Request = httptest.NewRequest("POST", "/v1/messages", nil)
					ctx.Request.Header.Set("User-Agent", ua)
					ctx.Set(common.RequestIdKey, "consume-ua")
					ctx.Set("username", "ua-owner")
					RecordConsumeLog(ctx, 11, RecordConsumeLogParams{Quota: 3, PromptTokens: 4, CompletionTokens: 5, Other: NewLogOther()})
					ctx.Set(common.RequestIdKey, "error-ua")
					RecordErrorLog(ctx, 11, 0, "", "", "failed", 0, 0, false, "", NewLogOther())
					snapshotJSON, err := common.Marshal(TaskPrivateData{Execution: &TaskExecutionSnapshot{UserAgent: ua}})
					require.NoError(t, err)
					var restored TaskPrivateData
					require.NoError(t, common.Unmarshal(snapshotJSON, &restored))
					RecordTaskBillingLog(RecordTaskBillingLogParams{UserId: 11, LogType: LogTypeRefund, Execution: restored.Execution, Other: NewLogOther()})
					require.NoError(t, createLog(&Log{UserId: 22, Type: LogTypeConsume, Quota: 100, UserAgent: ua}))
					require.NoError(t, createLog(&Log{UserId: 11, Type: LogTypeConsume, UserAgent: "ClaudeXCode/2.0 500x", RequestId: "decoy"}))
					logs, total, err := GetUserLogs(11, 0, 0, 0, "", "", 0, 10, "", "", "", "cLaUdE_CoDe")
					require.NoError(t, err)
					require.EqualValues(t, 3, total)
					require.Len(t, logs, 3)
					for _, entry := range logs {
						assert.Equal(t, ua, entry.UserAgent)
						assert.Equal(t, 11, entry.UserId)
					}
					for _, filter := range []string{"50%!", "O'Reilly"} {
						_, total, err = GetAllLogs(0, 0, 0, "", "", "", 0, 10, 0, "", "", "", filter)
						require.NoError(t, err)
						assert.EqualValues(t, 4, total)
					}
					stat, err := SumUsedQuota(11, 0, 0, 0, "", "", "", 0, "", "", "", "claude_code")
					require.NoError(t, err)
					assert.Equal(t, 3, stat.Quota)
					assert.EqualValues(t, 9, stat.TotalTokens)
					assert.Equal(t, 1, stat.Rpm)
					var output bytes.Buffer
					require.NoError(t, WriteLogsCSV(&output, 11, 0, 0, 0, "", "", "", 0, "", "", "", common.RoleCommonUser, true, "claude_code"))
					records, err := csv.NewReader(&output).ReadAll()
					require.NoError(t, err)
					require.Len(t, records, 4)
					column := slices.Index(records[0], "user_agent")
					require.GreaterOrEqual(t, column, 0)
					for _, record := range records[1:] {
						assert.Equal(t, ua, record[column])
					}
					for _, tc := range []struct{ name, input, want string }{
						{"absent", "", ""}, {"ascii", strings.Repeat("a", 1025), strings.Repeat("a", 1024)}, {"unicode", strings.Repeat("界", 1025), strings.Repeat("界", 1024)}, {"invalid", "A\xff\x00B", "A\uFFFd\uFFFdB"},
					} {
						ctx.Request.Header.Set("User-Agent", tc.input)
						ctx.Set(common.RequestIdKey, tc.name)
						RecordConsumeLog(ctx, 11, RecordConsumeLogParams{Other: NewLogOther()})
						var stored Log
						require.NoError(t, db.Where("request_id = ?", tc.name).Take(&stored).Error)
						assert.Equal(t, tc.want, stored.UserAgent)
						assert.True(t, utf8.ValidString(stored.UserAgent))
						assert.LessOrEqual(t, utf8.RuneCountInString(stored.UserAgent), 1024)
					}
					require.NoError(t, createLog(&Log{UserId: 11, UserAgent: "=1+1", RequestId: "formula"}))
					output.Reset()
					require.NoError(t, WriteLogsCSV(&output, 11, 0, 0, 0, "", "", "", 0, "", "formula", "", common.RoleCommonUser, true, ""))
					records, err = csv.NewReader(&output).ReadAll()
					require.NoError(t, err)
					require.Len(t, records, 2)
					assert.Equal(t, "'=1+1", records[1][column])
					require.NoError(t, db.Migrator().DropTable(&Log{}))
				})
			}
		})
	}
}

// Schema fixture from release b2ae0a33a.
type logBeforeUserAgent struct {
	Id                int    `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId            int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel" gorm:"index"`
	ChannelName       string `json:"channel_name" gorm:"->"`
	TokenId           int    `json:"token_id" gorm:"default:0;index"`
	Group             string `json:"group" gorm:"index"`
	Ip                string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other             string `json:"other"`
	ModelIcon         string `json:"model_icon,omitempty" gorm:"-"`
	ProviderIcon      string `json:"provider_icon,omitempty" gorm:"-"`
}

func (logBeforeUserAgent) TableName() string { return "logs" }
