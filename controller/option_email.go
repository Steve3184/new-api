package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type smtpTestEmailRequest struct {
	Email         string `json:"email"`
	Type          string `json:"type"`
	SecurityEvent string `json:"security_event"`
}

var smtpSecurityEvents = map[string]string{
	"password_updated":         i18n.MsgSecurityEventPasswordUpdated,
	"password_reset":           i18n.MsgSecurityEventPasswordReset,
	"email_change_requested":   i18n.MsgSecurityEventEmailChangeRequested,
	"email_changed":            i18n.MsgSecurityEventEmailChanged,
	"email_confirmed":          i18n.MsgSecurityEventEmailConfirmed,
	"login_linked":             i18n.MsgSecurityEventLoginLinked,
	"login_unlinked":           i18n.MsgSecurityEventLoginUnlinked,
	"wechat_linked":            i18n.MsgSecurityEventWeChatLinked,
	"twofa_enabled":            i18n.MsgSecurityEventTwoFAEnabled,
	"twofa_disabled":           i18n.MsgSecurityEventTwoFADisabled,
	"twofa_reset":              i18n.MsgSecurityEventTwoFAReset,
	"login_twofa_enabled":      i18n.MsgSecurityEventLoginTwoFAEnabled,
	"login_twofa_disabled":     i18n.MsgSecurityEventLoginTwoFADisabled,
	"backup_codes_regenerated": i18n.MsgSecurityEventBackupCodesRegenerated,
	"passkey_registered":       i18n.MsgSecurityEventPasskeyRegistered,
	"passkey_removed":          i18n.MsgSecurityEventPasskeyRemoved,
	"passkey_reset":            i18n.MsgSecurityEventPasskeyReset,
}

func SendSMTPTestEmail(c *gin.Context) {
	var request smtpTestEmailRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	email := model.NormalizeEmail(request.Email)
	if err := common.Validate.Var(email, "required,email"); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	lang := i18n.StatedLang(c)
	var err error
	switch request.Type {
	case "", "smtp": // Keep the original request contract working.
		err = common.SendSMTPTestEmail(email)
	case "verification":
		err = service.SendVerificationEmail(lang, email, "123456")
	case "password_reset":
		// A reserved domain and an unregistered token cannot reset an account.
		err = service.SendPasswordResetEmail(lang, email, "https://example.invalid/user/reset?token=smtp-test-invalid-token")
	case "email_binding", "email_change":
		err = service.SendEmailBindingVerification(lang, email, "123456", request.Type == "email_change")
	case "security":
		eventKey, ok := smtpSecurityEvents[request.SecurityEvent]
		if !ok {
			common.ApiErrorT(c, "Invalid parameters")
			return
		}
		err = service.NotifyAccountSecurityChange(lang, email, eventKey, map[string]any{"Provider": "GitHub"})
	case "notification":
		err = service.SendNotificationEmail(email, "SMTP notification test", "This is a sample notification sent using your saved email template.")
	case "low_balance", "subscription_low_balance":
		titleKey := i18n.MsgNotifyQuotaLowTitle
		if request.Type == "subscription_low_balance" {
			titleKey = i18n.MsgNotifySubscriptionQuotaLowTitle
		}
		title := i18n.Translate(lang, titleKey)
		content := i18n.Translate(lang, i18n.MsgNotifyQuotaLowEmail, map[string]any{"Prompt": title, "Quota": "10", "Link": "https://example.invalid/console/topup"})
		err = service.SendNotificationEmail(email, title, content)
	case "channel_disabled", "channel_enabled":
		subjectKey, contentKey := i18n.MsgChannelNotifyDisabledSubject, i18n.MsgChannelNotifyDisabledContent
		if request.Type == "channel_enabled" {
			subjectKey, contentKey = i18n.MsgChannelNotifyEnabledSubject, i18n.MsgChannelNotifyEnabledContent
		}
		params := map[string]any{"Name": "SMTP test", "Id": 1, "Reason": "SMTP test"}
		err = service.SendNotificationEmail(email, i18n.Translate(lang, subjectKey, params), i18n.Translate(lang, contentKey, params))
	case "channel_test":
		err = service.SendNotificationEmail(email, i18n.Translate(lang, i18n.MsgChannelTestCompletedSubject), i18n.Translate(lang, i18n.MsgChannelTestCompletedContent))
	case "upstream_update":
		content := i18n.Translate(lang, i18n.MsgChannelUpstreamUpdateSummary, map[string]any{"Checked": 1, "Changed": 1, "Added": 1, "Removed": 0, "AutoAdded": 0, "Failed": 0})
		err = service.SendNotificationEmail(email, i18n.Translate(lang, i18n.MsgChannelUpstreamUpdateNotifySubject), content)
	default:
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
