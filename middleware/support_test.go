/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSupportEnabledMiddleware(t *testing.T) {
	previous := common.SupportEnabled
	t.Cleanup(func() {
		common.SupportEnabled = previous
	})

	tests := []struct {
		name       string
		enabled    bool
		statusCode int
	}{
		{name: "allows requests while support is enabled", enabled: true, statusCode: http.StatusNoContent},
		{name: "rejects requests while support is disabled", enabled: false, statusCode: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			common.SupportEnabled = test.enabled
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/support", SupportEnabled(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/support", nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			assert.Equal(t, test.statusCode, response.Code)
		})
	}
}
