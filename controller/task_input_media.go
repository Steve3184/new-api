package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// TaskInputMediaContent requires a capability even with a valid API token.
func TaskInputMediaContent(c *gin.Context) {
	if !middleware.IsTaskArtifactAccess(c) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	service.ServeTaskInputMedia(c)
}
