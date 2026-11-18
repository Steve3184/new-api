package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/novelai"
	"github.com/QuantumNous/new-api/relay/channel/unrealspeech"
	"github.com/stretchr/testify/require"
)

func TestGetAdaptorUsesUnrealSpeechAdaptor(t *testing.T) {
	adaptor := GetAdaptor(constant.APITypeUnrealSpeech)
	require.IsType(t, &unrealspeech.Adaptor{}, adaptor)
}

func TestGetAdaptorUsesNovelAIAdaptor(t *testing.T) {
	require.IsType(t, &novelai.Adaptor{}, GetAdaptor(constant.APITypeNovelAI))
}
