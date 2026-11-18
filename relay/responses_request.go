package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relay/requestplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

type responsesRequestPreparation struct {
	request                 *dto.OpenAIResponsesRequest
	remoteCompactV2Modified bool
}

func prepareResponsesRequestInput(c *gin.Context, info *relaycommon.RelayInfo, req *dto.OpenAIResponsesRequest) (*responsesRequestPreparation, *types.NewAPIError) {
	if req == nil {
		return nil, types.NewError(errors.New("responses request is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact &&
		!common.SupportsResponsesCompact(info.ChannelType, info.ApiType) {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(req)
	if err != nil {
		return nil, types.NewError(fmt.Errorf("failed to copy responses request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ApplyReasoningModelSuffix(c, info, request); err != nil {
		return nil, newConvertRequestFailedError(c, info, err)
	}
	remoteCompactV2, err := helper.PrepareSimulatedRemoteCompactV2Request(request, info.ChannelSetting.SimulateRemoteCompactV2)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if remoteCompactV2.Simulating {
		helper.EnableSimulatedRemoteCompactV2(c)
	}

	return &responsesRequestPreparation{
		request:                 request,
		remoteCompactV2Modified: remoteCompactV2.Modified,
	}, nil
}

// PrepareResponsesRequest applies the same model, conversion and channel rules
// for HTTP and WebSocket requests. The caller closes closer after the attempt;
// passthrough bodies remain owned by the incoming request's BodyStorage.
// The returned adaptor retains route/conversion state for DoRequest/DoResponse.
func PrepareResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.OpenAIResponsesRequest) (relaychannel.Adaptor, common.ReplayableBody, io.Closer, *types.NewAPIError) {
	prepared, apiErr := prepareResponsesRequestInput(c, info, req)
	if apiErr != nil {
		return nil, nil, nil, apiErr
	}
	return prepareResponsesRequestBody(c, info, prepared)
}

func prepareResponsesRequestBody(c *gin.Context, info *relaycommon.RelayInfo, prepared *responsesRequestPreparation) (relaychannel.Adaptor, common.ReplayableBody, io.Closer, *types.NewAPIError) {
	if prepared == nil || prepared.request == nil {
		return nil, nil, nil, types.NewError(errors.New("responses request preparation is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if info == nil || c == nil || c.Request == nil {
		return nil, nil, nil, types.NewErrorWithStatusCode(errors.New("responses relay context is missing"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, nil, nil, types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	if (model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled) && !prepared.remoteCompactV2Modified {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, nil, nil, types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		if apiErr := requestplugin.PassThrough(c, info, storage); apiErr != nil {
			return nil, nil, nil, apiErr
		}
		body, closer, err := prepareMappedPassThroughBody(c, storage, info, true)
		if err != nil {
			return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		replayable, ok := body.(common.ReplayableBody)
		if !ok {
			if closer != nil {
				_ = closer.Close()
			}
			return nil, nil, nil, types.NewError(errors.New("responses passthrough body is not replayable"), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if closer == nil {
			closer = io.NopCloser(replayable)
		}
		return adaptor, replayable, closer, nil
	}

	convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *prepared.request)
	if err != nil {
		return nil, nil, nil, newConvertRequestFailedError(c, info, err)
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, apiErr := requestplugin.Outbound(c, info, jsonData)
	if apiErr != nil {
		return nil, nil, nil, apiErr
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, nil, nil, newAPIErrorFromParamOverride(err)
		}
	}
	jsonData, err = applyClaudeCacheControl(info, jsonData)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if finalizer, ok := adaptor.(relaychannel.ResponsesRequestFinalizer); ok {
		jsonData, err = finalizer.FinalizeResponsesRequest(info, jsonData)
		if err != nil {
			return nil, nil, nil, newConvertRequestFailedError(c, info, err)
		}
	}
	if apiErr = requestplugin.Egress(c, info, jsonData); apiErr != nil {
		return nil, nil, nil, apiErr
	}

	logger.LogDebug(c, "requestBody: %s", jsonData)
	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return adaptor, body, closer, nil
}
