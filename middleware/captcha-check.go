package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// CaptchaCheck routes to the appropriate captcha middleware depending on
// the operator's configured CaptchaType.
// For routes that require a fresh captcha, use their purpose-specific wrapper.
func CaptchaCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch common.CaptchaType {
		case "cap":
			CapCheck()(c)
		case "hcaptcha":
			HCaptchaCheck()(c)
		default:
			// "turnstile" or any unrecognised value falls back to Turnstile
			TurnstileCheck()(c)
		}
	}
}

// CaptchaCheckCheckin is like CaptchaCheck but only enforces the captcha when
// ForceCheckinCaptcha is true; otherwise it passes through unconditionally.
func CaptchaCheckCheckin() gin.HandlerFunc {
	return captchaCheckFresh(func() bool {
		return common.ForceCheckinCaptcha
	}, CapCheckCheckin)
}

// CaptchaCheckRedemption requires a fresh captcha for each redemption when the
// operator enables ForceRedemptionCaptcha.
func CaptchaCheckRedemption() gin.HandlerFunc {
	return captchaCheckFresh(func() bool {
		return common.ForceRedemptionCaptcha
	}, CapCheck)
}

func captchaCheckFresh(required func() bool, capMiddleware func() gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !required() {
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
			capMiddleware()(c)
		case "hcaptcha":
			if !common.HCaptchaEnabled {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "hCaptcha is not enabled"})
				c.Abort()
				return
			}
			HCaptchaCheckFresh()(c)
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
