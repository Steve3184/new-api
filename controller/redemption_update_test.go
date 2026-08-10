package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateRedemptionSwitchesBetweenWalletAndSubscriptionPlan(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Redemption{}, &model.SubscriptionPlan{}))

	plan := model.SubscriptionPlan{
		Title: "Editable plan", PriceAmount: 10, Currency: "USD",
		DurationUnit: "month", DurationValue: 1, Enabled: true,
	}
	require.NoError(t, db.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	redemption := model.Redemption{
		UserId: 1, Name: "wallet-code", Key: "redemption-update-test-key",
		Status: common.RedemptionCodeStatusEnabled, Quota: 500000,
	}
	require.NoError(t, db.Create(&redemption).Error)

	t.Cleanup(func() {
		model.InvalidateSubscriptionPlanCache(plan.Id)
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	performUpdate := func(body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/redemption/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		UpdateRedemption(c)
		return recorder
	}

	recorder := performUpdate(fmt.Sprintf(
		`{"id":%d,"name":"subscription-code","quota":999,"subscription_plan_id":%d,"expired_time":0}`,
		redemption.Id, plan.Id,
	))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.Redemption
	require.NoError(t, db.First(&updated, redemption.Id).Error)
	assert.Equal(t, plan.Id, updated.SubscriptionPlanId)
	assert.Zero(t, updated.Quota)

	require.NoError(t, db.Model(&plan).Update("enabled", false).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	recorder = performUpdate(fmt.Sprintf(
		`{"id":%d,"name":"sub-renamed","quota":999,"expired_time":0}`,
		redemption.Id,
	))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	require.NoError(t, db.First(&updated, redemption.Id).Error)
	assert.Equal(t, plan.Id, updated.SubscriptionPlanId)
	assert.Zero(t, updated.Quota)

	recorder = performUpdate(fmt.Sprintf(
		`{"id":%d,"name":"wallet-code-again","quota":700000,"subscription_plan_id":0,"expired_time":0}`,
		redemption.Id,
	))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	require.NoError(t, db.First(&updated, redemption.Id).Error)
	assert.Zero(t, updated.SubscriptionPlanId)
	assert.Equal(t, 700000, updated.Quota)
}
