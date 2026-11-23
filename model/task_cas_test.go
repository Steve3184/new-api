package model

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	DB = db
	LOG_DB = db

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	initCol()

	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&Task{},
		&User{},
		&UserSession{},
		&AuthFlow{},
		&ExternalIdentityClaim{},
		&Token{},
		&PasskeyCredential{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Log{},
		&Channel{},
		&QuotaData{},
		&Ability{},
		&TopUp{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&SubscriptionExtraCreditPurchase{},
		&UserOAuthBinding{},
		&PerfMetric{},
		&StatusCheckProbeState{},
		&SystemInstance{},
		&SystemTask{},
		&SystemTaskLock{},
		&SupportConversation{},
		&SupportMessage{},
		&UserAccessToken{},
		&RegistrationEmail{},
		&OAuthBindReward{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

func truncateTables(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM tasks")
		DB.Exec("DELETE FROM auth_flows")
		DB.Exec("DELETE FROM external_identity_claims")
		DB.Exec("DELETE FROM user_sessions")
		DB.Exec("DELETE FROM passkey_credentials")
		DB.Exec("DELETE FROM two_fa_backup_codes")
		DB.Exec("DELETE FROM two_fas")
		DB.Exec("DELETE FROM tokens")
		DB.Exec("DELETE FROM user_access_tokens")
		DB.Exec("DELETE FROM user_oauth_bindings")
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM registration_emails")
		DB.Exec("DELETE FROM oauth_bind_rewards")
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM quota_data")
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM top_ups")
		DB.Exec("DELETE FROM subscription_orders")
		DB.Exec("DELETE FROM subscription_plans")
		invalidateSubscriptionBindingCache()
		DB.Exec("DELETE FROM user_subscriptions")
		DB.Exec("DELETE FROM subscription_pre_consume_records")
		DB.Exec("DELETE FROM subscription_extra_credit_purchases")
		DB.Exec("DELETE FROM perf_metrics")
		DB.Exec("DELETE FROM status_check_probe_states")
		DB.Exec("DELETE FROM system_instances")
		DB.Exec("DELETE FROM system_task_locks")
		DB.Exec("DELETE FROM system_tasks")
		DB.Exec("DELETE FROM support_messages")
		DB.Exec("DELETE FROM support_conversations")
	})
}

func insertTask(t *testing.T, task *Task) {
	t.Helper()
	task.CreatedAt = time.Now().Unix()
	task.UpdatedAt = time.Now().Unix()
	require.NoError(t, DB.Create(task).Error)
}

func TestUnfinishedTaskQueriesDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "task_polling_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB := DB
			DB = db
			t.Cleanup(func() {
				DB = previousDB
				assert.NoError(t, db.Migrator().DropTable(&Task{}))
				assert.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Task{}))
			var version string
			versionQuery := "SELECT version()"
			if dialect == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database version: %s", version)
			assert.False(t, HasUnfinishedSyncTasks())

			const cutoff int64 = 1_800_000_000
			for _, status := range []TaskStatus{TaskStatusNotStart, TaskStatusSubmitted, TaskStatusQueued, TaskStatusInProgress, TaskStatusSuccess, TaskStatusFailure} {
				for _, progress := range []string{"99%", "100%"} {
					t.Run(string(status)+"/"+progress, func(t *testing.T) {
						task := &Task{TaskID: "query_candidate", Status: status, Progress: progress, SubmitTime: cutoff - 1}
						insertTask(t, task)
						t.Cleanup(func() { assert.NoError(t, db.Delete(task).Error) })
						unfinished := status != TaskStatusSuccess && status != TaskStatusFailure
						assert.Equal(t, unfinished, HasUnfinishedSyncTasks())
						if unfinished {
							require.Len(t, GetAllUnFinishSyncTasks(1), 1)
							assert.Equal(t, task.ID, GetAllUnFinishSyncTasks(1)[0].ID)
							require.Len(t, GetTimedOutUnfinishedTasks(cutoff, 1), 1)
							assert.Equal(t, task.ID, GetTimedOutUnfinishedTasks(cutoff, 1)[0].ID)
						} else {
							assert.Empty(t, GetAllUnFinishSyncTasks(1))
							assert.Empty(t, GetTimedOutUnfinishedTasks(cutoff, 1))
						}
						require.NoError(t, db.Model(task).Update("submit_time", cutoff).Error)
						assert.Empty(t, GetTimedOutUnfinishedTasks(cutoff, 1), "the timeout cutoff is exclusive")
						assert.Equal(t, unfinished, HasUnfinishedSyncTasks())
					})
				}
			}
		})
	}
}

