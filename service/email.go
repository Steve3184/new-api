package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
)

// These senders render mail only. Creating and consuming verification codes
// remains the responsibility of the authenticated account flow.
func SendVerificationEmail(lang, email, code string) error {
	content, err := common.RenderEmailVerificationTemplate(code)
	if err != nil {
		return err
	}
	subject := i18n.Translate(lang, i18n.MsgEmailVerificationSubject, map[string]any{"SystemName": common.SystemName})
	return common.SendEmail(subject, email, content)
}

func SendPasswordResetEmail(lang, email, link string) error {
	params := map[string]any{"SystemName": common.SystemName}
	content, err := common.RenderEmailTemplate("EmailPasswordResetTemplate", map[string]any{
		"SystemName":   common.SystemName,
		"Greeting":     i18n.Translate(lang, i18n.MsgEmailPasswordResetGreeting, params),
		"ActionText":   i18n.Translate(lang, i18n.MsgEmailPasswordResetAction),
		"Link":         link,
		"ValidMinutes": common.VerificationValidMinutes,
		"Validity":     i18n.Translate(lang, i18n.MsgEmailPasswordResetValidity, map[string]any{"Minutes": common.VerificationValidMinutes}),
	})
	if err != nil {
		return err
	}
	return common.SendEmail(i18n.Translate(lang, i18n.MsgEmailPasswordResetSubject, params), email, content)
}

func SendEmailBindingVerification(lang, email, code string, oldEmail bool) error {
	introKey, validityKey := i18n.MsgEmailBindingNewIntro, i18n.MsgEmailBindingNewValidity
	if oldEmail {
		introKey, validityKey = i18n.MsgEmailBindingOldIntro, i18n.MsgEmailBindingOldValidity
	}
	minutes := int(model.EmailBindingTTL / time.Minute)
	content, err := common.RenderEmailTemplate("EmailBindingTemplate", map[string]any{
		"SystemName": common.SystemName, "Code": code, "IsOldEmail": oldEmail,
		"Intro": i18n.Translate(lang, introKey), "ValidMinutes": minutes,
		"Validity": i18n.Translate(lang, validityKey, map[string]any{"Minutes": minutes}),
	})
	if err != nil {
		return err
	}
	subject := i18n.Translate(lang, i18n.MsgEmailBindingSubject, map[string]any{"SystemName": common.SystemName})
	return common.SendEmail(subject, email, content)
}
