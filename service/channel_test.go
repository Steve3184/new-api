package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func TestShouldDisableChannelKeepsStreamFirstResponseTimeoutRetryable(t *testing.T) {
	original := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = original
	})

	err := types.NewErrorWithStatusCode(
		errors.New("first response timeout"),
		types.ErrorCodeChannelStreamFirstResponseTimeout,
		http.StatusGatewayTimeout,
	)

	assert.True(t, types.IsChannelError(err))
	assert.False(t, ShouldDisableChannel(err))
}