func TestGetTaskForProtocolObservationScopesOwnerAndPlatform(t *testing.T) {
	truncateTables(t)
	task := &Task{
		TaskID:   "task_protocol_scope",
		UserId:   7,
		Platform: "plugin-a",
		Status:   TaskStatusInProgress,
	}
	insertTask(t, task)

	got, exists, err := GetTaskForProtocolObservation(context.Background(), 7, "plugin-a", task.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, task.ID, got.ID)

	for _, query := range []struct {
		userID   int
		platform string
	}{
		{userID: 8, platform: "plugin-a"},
		{userID: 7, platform: "plugin-b"},
	} {
		got, exists, err = GetTaskForProtocolObservation(context.Background(), query.userID, constant.TaskPlatform(query.platform), task.TaskID)
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, got)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = GetTaskForProtocolObservation(cancelled, 7, "plugin-a", task.TaskID)
	require.ErrorIs(t, err, context.Canceled)
}

// ---------------------------------------------------------------------------
// Snapshot / Equal — pure logic tests (no DB)
// ---------------------------------------------------------------------------

func TestSnapshotEqual_Same(t *testing.T) {
	s := taskSnapshot{
		Status:     TaskStatusInProgress,
		Progress:   "50%",
		StartTime:  1000,
		FinishTime: 0,
		FailReason: "",
		ResultURL:  "",
		Data:       json.RawMessage(`{"key":"value"}`),
	}
	assert.True(t, s.Equal(s))
}

func TestSnapshotEqual_DifferentStatus(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage(`{}`)}
	b := taskSnapshot{Status: TaskStatusSuccess, Data: json.RawMessage(`{}`)}
	assert.False(t, a.Equal(b))
}

func TestSnapshotEqual_DifferentProgress(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Progress: "30%", Data: json.RawMessage(`{}`)}
	b := taskSnapshot{Status: TaskStatusInProgress, Progress: "60%", Data: json.RawMessage(`{}`)}
	assert.False(t, a.Equal(b))
}

func TestSnapshotEqual_DifferentData(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage(`{"a":1}`)}
	b := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage(`{"a":2}`)}
	assert.False(t, a.Equal(b))
}

func TestSnapshotEqual_NilVsEmpty(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Data: nil}
	b := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage{}}
	// bytes.Equal(nil, []byte{}) == true
	assert.True(t, a.Equal(b))
}

func TestSnapshotEqual_PluginStateAndPollFailures(t *testing.T) {
	base := taskSnapshot{
		Status:       TaskStatusInProgress,
		PluginState:  json.RawMessage(`{"req_key":"a"}`),
		PollFailures: 2,
	}
	assert.True(t, base.Equal(taskSnapshot{
		Status:       TaskStatusInProgress,
		PluginState:  json.RawMessage(`{"req_key":"a"}`),
		PollFailures: 2,
	}))
	assert.False(t, base.Equal(taskSnapshot{
		Status:       TaskStatusInProgress,
		PluginState:  json.RawMessage(`{"req_key":"b"}`),
		PollFailures: 2,
	}))
	assert.False(t, base.Equal(taskSnapshot{
		Status:       TaskStatusInProgress,
		PluginState:  json.RawMessage(`{"req_key":"a"}`),
		PollFailures: 3,
	}))
}

