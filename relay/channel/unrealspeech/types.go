package unrealspeech

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	ChannelName = "UnrealSpeech"
	ModeStream  = "stream"
	ModeSpeech  = "speech"
	ModeTask    = "task"
)

var ModelList = []string{"unreal-speech-v8"}

type Request struct {
	Text          string   `json:"Text"`
	VoiceID       string   `json:"VoiceId"`
	Bitrate       string   `json:"Bitrate,omitempty"`
	Speed         *float64 `json:"Speed,omitempty"`
	Pitch         *float64 `json:"Pitch,omitempty"`
	Codec         string   `json:"Codec,omitempty"`
	Temperature   *float64 `json:"Temperature,omitempty"`
	TimestampType string   `json:"TimestampType,omitempty"`
}

type Options struct {
	Bitrate       string   `json:"bitrate,omitempty"`
	Pitch         *float64 `json:"pitch,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	TimestampType string   `json:"timestamp_type,omitempty"`
}

type SynthesisTask struct {
	CreationTime      string          `json:"CreationTime"`
	OutputURI         json.RawMessage `json:"OutputUri"`
	RequestCharacters int             `json:"RequestCharacters"`
	StatusDetails     string          `json:"StatusDetails,omitempty"`
	TaskID            string          `json:"TaskId"`
	TaskStatus        string          `json:"TaskStatus"`
	TimestampsURI     json.RawMessage `json:"TimestampsUri"`
	VoiceID           string          `json:"VoiceId"`
}

type SynthesisTaskEnvelope struct {
	SynthesisTask SynthesisTask `json:"SynthesisTask"`
}

func ResolveMode(request dto.AudioRequest) (string, error) {
	stream := request.Stream != nil && *request.Stream
	speech := request.Speech != nil && *request.Speech
	if stream && speech {
		return "", errors.New("stream and speech cannot both be true")
	}
	if stream {
		return ModeStream, nil
	}
	return ModeSpeech, nil
}

func BuildRequest(request dto.AudioRequest, mode string) (Request, error) {
	textLimit := 5000
	if mode == ModeStream {
		textLimit = 1000
	} else if mode == ModeTask {
		textLimit = 500000
	} else if mode != ModeSpeech {
		return Request{}, fmt.Errorf("unsupported UnrealSpeech mode: %s", mode)
	}
	if strings.TrimSpace(request.Input) == "" {
		return Request{}, errors.New("input is required")
	}
	if utf8.RuneCountInString(request.Input) > textLimit {
		return Request{}, fmt.Errorf("input must not exceed %d characters in %s mode", textLimit, mode)
	}
	if strings.TrimSpace(request.Voice) == "" {
		return Request{}, errors.New("voice is required")
	}

	options := Options{}
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &options); err != nil {
			return Request{}, fmt.Errorf("invalid UnrealSpeech metadata: %w", err)
		}
	}
	if request.Bitrate != "" {
		options.Bitrate = request.Bitrate
	}
	if request.Pitch != nil {
		options.Pitch = request.Pitch
	}
	if request.Temperature != nil {
		options.Temperature = request.Temperature
	}
	if request.TimestampType != "" {
		options.TimestampType = request.TimestampType
	}
	if options.Bitrate != "" && !isSupportedBitrate(options.Bitrate) {
		return Request{}, fmt.Errorf("unsupported bitrate: %s", options.Bitrate)
	}
	if options.Pitch != nil && (math.IsNaN(*options.Pitch) || math.IsInf(*options.Pitch, 0) || *options.Pitch < 0.5 || *options.Pitch > 1.5) {
		return Request{}, errors.New("metadata.pitch must be between 0.5 and 1.5")
	}
	if options.Temperature != nil && (math.IsNaN(*options.Temperature) || math.IsInf(*options.Temperature, 0) || *options.Temperature < 0.1 || *options.Temperature > 0.8) {
		return Request{}, errors.New("metadata.temperature must be between 0.1 and 0.8")
	}
	if options.TimestampType != "" && options.TimestampType != "word" && options.TimestampType != "sentence" {
		return Request{}, errors.New("metadata.timestamp_type must be word or sentence")
	}

	responseFormat := strings.ToLower(strings.TrimSpace(request.ResponseFormat))
	if responseFormat == "" {
		responseFormat = "mp3"
	}
	if (mode == ModeSpeech || mode == ModeTask) && responseFormat != "mp3" {
		return Request{}, errors.New("UnrealSpeech speech mode only supports response_format mp3")
	}
	if mode == ModeStream && responseFormat != "mp3" && responseFormat != "pcm" {
		return Request{}, errors.New("UnrealSpeech stream mode only supports response_format mp3 or pcm")
	}

	var speed *float64
	if request.Speed != nil {
		if math.IsNaN(*request.Speed) || math.IsInf(*request.Speed, 0) || *request.Speed < -1 || *request.Speed > 1 {
			return Request{}, errors.New("UnrealSpeech speed must be between -1 and 1")
		}
		value := *request.Speed
		speed = &value
	}

	upstream := Request{
		Text:          request.Input,
		VoiceID:       request.Voice,
		Bitrate:       options.Bitrate,
		Speed:         speed,
		Pitch:         options.Pitch,
		Temperature:   options.Temperature,
		TimestampType: options.TimestampType,
	}
	if mode == ModeStream {
		if request.Codec != "" {
			if request.Codec != "libmp3lame" && request.Codec != "pcm_s16le" && request.Codec != "pcm_mulaw" {
				return Request{}, fmt.Errorf("unsupported codec: %s", request.Codec)
			}
			upstream.Codec = request.Codec
		} else if responseFormat == "pcm" {
			upstream.Codec = "pcm_s16le"
		} else {
			upstream.Codec = "libmp3lame"
		}
		upstream.TimestampType = ""
	} else {
		upstream.Temperature = nil
	}
	return upstream, nil
}

func FirstURI(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var single string
	if err := common.Unmarshal(raw, &single); err == nil {
		return strings.TrimSpace(single)
	}
	var values []string
	if err := common.Unmarshal(raw, &values); err == nil && len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	return ""
}

func isSupportedBitrate(value string) bool {
	switch value {
	case "16k", "32k", "48k", "64k", "128k", "192k", "256k", "320k":
		return true
	default:
		return false
	}
}
