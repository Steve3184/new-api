package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A per-purpose captcha option is stored as its provider name, and an empty
// value is a valid choice that means "follow the global provider".
func TestUpdateOptionMapPerPurposeCaptchaTypes(t *testing.T) {
	previousLogin := common.LoginCaptchaType
	previousRegister := common.RegisterCaptchaType
	previousReset := common.ResetPasswordCaptchaType
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		common.LoginCaptchaType = previousLogin
		common.RegisterCaptchaType = previousRegister
		common.ResetPasswordCaptchaType = previousReset
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, updateOptionMap("LoginCaptchaType", "none"))
	require.NoError(t, updateOptionMap("RegisterCaptchaType", "cap"))
	require.NoError(t, updateOptionMap("ResetPasswordCaptchaType", "hcaptcha"))

	assert.Equal(t, "none", common.LoginCaptchaType)
	assert.Equal(t, "cap", common.RegisterCaptchaType)
	assert.Equal(t, "hcaptcha", common.ResetPasswordCaptchaType)
	assert.Equal(t, "none", common.OptionMap["LoginCaptchaType"])

	require.NoError(t, updateOptionMap("LoginCaptchaType", ""))
	assert.Empty(t, common.LoginCaptchaType)
	assert.Equal(t, "", common.OptionMap["LoginCaptchaType"])
}