func TestSnapshot_Roundtrip(t *testing.T) {
	task := &Task{
		Status:     TaskStatusInProgress,
		Progress:   "42%",
		StartTime:  1234,
		FinishTime: 5678,
		FailReason: "timeout",
		PrivateData: TaskPrivateData{
			ResultURL:    "https://example.com/result.mp4",
			PluginState:  json.RawMessage(`{"req_key":"keep"}`),
			PollFailures: 3,
		},
		Data: json.RawMessage(`{"model":"test-model"}`),
	}
	snap := task.Snapshot()
	assert.Equal(t, task.Status, snap.Status)
	assert.Equal(t, task.Progress, snap.Progress)
	assert.Equal(t, task.StartTime, snap.StartTime)
	assert.Equal(t, task.FinishTime, snap.FinishTime)
	assert.Equal(t, task.FailReason, snap.FailReason)
	assert.Equal(t, task.PrivateData.ResultURL, snap.ResultURL)
	assert.JSONEq(t, string(task.Data), string(snap.Data))
	assert.Equal(t, task.PrivateData.PluginState, snap.PluginState)
	assert.Equal(t, task.PrivateData.PollFailures, snap.PollFailures)
}

// ---------------------------------------------------------------------------
// UpdateWithStatus CAS — DB integration tests
// ---------------------------------------------------------------------------

func TestUpdateWithStatus_Win(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID:   "task_cas_win",
		Status:   TaskStatusInProgress,
		Progress: "50%",
		Data:     json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = TaskStatusSuccess
	task.Progress = "100%"
	won, err := task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	assert.True(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusSuccess, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
}

func TestUpdateWithStatus_Lose(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID: "task_cas_lose",
		Status: TaskStatusFailure,
		Data:   json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = TaskStatusSuccess
	won, err := task.UpdateWithStatus(TaskStatusInProgress) // wrong fromStatus
	require.NoError(t, err)
	assert.False(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusFailure, reloaded.Status) // unchanged
}

func TestUpdateWithStatus_ConcurrentWinner(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID: "task_cas_race",
		Status: TaskStatusInProgress,
		Quota:  1000,
		Data:   json.RawMessage(`{}`),
	}
	insertTask(t, task)

	const goroutines = 5
	wins := make([]bool, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			t := &Task{}
			*t = Task{
				ID:       task.ID,
				TaskID:   task.TaskID,
				Status:   TaskStatusSuccess,
				Progress: "100%",
				Quota:    task.Quota,
				Data:     json.RawMessage(`{}`),
			}
			t.CreatedAt = task.CreatedAt
			t.UpdatedAt = time.Now().Unix()
			won, err := t.UpdateWithStatus(TaskStatusInProgress)
			if err == nil {
				wins[idx] = won
			}
		}(i)
	}
	wg.Wait()

	winCount := 0
	for _, w := range wins {
		if w {
			winCount++
		}
	}
	assert.Equal(t, 1, winCount, "exactly one goroutine should win the CAS")
}

// The submit alignment can commit while a poller still holds the row it read.
// The transition must keep the authoritative charge and receipt marker, and
// hand the winner those values so its settlement uses the aligned charge.
func TestUpdateWithStatus_KeepsNewerBillingFacts(t *testing.T) {
	truncateTables(t)

	target := 600
	task := &Task{
		TaskID:                     "task_cas_stale_billing",
		Status:                     TaskStatusInProgress,
		Progress:                   "30%",
		Quota:                      1000,
		SubscriptionBillingPending: true,
		Data:                       json.RawMessage(`{}`),
		PrivateData: TaskPrivateData{
			BillingSource:           "subscription",
			SubscriptionId:          9,
			SubscriptionRequestID:   "receipt-1",
			TokenId:                 5,
			SubscriptionSubmitQuota: &target,
		},
	}
	insertTask(t, task)

	aligned := task.PrivateData
	aligned.SubscriptionSubmitQuota = nil
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).
		Updates(map[string]any{"quota": 600, "private_data": aligned}).Error)

	task.Status = TaskStatusSuccess
	task.Progress = "100%"
	won, err := task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, 600, reloaded.Quota, "a stale snapshot must not restore the earlier charge")
	assert.Nil(t, reloaded.PrivateData.SubscriptionSubmitQuota, "a stale snapshot must not restore a cleared submission target")
	assert.True(t, reloaded.SubscriptionBillingPending)
	assert.EqualValues(t, TaskStatusSuccess, reloaded.Status)

	assert.Equal(t, 600, task.Quota, "the winner settles on the authoritative charge")
	assert.Nil(t, task.PrivateData.SubscriptionSubmitQuota)
	assert.Equal(t, "receipt-1", task.PrivateData.SubscriptionRequestID)
}

