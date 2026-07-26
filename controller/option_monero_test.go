package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionRejectsInvalidMoneroUSDToCurrencyRate(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"MoneroUSDToCurrencyRate","value":-1}`),
	)

	UpdateOption(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.False(t, payload.Success)
	assert.Equal(t, "Monero USD to system currency rate must be a non-negative finite number", payload.Message)
}

func TestUpdateOptionRejectsInvalidMoneroSubaddressLimit(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"MoneroMaxSubaddresses","value":0}`),
	)

	UpdateOption(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.False(t, payload.Success)
	assert.Equal(t, "Monero subaddress limit must be between 1 and 1000000", payload.Message)
}
