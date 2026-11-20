package model

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createReserveTestUser(t *testing.T, quota int) User {
	t.Helper()
	user := User{
		Username:    "reserve-user-" + common.GetRandomString(6),
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		Quota:       quota,
		AffCode:     "reserve-aff-" + common.GetRandomString(8),
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createReserveTestToken(t *testing.T, remainQuota int) Token {
	t.Helper()
	token := Token{
		UserId:      1,
		Key:         "reserve-token-" + common.GetRandomString(8),
		Name:        "reserve-test",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: remainQuota,
	}
	require.NoError(t, token.Insert())
	return token
}

func getUserQuotaFromDB(t *testing.T, id int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").First(&user, id).Error)
	return user.Quota
}

func getTokenFromDB(t *testing.T, id int) Token {
	t.Helper()
	var token Token
	require.NoError(t, DB.First(&token, id).Error)
	return token
}

func resetBatchUpdateTestState(t *testing.T) {
	t.Helper()
	oldBatchEnabled := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	for i := range BatchUpdateTypeCount {
		batchUpdateLocks[i].Lock()
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
	}
	t.Cleanup(func() {
		common.BatchUpdateEnabled = oldBatchEnabled
		for i := range BatchUpdateTypeCount {
			batchUpdateLocks[i].Lock()
			batchUpdateStores[i] = make(map[int]int)
			batchUpdateLocks[i].Unlock()
		}
	})
}

func TestTryReserveQuotaWithoutRedis(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)

	user := createReserveTestUser(t, 100)
	reserved, err := TryReserveUserQuota(user.Id, 60)
	require.NoError(t, err)
	assert.True(t, reserved)
	assert.Equal(t, 40, getUserQuotaFromDB(t, user.Id))

	reserved, err = TryReserveUserQuota(user.Id, 41)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 40, getUserQuotaFromDB(t, user.Id))

	token := createReserveTestToken(t, 80)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 25, false)
	require.NoError(t, err)
	assert.True(t, reserved)
	reloaded := getTokenFromDB(t, token.Id)
	assert.Equal(t, 55, reloaded.RemainQuota)
	assert.Equal(t, 25, reloaded.UsedQuota)

	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 56, false)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 55, getTokenFromDB(t, token.Id).RemainQuota)
}

func TestRedisBatchReserveNeverFallsBackToStaleDatabaseBalance(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true

	user := createReserveTestUser(t, 10)
	reserved, err := TryReserveUserQuota(user.Id, 8)
	require.NoError(t, err)
	assert.True(t, reserved)
	assert.Equal(t, 2, getUserQuotaFromDB(t, user.Id), "spendable money persists before reporting a successful reservation")

	reserved, err = TryReserveUserQuota(user.Id, 3)
	require.NoError(t, err)
	assert.False(t, reserved, "stale DB balance must not authorize a second spend")
	cachedUser, err := GetUserCache(user.Id)
	require.NoError(t, err)
	assert.Equal(t, 2, cachedUser.Quota)

	token := createReserveTestToken(t, 9)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 7, false)
	require.NoError(t, err)
	assert.True(t, reserved)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 3, false)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 2, getTokenFromDB(t, token.Id).RemainQuota)

	batchUpdate()
	assert.Equal(t, 2, getUserQuotaFromDB(t, user.Id))
	reloadedToken := getTokenFromDB(t, token.Id)
	assert.Equal(t, 2, reloadedToken.RemainQuota)
	assert.Equal(t, 7, reloadedToken.UsedQuota)
}

// Interleave a second operation after the real SQL commit but before the
// first caller can publish its cache change, independently of Redis commands.
type walletCommitPool struct {
	*sql.DB
	afterCommit func()
}

func (p walletCommitPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &walletCommitTx{tx, p.afterCommit}, nil
}

type walletCommitTx struct {
	*sql.Tx
	afterCommit func()
}

func (tx *walletCommitTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	tx.afterCommit()
	return nil
}

