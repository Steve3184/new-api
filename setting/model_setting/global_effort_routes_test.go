package model_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteModelByEffortUsesConfiguredModelAndEffort(t *testing.T) {
	original := GetGlobalSettings().EffortModelRoutes
	t.Cleanup(func() { GetGlobalSettings().EffortModelRoutes = original })
	GetGlobalSettings().EffortModelRoutes = map[string]map[string]string{
		"gemini-3.1-flash-lite": {
			"high": "gemini-3.1-flash-lite-high",
			"low":  "gemini-3.1-flash-lite-low",
		},
	}

	routed, ok := GetGlobalSettings().RouteModelByEffort("gemini-3.1-flash-lite", "HIGH")
	require.True(t, ok)
	assert.Equal(t, "gemini-3.1-flash-lite-high", routed)
	routed, ok = GetGlobalSettings().RouteModelByEffort("gemini-3.1-flash-lite", "low")
	require.True(t, ok)
	assert.Equal(t, "gemini-3.1-flash-lite-low", routed)
}

func TestValidateEffortModelRoutesRejectsEmptyEntries(t *testing.T) {
	err := ValidateEffortModelRoutes(`{"model":{"":"upstream"}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty effort and target")
}
