package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCheckProbeStateResetsStreakAfterPassiveActivity(t *testing.T) {
	truncateTables(t)
	require.NoError(t, InitializeStatusCheckProbeState("default", 100))
	require.NoError(t, TouchStatusCheckProbePassiveActivity("default", 120))
	require.NoError(t, RecordStatusCheckProbeResult("default", 130))
	require.NoError(t, RecordStatusCheckProbeResult("default", 140))

	states, err := GetStatusCheckProbeStates([]string{"default"})
	require.NoError(t, err)
	assert.Equal(t, 2, states["default"].ConsecutiveProbeCount)
	assert.Equal(t, int64(140), states["default"].LastProbeAt)

	require.NoError(t, TouchStatusCheckProbePassiveActivity("default", 150))
	require.NoError(t, RecordStatusCheckProbeResult("default", 160))

	states, err = GetStatusCheckProbeStates([]string{"default"})
	require.NoError(t, err)
	assert.Equal(t, 1, states["default"].ConsecutiveProbeCount)
	assert.Equal(t, int64(150), states["default"].LastPassiveAt)
	assert.Equal(t, int64(160), states["default"].LastProbeAt)
}