func TestExplicitWalletMutation(t *testing.T) {
	// A single FIFO worker makes all earlier asynchronous deltas observable at the SQL boundary.
	gopool.SetCap(1)
	t.Cleanup(func() { gopool.SetCap(math.MaxInt32) })
	drain := func() {
		done := make(chan struct{})
		gopool.Go(func() { close(done) })
		<-done
	}
	for _, engine := range []struct{ name, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(engine.name, func(t *testing.T) {
			if engine.env != "" {
				if os.Getenv(engine.env) == "" {
					t.Skip("isolated test database not configured")
				}
				db, dialect, err := chooseDB(engine.env, false)
				require.NoError(t, err)
				require.Equal(t, engine.name, string(dialect))
				conn, err := db.DB()
				require.NoError(t, err)
				oldDB, oldLog := DB, LOG_DB
				oldDialect, oldLogDialect := common.MainDatabaseType(), common.LogDatabaseType()
				DB, LOG_DB = db, db
				common.SetDatabaseTypes(dialect, dialect)
				initCol()
				t.Cleanup(func() {
					DB, LOG_DB = oldDB, oldLog
					common.SetDatabaseTypes(oldDialect, oldLogDialect)
					initCol()
					assert.NoError(t, conn.Close())
				})
				require.NoError(t, db.AutoMigrate(&User{}, &Log{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}))
			} else {
				truncateTables(t)
			}
			versionQuery := "SELECT version()"
			if engine.name == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, DB.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database version: %s", version)
			for _, tc := range []struct {
				name      string
				committed int
			}{{"purchase_after_refill", 6}, {"adjust_after_refill", 14}} {
				t.Run(tc.name, func(t *testing.T) {
					useUserCacheMiniRedis(t)
					oldUnit := common.QuotaPerUnit
					common.QuotaPerUnit = 1
					t.Cleanup(func() { common.QuotaPerUnit = oldUnit })
					user := createReserveTestUser(t, 10)
					require.NoError(t, populateUserCache(user))
					plan := SubscriptionPlan{Title: "Wallet purchase", Enabled: true, PriceAmount: 4, DurationUnit: SubscriptionDurationMonth, DurationValue: 1}
					require.NoError(t, DB.Create(&plan).Error)
					conn, err := DB.DB()
					require.NoError(t, err)
					pool := DB.Statement.ConnPool
					t.Cleanup(func() { DB.Statement.ConnPool = pool })
					DB.Statement.ConnPool = walletCommitPool{conn, func() {
						DB.Statement.ConnPool = pool
						require.Equal(t, tc.committed, getUserQuotaFromDB(t, user.Id))
						require.NoError(t, IncreaseUserQuota(user.Id, 1, true))
						cached, err := GetUserCache(user.Id)
						require.NoError(t, err)
						require.Equal(t, tc.committed+1, cached.Quota)
					}}
					if tc.name == "purchase_after_refill" {
						require.NoError(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id, 1, false))
					} else {
						_, err := AdjustUserQuota(user.Id, common.RoleRootUser, "add", 4)
						require.NoError(t, err)
					}
					cached, err := GetUserCache(user.Id)
					require.NoError(t, err)
					assert.Equal(t, tc.committed+1, cached.Quota)
					reserved, err := TryReserveUserQuota(user.Id, 5)
					require.NoError(t, err)
					assert.True(t, reserved)
					assert.Equal(t, tc.committed+1-5, getUserQuotaFromDB(t, user.Id))
				})
			}
			t.Run("reward_logs", func(t *testing.T) {
				server := useUserCacheMiniRedis(t)
				payment := operation_setting.GetPaymentSetting()
				confirmed, terms := payment.ComplianceConfirmed, payment.ComplianceTermsVersion
				newUser, invitee, inviter := common.QuotaForNewUser, common.QuotaForInvitee, common.QuotaForInviter
				t.Cleanup(func() {
					payment.ComplianceConfirmed, payment.ComplianceTermsVersion = confirmed, terms
					common.QuotaForNewUser, common.QuotaForInvitee, common.QuotaForInviter = newUser, invitee, inviter
				})
				payment.ComplianceConfirmed, payment.ComplianceTermsVersion = true, operation_setting.CurrentComplianceTermsVersion
				common.QuotaForNewUser, common.QuotaForInvitee, common.QuotaForInviter = 0, 2, 3
				for _, failCache := range []bool{true, false} {
					user, referrer := createReserveTestUser(t, 10), createReserveTestUser(t, 0)
					user.RegistrationRewardEligible = true
					wantQuota, wantAff := 10, 3
					paidID, content := referrer.Id, "Bonus for inviting a user "+logger.FormatQuota(3)
					if failCache {
						server.SetError("ERR reward cache unavailable")
					} else {
						require.NoError(t, DB.Delete(&referrer).Error)
						wantQuota, wantAff = 12, 0
						paidID, content = user.Id, "Invitation code bonus "+logger.FormatQuota(2)
					}
					user.grantRegistrationRewards(referrer.Id)
					drain()
					server.SetError("")
					assert.Equal(t, wantQuota, getUserQuotaFromDB(t, user.Id))
					require.NoError(t, DB.Unscoped().First(&referrer, referrer.Id).Error)
					assert.Equal(t, wantAff, referrer.AffQuota)
					var logs []Log
					require.NoError(t, LOG_DB.Where("user_id IN ? AND type = ?", []int{user.Id, referrer.Id}, LogTypeSystem).Find(&logs).Error)
					require.Len(t, logs, 1)
					assert.Equal(t, paidID, logs[0].UserId)
					assert.Equal(t, content, logs[0].Content)
				}
			})
			for _, tc := range []struct {
				name   string
				mutate func(int, int, bool) error
				want   int
			}{{"debit", DecreaseUserQuota, -2}, {"credit", IncreaseUserQuota, 22}} {
				t.Run(tc.name, func(t *testing.T) {
					server := useUserCacheMiniRedis(t)
					user := createReserveTestUser(t, 10)
					t.Cleanup(func() { assert.NoError(t, DB.Unscoped().Delete(&user).Error) })
					require.NoError(t, populateUserCache(user))
					key := getUserCacheKey(user.Id)

					sqlFailure := errors.New("injected wallet update failure")
					rejectUpdate := true
					const callback = "test:wallet_before_sql_update"
					require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
						drain()
						assert.Equal(t, "10", server.HGet(key, "Quota"), "uncommitted wallet delta reached Redis before SQL")
						if rejectUpdate {
							tx.AddError(sqlFailure)
						}
					}))
					t.Cleanup(func() { assert.NoError(t, DB.Callback().Update().Remove(callback)) })

					require.ErrorIs(t, tc.mutate(user.Id, 12, false), sqlFailure)
					assert.Equal(t, "10", server.HGet(key, "Quota"))
					assert.Equal(t, 10, getUserQuotaFromDB(t, user.Id))
					rejectUpdate = false
					server.SetError("ERR injected cache failure")
					err := tc.mutate(user.Id, 12, false)
					drain()
					server.SetError("")
					assert.ErrorContains(t, err, "injected cache failure")
					assert.Equal(t, 10, getUserQuotaFromDB(t, user.Id), "failed invalidation must roll back SQL")
					assert.Equal(t, "10", server.HGet(key, "Quota"))

					require.NoError(t, tc.mutate(user.Id, 12, false))
					drain()
					assert.False(t, server.Exists(key), "a committed mutation invalidates the old snapshot")
					cached, err := GetUserCache(user.Id)
					require.NoError(t, err)
					assert.Equal(t, tc.want, cached.Quota)
					assert.Equal(t, tc.want, getUserQuotaFromDB(t, user.Id))
				})
			}
		})
	}
}

