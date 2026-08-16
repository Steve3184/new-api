package relay

import (
	"io"

	"github.com/QuantumNous/new-api/common"
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

func prepareClaudePassThroughBody(storage common.BodyStorage, info *relaycommon.RelayInfo) (io.Reader, io.Closer, error) {
	if !shouldApplyClaudeCacheControl(info) {
		return common.NewReplayableBodyReader(storage), nil, nil
	}

	jsonData, err := storage.Bytes()
	if err != nil {
		return nil, nil, err
	}
	jsonData, err = applyClaudeCacheControl(info, jsonData)
	if err != nil {
		return nil, nil, err
	}
	return relaycommon.NewOutboundJSONBody(jsonData)
}
