package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestQuotaReminderSwitchDisablesWalletAndSubscriptionAlerts(t *testing.T) {
	original := common.QuotaRemindEnabled
	t.Cleanup(func() {
		common.QuotaRemindEnabled = original
	})

	common.QuotaRemindEnabled = false
	assert.False(t, shouldSendQuotaReminder())

	common.QuotaRemindEnabled = true
	assert.True(t, shouldSendQuotaReminder())
}
