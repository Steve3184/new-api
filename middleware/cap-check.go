package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type capCheckResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

var capHTTPClient = &http.Client{Timeout: 10 * time.Second}

func CapCheck() gin.HandlerFunc {
	return capCheck(common.CapSiteKey, common.CapSecretKey)
}

func CapCheckCheckin() gin.HandlerFunc {
	return capCheck(common.CapCheckinSiteKey, common.CapCheckinSecretKey)
}

func capCheck(siteKey, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.CapEnabled {
			c.Next()
			return
		}

		token := c.Query("cap_token")
		if token == "" {
			token = c.Query("cap-token")
		}
		if token == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cap token 为空",
			})
			c.Abort()
			return
		}

		serverURL := strings.TrimRight(strings.TrimSpace(common.CapServerURL), "/")
		if serverURL == "" || siteKey == "" || secretKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cap configuration is incomplete",
			})
			c.Abort()
			return
		}

		payload, err := common.Marshal(gin.H{
			"secret":   secretKey,
			"response": token,
		})
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}

		verifyURL := fmt.Sprintf("%s/%s/siteverify", serverURL, url.PathEscape(siteKey))
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, verifyURL, bytes.NewReader(payload))
		if err != nil {
			common.ApiError(c, err)
			c.Abort()
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := capHTTPClient.Do(req)
		if err != nil {
			common.SysLog(err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
		defer resp.Body.Close()

		var result capCheckResponse
		if err := common.DecodeJson(resp.Body, &result); err != nil {
			common.SysLog(err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.Success {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cap 校验失败，请重试！",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
