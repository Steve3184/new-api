package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderEmailVerificationTemplateEscapesDynamicValues(t *testing.T) {
	previousTemplate := EmailVerificationTemplate
	previousSystemName := SystemName
	previousValidMinutes := VerificationValidMinutes
	t.Cleanup(func() {
		EmailVerificationTemplate = previousTemplate
		SystemName = previousSystemName
		VerificationValidMinutes = previousValidMinutes
	})

	EmailVerificationTemplate = `<p>{{.SystemName}}</p><div>{{.Code}}</div><span>{{.ValidMinutes}}</span>`
	SystemName = `Example <script>alert("unsafe")</script>`
	VerificationValidMinutes = 15

	content, err := RenderEmailVerificationTemplate("12&3456")
	require.NoError(t, err)
	require.Contains(t, content, `Example &lt;script&gt;alert(&#34;unsafe&#34;)&lt;/script&gt;`)
	require.Contains(t, content, "12&amp;3456")
	require.Contains(t, content, ">15<")
}

func TestDefaultEmailVerificationTemplateRendersVerificationData(t *testing.T) {
	previousTemplate := EmailVerificationTemplate
	previousSystemName := SystemName
	previousValidMinutes := VerificationValidMinutes
	t.Cleanup(func() {
		EmailVerificationTemplate = previousTemplate
		SystemName = previousSystemName
		VerificationValidMinutes = previousValidMinutes
	})

	EmailVerificationTemplate = DefaultEmailVerificationTemplate
	SystemName = "Example API"
	VerificationValidMinutes = 10

	content, err := RenderEmailVerificationTemplate("123456")
	require.NoError(t, err)
	require.Contains(t, content, "Example API")
	require.Contains(t, content, ">123456<")
	require.Contains(t, content, "10 minutes")
}

func TestValidateEmailVerificationTemplateRejectsInvalidTemplates(t *testing.T) {
	require.Error(t, ValidateEmailVerificationTemplate(""))
	require.Error(t, ValidateEmailVerificationTemplate("{{if .Code}}"))
	require.NoError(t, ValidateEmailVerificationTemplate("<p>{{.Code}}</p>"))
}
