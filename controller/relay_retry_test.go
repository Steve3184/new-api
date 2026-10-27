package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

func TestShouldRetryStreamFirstResponseTimeout(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := types.NewErrorWithStatusCode(
		errors.New("first response timeout"),
		types.ErrorCodeChannelStreamFirstResponseTimeout,
		http.StatusGatewayTimeout,
	)

	assert.True(t, shouldRetry(ctx, err, 1))
}

func TestDecideSameChannelRetry(t *testing.T) {
	previousAutoDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = previousAutoDisable })

	rateLimited := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	invalidKey := types.NewError(errors.New("invalid channel key"), types.ErrorCodeChannelInvalidKey)
	firstResponseTimeout := types.NewErrorWithStatusCode(errors.New("first response timeout"), types.ErrorCodeChannelStreamFirstResponseTimeout, http.StatusGatewayTimeout)
	badRequest := types.NewErrorWithStatusCode(errors.New("bad request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	sameChannel := service.PolicyDecision{Action: "retry", Reason: "same_channel_retry", Source: "channel"}

	for _, tc := range []struct {
		name      string
		err       *types.NewAPIError
		remaining int
		autoBan   bool
		expected  service.PolicyDecision
	}{
		{name: "disabled by default", err: rateLimited, remaining: 0, expected: service.PolicyDecision{Action: "stop", Reason: "same_channel_retry_exhausted", Source: "channel"}},
		{name: "retryable status", err: rateLimited, remaining: 1, autoBan: true, expected: sameChannel},
		{name: "non-retryable error", err: badRequest, remaining: 1, expected: service.PolicyDecision{Action: "stop", Reason: "non_retryable_error", Source: "system"}},
		{name: "channel about to be disabled", err: invalidKey, remaining: 1, autoBan: true, expected: service.PolicyDecision{Action: "stop", Reason: "channel_disable_requested", Source: "channel"}},
		{name: "channel error without auto ban", err: invalidKey, remaining: 1, expected: sameChannel},
		{name: "first response timeout keeps the channel", err: firstResponseTimeout, remaining: 1, autoBan: true, expected: sameChannel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, service.DecideSameChannelRetry(tc.err, tc.remaining, tc.autoBan))
		})
	}
}
