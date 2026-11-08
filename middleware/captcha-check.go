package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// CaptchaPurpose names the form a captcha protects. Each purpose resolves its
// own provider, so an operator can protect registration while leaving login
// open, or give login and registration different captcha services.
type CaptchaPurpose string

const (
	// CaptchaPurposeLogin covers password login.
	CaptchaPurposeLogin CaptchaPurpose = "login"
	// CaptchaPurposeRegister covers account registration.
	CaptchaPurposeRegister CaptchaPurpose = "register"
	// CaptchaPurposePasswordReset covers the forgot-password flow.
	CaptchaPurposePasswordReset CaptchaPurpose = "reset_password"
	// CaptchaPurposeCheckin covers the daily check-in.
	CaptchaPurposeCheckin CaptchaPurpose = "checkin"
	// CaptchaPurposeRedemption covers redeeming a code for quota.
	CaptchaPurposeRedemption CaptchaPurpose = "redemption"
)

// captchaProviderFor returns the provider a purpose must use. An empty
// per-purpose value follows the global CaptchaType, which keeps existing
// deployments on their single configured provider.
func captchaProviderFor(purpose CaptchaPurpose) string {
	var configured string
	switch purpose {
	case CaptchaPurposeLogin:
		configured = common.LoginCaptchaType
	case CaptchaPurposeRegister:
		configured = common.RegisterCaptchaType
	case CaptchaPurposePasswordReset:
		configured = common.ResetPasswordCaptchaType
	case CaptchaPurposeCheckin:
		configured = common.CheckinCaptchaType
	case CaptchaPurposeRedemption:
		configured = common.RedemptionCaptchaType
	}
	if configured != "" {
		return configured
	}
	return common.CaptchaType
}

// captchaCheckWith enforces whichever provider the resolver reports. The
// provider is read per request, so an option change applies without a restart.
func captchaCheckWith(resolve func() string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch resolve() {
		case "cap":
			CapCheck()(c)
		case "hcaptcha":
			HCaptchaCheck()(c)
		case "none":
			c.Next()
		default:
			// "turnstile" or any unrecognised value falls back to Turnstile
			TurnstileCheck()(c)
		}
	}
}

// CaptchaCheck routes to the appropriate captcha middleware depending on
// the operator's configured CaptchaType.
// For routes that require a fresh captcha, use their purpose-specific wrapper.
func CaptchaCheck() gin.HandlerFunc {
	return captchaCheckWith(func() string { return common.CaptchaType })
}

// CaptchaCheckFor enforces the provider configured for one purpose.
func CaptchaCheckFor(purpose CaptchaPurpose) gin.HandlerFunc {
	return captchaCheckWith(func() string { return captchaProviderFor(purpose) })
}

// CaptchaCheckRegister skips the registration captcha when email verification
// is enabled. The registration controller then requires and validates the
// email verification code, while registrations without email verification
// continue to require a captcha token.
func CaptchaCheckRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.EmailVerificationEnabled {
			c.Next()
			return
		}

		CaptchaCheckFor(CaptchaPurposeRegister)(c)
	}
}

// CaptchaCheckCheckin is like CaptchaCheck but only enforces the captcha when
// ForceCheckinCaptcha is true; otherwise it passes through unconditionally.
func CaptchaCheckCheckin() gin.HandlerFunc {
	return captchaCheckFresh(
		func() bool { return common.ForceCheckinCaptcha },
		func() string { return captchaProviderFor(CaptchaPurposeCheckin) },
		CapCheckCheckin,
	)
}

// CaptchaCheckRedemption requires a fresh captcha for each redemption when the
// operator enables ForceRedemptionCaptcha.
func CaptchaCheckRedemption() gin.HandlerFunc {
	return captchaCheckFresh(
		func() bool { return common.ForceRedemptionCaptcha },
		func() string { return captchaProviderFor(CaptchaPurposeRedemption) },
		CapCheck,
	)
}

// captchaCheckFresh enforces a captcha only when required() is true, using the
// provider resolved for that purpose.
func captchaCheckFresh(required func() bool, provider func() string, capMiddleware func() gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !required() {
			c.Next()
			return
		}
		switch provider() {
		case "none":
			c.Next()
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