func TestBatchUpdateAccumulatesTwoMaximumRequestCharges(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	common.BatchUpdateEnabled = true

	user := createReserveTestUser(t, common.MaxQuota*2+100)
	require.NoError(t, DecreaseUserQuota(user.Id, common.MaxQuota, false))
	require.NoError(t, DecreaseUserQuota(user.Id, common.MaxQuota, false))

	batchUpdate()
	assert.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
}

func TestBatchUpdateAccumulatorSaturatesOverflow(t *testing.T) {
	resetBatchUpdateTestState(t)

	addNewRecord(BatchUpdateTypeUserQuota, 1, math.MaxInt)
	addNewRecord(BatchUpdateTypeUserQuota, 1, 1)
	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	assert.Equal(t, math.MaxInt, batchUpdateStores[BatchUpdateTypeUserQuota][1])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()

	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	batchUpdateStores[BatchUpdateTypeUserQuota] = make(map[int]int)
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
	addNewRecord(BatchUpdateTypeUserQuota, 1, math.MinInt)
	addNewRecord(BatchUpdateTypeUserQuota, 1, -1)
	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	assert.Equal(t, math.MinInt, batchUpdateStores[BatchUpdateTypeUserQuota][1])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
}

func TestReserveFallsBackToDatabaseWhenRedisIsUnavailable(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	server := useUserCacheMiniRedis(t)

	user := createReserveTestUser(t, 20)
	require.NoError(t, populateUserCache(user))
	server.Close()

	// Redis 故障时降级为数据库条件更新：服务保持可用且不会超扣。
	reserved, err := TryReserveUserQuota(user.Id, 5)
	require.NoError(t, err)
	assert.True(t, reserved)
	assert.Equal(t, 15, getUserQuotaFromDB(t, user.Id))

	reserved, err = TryReserveUserQuota(user.Id, 16)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 15, getUserQuotaFromDB(t, user.Id))
}

