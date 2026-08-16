package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapQuotaRemindEnabled(t *testing.T) {
	original := common.QuotaRemindEnabled
	t.Cleanup(func() {
		common.QuotaRemindEnabled = original
	})

	require.NoError(t, updateOptionMap("QuotaRemindEnabled", "false"))
	require.False(t, common.QuotaRemindEnabled)

	require.NoError(t, updateOptionMap("QuotaRemindEnabled", "true"))
	require.True(t, common.QuotaRemindEnabled)
}
