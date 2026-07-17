package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidCustomTabURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "internal path", url: "/internal-page", want: true},
		{name: "https external page", url: "https://example.com/docs", want: true},
		{name: "http external page", url: "http://example.com", want: true},
		{name: "trimmed external page", url: "  https://example.com  ", want: true},
		{name: "bare hostname", url: "example.com", want: false},
		{name: "javascript URL", url: "javascript:alert(1)", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidCustomTabURL(tt.url))
		})
	}
}
