package relay

import (
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func shouldApplyClaudeCacheControl(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.GetChannelType() == constant.ChannelTypeAnthropic &&
		info.ChannelOtherSettings.ClaudeCacheControl
}

func applyClaudeCacheControl(info *relaycommon.RelayInfo, jsonData []byte) ([]byte, error) {
	if !shouldApplyClaudeCacheControl(info) {
		return jsonData, nil
	}
	return relaycommon.ApplyClaudeStablePrefixCacheControl(jsonData)
}
