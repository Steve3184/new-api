package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapStatusCheckAnnouncement(t *testing.T) {
	previousAnnouncement := common.StatusCheckAnnouncement
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		common.StatusCheckAnnouncement = previousAnnouncement
		common.OptionMap = previousOptionMap
	})

	announcement := "## Scheduled maintenance\nStatus data may be delayed."
	require.NoError(t, updateOptionMap("StatusCheckAnnouncement", announcement))

	assert.Equal(t, announcement, common.StatusCheckAnnouncement)
	assert.Equal(t, announcement, common.OptionMap["StatusCheckAnnouncement"])
}

func TestUpdateOptionMapStatusCheckFlexibleMode(t *testing.T) {
	previousFlexibleMode := common.StatusCheckFlexibleMode
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		common.StatusCheckFlexibleMode = previousFlexibleMode
		common.OptionMap = previousOptionMap
	})

	flexibleMode := `{"groups":{"default":{"enabled":true,"idle_minutes":20,"max_consecutive_probes":40}}}`
	require.NoError(t, updateOptionMap("StatusCheckFlexibleMode", flexibleMode))

	assert.Equal(t, flexibleMode, common.StatusCheckFlexibleMode)
	assert.Equal(t, flexibleMode, common.OptionMap["StatusCheckFlexibleMode"])
}
