package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMultiKeyDatabase(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMaster, previousCache, previousRedis, previousSQLite := common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousType, previousLogType)
		common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath = previousMaster, previousCache, previousRedis, previousSQLite
	})
	t.Setenv("SQL_DSN", os.Getenv("TEST_CHANNEL_SQL_DSN"))
	t.Setenv("LOG_SQL_DSN", "")
	common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled = false, false, false
	common.SQLitePath = filepath.Join(t.TempDir(), "channel.db")
	require.NoError(t, model.InitDB())
	database := model.DB
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	model.LOG_DB = database
	common.SetLogDatabaseType(common.MainDatabaseType())
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}, &model.AuditLog{}))
	root := &model.User{Username: "multi-key-review-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	require.NoError(t, database.Create(root).Error)
	t.Cleanup(func() { require.NoError(t, database.Unscoped().Delete(root).Error) })
	versionQuery := "SELECT VERSION()"
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		versionQuery = "SELECT sqlite_version()"
	}
	var version string
	require.NoError(t, database.Raw(versionQuery).Scan(&version).Error)
	t.Logf("database=%s version=%s", common.MainDatabaseType(), version)
	return database, root
}

func TestMultiKeyEnableRestoresOnlyExhaustedChannels(t *testing.T) {
	database, root := setupMultiKeyDatabase(t)
	for _, cacheEnabled := range []bool{false, true} {
		for _, action := range []string{"enable_key", "enable_all_keys"} {
			for _, tc := range []struct {
				name           string
				initialStatus  int
				manualOverride string
				wantStatus     int
			}{
				{name: "key exhaustion restores", initialStatus: common.ChannelStatusEnabled, wantStatus: common.ChannelStatusEnabled},
				{name: "manual disable is preserved", initialStatus: common.ChannelStatusManuallyDisabled, wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "manual disable after exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, manualOverride: "status", wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "tag disable after exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, manualOverride: "tag", wantStatus: common.ChannelStatusManuallyDisabled},
			} {
				t.Run(fmt.Sprintf("cache=%t/%s/%s", cacheEnabled, action, tc.name), func(t *testing.T) {
					common.MemoryCacheEnabled = cacheEnabled
					tag := t.Name()
					channel := &model.Channel{Name: t.Name(), Type: 1, Key: "key-one\nkey-two", Status: tc.initialStatus, Models: "test-model", Group: "default", Tag: &tag,
						ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled}},
					}
					require.NoError(t, channel.Insert())
					t.Cleanup(func() {
						require.NoError(t, channel.Delete())
						model.InitChannelCache()
					})
					for _, operation := range []string{"disable_key", action} {
						if operation == action {
							if tc.manualOverride == "status" {
								model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual operation")
							} else if tc.manualOverride == "tag" {
								require.NoError(t, model.DisableChannelByTag(tag))
							}
						}
						payload, err := common.Marshal(MultiKeyManageRequest{ChannelId: channel.Id, Action: operation, KeyIndex: common.GetPointer(0)})
						require.NoError(t, err)
						recorder := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(recorder)
						c.Set("id", root.Id)
						c.Set("role", common.RoleRootUser)
						c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/multi_key", bytes.NewReader(payload))
						c.Request.Header.Set("Content-Type", "application/json")
						ManageMultiKeys(c)
						var result struct {
							Success bool `json:"success"`
						}
						require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
						require.True(t, result.Success, recorder.Body.String())
					}
					loaded, err := model.GetChannelById(channel.Id, true)
					require.NoError(t, err)
					assert.Equal(t, tc.wantStatus, loaded.Status)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyStatusList, 0)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledReason, 0)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledTime, 0)
					var ability model.Ability
					require.NoError(t, database.Where("channel_id = ?", channel.Id).First(&ability).Error)
					assert.Equal(t, tc.wantStatus == common.ChannelStatusEnabled, ability.Enabled)
				})
			}
		}
	}
}

