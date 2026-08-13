package router

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestRenderSPAIndexUpdatesPublicMetadata(t *testing.T) {
	input := []byte(`<!doctype html><html><head><title>New API</title><link rel="icon" href="/logo.png"><meta name="title" content="New API"><meta name="description" content="old"></head><body></body></html>`)

	output, err := renderSPAIndex(input, "/_custom/img/logo.png", "Configured Site", console_setting.SPAMetaSetting{
		Description:   "Configured description",
		OGType:        "website",
		OGDescription: "Configured Open Graph description",
	})
	require.NoError(t, err)

	_, err = html.Parse(strings.NewReader(string(output)))
	require.NoError(t, err)
	assert.Contains(t, string(output), "<title>Configured Site</title>")
	assert.Contains(t, string(output), `<meta name="title" content="Configured Site"/>`)
	assert.Contains(t, string(output), `<meta property="og:title" content="Configured Site"/>`)
	assert.Contains(t, string(output), `href="/_custom/img/logo.png"`)
	assert.Contains(t, string(output), `content="Configured description"`)
	assert.Contains(t, string(output), `property="og:type"`)
	assert.Contains(t, string(output), `content="Configured Open Graph description"`)
}
