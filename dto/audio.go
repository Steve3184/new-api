package dto

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type AudioRequest struct {
	Model          string          `json:"model"`
	Input          string          `json:"input"`
	Voice          string          `json:"voice"`
	Text           string          `json:"text,omitempty"`
	VoiceID        string          `json:"voiceid,omitempty"`
	Instructions   string          `json:"instructions,omitempty"`
	ResponseFormat string          `json:"response_format,omitempty"`
	Speed          *float64        `json:"speed,omitempty"`
	Volume         *float64        `json:"volume,omitempty"`
	Pitch          *float64        `json:"pitch,omitempty"`
	Bitrate        string          `json:"bitrate,omitempty"`
	Codec          string          `json:"codec,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	TimestampType  string          `json:"timestamptype,omitempty"`
	StreamFormat   string          `json:"stream_format,omitempty"`
	Stream         *bool           `json:"stream,omitempty"`
	Speech         *bool           `json:"speech,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	// vllm-omini
	TaskType                json.RawMessage `json:"task_type,omitempty"`
	Language                json.RawMessage `json:"language,omitempty"`
	RefAudio                json.RawMessage `json:"ref_audio,omitempty"`
	RefText                 json.RawMessage `json:"ref_text,omitempty"`
	XVectorOnlyMode         json.RawMessage `json:"x_vector_only_mode,omitempty"`
	MaxNewTokens            json.RawMessage `json:"max_new_tokens,omitempty"`
	InitialCodecChunkFrames json.RawMessage `json:"initial_codec_chunk_frames,omitempty"`
	// TODO：ensure that the logic remains correct after the stream is started.
	//Stream                  json.RawMessage `json:"stream,omitempty"`
}

func (r *AudioRequest) NormalizeUnrealSpeechAliases() {
	if r.Input == "" {
		r.Input = r.Text
	}
	if r.Voice == "" {
		r.Voice = r.VoiceID
	}
	r.Text = ""
	r.VoiceID = ""
}

func (r *AudioRequest) GetTokenCountMeta() *types.TokenCountMeta {
	meta := &types.TokenCountMeta{
		CombineText: r.Input,
		TokenType:   types.TokenTypeTextNumber,
	}
	if strings.Contains(r.Model, "gpt") {
		meta.TokenType = types.TokenTypeTokenizer
	}
	return meta
}

func (r *AudioRequest) IsStream(c *gin.Context) bool {
	return r.StreamFormat == "sse"
}

func (r *AudioRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

type AudioResponse struct {
	Text string `json:"text"`
}

type AudioSpeechTaskError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AudioSpeechTaskResponse struct {
	ID            string                `json:"id"`
	Object        string                `json:"object"`
	CreatedAt     int64                 `json:"created_at"`
	Status        string                `json:"status"`
	Model         string                `json:"model"`
	Progress      int                   `json:"progress"`
	ContentURL    string                `json:"content_url,omitempty"`
	TimestampsURL string                `json:"timestamps_url,omitempty"`
	Error         *AudioSpeechTaskError `json:"error,omitempty"`
}

type WhisperVerboseJSONResponse struct {
	Task     string    `json:"task,omitempty"`
	Language string    `json:"language,omitempty"`
	Duration float64   `json:"duration,omitempty"`
	Text     string    `json:"text,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
}

type Segment struct {
	Id               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}
