package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoCheckinRequiresBalanceStrictlyAboveConfiguredThreshold(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Checkin{}, &model.Log{}))

	setting := operation_setting.GetCheckinSetting()
	previous := *setting
	setting.Enabled = true
	setting.MinQuota = 1
	setting.MaxQuota = 1
	setting.MinUserQuota = 100
	t.Cleanup(func() { *setting = previous })

	tests := []struct {
		name    string
		quota   int
		success bool
		message string
	}{
		{name: "equal balance is rejected", quota: 100, success: false, message: "用户余额必须大于 100 才能签到"},
		{name: "greater balance is accepted", quota: 101, success: true, message: "签到成功"},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := &model.User{
				Id:       index + 1,
				Username: test.name,
				AffCode:  test.name,
				Quota:    test.quota,
			}
			require.NoError(t, db.Create(user).Error)

			router := gin.New()
			router.POST("/checkin", func(context *gin.Context) {
				context.Set("id", user.Id)
				DoCheckin(context)
			})

			request := httptest.NewRequest(http.MethodPost, "/checkin", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.DecodeJson(recorder.Body, &response))
			assert.Equal(t, test.success, response.Success)
			assert.Equal(t, test.message, response.Message)
		})
	}
}