func TestMultiKeyInventoryLifecycle(t *testing.T) {
	database, root := setupMultiKeyDatabase(t)
	for _, cacheEnabled := range []bool{false, true} {
		for _, mode := range []constant.MultiKeyMode{constant.MultiKeyModeRandom, constant.MultiKeyModePolling} {
			t.Run(fmt.Sprintf("cache=%t/%s", cacheEnabled, mode), func(t *testing.T) {
				common.MemoryCacheEnabled = cacheEnabled
				channel := &model.Channel{Name: t.Name(), Type: 1, Key: "active\nreserve\nspare\nmanual", Status: common.ChannelStatusEnabled, Models: "inventory-test", Group: "default", ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 4, MultiKeyMode: mode, MultiKeyInventoryEnabled: true, MultiKeyInventoryTarget: 1, MultiKeyStatusList: map[int]int{3: common.ChannelStatusManuallyDisabled}}}
				require.NoError(t, model.BatchInsertChannels([]model.Channel{*channel}))
				require.NoError(t, database.Where("name = ?", t.Name()).First(channel).Error)
				t.Cleanup(func() { require.NoError(t, channel.Delete()); model.InitChannelCache() })
				model.InitChannelCache()
				assert.Equal(t, map[int]int{1: model.MultiKeyStatusStandby, 2: model.MultiKeyStatusStandby, 3: common.ChannelStatusManuallyDisabled}, channel.ChannelInfo.MultiKeyStatusList)
				selected, err := model.CacheGetChannel(channel.Id)
				require.NoError(t, err)
				key, index, selectionErr := selected.GetNextEnabledKey()
				require.Nil(t, selectionErr)
				assert.Equal(t, "active", key)
				assert.Equal(t, 0, index)
				require.True(t, model.UpdateChannelStatus(channel.Id, key, common.ChannelStatusAutoDisabled, "inventory test failure"))
				assert.True(t, selected.HasEnabledMultiKey())
				key, index, selectionErr = selected.GetNextEnabledKey()
				require.Nil(t, selectionErr)
				assert.Equal(t, "reserve", key)
				assert.Equal(t, 1, index)
				loaded, err := model.GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, common.ChannelStatusEnabled, loaded.Status)
				assert.Equal(t, map[int]int{0: common.ChannelStatusAutoDisabled, 2: model.MultiKeyStatusStandby, 3: common.ChannelStatusManuallyDisabled}, loaded.ChannelInfo.MultiKeyStatusList)
				require.True(t, model.UpdateChannelStatus(channel.Id, "active", common.ChannelStatusEnabled, "recovered"))
				loaded, err = model.GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, model.MultiKeyStatusStandby, loaded.ChannelInfo.MultiKeyStatusList[0])
				assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledReason, 0)
				assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledTime, 0)
				for _, action := range []string{"get_key_status", "disable_all_keys", "enable_all_keys"} {
					payload, err := common.Marshal(MultiKeyManageRequest{ChannelId: channel.Id, Action: action})
					require.NoError(t, err)
					recorder := httptest.NewRecorder()
					context, _ := gin.CreateTestContext(recorder)
					context.Set("id", root.Id)
					context.Set("role", common.RoleRootUser)
					context.Request = httptest.NewRequest(http.MethodPost, "/api/channel/multi_key", bytes.NewReader(payload))
					context.Request.Header.Set("Content-Type", "application/json")
					ManageMultiKeys(context)
					var result struct {
						Success bool                   `json:"success"`
						Data    MultiKeyStatusResponse `json:"data"`
					}
					require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
					require.True(t, result.Success, recorder.Body.String())
					if action == "get_key_status" {
						assert.Equal(t, 1, result.Data.EnabledCount)
						assert.Equal(t, 2, result.Data.StandbyCount)
						assert.Equal(t, 1, result.Data.ManualDisabledCount)
					}
					loaded, err = model.GetChannelById(channel.Id, true)
					require.NoError(t, err)
					if action == "disable_all_keys" {
						assert.False(t, loaded.HasEnabledMultiKey())
					} else {
						assert.Equal(t, common.ChannelStatusEnabled, loaded.Status)
					}
				}
				for _, key := range []string{"active", "reserve", "spare", "manual"} {
					require.True(t, model.UpdateChannelStatus(channel.Id, key, common.ChannelStatusAutoDisabled, "exhausted"))
				}
				loaded, err = model.GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, common.ChannelStatusAutoDisabled, loaded.Status)
				require.True(t, model.UpdateChannelStatus(channel.Id, "spare", common.ChannelStatusEnabled, "recovered"))
				var ability model.Ability
				require.NoError(t, database.Where("channel_id = ?", channel.Id).First(&ability).Error)
				assert.True(t, ability.Enabled)
				loaded, err = model.GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, common.ChannelStatusEnabled, loaded.Status)
			})
		}
	}
}

