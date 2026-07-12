package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncCapDifficultyUsesStandaloneConfigAPI(t *testing.T) {
	var receivedDifficulty int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/server/keys/login-key/config", r.URL.Path)
		assert.Equal(t, "Bot admin-api-key", r.Header.Get("Authorization"))

		var body map[string]int
		require.NoError(t, common.DecodeJson(r.Body, &body))
		receivedDifficulty = body["difficulty"]
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"success":true}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	previousClient := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = previousClient })

	err := SyncCapDifficulty(
		context.Background(),
		server.URL,
		"admin-api-key",
		"login-key",
		6,
	)
	require.NoError(t, err)
	assert.Equal(t, 6, receivedDifficulty)
}