// Only the facts a poll owns may reach the row; identity and billing columns
// stay as stored, so a stale reader cannot rewrite another writer's data.
func TestUpdateWithStatus_MergesPollingFactsIntoTheStoredRow(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID:    "task_cas_merge",
		UserId:    3,
		ChannelId: 7,
		Group:     "vip",
		Status:    TaskStatusInProgress,
		Progress:  "30%",
		Quota:     900,
		Properties: Properties{
			OriginModelName:   "kling-video",
			UpstreamModelName: "kling-video-2",
		},
		Data: json.RawMessage(`{"step":1}`),
		PrivateData: TaskPrivateData{
			UpstreamTaskID: "upstream_1",
			Key:            "sk-private",
			ResultURL:      "https://old.example/v.mp4",
			PollFailures:   1,
			BillingSource:  "subscription",
			TokenId:        5,
		},
	}
	insertTask(t, task)
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).
		Update("subscription_billing_pending", true).Error)

	stale := *task
	stale.UserId = 99
	stale.ChannelId = 999
	stale.Group = "stale"
	stale.Quota = 5
	stale.Properties = Properties{OriginModelName: "stale-model"}
	stale.Progress = "60%"
	stale.PrivateData.ResultURL = "https://new.example/v.mp4"
	stale.PrivateData.PluginState = json.RawMessage(`{"req_key":"next"}`)
	stale.PrivateData.PollFailures = 4
	won, err := stale.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "60%", reloaded.Progress, "polling facts are written")
	assert.Equal(t, "https://new.example/v.mp4", reloaded.PrivateData.ResultURL)
	assert.JSONEq(t, `{"req_key":"next"}`, string(reloaded.PrivateData.PluginState))
	assert.Equal(t, 4, reloaded.PrivateData.PollFailures)

	assert.Equal(t, 3, reloaded.UserId, "identity stays as stored")
	assert.Equal(t, 7, reloaded.ChannelId)
	assert.Equal(t, "vip", reloaded.Group)
	assert.Equal(t, 900, reloaded.Quota)
	assert.True(t, reloaded.SubscriptionBillingPending)
	assert.Equal(t, "kling-video", reloaded.Properties.OriginModelName)
	assert.Equal(t, "sk-private", reloaded.PrivateData.Key, "billing facts stay as stored")
	assert.Equal(t, "upstream_1", reloaded.PrivateData.UpstreamTaskID)
	assert.Equal(t, "subscription", reloaded.PrivateData.BillingSource)

	assert.Equal(t, 3, stale.UserId, "the winner continues with the authoritative row")
	assert.Equal(t, "vip", stale.Group)
	assert.Equal(t, 900, stale.Quota)
}

// A pre-refund-era task is retired with its charge cleared in the same
// transition, so no later reader can refund it.
func TestUpdateWithStatusRetiringQuota_ClearsQuotaWithTheTransition(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID:   "task_cas_retire",
		Status:   TaskStatusInProgress,
		Progress: "50%",
		Quota:    1200,
		Data:     json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = TaskStatusFailure
	task.Progress = "100%"
	task.FailReason = "Task timed out"
	task.Quota = 0
	won, err := task.UpdateWithStatusRetiringQuota(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusFailure, reloaded.Status)
	assert.Zero(t, reloaded.Quota)
}