func TestMultiKeyInventorySettingsAndKeyEdits(t *testing.T) {
	database, root := setupMultiKeyDatabase(t)
	for _, cacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%t", cacheEnabled), func(t *testing.T) {
			common.MemoryCacheEnabled = cacheEnabled
			channel := &model.Channel{Name: t.Name(), Type: 1, Key: "first\nsecond\nthird\nmanual", Status: common.ChannelStatusEnabled, Models: "inventory-test", Group: "default", ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 4, MultiKeyStatusList: map[int]int{3: common.ChannelStatusManuallyDisabled}}}
			require.NoError(t, channel.Insert())
			t.Cleanup(func() { require.NoError(t, channel.Delete()); model.InitChannelCache() })
			model.InitChannelCache()
			for _, testCase := range []struct {
				name          string
				fields        map[string]any
				wantSuccess   bool
				wantInventory bool
				wantTarget    int
				wantStatuses  map[int]int
			}{
				{name: "reject invalid target", fields: map[string]any{"channel_info": map[string]any{"multi_key_inventory_enabled": true, "multi_key_inventory_target": 0}}, wantSuccess: false, wantStatuses: map[int]int{3: common.ChannelStatusManuallyDisabled}},
				{name: "enable inventory", fields: map[string]any{"channel_info": map[string]any{"multi_key_inventory_enabled": true, "multi_key_inventory_target": 1}}, wantSuccess: true, wantInventory: true, wantTarget: 1, wantStatuses: map[int]int{1: model.MultiKeyStatusStandby, 2: model.MultiKeyStatusStandby, 3: common.ChannelStatusManuallyDisabled}},
				{name: "legacy save preserves inventory", fields: map[string]any{"channel_info": map[string]any{"multi_key_auto_retry": true, "multi_key_status_list": map[int]int{}}}, wantSuccess: true, wantInventory: true, wantTarget: 1, wantStatuses: map[int]int{1: model.MultiKeyStatusStandby, 2: model.MultiKeyStatusStandby, 3: common.ChannelStatusManuallyDisabled}},
				{name: "append stays in inventory", fields: map[string]any{"key": "fourth", "key_mode": "append"}, wantSuccess: true, wantInventory: true, wantTarget: 1, wantStatuses: map[int]int{1: model.MultiKeyStatusStandby, 2: model.MultiKeyStatusStandby, 3: common.ChannelStatusManuallyDisabled, 4: model.MultiKeyStatusStandby}},
				{name: "reordering preserves key identity", fields: map[string]any{"key": "fourth\nmanual\nfirst\nsecond\nthird", "key_mode": "replace"}, wantSuccess: true, wantInventory: true, wantTarget: 1, wantStatuses: map[int]int{0: model.MultiKeyStatusStandby, 1: common.ChannelStatusManuallyDisabled, 3: model.MultiKeyStatusStandby, 4: model.MultiKeyStatusStandby}},
				{name: "increase target promotes reserve", fields: map[string]any{"channel_info": map[string]any{"multi_key_inventory_target": 2}}, wantSuccess: true, wantInventory: true, wantTarget: 2, wantStatuses: map[int]int{1: common.ChannelStatusManuallyDisabled, 3: model.MultiKeyStatusStandby, 4: model.MultiKeyStatusStandby}},
				{name: "turn off only enables standby", fields: map[string]any{"channel_info": map[string]any{"multi_key_inventory_enabled": false}}, wantSuccess: true, wantTarget: 2, wantStatuses: map[int]int{1: common.ChannelStatusManuallyDisabled}},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					testCase.fields["id"] = channel.Id
					testCase.fields["type"] = 1
					testCase.fields["name"] = channel.Name
					testCase.fields["models"] = channel.Models
					testCase.fields["group"] = channel.Group
					payload, err := common.Marshal(testCase.fields)
					require.NoError(t, err)
					recorder := httptest.NewRecorder()
					context, _ := gin.CreateTestContext(recorder)
					context.Set("id", root.Id)
					context.Set("role", common.RoleRootUser)
					context.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(payload))
					context.Request.Header.Set("Content-Type", "application/json")
					UpdateChannel(context)
					var response struct {
						Success bool `json:"success"`
					}
					require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
					require.Equal(t, testCase.wantSuccess, response.Success, recorder.Body.String())
					loaded, err := model.GetChannelById(channel.Id, true)
					require.NoError(t, err)
					assert.Equal(t, testCase.wantInventory, loaded.ChannelInfo.MultiKeyInventoryEnabled)
					assert.Equal(t, testCase.wantTarget, loaded.ChannelInfo.MultiKeyInventoryTarget)
					assert.Equal(t, testCase.wantStatuses, loaded.ChannelInfo.MultiKeyStatusList)
					if testCase.wantSuccess {
						cached, err := model.CacheGetChannel(channel.Id)
						require.NoError(t, err)
						assert.Equal(t, loaded.ChannelInfo, cached.ChannelInfo)
						var ability model.Ability
						require.NoError(t, database.Where("channel_id = ?", channel.Id).First(&ability).Error)
						assert.True(t, ability.Enabled)
					}
				})
			}
		})
	}
}
