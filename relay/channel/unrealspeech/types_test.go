package unrealspeech

import (
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialAndLowercaseRequestFieldsNormalize(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "official capitalization",
			body: `{"model":"unreal-speech-v8","Text":"你好。","VoiceId":"Sierra","Bitrate":"16k","Speed":0,"Pitch":1.02,"TimestampType":"word"}`,
		},
		{
			name: "lowercase",
			body: `{"model":"unreal-speech-v8","text":"你好。","voiceid":"Sierra","bitrate":"16k","speed":0,"pitch":1.02,"timestamptype":"word"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request dto.AudioRequest
			require.NoError(t, common.Unmarshal([]byte(test.body), &request))
			request.NormalizeUnrealSpeechAliases()

			upstream, err := BuildRequest(request, ModeSpeech)
			require.NoError(t, err)
			assert.Equal(t, "你好。", upstream.Text)
			assert.Equal(t, "Sierra", upstream.VoiceID)
			assert.Equal(t, "16k", upstream.Bitrate)
			require.NotNil(t, upstream.Speed)
			assert.Zero(t, *upstream.Speed)
			require.NotNil(t, upstream.Pitch)
			assert.Equal(t, 1.02, *upstream.Pitch)
			assert.Equal(t, "word", upstream.TimestampType)
		})
	}
}

func TestResolveModeRejectsAmbiguousRequest(t *testing.T) {
	request := dto.AudioRequest{Stream: common.GetPointer(true), Speech: common.GetPointer(true)}
	_, err := ResolveMode(request)
	require.EqualError(t, err, "stream and speech cannot both be true")
}

func TestBuildRequestEnforcesModeContracts(t *testing.T) {
	request := dto.AudioRequest{
		Input:          "hello",
		Voice:          "Sierra",
		ResponseFormat: "wav",
	}
	_, err := BuildRequest(request, ModeSpeech)
	require.EqualError(t, err, "UnrealSpeech speech mode only supports response_format mp3")

	request.ResponseFormat = "pcm"
	upstream, err := BuildRequest(request, ModeStream)
	require.NoError(t, err)
	assert.Equal(t, "pcm_s16le", upstream.Codec)
}

func TestBuildRequestEnforcesSynchronousCharacterLimits(t *testing.T) {
	request := dto.AudioRequest{Input: strings.Repeat("a", 5000), Voice: "Sierra"}
	_, err := BuildRequest(request, ModeSpeech)
	require.NoError(t, err)

	request.Input += "a"
	_, err = BuildRequest(request, ModeSpeech)
	require.EqualError(t, err, "input must not exceed 5000 characters in speech mode")

	request.Input = strings.Repeat("a", 1001)
	_, err = BuildRequest(request, ModeStream)
	require.EqualError(t, err, "input must not exceed 1000 characters in stream mode")
}

func TestFirstURIAcceptsStringAndArray(t *testing.T) {
	assert.Equal(t, "https://example.com/a.mp3", FirstURI([]byte(`"https://example.com/a.mp3"`)))
	assert.Equal(t, "https://example.com/a.mp3", FirstURI([]byte(`["https://example.com/a.mp3"]`)))
}

func TestTrustedS3AudioURLOnlyAllowsHTTPSAWSHosts(t *testing.T) {
	tests := []struct {
		raw     string
		trusted bool
	}{
		{raw: "https://unreal-synthesis-expire-in-90-days.s3-us-west-2.amazonaws.com/audio.mp3", trusted: true},
		{raw: "https://bucket.s3.us-west-2.amazonaws.com/audio.mp3", trusted: true},
		{raw: "https://s3.amazonaws.com/bucket/audio.mp3", trusted: true},
		{raw: "http://bucket.s3.amazonaws.com/audio.mp3", trusted: false},
		{raw: "https://bucket.s3.amazonaws.com:8443/audio.mp3", trusted: false},
		{raw: "https://bucket.s3.amazonaws.com.evil.example/audio.mp3", trusted: false},
		{raw: "https://127.0.0.1/audio.mp3", trusted: false},
	}
	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			parsed, err := url.Parse(test.raw)
			require.NoError(t, err)
			assert.Equal(t, test.trusted, isTrustedS3AudioURL(parsed))
		})
	}
}
