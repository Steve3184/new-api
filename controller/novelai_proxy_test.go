package controller

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNovelAIProxyAllowsOnlyReadOnlyTagSuggestions(t *testing.T) {
	for _, test := range []struct {
		method   string
		endpoint string
		want     bool
	}{
		{method: http.MethodGet, endpoint: "generate-image/suggest-tags", want: true},
		{method: http.MethodPost, endpoint: "generate-image/suggest-tags"},
		{method: http.MethodPost, endpoint: "augment-image"},
		{method: http.MethodPost, endpoint: "encode-vibe"},
		{method: http.MethodPost, endpoint: "upscale"},
		{method: http.MethodPost, endpoint: "annotate-image"},
		{method: http.MethodGet, endpoint: "user/subscription"},
		{method: http.MethodGet, endpoint: "../user/subscription"},
	} {
		t.Run(test.method+" /v1/ai/"+test.endpoint, func(t *testing.T) {
			assert.Equal(t, test.want, novelAIProxyRouteAllowed(test.method, test.endpoint))
		})
	}
}
