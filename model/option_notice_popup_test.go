package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapNoticePopupSettings(t *testing.T) {
	previousPopupEnabled := common.NoticePopupEnabled
	previousPopupMode := common.NoticePopupMode
	previousDashboardEnabled := common.NoticePopupOnDashboardEnabled
	previousHeaderButtonMode := common.NoticeHeaderButtonMode
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		common.NoticePopupEnabled = previousPopupEnabled
		common.NoticePopupMode = previousPopupMode
		common.NoticePopupOnDashboardEnabled = previousDashboardEnabled
		common.NoticeHeaderButtonMode = previousHeaderButtonMode
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, updateOptionMap("NoticePopupEnabled", "true"))
	require.NoError(t, updateOptionMap("NoticePopupMode", "dashboard"))
	require.NoError(t, updateOptionMap("NoticePopupOnDashboardEnabled", "true"))
	require.NoError(t, updateOptionMap("NoticeHeaderButtonMode", "dialog"))

	assert.True(t, common.NoticePopupEnabled)
	assert.Equal(t, "dashboard", common.NoticePopupMode)
	assert.True(t, common.NoticePopupOnDashboardEnabled)
	assert.Equal(t, "dialog", common.NoticeHeaderButtonMode)
	assert.Equal(t, "true", common.OptionMap["NoticePopupEnabled"])
	assert.Equal(t, "dashboard", common.OptionMap["NoticePopupMode"])
	assert.Equal(t, "true", common.OptionMap["NoticePopupOnDashboardEnabled"])
	assert.Equal(t, "dialog", common.OptionMap["NoticeHeaderButtonMode"])

	require.Error(t, updateOptionMap("NoticePopupMode", "invalid"))
	require.Error(t, updateOptionMap("NoticeHeaderButtonMode", "invalid"))
}