func TestSynchronousReserveCompensatesCacheWhenPersistenceFails(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)

	user := createReserveTestUser(t, 10)
	require.NoError(t, populateUserCache(user))
	require.NoError(t, DB.Delete(&user).Error)

	reserved, err := TryReserveUserQuota(user.Id, 6)
	assert.False(t, reserved)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	cached, cacheErr := cacheGetUserBase(user.Id)
	require.NoError(t, cacheErr)
	assert.Equal(t, 10, cached.Quota)

	token := createReserveTestToken(t, 12)
	_, err = GetTokenByKey(token.Key, true)
	require.NoError(t, err)
	require.NoError(t, DB.Delete(&token).Error)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 7, false)
	assert.False(t, reserved)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	cachedToken, cacheErr := cacheGetTokenByKey(token.Key)
	require.NoError(t, cacheErr)
	assert.Equal(t, 12, cachedToken.RemainQuota)
	assert.Zero(t, cachedToken.UsedQuota)
}

func TestTokenReserveTreatsStaleHighCacheAsInsufficientQuota(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)

	token := createReserveTestToken(t, 10)
	_, err := GetTokenByKey(token.Key, true)
	require.NoError(t, err)
	// 缓存被逐出后按旧 SQL 余额重建，随后先前的预扣才落库：缓存余额高于 SQL。
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).
		Updates(map[string]any{"remain_quota": 2, "used_quota": 8}).Error)

	reserved, err := TryReserveTokenQuota(token.Id, token.Key, 5, false)
	require.NoError(t, err, "SQL guard rejection is an insufficient-quota result")
	assert.False(t, reserved)
	_, cacheErr := cacheGetTokenByKey(token.Key)
	assert.Error(t, cacheErr, "the stale high balance must not be restored")

	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 5, false)
	require.NoError(t, err)
	assert.False(t, reserved)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 2, false)
	require.NoError(t, err)
	assert.True(t, reserved)
	reloaded := getTokenFromDB(t, token.Id)
	assert.Zero(t, reloaded.RemainQuota)
	assert.Equal(t, 10, reloaded.UsedQuota)
}

// topUpAfterRejectedReserve 在预扣条件更新零行提交之后、TryReserve 重查之前同步执行一次充值，
// 确定性复现“并发充值使重查余额已足够”的竞态，无需 sleep。
func topUpAfterRejectedReserve(t *testing.T, table string, topUp func()) *bool {
	t.Helper()
	const name = "test:topup_after_rejected_reserve"
	fired := false
	require.NoError(t, DB.Callback().Update().After("gorm:commit_or_rollback_transaction").Register(name, func(tx *gorm.DB) {
		if fired || tx.Error != nil || tx.RowsAffected != 0 || tx.Statement.Table != table {
			return
		}
		fired = true
		topUp()
	}))
	t.Cleanup(func() { _ = DB.Callback().Update().Remove(name) })
	return &fired
}

