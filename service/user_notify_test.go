package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
)

func TestChannelAutoStatusEmailSwitchPreservesNonEmailNotifications(t *testing.T) {
	original := common.ChannelAutoStatusEmailEnabled
	defer func() { common.ChannelAutoStatusEmailEnabled = original }()

	common.ChannelAutoStatusEmailEnabled = false
	assert.False(t, shouldSendChannelAutoStatusNotification(dto.UserSetting{}))
	assert.False(t, shouldSendChannelAutoStatusNotification(dto.UserSetting{
		NotifyType: dto.NotifyTypeEmail,
	}))
	assert.True(t, shouldSendChannelAutoStatusNotification(dto.UserSetting{
		NotifyType: dto.NotifyTypeWebhook,
	}))

	common.ChannelAutoStatusEmailEnabled = true
	assert.True(t, shouldSendChannelAutoStatusNotification(dto.UserSetting{}))
}
