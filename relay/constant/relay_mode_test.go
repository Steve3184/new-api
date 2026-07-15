package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnrealSpeechRelayModesTakePriorityOverSpeechPrefix(t *testing.T) {
	tests := []struct {
		path string
		mode int
	}{
		{path: "/v1/audio/speech", mode: RelayModeAudioSpeech},
		{path: "/v1/audio/speech/websocket", mode: RelayModeAudioSpeechWebSocket},
		{path: "/v1/audio/speech/tasks", mode: RelayModeAudioSpeechTaskSubmit},
		{path: "/v1/audio/speech/tasks/task_public", mode: RelayModeAudioSpeechTaskFetchByID},
		{path: "/v1/audio/speech/tasks/task_public/content", mode: RelayModeAudioSpeechTaskFetchByID},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			assert.Equal(t, test.mode, Path2RelayMode(test.path))
		})
	}
}
