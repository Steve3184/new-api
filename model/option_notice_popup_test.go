package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapNoticePopupSettings(t *testing.T) {
	previousPopupEnabled := common.NoticePopupEnabled
	previousDashboardEnabled := common.NoticePopupOnDashboardEnabled
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		common.NoticePopupEnabled = previousPopupEnabled
		common.NoticePopupOnDashboardEnabled = previousDashboardEnabled
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, updateOptionMap("NoticePopupEnabled", "true"))
	require.NoError(t, updateOptionMap("NoticePopupOnDashboardEnabled", "true"))

	assert.True(t, common.NoticePopupEnabled)
	assert.True(t, common.NoticePopupOnDashboardEnabled)
	assert.Equal(t, "true", common.OptionMap["NoticePopupEnabled"])
	assert.Equal(t, "true", common.OptionMap["NoticePopupOnDashboardEnabled"])
}
