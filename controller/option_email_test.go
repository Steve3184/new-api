package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendSMTPTestEmailRejectsInvalidRecipient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/option/smtp/test",
		strings.NewReader(`{"email":"not-an-email"}`),
	)

	SendSMTPTestEmail(context)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.False(t, payload.Success)
}

func TestSMTPTestEmailTypesUseTemplatesWithoutIssuingCredentials(t *testing.T) {
	require.NoError(t, i18n.Init())
	mailbox := newSecurityMailbox(t)
	for _, tc := range []struct {
		mailType, event, content string
	}{
		{"smtp", "", "Your SMTP configuration is working"},
		{"verification", "", "123456"},
		{"password_reset", "", "smtp-test-invalid-token"},
		{"email_binding", "", "Confirm linking this email address"},
		{"email_change", "", "Confirm replacing your current address"},
		{"security", "twofa_enabled", "Two-factor authentication enabled"},
		{"security", "passkey_registered", "Passkey registered"},
		{"notification", "", "SMTP notification test"},
	} {
		t.Run(tc.mailType+"/"+tc.event, func(t *testing.T) {
			request, err := common.Marshal(map[string]string{"email": "preview@example.com", "type": tc.mailType, "security_event": tc.event})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodPost, "/api/option/smtp/test", strings.NewReader(string(request)))
			context.Request.Header.Set("Accept-Language", "en")
			SendSMTPTestEmail(context)
			var payload struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.True(t, payload.Success, response.Body.String())
			mailbox.mutex.Lock()
			messages := append([]string(nil), mailbox.mail["preview@example.com"]...)
			mailbox.mutex.Unlock()
			require.NotEmpty(t, messages)
			assert.Contains(t, messages[len(messages)-1], tc.content)
			assert.False(t, common.VerifyCodeWithKey("preview@example.com", "123456", common.EmailVerificationPurpose))
			assert.False(t, common.VerifyCodeWithKey("preview@example.com", "smtp-test-invalid-token", common.PasswordResetPurpose))
		})
	}
}

func TestSMTPTestEmailRejectsUnknownTypesAndEvents(t *testing.T) {
	for _, body := range []string{
		`{"email":"preview@example.com","type":"unknown"}`,
		`{"email":"preview@example.com","type":"security","security_event":"unknown"}`,
	} {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = httptest.NewRequest(http.MethodPost, "/api/option/smtp/test", strings.NewReader(body))
		SendSMTPTestEmail(context)
		var payload struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
		assert.False(t, payload.Success)
	}
}

func TestUpdateOptionKeepsEmailDomainRestrictionsMutuallyExclusive(t *testing.T) {
	oldRestriction, oldReward := common.EmailDomainRestrictionEnabled, common.EmailDomainRewardRestrictionEnabled
	t.Cleanup(func() {
		common.EmailDomainRestrictionEnabled, common.EmailDomainRewardRestrictionEnabled = oldRestriction, oldReward
	})
	for _, tc := range []struct {
		enabledKey, requestedKey string
	}{
		{"EmailDomainRestrictionEnabled", "EmailDomainRewardRestrictionEnabled"},
		{"EmailDomainRewardRestrictionEnabled", "EmailDomainRestrictionEnabled"},
	} {
		t.Run(tc.requestedKey, func(t *testing.T) {
			common.EmailDomainRestrictionEnabled = tc.enabledKey == "EmailDomainRestrictionEnabled"
			common.EmailDomainRewardRestrictionEnabled = tc.enabledKey == "EmailDomainRewardRestrictionEnabled"
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(`{"key":"`+tc.requestedKey+`","value":true}`))

			UpdateOption(context)

			var payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			assert.False(t, payload.Success)
			assert.Contains(t, payload.Message, "cannot be enabled at the same time")
			assert.NotEqual(t, common.EmailDomainRestrictionEnabled, common.EmailDomainRewardRestrictionEnabled)
		})
	}
}
