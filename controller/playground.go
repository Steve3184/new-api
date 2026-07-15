package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/playground_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context, feature playground_setting.Feature, relayFormat types.RelayFormat, handler func(*gin.Context)) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		newAPIError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	if !playground_setting.IsFeatureEnabled(feature) {
		newAPIError = types.NewError(errors.New("playground feature is disabled"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}
	if !playground_setting.IsModelAllowed(feature, relayInfo.OriginModelName) {
		newAPIError = types.NewError(fmt.Errorf("model %s is not enabled for this playground feature", relayInfo.OriginModelName), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		return
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	handler(c)
}

func PlaygroundChat(c *gin.Context) {
	Playground(c, playground_setting.FeatureChat, types.RelayFormatOpenAI, func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAI)
	})
}

func PlaygroundImage(c *gin.Context) {
	Playground(c, playground_setting.FeatureImage, types.RelayFormatOpenAIImage, func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAIImage)
	})
}

func PlaygroundSpeech(c *gin.Context) {
	Playground(c, playground_setting.FeatureSpeech, types.RelayFormatOpenAIAudio, func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAIAudio)
	})
}

func PlaygroundSpeechTask(c *gin.Context) {
	Playground(c, playground_setting.FeatureSpeech, types.RelayFormatTask, RelayTask)
}

func PlaygroundSpeechTaskFetch(c *gin.Context) {
	if !playground_setting.IsFeatureEnabled(playground_setting.FeatureSpeech) {
		c.JSON(403, gin.H{"error": gin.H{"message": "playground feature is disabled", "type": "access_denied"}})
		return
	}
	RelayTaskFetch(c)
}

func PlaygroundSpeechContent(c *gin.Context) {
	if !playground_setting.IsFeatureEnabled(playground_setting.FeatureSpeech) {
		c.JSON(403, gin.H{"error": gin.H{"message": "playground feature is disabled", "type": "access_denied"}})
		return
	}
	AudioSpeechProxy(c)
}

func PlaygroundThreeD(c *gin.Context) {
	Playground(c, playground_setting.FeatureThreeD, types.RelayFormatTask, RelayTask)
}

func PlaygroundThreeDFetch(c *gin.Context) {
	if !playground_setting.IsFeatureEnabled(playground_setting.FeatureThreeD) {
		c.JSON(403, gin.H{"error": gin.H{"message": "playground feature is disabled", "type": "access_denied"}})
		return
	}
	RelayTaskFetch(c)
}
