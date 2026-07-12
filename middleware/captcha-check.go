package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// CaptchaCheck routes to the appropriate captcha middleware depending on
// the operator's configured CaptchaType ("turnstile" or "cap").
// For the check-in route, call CaptchaCheckCheckin instead.
func CaptchaCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch common.CaptchaType {
		case "cap":
			CapCheck()(c)
		default:
			// "turnstile" or any unrecognised value falls back to Turnstile
			TurnstileCheck()(c)
		}
	}
}

// CaptchaCheckCheckin is like CaptchaCheck but only enforces the captcha when
// ForceCheckinCaptcha is true; otherwise it passes through unconditionally.
func CaptchaCheckCheckin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.ForceCheckinCaptcha {
			c.Next()
			return
		}
		switch common.CaptchaType {
		case "cap":
			if !common.CapEnabled {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "Cap is not enabled"})
				c.Abort()
				return
			}
			CapCheckCheckin()(c)
		default:
			if !common.TurnstileCheckEnabled {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "Turnstile is not enabled"})
				c.Abort()
				return
			}
			TurnstileCheckFresh()(c)
		}
	}
}
