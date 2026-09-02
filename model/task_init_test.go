package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
)

func TestInitTaskPersistsSelectedAgnesKey(t *testing.T) {
	info := &relaycommon.RelayInfo{
		UserId:     1,
		UsingGroup: "default",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAgnes,
			ChannelId:   126,
			ApiKey:      "agnes-selected-key",
		},
	}

	task := InitTask(constant.TaskPlatform("63"), info)

	assert.Equal(t, "agnes-selected-key", task.PrivateData.Key)
}
