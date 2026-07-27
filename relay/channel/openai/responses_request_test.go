package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestNormalizesScalarInput(t *testing.T) {
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`"hello"`),
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)

	var input []map[string]any
	require.NoError(t, common.Unmarshal(request.Input, &input))
	require.Equal(t, []map[string]any{
		{
			"role":    "user",
			"content": "hello",
		},
	}, input)
}

func TestConvertOpenAIResponsesRequestPreservesListInput(t *testing.T) {
	input := []byte(`[{"role":"user","content":"hello"}]`)
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: input,
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.JSONEq(t, string(input), string(request.Input))
}
