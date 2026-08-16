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
