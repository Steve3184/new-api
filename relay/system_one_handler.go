package relay

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/typesafe"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func SystemOneHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)
	request, ok := info.Request.(*dto.SystemOneRequest)
	if !ok || request == nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.SystemOneRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor, ok := GetAdaptor(info.ApiType).(*typesafe.Adaptor)
	if !ok || adaptor == nil {
		return types.NewErrorWithStatusCode(errors.New("selected channel does not support TypeSafe System One"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	converted, err := adaptor.ConvertSystemOneRequest(info, request)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	jsonData, err := common.Marshal(converted)
	if err != nil {
		return types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	logger.LogDebug(c, "systemone request body: %s", jsonData)
	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()

	respAny, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
	}
	resp, ok := respAny.(*http.Response)
	if !ok || resp == nil || resp.Body == nil {
		return types.NewOpenAIError(errors.New("invalid HTTP response from TypeSafe"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := service.RelayErrorHandler(c.Request.Context(), resp, false)
		service.ResetStatusCode(apiErr, c.GetString("status_code_mapping"))
		return apiErr
	}
	usage, apiErr := adaptor.DoResponse(c, resp, info)
	if apiErr != nil {
		return apiErr
	}
	usageInfo, ok := usage.(*dto.Usage)
	if !ok {
		usageInfo = &dto.Usage{}
	}
	service.PostTextConsumeQuota(c, info, usageInfo, nil)
	return nil
}
