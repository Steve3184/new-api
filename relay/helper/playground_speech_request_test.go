package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/playground_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundSpeechRequestParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, playground_setting.UpdateByJSONString(`{
		"enabled_features":["speech"],
		"models":{"chat":[],"image":[],"speech":["tts-1","azure-tts"],"three_d":[]},
		"speech_model_types":{"azure-tts":"azure"}
	}`))
	t.Cleanup(func() {
		require.NoError(t, playground_setting.UpdateByJSONString(`{"enabled_features":["chat"],"models":{"chat":[],"image":[],"speech":[],"three_d":[]},"speech_model_types":{}}`))
	})

	newContext := func(body string) *gin.Context {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodPost, "/pg/audio/speech", bytes.NewBufferString(body))
		context.Request.Header.Set("Content-Type", "application/json")
		return context
	}

	_, err := GetAndValidAudioRequest(newContext(`{"model":"tts-1","input":"hello"}`), relayconstant.RelayModeAudioSpeech)
	require.EqualError(t, err, "speed is required")

	_, err = GetAndValidAudioRequest(newContext(`{"model":"tts-1","input":"hello","speed":4.01}`), relayconstant.RelayModeAudioSpeech)
	require.EqualError(t, err, "speed must be between 0.25 and 4")

	_, err = GetAndValidAudioRequest(newContext(`{"model":"tts-1","input":"hello","speed":1,"volume":1}`), relayconstant.RelayModeAudioSpeech)
	require.EqualError(t, err, "volume and pitch are only supported by Azure playground speech models")

	request, err := GetAndValidAudioRequest(newContext(`{"model":"azure-tts","input":"hello","speed":1,"volume":1,"pitch":-5}`), relayconstant.RelayModeAudioSpeech)
	require.NoError(t, err)
	require.NotNil(t, request.Speed)
	require.NotNil(t, request.Volume)
	require.NotNil(t, request.Pitch)
	require.Equal(t, 1.0, *request.Speed)
	require.Equal(t, 1.0, *request.Volume)
	require.Equal(t, -5, *request.Pitch)
}