// The poller must also run when its only remaining work is a final task whose
// receipt settlement or billing log has not committed yet.
func TestHasUnfinishedSyncTasks_CountsPendingSettlementWork(t *testing.T) {
	truncateTables(t)

	assert.False(t, HasUnfinishedSyncTasks())

	running := &Task{TaskID: "task_gate_running", Status: TaskStatusInProgress, Progress: "40%", Data: json.RawMessage(`{}`)}
	insertTask(t, running)
	assert.True(t, HasUnfinishedSyncTasks(), "an unfinished task keeps the poller running")
	require.NoError(t, DB.Model(running).Updates(map[string]any{"progress": "100%", "subscription_billing_pending": true, "submit_time": 1}).Error)
	require.Len(t, GetAllUnFinishSyncTasks(100), 1, "nonterminal subscription task remains pollable at upstream 100 percent")
	require.Len(t, GetTimedOutUnfinishedTasks(2, 100), 1, "same task must remain eligible for timeout recovery")
	require.NoError(t, DB.Model(running).UpdateColumn("subscription_billing_pending", false).Error)
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", running.ID).Updates(map[string]any{"status": TaskStatusSuccess, "progress": "100%"}).Error)
	assert.False(t, HasUnfinishedSyncTasks())

	pendingReceipt := &Task{TaskID: "task_gate_receipt", Status: TaskStatusSuccess, Progress: "100%", SubscriptionBillingPending: true, Data: json.RawMessage(`{}`)}
	insertTask(t, pendingReceipt)
	assert.True(t, HasUnfinishedSyncTasks(), "a final task with an unsettled receipt needs another pass")
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", pendingReceipt.ID).Update("subscription_billing_pending", false).Error)
	assert.False(t, HasUnfinishedSyncTasks())

	receipt := &SubscriptionPreConsumeRecord{RequestId: "receipt-log-pending", UserId: 1, Status: "settled", BillingLogPending: true}
	require.NoError(t, DB.Create(receipt).Error)
	assert.True(t, HasUnfinishedSyncTasks(), "an undelivered settlement log needs another pass")
	require.NoError(t, DB.Model(receipt).UpdateColumn("billing_log_pending", false).Error)
	assert.False(t, HasUnfinishedSyncTasks())
	pendingReceipt.PrivateData = TaskPrivateData{SubscriptionId: 1, SubscriptionFinancialComplete: true}
	require.NoError(t, DB.Model(pendingReceipt).Updates(map[string]any{"subscription_report_pending": true, "private_data": pendingReceipt.PrivateData}).Error)
	assert.True(t, HasUnfinishedSyncTasks(), "committed finance still needs its terminal report")
	claimed, err := ClaimSubscriptionTaskReport(pendingReceipt)
	require.NoError(t, err)
	require.True(t, claimed)
	assert.False(t, HasUnfinishedSyncTasks(), "claiming the report clears the last pending work")
}

func TestUpdateWithStatus_PersistsPluginStateAndPollFailures(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID: "task_cas_plugin_state",
		Status: TaskStatusInProgress,
		Data:   json.RawMessage(`{}`),
		PrivateData: TaskPrivateData{
			PluginState:  json.RawMessage(`{"req_key":"old"}`),
			PollFailures: 1,
		},
	}
	insertTask(t, task)

	task.PrivateData.PluginState = json.RawMessage(`{"req_key":"new"}`)
	task.PrivateData.PollFailures = 4
	won, err := task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusInProgress, reloaded.Status)
	assert.JSONEq(t, `{"req_key":"new"}`, string(reloaded.PrivateData.PluginState))
	assert.Equal(t, 4, reloaded.PrivateData.PollFailures)
}
