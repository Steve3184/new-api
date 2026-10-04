package model

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestLogTokenTotalsDatabaseMatrix(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres", "clickhouse"} {
		t.Run(engine, func(t *testing.T) {
			var dialector gorm.Dialector
			switch engine {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				dialector = postgres.Open(dsn)
			case "clickhouse":
				dsn := os.Getenv("TEST_CLICKHOUSE_DSN")
				if dsn == "" {
					t.Skip("TEST_CLICKHOUSE_DSN not configured")
				}
				dialector = clickhouse.Open(dsn)
			}
			db, err := gorm.Open(dialector, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			oldDB, oldType := LOG_DB, common.LogDatabaseType()
			LOG_DB = db
			common.SetLogDatabaseType(common.DatabaseType(engine))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Log{}))
				LOG_DB = oldDB
				common.SetLogDatabaseType(oldType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			if engine == "clickhouse" {
				require.NoError(t, db.Exec(clickHouseLogCreateTableSQL(0)).Error)
			} else {
				require.NoError(t, db.AutoMigrate(&Log{}))
			}
			var version string
			query := "SELECT version()"
			if engine == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("Database version: %s", version)
			now := time.Now().Unix()
			entries := []Log{
				{UserId: 11, Username: "alice", Type: LogTypeConsume, CreatedAt: now - 1000, ModelName: "model-a", TokenName: "key-a", ChannelId: 2, Group: "vip", RequestId: "req-a", UpstreamRequestId: "up-a", Quota: 7, PromptTokens: 100, CompletionTokens: 20, Other: `{"cache_tokens":30,"cache_creation_tokens":40}`},
				{UserId: 11, Username: "old-name", Type: LogTypeConsume, CreatedAt: now - 900, ModelName: "model-a", TokenName: "key-a", ChannelId: 2, Group: "vip", RequestId: "req-b", Quota: 8, PromptTokens: 50, CompletionTokens: 10, Other: `{"cache_tokens":5,"cache_creation_tokens":999,"cache_creation_tokens_5m":7,"cache_creation_tokens_1h":8}`},
				{UserId: 22, Username: "bob", Type: LogTypeConsume, CreatedAt: now - 800, ModelName: "model-b", TokenName: "key-b", ChannelId: 3, Group: "default", Quota: 1, PromptTokens: 2, CompletionTokens: 3, Other: `malformed historical data`},
				{UserId: 11, Username: "alice", Type: LogTypeError, CreatedAt: now - 800, PromptTokens: 999, CompletionTokens: 999, Other: `{"cache_tokens":999}`},
				{UserId: 11, Username: "alice", Type: LogTypeConsume, CreatedAt: now - 10, Quota: 2, PromptTokens: 3, CompletionTokens: 4, Other: `null`},
			}
			// Use the production log insert path, including ClickHouse's schema.
			for i := range entries {
				require.NoError(t, createLog(&entries[i]))
			}
			cases := []struct {
				name                     string
				user, kind               int
				start, end               int64
				model, userName, token   string
				channel                  int
				group, request, upstream string
				total                    int64
				quota                    int
			}{
				{name: "entire selected range", start: now - 1000, end: now - 800, total: 275, quota: 16},
				{name: "self includes old username only for owner", user: 11, start: now - 1000, end: now - 800, total: 270, quota: 15},
				{name: "all filters", start: now - 1000, end: now - 800, model: "model-a", userName: "alice", token: "key-a", channel: 2, group: "vip", request: "req-a", upstream: "up-a", total: 190, quota: 7},
				{name: "exact model", model: "model-b", total: 5, quota: 1},
				{name: "wildcard model", model: "model-%", total: 275, quota: 16},
				{name: "token filter", token: "key-b", total: 5, quota: 1},
				{name: "channel filter", channel: 3, total: 5, quota: 1},
				{name: "group filter", group: "vip", total: 270, quota: 15},
				{name: "request filter", request: "req-b", total: 80, quota: 8},
				{name: "upstream request filter", upstream: "up-a", total: 190, quota: 7},
				{name: "non consumption filter", kind: LogTypeError, total: 0, quota: 0},
				{name: "empty range", start: now + 100, total: 0, quota: 0},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					stat, err := SumUsedQuota(tc.user, tc.kind, tc.start, tc.end, tc.model, tc.userName, tc.token, tc.channel, tc.group, tc.request, tc.upstream)
					require.NoError(t, err)
					assert.Equal(t, tc.total, stat.TotalTokens)
					assert.Equal(t, tc.quota, stat.Quota)
				})
			}
			// Recent rates remain independent of the historical time range.
			stat, err := SumUsedQuota(0, 0, now-1000, now-800, "", "", "", 0, "", "", "")
			require.NoError(t, err)
			assert.Equal(t, 1, stat.Rpm)
			assert.Equal(t, 7, stat.Tpm)
			large := Log{Type: LogTypeConsume, RequestId: "large", PromptTokens: math.MaxInt32, CompletionTokens: 1, Other: `{"cache_tokens":2,"cache_creation_tokens_1h":3}`}
			require.NoError(t, createLog(&large))
			stat, err = SumUsedQuota(0, 0, 0, 0, "", "", "", 0, "", "large", "")
			require.NoError(t, err)
			assert.Equal(t, int64(math.MaxInt32)+6, stat.TotalTokens)
		})
	}
}
