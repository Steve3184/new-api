package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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

// CaptchaCheckRegister accepts a verified email code as proof that the user
// already completed the registration captcha when requesting that code. This
// avoids asking for a second challenge while preserving captcha enforcement for
// registrations without a valid email verification code.
func CaptchaCheckRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.EmailVerificationEnabled {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				common.ApiError(c, err)
				c.Abort()
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))

			var request struct {
				Email            string `json:"email"`
				VerificationCode string `json:"verification_code"`
			}
			if err := common.Unmarshal(body, &request); err == nil &&
				request.Email != "" && request.VerificationCode != "" &&
				common.VerifyCodeWithKey(model.NormalizeEmail(request.Email), request.VerificationCode, common.EmailVerificationPurpose) {
				c.Next()
				return
			}
		}

		CaptchaCheck()(c)
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
