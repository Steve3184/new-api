package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// useCaptchaOptions pins the captcha options for one test and restores them.
func useCaptchaOptions(t *testing.T, apply func()) {
	t.Helper()
	previousGlobal := common.CaptchaType
	previousLogin := common.LoginCaptchaType
	previousRegister := common.RegisterCaptchaType
	previousReset := common.ResetPasswordCaptchaType
	previousTurnstile := common.TurnstileCheckEnabled
	previousHCaptcha := common.HCaptchaEnabled
	previousCheckin := common.CheckinCaptchaType
	previousRedemption := common.RedemptionCaptchaType
	previousForceCheckin := common.ForceCheckinCaptcha
	previousForceRedemption := common.ForceRedemptionCaptcha
	t.Cleanup(func() {
		common.CaptchaType = previousGlobal
		common.LoginCaptchaType = previousLogin
		common.RegisterCaptchaType = previousRegister
		common.ResetPasswordCaptchaType = previousReset
		common.TurnstileCheckEnabled = previousTurnstile
		common.HCaptchaEnabled = previousHCaptcha
		common.CheckinCaptchaType = previousCheckin
		common.RedemptionCaptchaType = previousRedemption
		common.ForceCheckinCaptcha = previousForceCheckin
		common.ForceRedemptionCaptcha = previousForceRedemption
	})
	apply()
}

func runCaptchaGuard(t *testing.T, guard gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
	guard(ctx)
	return recorder
}

func TestCaptchaProviderFallsBackToTheGlobalProvider(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.CaptchaType = "hcaptcha"
		common.LoginCaptchaType = ""
		common.RegisterCaptchaType = ""
		common.ResetPasswordCaptchaType = ""
	})

	for _, purpose := range []CaptchaPurpose{
		CaptchaPurposeLogin,
		CaptchaPurposeRegister,
		CaptchaPurposePasswordReset,
		CaptchaPurposeCheckin,
		CaptchaPurposeRedemption,
	} {
		assert.Equal(t, "hcaptcha", captchaProviderFor(purpose), string(purpose))
	}
}

func TestCaptchaProviderReadsEachPurposeIndependently(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.CaptchaType = "turnstile"
		common.LoginCaptchaType = "none"
		common.RegisterCaptchaType = "cap"
		common.ResetPasswordCaptchaType = "hcaptcha"
		common.CheckinCaptchaType = "turnstile"
		common.RedemptionCaptchaType = "none"
	})

	assert.Equal(t, "none", captchaProviderFor(CaptchaPurposeLogin))
	assert.Equal(t, "cap", captchaProviderFor(CaptchaPurposeRegister))
	assert.Equal(t, "hcaptcha", captchaProviderFor(CaptchaPurposePasswordReset))
	assert.Equal(t, "turnstile", captchaProviderFor(CaptchaPurposeCheckin))
	assert.Equal(t, "none", captchaProviderFor(CaptchaPurposeRedemption))
}

func TestLoginGuardDoesNotRequireACaptchaWhenItsProviderIsNone(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.CaptchaType = "turnstile"
		common.TurnstileCheckEnabled = true
		common.LoginCaptchaType = "none"
	})

	recorder := runCaptchaGuard(t, CaptchaCheckFor(CaptchaPurposeLogin))

	// The global provider is still Turnstile, so a request without a token must
	// pass: this is the whole point of a per-purpose override.
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Body.String())
}

func TestRegisterGuardKeepsProtectingWhileLoginIsOpen(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.CaptchaType = "turnstile"
		common.TurnstileCheckEnabled = true
		common.LoginCaptchaType = "none"
		common.RegisterCaptchaType = "turnstile"
		common.EmailVerificationEnabled = false
	})

	recorder := runCaptchaGuard(t, CaptchaCheckRegister())

	assert.Contains(t, recorder.Body.String(), "Turnstile token is empty")
}

func TestPasswordResetGuardUsesItsOwnProvider(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.CaptchaType = "turnstile"
		common.TurnstileCheckEnabled = true
		common.HCaptchaEnabled = false
		common.ResetPasswordCaptchaType = "hcaptcha"
	})

	// hCaptcha is disabled, so its middleware lets the request through; a
	// Turnstile provider here would have rejected it instead.
	recorder := runCaptchaGuard(t, CaptchaCheckFor(CaptchaPurposePasswordReset))

	assert.NotContains(t, recorder.Body.String(), "Turnstile token is empty")
}

func TestRedemptionGuardUsesItsOwnProvider(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.ForceRedemptionCaptcha = true
		common.CaptchaType = "turnstile"
		common.TurnstileCheckEnabled = true
		common.HCaptchaEnabled = false
		common.RedemptionCaptchaType = "hcaptcha"
	})

	recorder := runCaptchaGuard(t, CaptchaCheckRedemption())

	// The global provider is Turnstile and would reject the token-less request;
	// the redemption override resolves to hCaptcha, which is disabled here and
	// therefore lets it through.
	assert.NotContains(t, recorder.Body.String(), "Turnstile token is empty")
}

func TestCheckinGuardKeepsTheRequiredSwitchIndependentFromTheProvider(t *testing.T) {
	useCaptchaOptions(t, func() {
		common.CaptchaType = "turnstile"
		common.TurnstileCheckEnabled = true
		common.CheckinCaptchaType = "turnstile"
	})

	common.ForceCheckinCaptcha = false
	assert.Empty(t, runCaptchaGuard(t, CaptchaCheckCheckin()).Body.String())

	// The switch decides whether a captcha runs at all; the provider only picks
	// which one.
	common.ForceCheckinCaptcha = true
	assert.Contains(
		t,
		runCaptchaGuard(t, CaptchaCheckCheckin()).Body.String(),
		"Turnstile token is empty",
	)
}
