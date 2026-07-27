package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath2RelayMode(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/v1/audio/speech", want: RelayModeAudioSpeech},
		{path: "/v1/audio/speech/websocket", want: RelayModeAudioSpeechWebSocket},
		{path: "/v1/audio/speech/tasks", want: RelayModeAudioSpeechTaskSubmit},
		{path: "/v1/audio/speech/tasks/task_public", want: RelayModeAudioSpeechTaskFetchByID},
		{path: "/v1/audio/speech/tasks/task_public/content", want: RelayModeAudioSpeechTaskFetchByID},
		{path: "/v1/audio/speech/tasks/task_public/timestamps", want: RelayModeAudioSpeechTaskFetchByID},
		{path: "/v1/alpha/search", want: RelayModeAlphaSearch},
		{path: "/v1/alpha/search?foo=1", want: RelayModeAlphaSearch},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}
