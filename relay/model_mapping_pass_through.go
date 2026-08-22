package relay

import (
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

func prepareMappedPassThroughBody(
	c *gin.Context,
	storage common.BodyStorage,
	info *relaycommon.RelayInfo,
	applyCacheControl bool,
) (io.Reader, io.Closer, error) {
	shouldMapModel := info != nil &&
		info.ChannelMeta != nil &&
		info.OriginModelName != "" &&
		info.UpstreamModelName != "" &&
		info.UpstreamModelName != info.OriginModelName &&
		c != nil &&
		strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "application/json")
	if !shouldMapModel && !(applyCacheControl && shouldApplyClaudeCacheControl(info)) {
		return common.NewReplayableBodyReader(storage), nil, nil
	}

	jsonData, err := storage.Bytes()
	if err != nil {
		return nil, nil, err
	}
	if shouldMapModel {
		jsonData, err = sjson.SetBytes(jsonData, "model", info.UpstreamModelName)
		if err != nil {
			return nil, nil, err
		}
	}
	if applyCacheControl {
		jsonData, err = applyClaudeCacheControl(info, jsonData)
		if err != nil {
			return nil, nil, err
		}
	}
	return relaycommon.NewOutboundJSONBody(jsonData)
}
