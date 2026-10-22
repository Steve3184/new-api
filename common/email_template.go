package common

import (
	"fmt"
	"html/template"
	"strings"
)

const DefaultEmailVerificationTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:24px;background:#f5f7fb;font-family:Arial,sans-serif;color:#1f2937;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0">
    <tr>
      <td align="center">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:560px;background:#ffffff;border:1px solid #e5e7eb;border-radius:8px;">
          <tr>
            <td style="padding:32px;">
              <h1 style="margin:0 0 20px;font-size:20px;line-height:28px;font-weight:600;">Verify your email address</h1>
              <p style="margin:0 0 16px;font-size:14px;line-height:22px;">Hello from {{.SystemName}},</p>
              <p style="margin:0 0 20px;font-size:14px;line-height:22px;">Use the verification code below to continue:</p>
              <div style="margin:0 0 20px;padding:14px 18px;background:#f3f4f6;border-radius:6px;font-family:monospace;font-size:28px;letter-spacing:6px;text-align:center;color:#111827;">{{.Code}}</div>
              <p style="margin:0;font-size:13px;line-height:20px;color:#4b5563;">This code expires in {{.ValidMinutes}} minutes. If you did not request it, you can safely ignore this email.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

type emailVerificationTemplateData struct {
	SystemName   string
	Code         string
	ValidMinutes int
}

// Blank custom templates use these localized layouts. Text and URLs are
// escaped by html/template; callers pass translated text as ordinary strings.
const (
	DefaultEmailPasswordResetTemplate        = `<p>{{.Greeting}}</p><p><a href="{{.Link}}">{{.ActionText}}</a></p><p>{{.Link}}</p><p>{{.Validity}}</p>`
	DefaultEmailBindingTemplate              = `<p>{{.Intro}}</p><p><strong>{{.Code}}</strong></p><p>{{.Validity}}</p>`
	DefaultEmailSecurityNotificationTemplate = `<p>{{.Intro}}</p><p>{{.Advice}}</p>`
	DefaultEmailNotificationTemplate         = `<h2>{{.Title}}</h2><div style="white-space:pre-wrap">{{.Content}}</div>`
)

// RenderEmailTemplate reads the saved option under the same lock used by
// option updates so concurrent mail delivery sees a complete template.
func RenderEmailTemplate(key string, data map[string]any) (string, error) {
	OptionMapRWMutex.RLock()
	value := OptionMap[key]
	OptionMapRWMutex.RUnlock()
	return renderEmailTemplate(key, value, data)
}

func ValidateEmailTemplate(key, value string) error {
	data := map[string]any{"SystemName": "Example API"}
	switch key {
	case "EmailVerificationTemplate":
		return ValidateEmailVerificationTemplate(value)
	case "EmailPasswordResetTemplate":
		data["Greeting"], data["ActionText"], data["Validity"] = "Reset your password", "Reset password", "Expires in 10 minutes"
		data["Link"], data["ValidMinutes"] = "https://example.invalid/user/reset", 10
	case "EmailBindingTemplate":
		data["Intro"], data["Validity"], data["Code"], data["ValidMinutes"], data["IsOldEmail"] = "Confirm your email address", "Expires in 10 minutes", "123456", 10, false
	case "EmailSecurityNotificationTemplate":
		data["Event"], data["Intro"], data["Advice"] = "Password updated", "Your account security settings have changed", "Contact your administrator if this was not you"
	case "EmailNotificationTemplate":
		data["Title"], data["Content"] = "Notification", "Example notification"
	default:
		return fmt.Errorf("unknown email template: %s", key)
	}
	_, err := renderEmailTemplate(key, value, data)
	return err
}

func renderEmailTemplate(key, value string, data map[string]any) (string, error) {
	if strings.TrimSpace(value) == "" {
		switch key {
		case "EmailPasswordResetTemplate":
			value = DefaultEmailPasswordResetTemplate
		case "EmailBindingTemplate":
			value = DefaultEmailBindingTemplate
		case "EmailSecurityNotificationTemplate":
			value = DefaultEmailSecurityNotificationTemplate
		case "EmailNotificationTemplate":
			value = DefaultEmailNotificationTemplate
		default:
			return "", fmt.Errorf("unknown email template: %s", key)
		}
	}
	tmpl, err := template.New(key).Option("missingkey=error").Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid email template %s: %w", key, err)
	}
	var content strings.Builder
	if err := tmpl.Execute(&content, data); err != nil {
		return "", fmt.Errorf("invalid email template %s: %w", key, err)
	}
	return content.String(), nil
}

func RenderEmailVerificationTemplate(code string) (string, error) {
	return renderEmailVerificationTemplate(EmailVerificationTemplate, code)
}

func ValidateEmailVerificationTemplate(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("email verification template cannot be empty")
	}
	_, err := renderEmailVerificationTemplate(value, "123456")
	return err
}

func renderEmailVerificationTemplate(value string, code string) (string, error) {
	tmpl, err := template.New("email-verification").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid email verification template: %w", err)
	}

	var content strings.Builder
	err = tmpl.Execute(&content, emailVerificationTemplateData{
		SystemName:   SystemName,
		Code:         code,
		ValidMinutes: VerificationValidMinutes,
	})
	if err != nil {
		return "", fmt.Errorf("invalid email verification template: %w", err)
	}
	return content.String(), nil
}
