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
