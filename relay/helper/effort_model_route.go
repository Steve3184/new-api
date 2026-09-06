package helper

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// ApplyEffortModelRoute resolves the configured model for the requested model
// and effort. It intentionally returns false when no route is configured,
// leaving the existing model-mapping and provider behavior unchanged.
func ApplyEffortModelRoute(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	settings := model_setting.GetGlobalSettings()
	if settings == nil {
		return false
	}
	modelName := info.ChannelMeta.UpstreamModelName
	effort := info.GetReasoningEffort()
	routed, ok := settings.RouteModelByEffort(modelName, effort)
	if !ok || routed == info.UpstreamModelName {
		return false
	}
	info.ChannelMeta.UpstreamModelName = routed
	info.EffortModelRouted = true
	return true
}
