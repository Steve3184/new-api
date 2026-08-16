package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimulatedRemoteCompactV2RoundTrip(t *testing.T) {
	encoded, err := EncodeSimulatedRemoteCompactV2("Keep the pending migration and its test result.")
	require.NoError(t, err)
	assert.Contains(t, encoded, SimulatedRemoteCompactV2Prefix)

	summary, simulated, err := DecodeSimulatedRemoteCompactV2(encoded)
	require.NoError(t, err)
	assert.True(t, simulated)
	assert.Equal(t, "Keep the pending migration and its test result.", summary)
}

func TestDecodeSimulatedRemoteCompactV2KeepsNativePayloadOpaque(t *testing.T) {
	summary, simulated, err := DecodeSimulatedRemoteCompactV2("provider-encrypted-content")
	require.NoError(t, err)
	assert.False(t, simulated)
	assert.Empty(t, summary)
}

func TestDecodeSimulatedRemoteCompactV2RejectsMalformedPayload(t *testing.T) {
	_, simulated, err := DecodeSimulatedRemoteCompactV2(SimulatedRemoteCompactV2Prefix + "not-base64!")
	require.Error(t, err)
	assert.True(t, simulated)
}

func TestResponsesTokenMetadataIncludesSimulatedCompactionSummary(t *testing.T) {
	encoded, err := EncodeSimulatedRemoteCompactV2("The selected channel requires a normal summary request.")
	require.NoError(t, err)
	input, err := kitutil.Marshal([]map[string]string{{
		"type":              "compaction",
		"encrypted_content": encoded,
	}})
	require.NoError(t, err)

	meta := (&OpenAIResponsesRequest{Input: input}).GetTokenCountMeta()
	assert.Contains(t, meta.CombineText, "selected channel requires a normal summary request")
}
