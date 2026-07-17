package controller

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRetryTimesForCurrentGroup(t *testing.T) {
	previous := ratio_setting.GroupRetryTimes2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRetryTimesByJSONString(previous))
	})
	require.NoError(t, ratio_setting.UpdateGroupRetryTimesByJSONString(`{"default":0,"fast":4}`))

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")

	assert.Equal(t, 0, getRetryTimesForCurrentGroup(ctx, "default"))

	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "fast")
	assert.Equal(t, 4, getRetryTimesForCurrentGroup(ctx, "auto"))
}

func TestShouldRetryHonorsZeroLimitForChannelErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := types.NewError(errors.New("invalid channel key"), types.ErrorCodeChannelInvalidKey)

	assert.False(t, shouldRetry(ctx, err, 0))
	assert.True(t, shouldRetry(ctx, err, 1))
}
