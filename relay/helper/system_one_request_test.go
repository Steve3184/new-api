package helper

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidateSystemOneRequestPreservesStructuredPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/v1/systemone", strings.NewReader(`{"model":"jev-1.13","state":{"ticket":{"attempts":2}},"questions":{"route":{"type":"choice","instructions":{"prompt":"Choose a team"},"criteria":{"billing":"Payment issues"}}}}`))
	context.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidateRequest(context, types.RelayFormatSystemOne)
	require.NoError(t, err)
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"jev-1.13","state":{"ticket":{"attempts":2}},"questions":{"route":{"type":"choice","instructions":{"prompt":"Choose a team"},"criteria":{"billing":"Payment issues"}}}}`, string(encoded))
}
