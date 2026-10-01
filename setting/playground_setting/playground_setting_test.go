package playground_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundSettingsDefaultAndModelAllowlist(t *testing.T) {
	require.NoError(t, UpdateByJSONString(`{
		"enabled_features":["chat","image","speech"],
		"models":{"chat":[],"image":["gpt-image-2","gpt-image-2"],"speech":["azure-tts","unreal-speech-v8"],"three_d":[],"video":[]},
		"speech_model_types":{"azure-tts":"azure","unreal-speech-v8":"unrealspeech"}
	}`))
	t.Cleanup(func() {
		require.NoError(t, UpdateByJSONString(`{"enabled_features":["chat"],"models":{"chat":[],"image":[],"speech":[],"three_d":[],"video":[]},"speech_model_types":{}}`))
	})

	assert.True(t, IsFeatureEnabled(FeatureChat))
	assert.False(t, IsFeatureEnabled(FeatureThreeD))
	assert.True(t, IsModelAllowed(FeatureChat, "any-chat-model"))
	assert.True(t, IsModelAllowed(FeatureImage, "gpt-image-2"))
	assert.False(t, IsModelAllowed(FeatureImage, "other-image-model"))
	assert.Equal(t, SpeechModelTypeAzure, Get().SpeechModelTypes["azure-tts"])
	assert.Equal(t, SpeechModelTypeUnreal, Get().SpeechModelTypes["unreal-speech-v8"])
}

func TestPlaygroundSettingsRejectUnsupportedValues(t *testing.T) {
	assert.NoError(t, UpdateByJSONString(`{"enabled_features":["video"],"models":{"chat":[],"image":[],"speech":[],"three_d":[],"video":[]}}`))
	assert.Error(t, UpdateByJSONString(`{"enabled_features":[]}`))
	assert.Error(t, UpdateByJSONString(`{"enabled_features":["speech"],"speech_model_types":{"tts":"unknown"}}`))
	assert.NoError(t, ValidateJSONString(`{"enabled_features":["chat"],"models":{"chat":[],"image":[],"speech":[],"three_d":[]}}`))
}

func TestChatPresetsRoundTripAndValidation(t *testing.T) {
	previous := ToJSONString()
	t.Cleanup(func() { require.NoError(t, UpdateByJSONString(previous)) })
	require.NoError(t, UpdateByJSONString("{}"))
	assert.Nil(t, Get().ChatPresets)
	require.NoError(t, UpdateByJSONString("{\"chat_presets\":[]}"))
	assert.NotNil(t, Get().ChatPresets)
	require.NoError(t, UpdateByJSONString("{\"chat_presets\":[{\"icon\":\" LuSparkles \",\"title\":\" Custom \",\"content\":\"  exact\\ncontent  \"}]}"))
	assert.Equal(t, ChatPreset{Icon: "LuSparkles", Title: "Custom", Content: "  exact\ncontent  "}, Get().ChatPresets[0])
	snapshot := Get()
	snapshot.ChatPresets[0].Title = "changed"
	assert.Equal(t, "Custom", Get().ChatPresets[0].Title)
	for _, presets := range [][]ChatPreset{
		make([]ChatPreset, 13),
		{{Title: strings.Repeat("x", 81), Content: "ok"}},
		{{Title: "ok", Content: strings.Repeat("x", 8001)}},
	} {
		encoded, err := common.Marshal(map[string]any{"chat_presets": presets})
		require.NoError(t, err)
		assert.Error(t, ValidateJSONString(string(encoded)))
	}
	require.NoError(t, UpdateByJSONString(ToJSONString()))
	for _, value := range []string{
		"{\"chat_presets\":[{\"title\":\" \",\"content\":\"x\"}]}",
		"{\"chat_presets\":[{\"title\":\"x\",\"content\":\" \"}]}",
		"{\"chat_presets\":[{\"title\":\"x\",\"content\":\"x\",\"icon\":\"<script>\"}]}",
	} {
		assert.Error(t, ValidateJSONString(value))
	}
	assert.Equal(t, "Custom", Get().ChatPresets[0].Title)
}
