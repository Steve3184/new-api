package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/unrealspeech"
	"github.com/stretchr/testify/require"
)

func TestGetAdaptorUsesUnrealSpeechAdaptor(t *testing.T) {
	adaptor := GetAdaptor(constant.APITypeUnrealSpeech)

	require.IsType(t, &unrealspeech.Adaptor{}, adaptor)
}