func TestReserveRejectionIgnoresConcurrentTopUpBeforeRelookup(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		truncateTables(t)
		resetBatchUpdateTestState(t)
		useUserCacheMiniRedis(t)

		user := createReserveTestUser(t, 10)
		require.NoError(t, populateUserCache(user))
		require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", 2).Error)
		fired := topUpAfterRejectedReserve(t, "users", func() {
			require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", 100).Error)
		})

		reserved, err := TryReserveUserQuota(user.Id, 5)
		require.True(t, *fired, "the SQL guard must reject before the top-up lands")
		require.NoError(t, err, "an existing user rejected by the SQL guard is not a missing record")
		assert.False(t, reserved)
		assert.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
		_, cacheErr := cacheGetUserBase(user.Id)
		assert.Error(t, cacheErr, "the stale high balance must not be restored")

		reserved, err = TryReserveUserQuota(user.Id, 5)
		require.NoError(t, err)
		assert.True(t, reserved)
		assert.Equal(t, 95, getUserQuotaFromDB(t, user.Id))
	})

	t.Run("token", func(t *testing.T) {
		truncateTables(t)
		resetBatchUpdateTestState(t)
		useUserCacheMiniRedis(t)

		token := createReserveTestToken(t, 10)
		_, err := GetTokenByKey(token.Key, true)
		require.NoError(t, err)
		require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).
			Updates(map[string]any{"remain_quota": 2, "used_quota": 8}).Error)
		fired := topUpAfterRejectedReserve(t, "tokens", func() {
			require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).Update("remain_quota", 100).Error)
		})

		reserved, err := TryReserveTokenQuota(token.Id, token.Key, 5, false)
		require.True(t, *fired, "the SQL guard must reject before the top-up lands")
		require.NoError(t, err, "an existing token rejected by the SQL guard is not a missing record")
		assert.False(t, reserved)
		_, cacheErr := cacheGetTokenByKey(token.Key)
		assert.Error(t, cacheErr, "the stale high balance must not be restored")

		reserved, err = TryReserveTokenQuota(token.Id, token.Key, 5, false)
		require.NoError(t, err)
		assert.True(t, reserved)
		reloaded := getTokenFromDB(t, token.Id)
		assert.Equal(t, 95, reloaded.RemainQuota)
		assert.Equal(t, 13, reloaded.UsedQuota)
	})
}

func TestTokenCacheInitPreservesLiveQuotaAndFenceBlocksStaleSnapshot(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	server := useUserCacheMiniRedis(t)

	token := createReserveTestToken(t, 100)
	loaded, err := GetTokenByKey(token.Key, true)
	require.NoError(t, err)
	stale := *loaded

	result, err := cacheApplyTokenQuotaDelta(token.Id, token.Key, -70)
	require.NoError(t, err)
	require.Equal(t, cacheQuotaOK, result)

	// 已存在的哈希只刷新 TTL：数据库快照不得覆盖已被原子预扣的余额。
	code, err := cacheInitToken(stale)
	require.NoError(t, err)
	assert.Equal(t, 2, code)
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, 30, cached.RemainQuota)

	// 变更期间：fence 删除缓存并拦截并发读者手中的过期快照。
	require.NoError(t, invalidateTokenCacheForMutation(token.Key))
	code, err = cacheInitToken(stale)
	require.NoError(t, err)
	assert.Zero(t, code, "the pre-mutation snapshot must not be published while fenced")
	_, err = cacheGetTokenByKey(token.Key)
	assert.Error(t, err)

	// fence 过期后可重新从数据库水合。
	server.FastForward(time.Duration(tokenCacheFenceSeconds+1) * time.Second)
	fresh, err := GetTokenByKey(token.Key, false)
	require.NoError(t, err)
	assert.Equal(t, 100, fresh.RemainQuota)
	cached, err = cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, 100, cached.RemainQuota)
}
