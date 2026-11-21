package model

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type releasedExtraQuotaData struct {
	Id                  int    `json:"id"`
	UserID              int    `json:"user_id" gorm:"index;index:idx_qdt_user_created_group,priority:1"`
	Username            string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName           string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2;index:idx_qdt_user_created_group,priority:2"`
	UseGroup            string `json:"use_group" gorm:"index;size:64;default:'';index:idx_qdt_user_created_group,priority:3"`
	TokenID             int    `json:"token_id" gorm:"index;default:0"`
	ChannelID           int    `json:"channel_id" gorm:"index;default:0"`
	NodeName            string `json:"node_name" gorm:"index;size:64;default:''"`
	TokenUsed           int    `json:"token_used" gorm:"default:0"`
	Count               int    `json:"count" gorm:"default:0"`
	Quota               int    `json:"quota" gorm:"default:0"`
	CacheReadTokens     int64  `json:"cache_read_tokens" gorm:"default:0"`
	CacheCreationTokens int64  `json:"cache_creation_tokens" gorm:"default:0"`
}

func TestSubscriptionExtraCreditsDatabaseMatrix(t *testing.T) {
	oldExport, oldConsume := common.DataExportEnabled, common.LogConsumeEnabled
	common.DataExportEnabled, common.LogConsumeEnabled = true, true
	t.Cleanup(func() { common.DataExportEnabled, common.LogConsumeEnabled = oldExport, oldConsume })
	for _, dialect := range []common.DatabaseType{common.DatabaseTypeSQLite, common.DatabaseTypeMySQL, common.DatabaseTypePostgreSQL} {
		t.Run(string(dialect), func(t *testing.T) {
			dsn := "local"
			if dialect == common.DatabaseTypeMySQL {
				dsn = os.Getenv("TEST_MYSQL_DSN")
			}
			if dialect == common.DatabaseTypePostgreSQL {
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			if dsn == "" {
				t.Skip("isolated database DSN not configured")
			}
			oldPath := common.SQLitePath
			common.SQLitePath = filepath.Join(t.TempDir(), "extra.db")
			t.Cleanup(func() { common.SQLitePath = oldPath })
			t.Setenv("EXTRA_TEST_DSN", dsn)
			db, _, err := chooseDB("EXTRA_TEST_DSN", false)
			require.NoError(t, err)
			oldDB, oldLog, oldType := DB, LOG_DB, common.MainDatabaseType()
			DB = db
			LOG_DB = db
			common.SetMainDatabaseType(dialect)
			initCol()
			conn, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() {
				DB = oldDB
				LOG_DB = oldLog
				common.SetMainDatabaseType(oldType)
				initCol()
				require.NoError(t, conn.Close())
			})
			models := []any{&User{}, &SubscriptionPlan{}, &UserSubscription{}, &SubscriptionOrder{}, &SubscriptionPreConsumeRecord{}, &SubscriptionExtraCreditPurchase{}, &Log{}, &SystemTask{}, &Option{}, &Task{}, &Token{}, &Channel{}, &QuotaData{}}
			require.NoError(t, db.AutoMigrate(models...))
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(models...)) })
			require.NoError(t, db.Migrator().DropTable(&QuotaData{}))
			require.NoError(t, db.Table("quota_data").AutoMigrate(&releasedExtraQuotaData{}))
			require.NoError(t, db.Create(&QuotaData{UserID: 90001, Username: "retained-report", ModelName: "legacy-model", Quota: 7}).Error)
			// Upgrade the released tables with existing data, then repeat startup.
			require.NoError(t, db.Migrator().DropColumn(&SubscriptionPlan{}, "allow_extra_credits"))
			require.NoError(t, db.Migrator().DropColumn(&SubscriptionPlan{}, "extra_credits_rate"))
			for _, column := range []string{"extra_credits_total", "extra_credits_used", "extra_credits_groups", "extra_credits_scoped_groups", "funding_reserved", "usage_reset_version", "five_hour_reset_version", "weekly_reset_version", "monthly_reset_version"} {
				require.NoError(t, db.Migrator().DropColumn(&UserSubscription{}, column))
			}
			for _, column := range []string{"allocations", "reserved_total", "request_group", "funding_mode", "durable", "billing_log", "billing_log_pending"} {
				require.NoError(t, db.Migrator().DropColumn(&SubscriptionPreConsumeRecord{}, column))
			}
			require.NoError(t, db.Migrator().DropColumn(&Task{}, "subscription_billing_pending"))
			require.NoError(t, db.Migrator().DropColumn(&Task{}, "subscription_report_pending"))
			require.NoError(t, db.Migrator().DropColumn(&SubscriptionExtraCreditPurchase{}, "log_pending"))
			require.NoError(t, db.Table("subscription_extra_credit_purchases").Create(map[string]any{"request_id": "previous-purchase", "user_id": 90001, "wallet_quota": 4, "credits": 8, "rate": 2}).Error)
			require.NoError(t, db.Table("user_subscriptions").Create(map[string]any{"id": 90001, "user_id": 90001, "plan_id": 90001, "status": "expired", "amount_total": 40, "amount_used": 7}).Error)
			require.NoError(t, db.Table("subscription_pre_consume_records").Create(map[string]any{"request_id": "released-schema-reservation", "user_id": 90001, "user_subscription_id": 90001, "pre_consumed": 7, "status": "consumed"}).Error)
			for range 2 {
				if dialect == common.DatabaseTypeSQLite {
					require.NoError(t, ensureSubscriptionPlanTableSQLite())
				} else {
					require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}))
				}
				require.NoError(t, db.AutoMigrate(&UserSubscription{}, &SubscriptionPreConsumeRecord{}, &SubscriptionExtraCreditPurchase{}, &Task{}, &QuotaData{}))
			}
			var priorPurchase SubscriptionExtraCreditPurchase
			require.NoError(t, db.Where("request_id = ?", "previous-purchase").First(&priorPurchase).Error)
			require.EqualValues(t, 8, priorPurchase.Credits)
			require.False(t, priorPurchase.LogPending, "migration must not duplicate historical logs")
			require.True(t, db.Migrator().HasIndex(&Task{}, "SubscriptionReportPending"))
			t.Run("large-allocation-receipt", func(t *testing.T) {
				allocations := make([]subscriptionQuotaAllocation, 600)
				for i := range allocations {
					allocations[i] = subscriptionQuotaAllocation{SubscriptionId: i + 1, Regular: 1, PeriodVersion: 1}
				}
				raw, err := common.Marshal(allocations)
				require.NoError(t, err)
				require.Greater(t, len(raw), 65535)
				r := SubscriptionPreConsumeRecord{RequestId: "large-allocation-receipt", Allocations: string(raw), ReservedTotal: 600}
				require.NoError(t, db.Create(&r).Error)
				var saved SubscriptionPreConsumeRecord
				require.NoError(t, db.First(&saved, r.Id).Error)
				require.Equal(t, r.Allocations, saved.Allocations)
			})
			var retainedReports []QuotaData
			require.NoError(t, db.Where("user_id = ?", 90001).Find(&retainedReports).Error)
			require.Len(t, retainedReports, 1)
			require.Equal(t, 7, retainedReports[0].Quota)
			require.True(t, db.Migrator().HasIndex(&QuotaData{}, "idx_qdt_model_user_name"))
			var restored UserSubscription
			require.NoError(t, db.First(&restored, 90001).Error)
			require.EqualValues(t, 40, restored.AmountTotal)
			require.EqualValues(t, 7, restored.AmountUsed)
			require.Zero(t, restored.ExtraCreditsTotal)
			require.Zero(t, restored.ExtraCreditsUsed)
			var oldRecord SubscriptionPreConsumeRecord
			require.NoError(t, db.Where("request_id = ?", "released-schema-reservation").First(&oldRecord).Error)
			require.EqualValues(t, 7, oldRecord.PreConsumed)
			require.Empty(t, oldRecord.Allocations)
			require.False(t, oldRecord.Durable)
			oldRatios := ratio_setting.GroupRatio2JSONString()
			t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios)) })
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"vip":1,"default":1}`))
			var version string
			q := "select version()"
			if dialect == common.DatabaseTypeSQLite {
				q = "select sqlite_version()"
			}
			require.NoError(t, db.Raw(q).Scan(&version).Error)
			t.Log(version)
			user := User{Username: "extra-matrix", Quota: 100, Group: "default", Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&user).Error)
			plan := SubscriptionPlan{Title: "Paid extras", Enabled: true, SubscriptionGroup: "vip", TotalAmount: 5, AllowExtraCredits: true, ExtraCreditsRate: 2}
			require.NoError(t, db.Create(&plan).Error)
			InvalidateSubscriptionPlanCache(plan.Id)
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", AmountTotal: 5, StartTime: GetDBTimestamp() - 10, EndTime: GetDBTimestamp() + 3600}
			require.NoError(t, db.Create(&sub).Error)
			receipt, err := PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 10, "extra-matrix-request-001", 2)
			require.NoError(t, err)
			require.EqualValues(t, 20, receipt.Credits)
			_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 1, "extra-matrix-rate-changed", 1.5)
			require.ErrorContains(t, err, "exchange rate changed")
			again, err := PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 10, "extra-matrix-request-001", 2)
			require.NoError(t, err)
			require.Equal(t, receipt.Id, again.Id)
			_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 10, "extra-matrix-request-001", 3)
			require.Error(t, err, "same key with different historical quote conflicts")
			_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 11, "extra-matrix-request-001", 2)
			require.Error(t, err)
			_, err = PurchaseSubscriptionExtraCredits(user.Id+1, sub.Id, 10, "extra-matrix-request-002", 2)
			require.Error(t, err)
			_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 1000, "extra-matrix-request-003", 2)
			require.Error(t, err)
			require.NoError(t, db.First(&user, user.Id).Error)
			require.Equal(t, 90, user.Quota)
			result, err := PreConsumeUserSubscriptionForGroup("extra-matrix-relay-001", user.Id, "model", 0, 12, "vip")
			require.NoError(t, err)
			require.EqualValues(t, 12, result.PreConsumed)
			require.NoError(t, db.First(&sub, sub.Id).Error)
			require.EqualValues(t, 5, sub.AmountUsed)
			require.EqualValues(t, 7, sub.ExtraCreditsUsed)
			_, err = PreConsumeUserSubscriptionForGroup("extra-matrix-relay-001", user.Id, "model", 0, 13, "vip")
			require.Error(t, err, "mismatched reservation retry must not be counted as a new preconsume")
			require.NoError(t, AdjustSubscriptionRequestQuota("extra-matrix-relay-001", 3))
			require.NoError(t, AdjustSubscriptionRequestQuota("extra-matrix-relay-001", -7))
			require.NoError(t, db.First(&sub, sub.Id).Error)
			require.EqualValues(t, 5, sub.AmountUsed)
			require.EqualValues(t, 3, sub.ExtraCreditsUsed)
			require.NoError(t, RefundSubscriptionPreConsume("extra-matrix-relay-001"))
			require.NoError(t, RefundSubscriptionPreConsume("extra-matrix-relay-001"))
			require.NoError(t, db.First(&sub, sub.Id).Error)
			require.Zero(t, sub.AmountUsed)
			require.Zero(t, sub.ExtraCreditsUsed)
			// An expired instance cannot acquire a newly bound group for the first time.
			neverPurchased := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", EndTime: GetDBTimestamp() - 1, AmountTotal: 5}
			require.NoError(t, db.Create(&neverPurchased).Error)
			_, err = PurchaseSubscriptionExtraCredits(user.Id, neverPurchased.Id, 1, "extra-expired-first-purchase", 2)
			require.Error(t, err)
			require.NoError(t, db.Model(&sub).Updates(map[string]any{"status": "expired", "end_time": GetDBTimestamp() - 1}).Error)
			_, err = PreConsumeUserSubscriptionForGroup("extra-matrix-wrong-group", user.Id, "model", 0, 1, "default")
			require.Error(t, err)
			_, err = PreConsumeUserSubscriptionForGroup("extra-matrix-expired", user.Id, "model", 0, 8, "vip")
			require.NoError(t, err)
			require.NoError(t, db.First(&sub, sub.Id).Error)
			require.Zero(t, sub.AmountUsed)
			require.EqualValues(t, 8, sub.ExtraCreditsUsed)
			groups, err := GetActiveUserSubscriptionGroups(user.Id, []string{"vip", "VIP"})
			require.NoError(t, err)
			require.Equal(t, []string{"vip"}, groups)
			_, eligible, err := GetActiveSubscriptionAvailabilityForGroup(user.Id, "vip")
			require.NoError(t, err)
			require.True(t, eligible)
			require.Error(t, DeleteSubscriptionPlan(plan.Id))
			_, err = AdminDeleteUserSubscription(sub.Id)
			require.Error(t, err)
			require.NoError(t, db.Model(&plan).Update("allow_extra_credits", false).Error)
			_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 1, "extra-matrix-disabled", 2)
			require.Error(t, err)
			require.NoError(t, RefundSubscriptionPreConsume("extra-matrix-expired"))
			_, err = PreConsumeUserSubscriptionForGroup("extra-matrix-full-reserve", user.Id, "model", 0, 20, "vip")
			require.NoError(t, err)
			_, err = AdminDeleteUserSubscription(sub.Id)
			require.Error(t, err)
			_, err = AdminInvalidateUserSubscription(sub.Id)
			require.Error(t, err)
			require.Error(t, DeleteSubscriptionPlan(plan.Id))
			require.NoError(t, RefundSubscriptionPreConsume("extra-matrix-full-reserve"))
			matrixRequest := "matrix-task-settlement"
			_, err = PreConsumeUserSubscriptionForGroup(matrixRequest, user.Id, "model", 0, 2, "vip")
			require.NoError(t, err)
			token := Token{UserId: user.Id, Key: matrixRequest, RemainQuota: 8, UsedQuota: 2}
			require.NoError(t, db.Create(&token).Error)
			longModel := strings.Repeat("m", 129)
			submitQuota := 2
			task := Task{UserId: user.Id, Quota: 2, SubscriptionBillingPending: true, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: matrixRequest, TokenId: token.Id, SubscriptionSubmitQuota: &submitQuota, SubscriptionSubmitLog: &RecordTaskBillingLogParams{ModelName: longModel, Other: NewLogOther()}}}
			require.NoError(t, db.Create(&task).Error)
			largeOther := NewLogOther()
			largeOther.SetPublic("retained_request_details", strings.Repeat("x", 35_000))
			task.PrivateData.SubscriptionSubmitLog.Other = largeOther
			task.PrivateData.SubscriptionSubmitLog.RequestId = strings.Repeat("r", 65)
			task.PrivateData.SubscriptionSubmitLog.UpstreamRequestId = strings.Repeat("u", 129)
			task.PrivateData.SubscriptionSubmitLog.NodeName = strings.Repeat("n", 65)
			require.NoError(t, db.Model(&task).UpdateColumn("private_data", task.PrivateData).Error)
			_, err = SettleSubscriptionTaskQuota(&task, 1, RecordTaskBillingLogParams{Other: largeOther})
			require.NoError(t, err, "two pending events larger than MySQL TEXT cannot block final finance")
			var queued SubscriptionPreConsumeRecord
			require.NoError(t, db.Where("request_id = ?", matrixRequest).First(&queued).Error)
			require.Greater(t, len(queued.BillingLog), 65_535)
			require.NoError(t, db.First(&token, token.Id).Error)
			require.Equal(t, 9, token.RemainQuota)
			require.NoError(t, FlushSubscriptionTaskBillingLogs())
			require.NoError(t, FlushSubscriptionTaskBillingLogs())
			var taskReceipt SubscriptionPreConsumeRecord
			require.NoError(t, db.Where("request_id = ?", matrixRequest).First(&taskReceipt).Error)
			require.False(t, taskReceipt.BillingLogPending)
			var longLog Log
			require.NoError(t, db.Where("request_id = ?", strings.Repeat("r", 65)).Take(&longLog).Error)
			require.Equal(t, strings.Repeat("u", 129), longLog.UpstreamRequestId)
			var longExport QuotaData
			require.NoError(t, db.Where("model_name = ?", longModel).Take(&longExport).Error)
			require.Equal(t, strings.Repeat("n", 65), longExport.NodeName)
			if dialect != common.DatabaseTypeSQLite {
				// Permanent width failures leave an independent root-only audit row;
				// a failed export must not consume its sibling log's delivery ack.
				wide := strings.Repeat("界", 300)
				events := []subscriptionTaskBillingEvent{
					{EventId: "wide-log", Log: Log{RequestId: wide, UpstreamRequestId: wide + "-upstream", Type: LogTypeConsume}, Export: &QuotaData{NodeName: "fits", Quota: 9}},
					{EventId: "wide-export", Log: Log{RequestId: "fits", Type: LogTypeConsume}, Export: &QuotaData{NodeName: wide, ModelName: wide + "-model", Quota: 8}},
				}
				raw, err := common.Marshal(events)
				require.NoError(t, err)
				r := SubscriptionPreConsumeRecord{RequestId: "quarantine-cleanup", Status: "settled", BillingLog: string(raw), BillingLogPending: true}
				require.NoError(t, db.Create(&r).Error)
				const hook = "test:reject_quarantine"
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register(hook, func(tx *gorm.DB) {
					if entry, ok := tx.Statement.Dest.(*Log); ok && entry.Type == LogTypeSystem {
						tx.AddError(fmt.Errorf("audit unavailable"))
					}
				}))
				err = FlushSubscriptionTaskBillingLogs()
				require.NoError(t, db.Callback().Create().Remove(hook))
				require.ErrorContains(t, err, "audit unavailable")
				require.NoError(t, db.First(&r, r.Id).Error)
				require.True(t, r.BillingLogPending)
				for range 2 {
					require.NoError(t, FlushSubscriptionTaskBillingLogs())
				}
				require.NoError(t, db.First(&r, r.Id).Error)
				require.False(t, r.BillingLogPending)
				require.Empty(t, r.BillingLog)
				require.NoError(t, db.Model(&r).UpdateColumn("updated_at", GetDBTimestamp()-1000).Error)
				_, err = CleanupSubscriptionPreConsumeRecords(500)
				require.NoError(t, err)
				require.ErrorIs(t, db.First(&SubscriptionPreConsumeRecord{}, r.Id).Error, gorm.ErrRecordNotFound)
				var archived []Log
				require.NoError(t, db.Where("type = ? AND subscription_event_id IN ?", LogTypeSystem, []string{"wide-log", "wide-export"}).Order("id").Find(&archived).Error)
				require.Len(t, archived, 2)
				require.Contains(t, archived[0].Other, wide+"-upstream")
				require.Contains(t, archived[0].Other, `"node_name":"fits"`)
				require.Contains(t, archived[1].Other, `"request_id":"fits"`)
				require.Contains(t, archived[1].Other, wide+"-model")
				require.NotContains(t, formatLogOtherJSON(archived[0].Other, logOtherVisibilityUser), wide)
				var count int64
				require.NoError(t, db.Model(&Log{}).Where("type = ? AND request_id = ?", LogTypeConsume, "fits").Count(&count).Error)
				require.EqualValues(t, 1, count)
				require.NoError(t, db.Model(&QuotaData{}).Where("node_name = ? AND quota = ?", "fits", 9).Count(&count).Error)
				require.EqualValues(t, 1, count)
				if dialect == common.DatabaseTypePostgreSQL {
					// The log model column is text, but an incompressible key can still exceed btree's page limit.
					var model strings.Builder
					for range 300 {
						model.WriteString(common.GetUUID())
					}
					e := subscriptionTaskBillingEvent{EventId: "wide-btree", Log: Log{ModelName: model.String(), RequestId: "fits-indexed-request", Type: LogTypeConsume}}
					raw, err := common.Marshal([]subscriptionTaskBillingEvent{e})
					require.NoError(t, err)
					r := SubscriptionPreConsumeRecord{RequestId: "wide-index", BillingLogPending: true, BillingLog: string(raw)}
					require.NoError(t, db.Create(&r).Error)
					require.NoError(t, FlushSubscriptionTaskBillingLogs())
					require.NoError(t, db.First(&r, r.Id).Error)
					require.False(t, r.BillingLogPending)
					var archive Log
					require.NoError(t, db.Where("subscription_event_id = ? AND type = ?", e.EventId, LogTypeSystem).Take(&archive).Error)
					require.Contains(t, archive.Other, model.String())
				}
			}

			// Deliver a second charge into an independently configured log DB.
			logDSN := "local"
			if dialect == common.DatabaseTypeMySQL {
				logDSN = os.Getenv("TEST_MYSQL_LOG_DSN")
			}
			if dialect == common.DatabaseTypePostgreSQL {
				logDSN = os.Getenv("TEST_POSTGRES_LOG_DSN")
			}
			require.NotEmpty(t, logDSN, "separate log database required in matrix")
			common.SQLitePath = filepath.Join(t.TempDir(), "separate-log.db")
			t.Setenv("EXTRA_LOG_TEST_DSN", logDSN)
			logDB, _, err := chooseDB("EXTRA_LOG_TEST_DSN", true)
			require.NoError(t, err)
			logConn, err := logDB.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, logDB.Migrator().DropTable(&Log{})); require.NoError(t, logConn.Close()) })
			for range 2 {
				require.NoError(t, logDB.AutoMigrate(&Log{}))
			}
			LOG_DB = logDB
			matrixRequest = "matrix-separate-log-settlement"
			_, err = PreConsumeUserSubscriptionForGroup(matrixRequest, user.Id, "model", 0, 2, "vip")
			require.NoError(t, err)
			task = Task{UserId: user.Id, Quota: 2, SubscriptionBillingPending: true, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: matrixRequest, TokenId: token.Id}}
			require.NoError(t, db.Create(&task).Error)
			_, err = SettleSubscriptionTaskQuota(&task, 3, RecordTaskBillingLogParams{ModelName: matrixRequest})
			require.NoError(t, err)
			const failAck = "test:reject_receipt_ack"
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register(failAck, func(tx *gorm.DB) {
				tx.AddError(fmt.Errorf("lost receipt ack"))
			}))
			err = FlushSubscriptionTaskBillingLogs()
			require.NoError(t, db.Callback().Update().Remove(failAck))
			require.ErrorContains(t, err, "lost receipt ack")
			var exports, logCount int64
			require.NoError(t, logDB.Model(&Log{}).Where("user_id = ?", user.Id).Count(&logCount).Error)
			require.EqualValues(t, 1, logCount, "separate log commit survived lost ack")
			require.NoError(t, db.Model(&QuotaData{}).Where("model_name = ?", matrixRequest).Count(&exports).Error)
			require.Zero(t, exports, "primary export rolled back with ack")
			require.NoError(t, FlushSubscriptionTaskBillingLogs())
			require.NoError(t, db.Model(&QuotaData{}).Where("model_name = ?", matrixRequest).Count(&exports).Error)
			require.EqualValues(t, 1, exports)
			require.NoError(t, logDB.Model(&Log{}).Where("user_id = ?", user.Id).Count(&logCount).Error)
			require.EqualValues(t, 1, logCount)
			LOG_DB = db
			if dialect == common.DatabaseTypeMySQL {
				stale := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
				require.NoError(t, stale.Error)
				var before []UserSubscription
				require.NoError(t, stale.Where("user_id = ?", user.Id).Find(&before).Error)
				upgraded := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: GetDBTimestamp(), EndTime: GetDBTimestamp() + 7200, UpgradeGroup: "vip"}
				require.NoError(t, db.Create(&upgraded).Error)
				require.NoError(t, db.Model(&user).UpdateColumn("group", "vip").Error)
				old := sub
				old.UpgradeGroup = "vip"
				old.PrevUserGroup = "default"
				target, err := downgradeUserGroupForSubscriptionTx(stale, &old, GetDBTimestamp())
				require.NoError(t, err)
				require.Empty(t, target, "a committed upgraded grant remains visible beyond an older snapshot")
				require.NoError(t, stale.Rollback().Error)
				require.NoError(t, db.Delete(&upgraded).Error)
			}

			if dialect != common.DatabaseTypeSQLite {
				require.NoError(t, db.Model(&plan).Update("allow_extra_credits", true).Error)
				// Only an active, unexpired subscription can buy credits.
				require.NoError(t, db.Model(&sub).Updates(map[string]any{"status": "active", "end_time": GetDBTimestamp() + 3600}).Error)
				var wg sync.WaitGroup
				errors := make(chan error, 2)
				for range 2 {
					wg.Go(func() {
						_, err := PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 10, "extra-matrix-concurrent", 2)
						errors <- err
					})
				}
				wg.Wait()
				close(errors)
				for err := range errors {
					require.NoError(t, err)
				}
				require.NoError(t, db.First(&user, user.Id).Error)
				require.Equal(t, 80, user.Quota)
				require.NoError(t, db.First(&sub, sub.Id).Error)
				require.EqualValues(t, 40, sub.ExtraCreditsTotal)
				require.NoError(t, db.Model(&sub).Updates(map[string]any{"status": "expired", "end_time": GetDBTimestamp() - 1}).Error)
			}

			// Expiry returns the unspent credits at the paid rate, exactly once.
			require.NoError(t, db.First(&sub, sub.Id).Error)
			require.Zero(t, sub.FundingReserved)
			unspent := sub.ExtraCreditsTotal - sub.ExtraCreditsUsed
			require.Positive(t, unspent)
			var paid struct{ Wallet, Credits int64 }
			require.NoError(t, db.Model(&SubscriptionExtraCreditPurchase{}).Where("subscription_id = ?", sub.Id).
				Select("SUM(wallet_quota) AS wallet, SUM(credits) AS credits").Scan(&paid).Error)
			require.NoError(t, db.First(&user, user.Id).Error)
			quotaBefore := user.Quota
			for range 2 {
				_, err = RefundExpiredExtraCredits(100)
				require.NoError(t, err)
			}
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.EqualValues(t, int64(quotaBefore)+unspent*paid.Wallet/paid.Credits, user.Quota)
			require.NoError(t, db.First(&sub, sub.Id).Error)
			assert.Equal(t, sub.ExtraCreditsUsed, sub.ExtraCreditsTotal)
		})
	}
}

func TestSubscriptionExtraCreditsRefundDoesNotRequireSubmissionTopup(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Refund without topup", SubscriptionGroup: "vip", TotalAmount: 10}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 10}
	require.NoError(t, DB.Create(&sub).Error)
	id := "refund-without-failed-topup"
	result, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	target := 15
	task := Task{UserId: user.Id, Quota: 10, SubscriptionBillingPending: true, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: id, SubscriptionReceiptId: result.ReceiptId, SubscriptionSubmitQuota: &target, SubscriptionSubmitLog: &RecordTaskBillingLogParams{ModelName: "model", Other: NewLogOther()}}}
	require.NoError(t, DB.Create(&task).Error)
	_, err = SettleSubscriptionTaskQuota(&task, 0)
	require.NoError(t, err, "a confirmed full refund cannot require five more credits")
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.Zero(t, sub.AmountUsed)
	require.Zero(t, sub.FundingReserved)
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Zero(t, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
	var old SubscriptionPreConsumeRecord
	require.NoError(t, DB.First(&old, result.ReceiptId).Error)
	require.NoError(t, DB.Delete(&old).Error)
	replacement, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 8, "vip")
	require.NoError(t, err)
	_, err = SettleSubscriptionTaskQuota(&task, 0)
	require.NoError(t, err, "completed task replay returns its saved outcome")
	var receipt SubscriptionPreConsumeRecord
	require.NoError(t, DB.First(&receipt, replacement.ReceiptId).Error)
	require.Equal(t, "consumed", receipt.Status)
	require.EqualValues(t, 8, receipt.ReservedTotal, "old task cannot retire replacement receipt")
}

func TestSubscriptionExtraCreditsTaskSubmissionAccounting(t *testing.T) {
	truncateTables(t)
	oldExport := common.DataExportEnabled
	common.DataExportEnabled = true
	t.Cleanup(func() { common.DataExportEnabled = oldExport })
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Submit accounting", SubscriptionGroup: "vip", TotalAmount: 20}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 20}
	require.NoError(t, DB.Create(&sub).Error)
	id := "atomic-task-submission"
	_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	token := Token{UserId: user.Id, Key: id, Name: "submission-token", RemainQuota: 90, UsedQuota: 10}
	require.NoError(t, DB.Create(&token).Error)
	target := 3
	task := Task{UserId: user.Id, Quota: 10, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: id, TokenId: token.Id, SubscriptionSubmitQuota: &target, SubscriptionSubmitLog: &RecordTaskBillingLogParams{ModelName: "submission-model", RequestId: "original-submission-request", UpstreamRequestId: "upstream-correlation", PromptTokens: 7, Ip: "192.0.2.10", Other: NewLogOther()}}}
	require.NoError(t, DB.Create(&task).Error)
	require.NoError(t, PrepareSubscriptionTaskQuota(&task, 10), "durable target overrides stale caller quota")
	require.NoError(t, DB.First(&token, token.Id).Error)
	require.Equal(t, 97, token.RemainQuota)
	require.Equal(t, 3, token.UsedQuota)
	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", id).First(&record).Error)
	require.Equal(t, "consumed", record.Status)
	require.True(t, record.Durable)
	require.EqualValues(t, 3, record.ReservedTotal)
	require.NoError(t, PrepareSubscriptionTaskQuota(&task, target))
	require.NoError(t, DB.First(&token, token.Id).Error)
	require.Equal(t, 97, token.RemainQuota)
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Equal(t, 3, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
	require.NoError(t, DB.Exec("CREATE TRIGGER fail_task_export BEFORE INSERT ON quota_data BEGIN SELECT RAISE(ABORT, 'injected export failure'); END").Error)
	t.Cleanup(func() { DB.Exec("DROP TRIGGER IF EXISTS fail_task_export") })
	_, err = SettleSubscriptionTaskQuota(&task, 5)
	require.NoError(t, err, "reporting failure cannot roll back financial settlement")
	require.Error(t, FlushSubscriptionTaskBillingLogs(), "export stays pending while its storage fails")
	var delivered int64
	require.NoError(t, DB.Model(&Log{}).Where("user_id = ? AND type = ?", user.Id, LogTypeConsume).Count(&delivered).Error)
	require.EqualValues(t, 2, delivered, "failed exports must not suppress either consumption log")
	require.NoError(t, DB.First(&record, record.Id).Error)
	require.NotContains(t, record.BillingLog, "192.0.2.10")
	require.NotContains(t, record.BillingLog, "upstream-correlation")
	require.NoError(t, DB.Exec("DROP TRIGGER fail_task_export").Error)
	require.NoError(t, DB.Create(&Log{UserId: user.Id, RequestId: "original-submission-request", Type: LogTypeError, ChannelId: 99}).Error)
	require.NoError(t, FlushSubscriptionTaskBillingLogs())
	require.NoError(t, FlushSubscriptionTaskBillingLogs())
	var entries []Log
	require.NoError(t, DB.Where("user_id = ? AND type IN ?", user.Id, []int{LogTypeConsume, LogTypeRefund}).Order("id").Find(&entries).Error)
	require.Len(t, entries, 2, "terminal log cannot overwrite undelivered submission log")
	require.Equal(t, 3, entries[0].Quota)
	require.Equal(t, "original-submission-request", entries[0].RequestId)
	require.Equal(t, "upstream-correlation", entries[0].UpstreamRequestId)
	require.Equal(t, 7, entries[0].PromptTokens)
	require.Equal(t, "192.0.2.10", entries[0].Ip)
	require.Equal(t, 2, entries[1].Quota)
	require.Equal(t, "submission-token", entries[1].TokenName)
	var reports []QuotaData
	require.NoError(t, DB.Where("user_id = ?", user.Id).Find(&reports).Error)
	require.Len(t, reports, 2)
	require.Equal(t, 5, reports[0].Quota+reports[1].Quota)
	require.NoError(t, DB.First(&record, record.Id).Error)
	require.Empty(t, record.BillingLog, "delivered IP and request details are not retained in the funding receipt")
	// One permanently bad event cannot prevent later users from receiving logs.
	bad := SubscriptionPreConsumeRecord{RequestId: "poisoned-log-event", UserId: user.Id, Status: "settled", BillingLogPending: true, BillingLog: "not-json"}
	require.NoError(t, DB.Create(&bad).Error)
	raw, err := common.Marshal([]subscriptionTaskBillingEvent{{Log: Log{UserId: user.Id, RequestId: "independent-healthy-event", Type: LogTypeConsume}, EventId: "independent-event-id"}})
	require.NoError(t, err)
	good := SubscriptionPreConsumeRecord{RequestId: "healthy-log-event", UserId: user.Id, Status: "settled", BillingLogPending: true, BillingLog: string(raw)}
	require.NoError(t, DB.Create(&good).Error)
	require.Error(t, FlushSubscriptionTaskBillingLogs())
	require.NoError(t, DB.First(&good, good.Id).Error)
	require.False(t, good.BillingLogPending)
	var healthyCount int64
	require.NoError(t, DB.Model(&Log{}).Where("request_id = ?", "independent-healthy-event").Count(&healthyCount).Error)
	require.EqualValues(t, 1, healthyCount)
}

func TestSubscriptionExtraCreditsZeroInterimReservation(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Interim zero", SubscriptionGroup: "vip", TotalAmount: 10}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 10}
	require.NoError(t, DB.Create(&sub).Error)
	id := "interim-zero-task"
	_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 1, "vip")
	require.NoError(t, err)
	require.NoError(t, AdjustSubscriptionRequestQuota(id, -1))
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", id).UpdateColumn("updated_at", GetDBTimestamp()-1000).Error)
	require.NoError(t, PreserveSubscriptionRequest(id), "zero interim estimate is still an open async receipt")
	removed, err := CleanupSubscriptionPreConsumeRecords(500)
	require.NoError(t, err)
	require.Zero(t, removed, "an accepted task is still between preserve and INSERT")
	task := Task{UserId: user.Id, Quota: 0, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: id}}
	require.NoError(t, DB.Create(&task).Error)
	require.NoError(t, DB.Model(&sub).Updates(map[string]any{"status": "expired", "end_time": GetDBTimestamp() - 1}).Error)
	_, err = SettleSubscriptionTaskQuota(&task, 3)
	require.NoError(t, err)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 3, sub.AmountUsed)
	require.NoError(t, DB.Model(&sub).Updates(map[string]any{"status": "active", "end_time": GetDBTimestamp() + 3600}).Error)
	second := id + "-deleted-plan"
	_, err = PreConsumeUserSubscriptionForGroup(second, user.Id, "model", 0, 1, "vip")
	require.NoError(t, err)
	require.NoError(t, AdjustSubscriptionRequestQuota(second, -1))
	require.NoError(t, DB.Model(&sub).Updates(map[string]any{"status": "expired", "end_time": GetDBTimestamp() - 1}).Error)
	require.NoError(t, DeleteSubscriptionPlan(plan.Id))
	replacementPlan := SubscriptionPlan{Title: "Retained paid credits", SubscriptionGroup: "vip"}
	require.NoError(t, DB.Create(&replacementPlan).Error)
	replacement := UserSubscription{UserId: user.Id, PlanId: replacementPlan.Id, Status: "expired", EndTime: GetDBTimestamp() - 1, ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, DB.Create(&replacement).Error)
	secondTask := Task{UserId: user.Id, Quota: 0, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: second}}
	require.NoError(t, DB.Create(&secondTask).Error)
	_, err = SettleSubscriptionTaskQuota(&secondTask, 3)
	require.NoError(t, err, "deleted empty funding history cannot hide available paid credits")
	require.NoError(t, DB.First(&replacement, replacement.Id).Error)
	require.EqualValues(t, 3, replacement.ExtraCreditsUsed)
}

func TestSubscriptionExtraCreditsTaskTokenSettlementAtomic(t *testing.T) {
	for _, actual := range []int{0, 15} {
		t.Run(fmt.Sprint(actual), func(t *testing.T) {
			truncateTables(t)
			require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
			user := createReserveTestUser(t, 100)
			plan := SubscriptionPlan{Title: "Atomic task token", SubscriptionGroup: "vip"}
			require.NoError(t, DB.Create(&plan).Error)
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", EndTime: GetDBTimestamp() - 1, ExtraCreditsTotal: 50, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
			require.NoError(t, DB.Create(&sub).Error)
			id := fmt.Sprintf("atomic-task-token-%d", actual)
			_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 10, "vip")
			require.NoError(t, err)
			require.NoError(t, DB.Model(&user).UpdateColumn("used_quota", 10).Error)
			token := Token{UserId: user.Id, Key: id, RemainQuota: 90, UsedQuota: 10}
			require.NoError(t, DB.Create(&token).Error)
			submitQuota := 10
			task := Task{UserId: user.Id, Quota: 10, SubscriptionBillingPending: true, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: id, TokenId: token.Id, SubscriptionSubmitQuota: &submitQuota}}
			require.NoError(t, DB.Create(&task).Error)
			if actual == 0 {
				require.NoError(t, DB.Delete(&user).Error)
			}
			require.NoError(t, DB.Exec(`CREATE TRIGGER fail_atomic_task_quota BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT, 'injected quota write failure'); END`).Error)
			t.Cleanup(func() { DB.Exec("DROP TRIGGER IF EXISTS fail_atomic_task_quota") })
			_, err = SettleSubscriptionTaskQuota(&task, actual)
			require.Error(t, err)
			claimed, claimErr := ClaimSubscriptionTaskReport(&task)
			require.NoError(t, claimErr)
			require.False(t, claimed, "failed finance cannot report a completed task")
			require.NoError(t, DB.First(&token, token.Id).Error)
			require.Equal(t, 90, token.RemainQuota, "failed task commit also rolls back its token difference")
			require.NoError(t, DB.First(&sub, sub.Id).Error)
			require.EqualValues(t, 10, sub.ExtraCreditsUsed)
			require.NoError(t, DB.Exec("DROP TRIGGER fail_atomic_task_quota").Error)
			delta, err := SettleSubscriptionTaskQuota(&task, actual+1)
			require.NoError(t, err)
			var stored Task
			require.NoError(t, DB.First(&stored, task.ID).Error)
			require.True(t, stored.SubscriptionReportPending)
			require.Equal(t, actual-10, delta)
			require.NoError(t, DB.First(&token, token.Id).Error)
			require.Equal(t, 100-actual, token.RemainQuota)
			require.Equal(t, actual, token.UsedQuota)
			require.NoError(t, DB.Unscoped().First(&user, user.Id).Error)
			require.Equal(t, actual, user.UsedQuota, "statistics commit with the financial change")
			var receipt SubscriptionPreConsumeRecord
			require.NoError(t, DB.Where("request_id = ?", id).First(&receipt).Error)
			require.True(t, receipt.BillingLogPending, "a committed settlement retains its undelivered log")
			require.NoError(t, FlushSubscriptionTaskBillingLogs())
			require.NoError(t, FlushSubscriptionTaskBillingLogs())
			var entries int64
			require.NoError(t, DB.Model(&Log{}).Where("user_id = ?", user.Id).Count(&entries).Error)
			require.EqualValues(t, 1, entries, "a crash before delivery recovers exactly one financial log")
			delta, err = SettleSubscriptionTaskQuota(&task, actual)
			require.NoError(t, err)
			require.Zero(t, delta, "replaying a committed settlement cannot apply the token difference twice")
			require.NoError(t, DB.First(&token, token.Id).Error)
			require.Equal(t, 100-actual, token.RemainQuota)
			claimed, err = ClaimSubscriptionTaskReport(&task)
			require.NoError(t, err)
			require.True(t, claimed)
			claimed, err = ClaimSubscriptionTaskReport(&task)
			require.NoError(t, err)
			require.False(t, claimed, "terminal retry cannot report twice")
		})
	}
}

func TestSubscriptionExtraCreditsManualResetRefundIsolation(t *testing.T) {
	for _, advance := range []bool{false, true} {
		t.Run(fmt.Sprint(advance), func(t *testing.T) {
			truncateTables(t)
			user := createReserveTestUser(t, 100)
			plan := SubscriptionPlan{Title: "No automatic reset", SubscriptionGroup: "vip", TotalAmount: 10, QuotaResetPeriod: SubscriptionResetNever}
			require.NoError(t, DB.Create(&plan).Error)
			now := GetDBTimestamp()
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: now + 1000, AmountTotal: 10, FiveHourLimit: 10, WeeklyLimit: 10, MonthlyLimit: 10}
			require.NoError(t, DB.Create(&sub).Error)
			for pass := range 2 {
				id := fmt.Sprintf("before-manual-reset-%t-%d", advance, pass)
				_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
				require.NoError(t, err)
				require.NoError(t, DB.First(&sub, sub.Id).Error)
				require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
					return resetUserSubscriptionTx(tx, &sub, &plan, now, advance, SubscriptionResetOptions{ResetTotal: true, ResetFiveHour: true, ResetWeekly: true, ResetMonthly: true})
				}))
				_, err = PreConsumeUserSubscriptionForGroup(fmt.Sprintf("after-manual-reset-%t-%d", advance, pass), user.Id, "model", 0, 4, "vip")
				require.NoError(t, err)
				require.NoError(t, RefundSubscriptionPreConsume(id))
				require.NoError(t, DB.First(&sub, sub.Id).Error)
				require.EqualValues(t, 4, sub.AmountUsed, "old refund cannot remove new usage after reset")
				require.EqualValues(t, 4, sub.FiveHourUsed)
				require.EqualValues(t, 4, sub.WeeklyUsed)
				require.EqualValues(t, 4, sub.MonthlyUsed)
				require.NoError(t, DB.Model(&sub).Updates(map[string]any{"amount_used": 0, "five_hour_used": 0, "weekly_used": 0, "monthly_used": 0}).Error)
			}
		})
	}
}

func TestSubscriptionExtraCreditsAbandonedReceiptCleanup(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Cleanup receipts", SubscriptionGroup: "vip", TotalAmount: 20}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 20}
	require.NoError(t, DB.Create(&sub).Error)
	for _, id := range []string{"cleanup-abandoned-sync", "cleanup-retained-task"} {
		_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
		require.NoError(t, err)
	}
	task := Task{UserId: user.Id, Quota: 5, Status: TaskStatusInProgress, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: "cleanup-retained-task"}}
	require.NoError(t, DB.Create(&task).Error)
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("user_id = ?", user.Id).UpdateColumn("updated_at", GetDBTimestamp()-1000).Error)
	removed, err := CleanupSubscriptionPreConsumeRecords(500)
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 5, sub.FundingReserved, "only the asynchronous task still owns a refundable reservation")
	require.EqualValues(t, 10, sub.AmountUsed, "cleanup keeps the original pre-consumed charge")
	var retained SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", "cleanup-retained-task").First(&retained).Error)
	require.True(t, retained.Durable)
}

func TestSubscriptionExtraCreditsRenewedPeriodRefundIsolation(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Renewed never-reset", SubscriptionGroup: "vip", TotalAmount: 10, QuotaResetPeriod: SubscriptionResetNever, DurationUnit: SubscriptionDurationDay, DurationValue: 1}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 1000, AmountTotal: 10}
	require.NoError(t, DB.Create(&sub).Error)
	_, err := PreConsumeUserSubscriptionForGroup("before-new-period", user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.NoError(t, sub.startBillingPeriod(&plan, time.Unix(GetDBTimestamp(), 0), 10))
	require.NoError(t, DB.Save(&sub).Error)
	_, err = PreConsumeUserSubscriptionForGroup("after-new-period", user.Id, "model", 0, 4, "vip")
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("before-new-period"))
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 4, sub.AmountUsed)
}

func TestSubscriptionExtraCreditsDeletedPlanEligibility(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Historical", SubscriptionGroup: "vip"}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", EndTime: GetDBTimestamp() - 1}
	require.NoError(t, DB.Create(&sub).Error)
	require.NoError(t, DeleteSubscriptionPlan(plan.Id))
	allowed, err := SubscriptionExtraCreditPurchaseEligibility(user.Id, &sub)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestSubscriptionExtraCreditsPurchaseEligibility(t *testing.T) {
	t.Run("disabled-before-user-lookup", func(t *testing.T) {
		truncateTables(t)
		plan := SubscriptionPlan{Title: "No extras"}
		require.NoError(t, DB.Create(&plan).Error)
		InvalidateSubscriptionPlanCache(plan.Id)
		allowed, err := SubscriptionExtraCreditPurchaseEligibility(777, &UserSubscription{UserId: 777, Status: "active", PlanId: plan.Id})
		require.NoError(t, err)
		require.False(t, allowed)
	})
	for _, changedKind := range []bool{false, true} {
		t.Run(fmt.Sprint(changedKind), func(t *testing.T) {
			truncateTables(t)
			user := createReserveTestUser(t, 100)
			plan := SubscriptionPlan{Title: "Purchase qualification", SubscriptionGroup: "vip", AllowExtraCredits: true, ExtraCreditsRate: 2}
			require.NoError(t, DB.Create(&plan).Error)
			InvalidateSubscriptionPlanCache(plan.Id)
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 1000, FundingReserved: 5}
			if changedKind {
				sub.Status = "expired"
				sub.EndTime = GetDBTimestamp() - 1
				sub.ExtraCreditsTotal = 4
				sub.ExtraCreditsUsed = 4
				sub.ExtraCreditsGroups = `["vip"]`
				sub.ExtraCreditsScopedGroups = `[]`
				sub.FundingReserved = 0
			}
			require.NoError(t, DB.Create(&sub).Error)
			if changedKind {
				require.False(t, CanPurchaseSubscriptionExtraCredits(user.Id, &sub), "a changed ordinary/scoped grant cannot be repurchased after expiry")
				_, err := PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 2, "eligibility-expired-scoped", 2)
				require.Error(t, err)
			} else {
				require.True(t, CanPurchaseSubscriptionExtraCredits(user.Id, &sub), "first purchase is available during ordinary in-flight usage")
				_, err := PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 2, "eligibility-first-purchase", 2)
				require.NoError(t, err)
			}
		})
	}
}

func TestSubscriptionExtraCreditsLegacyLargeReservation(t *testing.T) {
	for _, refund := range []bool{false, true} {
		t.Run(fmt.Sprint(refund), func(t *testing.T) {
			truncateTables(t)
			user := createReserveTestUser(t, 100)
			plan := SubscriptionPlan{Title: "Unlimited ordinary", SubscriptionGroup: "vip"}
			require.NoError(t, DB.Create(&plan).Error)
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 1000}
			require.NoError(t, DB.Create(&sub).Error)
			id := fmt.Sprintf("large-legacy-reservation-%t", refund)
			amount := int64(common.MaxWalletQuota) + 1
			_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, amount, "vip")
			require.NoError(t, err)
			if refund {
				require.NoError(t, RefundSubscriptionPreConsume(id))
			} else {
				require.NoError(t, SettleSubscriptionRequestQuota(id, amount))
				taskID := id + "-task"
				_, err = PreConsumeUserSubscriptionForGroup(taskID, user.Id, "model", 0, amount, "vip")
				require.NoError(t, err)
				task := Task{UserId: user.Id, Quota: int(amount), PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: taskID}}
				require.NoError(t, DB.Create(&task).Error)
				_, err = SettleSubscriptionTaskQuota(&task, int(amount))
				require.NoError(t, err, "large ordinary-only task remains compatible with int64 quota")
			}
			require.NoError(t, DB.First(&sub, sub.Id).Error)
			require.Zero(t, sub.FundingReserved)
		})
	}
}

func TestSubscriptionExtraCreditsDeniedOrdinaryAuthorization(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Denied ordinary", TotalAmount: 20, WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "blacklist", WalletOnlyGroups: "default"}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 20, ExtraCreditsTotal: 1, ExtraCreditsGroups: `["default"]`, ExtraCreditsScopedGroups: `[]`}
	require.NoError(t, DB.Create(&sub).Error)
	id := "extra-only-denied-ordinary"
	_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 1, "default")
	require.NoError(t, err)
	require.Error(t, AdjustSubscriptionRequestQuota(id, 1), "spending paid extras cannot acquire ordinary entitlement")
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.Zero(t, sub.AmountUsed)
	require.EqualValues(t, 1, sub.ExtraCreditsUsed)
}

func TestSubscriptionExtraCreditsOriginalPolicySettlement(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Ordinary original policy", TotalAmount: 20}
	require.NoError(t, DB.Create(&plan).Error)
	now := GetDBTimestamp()
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 120, LastResetTime: now - 120, NextResetTime: now + 100, EndTime: now + 1000, AmountTotal: 20}
	require.NoError(t, DB.Create(&sub).Error)
	id := "legacy-policy-change-settle"
	_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "default")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&plan).Updates(map[string]any{"wallet_only_groups_enabled": true, "wallet_only_groups_mode": "blacklist", "wallet_only_groups": "default"}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	require.NoError(t, AdjustSubscriptionRequestQuota(id, 3))
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 8, sub.AmountUsed)
	require.NoError(t, DB.Model(&plan).Updates(map[string]any{"quota_reset_period": SubscriptionResetCustom, "quota_reset_custom_seconds": 60}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	require.NoError(t, DB.Model(&sub).UpdateColumn("next_reset_time", now-60).Error)
	require.Error(t, AdjustSubscriptionRequestQuota(id, 1), "old authorization cannot cross a natural total reset before the scheduler runs")
}

func TestSubscriptionExtraCreditsReplacedReceiptIdentity(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Receipt identity", SubscriptionGroup: "vip", TotalAmount: 20}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 1000, AmountTotal: 20}
	require.NoError(t, DB.Create(&sub).Error)
	id := "recreated-receipt-identity"
	_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	var original SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", id).First(&original).Error)
	_, err = AdjustSubscriptionRequestQuotaWithResult(id, 3, original)
	require.NoError(t, err)
	_, err = AdjustSubscriptionRequestQuotaWithResult(id, 3, original)
	require.Error(t, err, "a stale snapshot cannot add the same adjustment twice")
	require.NoError(t, DB.Delete(&original).Error)
	replacement := original
	replacement.Id = 0
	require.NoError(t, DB.Create(&replacement).Error)
	require.NotEqual(t, original.Id, replacement.Id)
	_, err = AdjustSubscriptionRequestQuotaWithResult(id, -1, original)
	require.Error(t, err, "snapshot must not mutate a different receipt with the same exact key")
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 8, sub.AmountUsed)
}

func TestSubscriptionExtraCreditsReceiptIdentity(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Migrator().DropTable(&SubscriptionPreConsumeRecord{}))
	require.NoError(t, DB.Exec(`CREATE TABLE subscription_pre_consume_records (id INTEGER PRIMARY KEY AUTOINCREMENT, request_id TEXT COLLATE NOCASE UNIQUE, user_id INTEGER, user_subscription_id INTEGER, pre_consumed BIGINT, status TEXT, allocations TEXT, reserved_total BIGINT, request_group TEXT, funding_mode TEXT, durable NUMERIC, billing_log TEXT, billing_log_pending NUMERIC, created_at BIGINT, updated_at BIGINT)`).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Migrator().DropTable(&SubscriptionPreConsumeRecord{}))
		require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	})
	r := SubscriptionPreConsumeRecord{RequestId: "case-sensitive-receipt", UserId: 991, UserSubscriptionId: 992, Status: "settled", Allocations: "[]", ReservedTotal: 10}
	require.NoError(t, DB.Create(&r).Error)
	task := Task{UserId: 991, Quota: 10, PrivateData: TaskPrivateData{SubscriptionId: 992, SubscriptionRequestID: "CASE-sensitive-receipt"}}
	require.NoError(t, DB.Create(&task).Error)
	_, err := SettleSubscriptionTaskQuota(&task, 10)
	require.Error(t, err, "a case-insensitive SQL match must not replace a missing exact funding receipt")
	for _, operation := range []struct {
		name string
		call func() error
	}{
		{"finish", func() error { return FinishSubscriptionRequest(task.PrivateData.SubscriptionRequestID) }},
		{"settle", func() error { return SettleSubscriptionRequestQuota(task.PrivateData.SubscriptionRequestID, 10) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			require.Error(t, operation.call(), "synchronous receipt keys must also match exactly")
		})
	}
}

func TestSubscriptionExtraCreditsAdjustmentWindows(t *testing.T) {
	for _, scenario := range []string{"short-window-reset", "regular-restored", "expired-regular-restored", "no-cross-source-without-extras", "single-alternate-without-extras", "non-first-regular-restored"} {
		t.Run(scenario, func(t *testing.T) {
			truncateTables(t)
			require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
			user := createReserveTestUser(t, 100)
			plan := SubscriptionPlan{Title: scenario, SubscriptionGroup: "vip", TotalAmount: 20}
			require.NoError(t, DB.Create(&plan).Error)
			now := GetDBTimestamp()
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 20000, EndTime: now + 3600, AmountTotal: 20, LastResetTime: now - 20000, ExtraCreditsTotal: 1, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
			if scenario == "short-window-reset" {
				sub.FiveHourLimit = 5
				sub.FiveHourLastResetTime = now - 18001
				sub.FiveHourNextResetTime = now + 100
			}
			if scenario == "regular-restored" || scenario == "expired-regular-restored" {
				sub.AmountUsed = 20
			}
			if scenario == "no-cross-source-without-extras" || scenario == "single-alternate-without-extras" {
				sub.AmountTotal = 5
			}
			require.NoError(t, DB.Create(&sub).Error)
			id := "adjustment-" + scenario
			amount := int64(5)
			if scenario == "regular-restored" || scenario == "expired-regular-restored" {
				amount = 1
			}
			var other UserSubscription
			if scenario == "non-first-regular-restored" {
				sub.AmountUsed = 20
				require.NoError(t, DB.Save(&sub).Error)
			}
			if scenario == "no-cross-source-without-extras" || scenario == "single-alternate-without-extras" || scenario == "non-first-regular-restored" {
				other = sub
				other.Id = 0
				other.ExtraCreditsTotal = 0
				if scenario == "non-first-regular-restored" {
					other.ExtraCreditsTotal = 2
				}
				require.NoError(t, DB.Create(&other).Error)
				amount = 10
				if scenario == "non-first-regular-restored" {
					amount = 2
				}
			}
			_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, amount, "vip")
			require.NoError(t, err)
			switch scenario {
			case "short-window-reset":
				require.NoError(t, DB.Model(&sub).UpdateColumn("five_hour_next_reset_time", now-1).Error)
				require.NoError(t, AdjustSubscriptionRequestQuota(id, 3))
				require.NoError(t, DB.First(&sub, sub.Id).Error)
				require.EqualValues(t, 3, sub.FiveHourUsed)
				require.Zero(t, sub.ExtraCreditsUsed, "fresh ordinary window precedes paid extras")
			case "regular-restored", "expired-regular-restored":
				updates := map[string]any{"amount_used": 15}
				if scenario == "expired-regular-restored" {
					updates["status"] = "expired"
					updates["end_time"] = now - 1
				}
				require.NoError(t, DB.Model(&sub).Updates(updates).Error)
				require.NoError(t, AdjustSubscriptionRequestQuota(id, 3), "the original funding row can use newly available ordinary quota")
				require.NoError(t, DB.First(&sub, sub.Id).Error)
				require.EqualValues(t, 18, sub.AmountUsed)
			case "non-first-regular-restored":
				require.NoError(t, DB.Model(&other).UpdateColumn("amount_used", 17).Error)
				require.NoError(t, AdjustSubscriptionRequestQuota(id, 3))
				require.NoError(t, DB.First(&other, other.Id).Error)
				require.EqualValues(t, 20, other.AmountUsed)
			case "no-cross-source-without-extras", "single-alternate-without-extras":
				require.NoError(t, DB.Model(&sub).Updates(map[string]any{"amount_used": 3, "extra_credits_used": 1}).Error)
				require.NoError(t, DB.Model(&other).UpdateColumn("amount_used", 3).Error)
				if scenario == "single-alternate-without-extras" {
					require.NoError(t, DB.Model(&other).UpdateColumn("amount_total", 10).Error)
				}
				require.Error(t, AdjustSubscriptionRequestQuota(id, 3), "without spendable extras a single ordinary source must cover the addition")
			}
		})
	}
}

func TestSubscriptionExtraCreditsLegacyRefundPreservesNewReservation(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Legacy refund", SubscriptionGroup: "vip", TotalAmount: 20}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 20, AmountUsed: 7}
	require.NoError(t, DB.Create(&sub).Error)
	old := SubscriptionPreConsumeRecord{RequestId: "old-allocation-refund", UserId: user.Id, UserSubscriptionId: sub.Id, PreConsumed: 7, Status: "consumed"}
	require.NoError(t, DB.Create(&old).Error)
	_, err := PreConsumeUserSubscriptionForGroup("new-allocation-reserve", user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&sub).Updates(map[string]any{"usage_reset_version": 1, "five_hour_reset_version": 2, "weekly_reset_version": 3, "monthly_reset_version": 4, "five_hour_used": 7, "weekly_used": 7, "monthly_used": 7}).Error)
	require.NoError(t, AdjustSubscriptionRequestQuota(old.RequestId, -3))
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 9, sub.FundingReserved, "converting an old partial refund tracks its remaining four alongside the new five")
	var partial SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", old.RequestId).First(&partial).Error)
	require.True(t, partial.Durable, "a converted legacy receipt must survive cleanup until remaining allocations release")
	require.NoError(t, RefundSubscriptionPreConsume(old.RequestId))
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 5, sub.FundingReserved, "old receipt never contributed to the new reservation count")
	require.EqualValues(t, 5, sub.AmountUsed)
	require.Equal(t, []int64{0, 0, 0}, []int64{sub.FiveHourUsed, sub.WeeklyUsed, sub.MonthlyUsed})
	old = SubscriptionPreConsumeRecord{RequestId: "old-maximum-reservation", UserId: user.Id, UserSubscriptionId: sub.Id, PreConsumed: math.MaxInt64, Status: "consumed"}
	require.NoError(t, DB.Create(&old).Error)
	require.Error(t, AdjustSubscriptionRequestQuota(old.RequestId, -1), "legacy conversion must reject overflowing refundable totals")
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 5, sub.FundingReserved, "overflow cannot erase another pending reservation")
}

func TestSubscriptionExtraCreditsAllocatorHeadroom(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 10)
	plan := SubscriptionPlan{Title: "Headroom", SubscriptionGroup: "vip"}
	require.NoError(t, DB.Create(&plan).Error)
	first := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: GetDBTimestamp(), EndTime: GetDBTimestamp() + 3600, FiveHourLimit: math.MaxInt64, WeeklyLimit: math.MaxInt64, MonthlyLimit: math.MaxInt64}
	extra := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 5, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, DB.Create(&first).Error)
	_, err := PreConsumeUserSubscriptionForGroup("pending-headroom", user.Id, "model", 0, math.MaxInt64-1, "vip")
	require.NoError(t, err)
	require.NoError(t, DB.Create(&extra).Error)
	_, err = PreConsumeUserSubscriptionForGroup("funding-headroom", user.Id, "model", 0, 3, "vip")
	require.NoError(t, err)
	require.NoError(t, DB.First(&first, first.Id).Error)
	require.NoError(t, DB.First(&extra, extra.Id).Error)
	require.EqualValues(t, math.MaxInt64, first.FundingReserved)
	for _, used := range []int64{first.AmountUsed, first.FiveHourUsed, first.WeeklyUsed, first.MonthlyUsed} {
		require.EqualValues(t, math.MaxInt64, used)
	}
	require.EqualValues(t, 2, extra.ExtraCreditsUsed)
	require.NoError(t, SettleSubscriptionRequestQuota("pending-headroom", math.MaxInt64-1))
	require.NoError(t, SettleSubscriptionRequestQuota("funding-headroom", 3))
	require.NoError(t, DB.First(&first, first.Id).Error)
	require.Zero(t, first.FundingReserved)
	require.EqualValues(t, math.MaxInt64, first.AmountUsed)
	second := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: first.EndTime + 1, AmountTotal: 1}
	require.NoError(t, DB.Create(&second).Error)

	result, err := PreConsumeUserSubscriptionForGroup("used-headroom", user.Id, "model", 0, 1, "vip")
	require.NoError(t, err)
	require.Equal(t, second.Id, result.UserSubscriptionId)
}

func TestSubscriptionExtraCreditsUnreservableExtras(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 10)
	plan := SubscriptionPlan{Title: "Single source", SubscriptionGroup: "vip", AllowExtraCredits: true, ExtraCreditsRate: 1}
	require.NoError(t, DB.Create(&plan).Error)
	row := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 1}
	extra := row
	extra.AmountTotal = 0
	require.NoError(t, DB.Create(&extra).Error)
	_, err := PreConsumeUserSubscriptionForGroup("all-reservation-space", user.Id, "model", 0, math.MaxInt64, "vip")
	require.NoError(t, err)
	_, err = PurchaseSubscriptionExtraCredits(user.Id, extra.Id, 1, "extra-no-reservation-space", 1)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&[]UserSubscription{row, row}).Error)
	_, err = PreConsumeUserSubscriptionForGroup("must-not-combine", user.Id, "model", 0, 2, "vip")
	require.Error(t, err)
	require.NoError(t, DB.First(&extra, extra.Id).Error)
	require.EqualValues(t, math.MaxInt64, extra.FundingReserved)
	require.EqualValues(t, math.MaxInt64, extra.AmountUsed)
	require.EqualValues(t, 1, extra.ExtraCreditsTotal)
	require.Zero(t, extra.ExtraCreditsUsed)
	var count int64
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", "must-not-combine").Count(&count).Error)
	require.Zero(t, count)
	// Released receipts already charged AmountUsed, but did not contribute to FundingReserved.
	require.NoError(t, RefundSubscriptionPreConsume("all-reservation-space"))
	old := SubscriptionPreConsumeRecord{RequestId: "released-last-space", UserId: user.Id, UserSubscriptionId: extra.Id, RequestGroup: "vip", PreConsumed: 1, Status: "consumed"}
	require.NoError(t, DB.Create(&old).Error)
	require.NoError(t, DB.Model(&extra).UpdateColumn("amount_used", 1).Error)
	_, err = PreConsumeUserSubscriptionForGroup("modern-pending-space", user.Id, "model", 0, math.MaxInt64-1, "vip")
	require.NoError(t, err)
	require.Error(t, AdjustSubscriptionRequestQuota(old.RequestId, 2), "account for the converted reservation before allowing other ordinary sources")
	require.NoError(t, DB.First(&extra, extra.Id).Error)
	require.EqualValues(t, math.MaxInt64-1, extra.FundingReserved)
}

func TestSubscriptionExtraCreditsScheduleInitialization(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{Title: "New reset schedule", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 20, QuotaResetPeriod: SubscriptionResetNever}
	user, sub := createSubscriptionUsageLimitFixture(t, plan)
	_, err := PreConsumeUserSubscription("before-schedule", user.Id, "model", 0, 5)
	require.NoError(t, err)
	require.NoError(t, DB.Model(plan).UpdateColumn("quota_reset_period", SubscriptionResetDaily).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	_, err = PreConsumeUserSubscription("after-schedule", user.Id, "model", 0, 1)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("before-schedule"))
	require.NoError(t, DB.First(sub, sub.Id).Error)
	require.EqualValues(t, 1, sub.AmountUsed)
	require.EqualValues(t, 1, sub.FundingReserved)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		plan.QuotaResetPeriod = SubscriptionResetDaily
		return maybeResetUserSubscriptionWithPlanTx(tx, sub, plan, sub.NextResetTime)
	}))
	_, err = PreConsumeUserSubscription("after-real-reset", user.Id, "model", 0, 2)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("after-schedule"))
	require.NoError(t, DB.First(sub, sub.Id).Error)
	require.EqualValues(t, 2, sub.AmountUsed)
}

func TestSubscriptionExtraCreditsDeletedOwnerSettlement(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 10)
	plan := SubscriptionPlan{Title: "Deleted owner", SubscriptionGroup: "vip"}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600, AmountTotal: 2, ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, DB.Create(&sub).Error)
	r, err := PreConsumeUserSubscriptionForGroup("submitted-before-delete", user.Id, "model", 0, 1, "vip")
	require.NoError(t, err)
	task := Task{UserId: user.Id, Quota: 1, PrivateData: TaskPrivateData{SubscriptionId: sub.Id, SubscriptionRequestID: "submitted-before-delete", SubscriptionReceiptId: r.ReceiptId}}
	require.NoError(t, DB.Create(&task).Error)
	require.NoError(t, DB.Delete(&User{}, user.Id).Error)
	_, err = PreConsumeUserSubscriptionForGroup("new-after-delete", user.Id, "model", 0, 1, "vip")
	require.Error(t, err)
	err = AdjustSubscriptionRequestQuota("submitted-before-delete", 2)
	require.Error(t, err, "deleted account cannot authorize a new extra-credit source")
	_, err = SettleSubscriptionTaskQuota(&task, 2)
	require.NoError(t, err)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 2, sub.AmountUsed)
	require.Zero(t, sub.ExtraCreditsUsed)
	require.Zero(t, sub.FundingReserved)
}

func TestSubscriptionExtraCreditsPurchaseLogReplay(t *testing.T) {
	truncateTables(t)
	user := createReserveTestUser(t, 10)
	plan := SubscriptionPlan{Title: "Replay", SubscriptionGroup: "vip", AllowExtraCredits: true, ExtraCreditsRate: 2}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: GetDBTimestamp() + 3600}
	require.NoError(t, DB.Create(&sub).Error)
	require.NoError(t, DB.Exec("CREATE TRIGGER fail_purchase_log BEFORE INSERT ON logs BEGIN SELECT RAISE(ABORT, 'log offline'); END").Error)
	t.Cleanup(func() { DB.Exec("DROP TRIGGER IF EXISTS fail_purchase_log") })
	for i := range 3 {
		_, err := PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 3, "purchase-log-replay", 2)
		require.NoError(t, err)
		if i == 0 {
			require.NoError(t, DB.Exec("DROP TRIGGER fail_purchase_log").Error)
		}
	}
	var entries []Log
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, LogTypeTopup).Find(&entries).Error)
	require.Len(t, entries, 1)
	require.NotEmpty(t, entries[0].SubscriptionEventId)
	require.Equal(t, 7, getUserQuotaFromDB(t, user.Id))
}

func TestSubscriptionExtraCreditsFailedFirstLog(t *testing.T) {
	truncateTables(t)
	raw, err := common.Marshal([]subscriptionTaskBillingEvent{
		{EventId: "first-event", Log: Log{RequestId: "first-log", Type: LogTypeConsume}, Export: &QuotaData{Quota: 4}},
		{EventId: "second-event", Log: Log{RequestId: "second-log", Type: LogTypeRefund, Ip: "192.0.2.42"}},
	})
	require.NoError(t, err)
	r := SubscriptionPreConsumeRecord{RequestId: "two-events", BillingLogPending: true, BillingLog: string(raw)}
	require.NoError(t, DB.Create(&r).Error)
	require.NoError(t, DB.Exec("CREATE TRIGGER fail_first_log BEFORE INSERT ON logs WHEN NEW.request_id='first-log' BEGIN SELECT RAISE(ABORT, 'invalid entry'); END").Error)
	t.Cleanup(func() { DB.Exec("DROP TRIGGER IF EXISTS fail_first_log") })
	require.Error(t, FlushSubscriptionTaskBillingLogs())
	var ids []string
	require.NoError(t, DB.Model(&Log{}).Pluck("request_id", &ids).Error)
	require.Equal(t, []string{"second-log"}, ids)
	require.NoError(t, DB.First(&r, r.Id).Error)
	require.NotContains(t, r.BillingLog, "192.0.2.42")
	require.NotContains(t, r.BillingLog, "subscription_export")
	require.NoError(t, DB.Exec("DROP TRIGGER fail_first_log").Error)
	require.NoError(t, FlushSubscriptionTaskBillingLogs())
	var exports []int
	require.NoError(t, DB.Model(&QuotaData{}).Pluck("quota", &exports).Error)
	require.Equal(t, []int{4}, exports)
}

func TestSubscriptionExtraCreditsUsesReservedWalletBalance(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true
	user := createReserveTestUser(t, 100)
	plan := SubscriptionPlan{Title: "Redis extras", SubscriptionGroup: "vip", AllowExtraCredits: true, ExtraCreditsRate: 2}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: common.GetTimestamp() + 1000}
	require.NoError(t, DB.Create(&sub).Error)
	reserved, err := TryReserveUserQuota(user.Id, 95)
	require.NoError(t, err)
	require.True(t, reserved)
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 10, "redis-extra-rejected-001", 2)
	require.Error(t, err)
	require.NoError(t, common.RDB.Del(context.Background(), getUserCacheKey(user.Id)).Err())
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 10, "redis-evicted-reservation", 2)
	require.Error(t, err, "cache eviction must not restore already reserved wallet balance")
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 4, "redis-extra-purchase-001", 2)
	require.NoError(t, err)
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 4, "redis-extra-purchase-001", 2)
	require.NoError(t, err)
	cache, err := GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 1, cache.Quota)
	require.NoError(t, common.RDB.HSet(t.Context(), getUserCacheKey(user.Id), "Quota", 10).Err())
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 5, "stale-high-wallet-purchase", 2)
	require.Error(t, err)
	cache, err = GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 1, cache.Quota, "SQL rejected debit must invalidate the stale high cache")
	batchUpdate()
	require.Equal(t, 1, getUserQuotaFromDB(t, user.Id))
	// Reproduce a worker exit after the Redis hold but before SQL commit.
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).UpdateColumn("quota", 10).Error)
	_, err = cacheApplyUserQuotaDelta(user.Id, 9)
	require.NoError(t, err)
	result, err := reserveExtraCreditWallet(user.Id, 3, "redis-orphaned-hold-001")
	require.NoError(t, err)
	require.Equal(t, cacheQuotaOK, result)
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 3, "redis-orphaned-hold-001", 2)
	require.NoError(t, err)
	cache, err = GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 7, cache.Quota)
	// Failed callers must reconcile against SQL, never refund a committed retry.
	require.NoError(t, reconcileExtraCreditWallet(user.Id, "redis-orphaned-hold-001"))
	cache, err = GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 7, cache.Quota)
	result, err = reserveExtraCreditWallet(user.Id, 2, "redis-abandoned-hold-001")
	require.NoError(t, err)
	require.Equal(t, cacheQuotaOK, result)
	_, err = common.RDB.ZAdd(context.Background(), "extra-credit-wallet-holds", &redis.Z{Score: 1, Member: fmt.Sprintf("%d:redis-abandoned-hold-001", user.Id)}).Result()
	require.NoError(t, err)
	require.NoError(t, common.RDB.ZAdd(t.Context(), "extra-credit-wallet-holds", &redis.Z{Score: 0, Member: "bad"}, &redis.Z{Score: 0, Member: fmt.Sprintf("%d:", user.Id)}).Err())
	require.Error(t, ReconcileExtraCreditWalletHolds())
	require.Zero(t, common.RDB.ZCard(t.Context(), "extra-credit-wallet-holds").Val())
	require.NoError(t, ReconcileExtraCreditWalletHolds())
	cache, err = GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 7, cache.Quota, "an abandoned purchase must release its hold without a client retry")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).UpdateColumn("quota", 1).Error)
	result, err = reserveExtraCreditWallet(user.Id, 3, "stale-crashed-purchase")
	require.NoError(t, err)
	require.Equal(t, cacheQuotaOK, result)
	// The SQL debit would reject; reproduce the crash before caller compensation.
	require.NoError(t, reconcileExtraCreditWallet(user.Id, "stale-crashed-purchase"))
	cache, err = GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 1, cache.Quota, "orphan reconciliation cannot restore a balance already disproved by SQL")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).UpdateColumn("quota", 10).Error)
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 5, "stale-low-wallet-purchase", 2)
	require.NoError(t, err, "SQL funded purchase recovers stale low cache after a missed credit update")
	const hook = "test:evict_before_wallet_hold"
	evicted := false
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(hook, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*UserSubscription); ok && !evicted {
			evicted = true
			require.NoError(t, common.RDB.Del(t.Context(), getUserCacheKey(user.Id)).Err())
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(hook) })
	_, err = PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 1, "cache-miss-before-hold", 2)
	require.True(t, evicted)
	require.NoError(t, err)
	cache, err = GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 4, cache.Quota)
	require.Equal(t, 4, getUserQuotaFromDB(t, user.Id))
}

func TestRefundExpiredExtraCredits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reserved  int64
		wantQuota int
		wantTotal int64
	}{
		{"refunds unspent credits at the purchase rate", 0, 17, 6},
		{"waits for an in-flight reservation", 5, 10, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			user := createReserveTestUser(t, 10)
			plan := SubscriptionPlan{Title: "Refund", SubscriptionGroup: "vip", AllowExtraCredits: true, ExtraCreditsRate: 2}
			require.NoError(t, DB.Create(&plan).Error)
			sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", EndTime: GetDBTimestamp() - 1, ExtraCreditsTotal: 20, ExtraCreditsUsed: 6, FundingReserved: tc.reserved, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
			require.NoError(t, DB.Create(&sub).Error)
			receipt := SubscriptionExtraCreditPurchase{UserId: user.Id, SubscriptionId: sub.Id, RequestId: "refund-receipt-0000001", WalletQuota: 10, Credits: 20, Rate: 2}
			require.NoError(t, DB.Create(&receipt).Error)
			for range 2 { // the second pass proves the refund is idempotent
				_, err := RefundExpiredExtraCredits(10)
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantQuota, getUserQuotaFromDB(t, user.Id))
			require.NoError(t, DB.First(&sub, sub.Id).Error)
			require.Equal(t, tc.wantTotal, sub.ExtraCreditsTotal)
		})
	}
}
