package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The released schema differs only in the group-list column.
type channelWithLimitedGroups struct {
	Channel
	Group string `gorm:"type:varchar(64);default:'default'"`
}

func TestChannelGroupMigration(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var dsn string
			switch engine {
			case "sqlite":
				dsn = "local"
				previousPath := common.SQLitePath
				common.SQLitePath = filepath.Join(t.TempDir(), "channels.db")
				t.Cleanup(func() { common.SQLitePath = previousPath })
			case "mysql":
				dsn = os.Getenv("TEST_MYSQL_DSN")
			case "postgres":
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			if dsn == "" {
				t.Skip("test database DSN is not configured")
			}
			t.Setenv("CHANNEL_GROUP_TEST_DSN", dsn)
			db, dbType, err := chooseDB("CHANNEL_GROUP_TEST_DSN", false)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			oldDB, oldType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(dbType)
			initCol()
			t.Cleanup(func() { DB = oldDB; common.SetMainDatabaseType(oldType); initCol() })
			var version string
			versionQuery := "SELECT version()"
			if engine == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("Database version: %s", version)

			for _, upgrade := range []bool{false, true} {
				t.Run(fmt.Sprintf("upgrade=%t", upgrade), func(t *testing.T) {
					t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&Ability{}, &Channel{})) })
					require.NoError(t, db.AutoMigrate(&Ability{}))
					if upgrade {
						require.NoError(t, db.Table("channels").AutoMigrate(&channelWithLimitedGroups{}))
						legacy := channelWithLimitedGroups{Channel: Channel{Id: 101, Name: "existing", Key: "fixture", Models: "model-a"}, Group: "default,vip"}
						require.NoError(t, db.Table("channels").Create(&legacy).Error)
						legacy.Channel.Group = legacy.Group
						require.NoError(t, legacy.Channel.AddAbilities(db))
					}
					require.NoError(t, db.AutoMigrate(&Channel{}))
					if upgrade {
						var existing Channel
						require.NoError(t, db.First(&existing, 101).Error)
						assert.Equal(t, "default,vip", existing.Group)
						assert.Equal(t, "existing", existing.Name)
						assert.Equal(t, "fixture", existing.Key)
						assert.Equal(t, "model-a", existing.Models)
						assert.Equal(t, []string{"model-a"}, GetGroupEnabledModels("vip"))
					}
					for _, field := range []string{"Name", "Tag"} {
						assert.True(t, db.Migrator().HasIndex(&Channel{}, field))
					}
					groups := []string{"default", "premium_international", "premium_enterprise", "premium_development", "中文分组"}
					channel := Channel{Name: "many groups", Key: "fixture", Group: strings.Join(groups, ","), Models: "model-b"}
					require.Greater(t, len(channel.Group), 64)
					require.NoError(t, channel.Insert())
					var saved Channel
					require.NoError(t, db.First(&saved, channel.Id).Error)
					assert.Equal(t, groups, saved.GetGroups())
					for _, group := range groups {
						assert.Contains(t, GetGroupEnabledModels(group), "model-b")
					}
					channel.Group += ",another_group"
					require.NoError(t, channel.Update())
					require.NoError(t, db.First(&saved, channel.Id).Error)
					assert.Equal(t, channel.Group, saved.Group)
					assert.Equal(t, []string{"model-b"}, GetGroupEnabledModels("another_group"))
					var ability Ability
					require.NoError(t, db.Where("channel_id = ?", channel.Id).First(&ability).Error)
					assert.Error(t, db.Create(&ability).Error, "ability composite primary key must remain unique")
					assert.Error(t, db.Create(&Channel{Id: channel.Id, Key: "duplicate"}).Error)

					defaultSingle := Channel{Name: "single default", Key: "fixture", Models: "model-single"}
					require.NoError(t, defaultSingle.Insert())
					assert.Equal(t, "default", defaultSingle.Group)
					assert.Contains(t, GetGroupEnabledModels("default"), "model-single")
					defaults := []Channel{
						{Name: "default group", Key: "fixture", Models: "model-default"},
						{Name: "batch groups", Key: "fixture", Models: "model-batch", Group: channel.Group},
					}
					require.NoError(t, BatchInsertChannels(defaults))
					var defaultChannel Channel
					require.NoError(t, db.Where("name = ?", "default group").First(&defaultChannel).Error)
					assert.Equal(t, "default", defaultChannel.Group)
					assert.Contains(t, GetGroupEnabledModels("default"), "model-default")
					assert.Contains(t, GetGroupEnabledModels("another_group"), "model-batch")

					recorder := &migrationSQLRecorder{}
					require.NoError(t, db.Session(&gorm.Session{Logger: recorder}).AutoMigrate(&Channel{}, &Ability{}))
					assert.Empty(t, recorder.schemaMutations(), "second startup must not change schema")
					require.NoError(t, db.First(&saved, channel.Id).Error)
					assert.Equal(t, channel.Group, saved.Group)
				})
			}
		})
	}
}
