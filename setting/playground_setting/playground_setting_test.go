package playground_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundSettingsDefaultAndModelAllowlist(t *testing.T) {
	require.NoError(t, UpdateByJSONString(`{
		"enabled_features":["chat","image","speech"],
		"models":{"chat":[],"image":["gpt-image-2","gpt-image-2"],"speech":["azure-tts"],"three_d":[]},
		"speech_model_types":{"azure-tts":"azure"}
	}`))
	t.Cleanup(func() {
		require.NoError(t, UpdateByJSONString(`{"enabled_features":["chat"],"models":{"chat":[],"image":[],"speech":[],"three_d":[]},"speech_model_types":{}}`))
	})

	assert.True(t, IsFeatureEnabled(FeatureChat))
	assert.False(t, IsFeatureEnabled(FeatureThreeD))
	assert.True(t, IsModelAllowed(FeatureChat, "any-chat-model"))
	assert.True(t, IsModelAllowed(FeatureImage, "gpt-image-2"))
	assert.False(t, IsModelAllowed(FeatureImage, "other-image-model"))
	assert.Equal(t, SpeechModelTypeAzure, Get().SpeechModelTypes["azure-tts"])
}

func TestPlaygroundSettingsRejectUnsupportedValues(t *testing.T) {
	assert.Error(t, UpdateByJSONString(`{"enabled_features":["video"]}`))
	assert.Error(t, UpdateByJSONString(`{"enabled_features":[]}`))
	assert.Error(t, UpdateByJSONString(`{"enabled_features":["speech"],"speech_model_types":{"tts":"unknown"}}`))
	assert.NoError(t, ValidateJSONString(`{"enabled_features":["chat"],"models":{"chat":[],"image":[],"speech":[],"three_d":[]}}`))
}
