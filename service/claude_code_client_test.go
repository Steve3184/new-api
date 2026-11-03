package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const claudeCodeIdentityPrompt = "You are Claude Code, Anthropic's CLI for Claude."

// newClaudeCodeRequest builds a gin context backed by real body storage so the
// detector can read the payload the way the distributor does.
func newClaudeCodeRequest(t *testing.T, path, userAgent, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userAgent != "" {
		c.Request.Header.Set("User-Agent", userAgent)
	}
	storage, err := common.CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })
	c.Set(common.KeyBodyStorage, storage)
	return c
}

func TestIsClaudeCodeClient(t *testing.T) {
	const claudeUA = "claude-cli/2.1.22 (external, cli)"

	messagesBody := `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are Claude Code, Anthropic's CLI for Claude."}],"messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":"user_abc"}}`

	tests := []struct {
		name      string
		path      string
		userAgent string
		body      string
		headers   map[string]string
		want      bool
	}{
		{
			name:      "rejects a request without the Claude Code user agent",
			path:      "/v1/messages",
			userAgent: "OpenAI/Python 1.0",
			body:      messagesBody,
			headers:   map[string]string{"X-App": "cli", "anthropic-beta": "x", "anthropic-version": "2023-06-01"},
			want:      false,
		},
		{
			name:      "accepts a messages request from the official CLI",
			path:      "/v1/messages",
			userAgent: claudeUA,
			body:      messagesBody,
			headers:   map[string]string{"X-App": "cli", "anthropic-beta": "x", "anthropic-version": "2023-06-01"},
			want:      true,
		},
		{
			name:      "treats the user agent case-insensitively",
			path:      "/v1/messages",
			userAgent: "Claude-CLI/2.1.22",
			body:      messagesBody,
			headers:   map[string]string{"X-App": "cli", "anthropic-beta": "x", "anthropic-version": "2023-06-01"},
			want:      true,
		},
		{
			name:      "rejects a messages request without the identity prompt",
			path:      "/v1/messages",
			userAgent: claudeUA,
			body:      `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are a helpful assistant."}],"messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":"user_abc"}}`,
			headers:   map[string]string{"X-App": "cli", "anthropic-beta": "x", "anthropic-version": "2023-06-01"},
			want:      false,
		},
		{
			name:      "rejects when a required anthropic header is missing",
			path:      "/v1/messages",
			userAgent: claudeUA,
			body:      messagesBody,
			headers:   map[string]string{"X-App": "cli", "anthropic-version": "2023-06-01"},
			want:      false,
		},
		{
			name:      "rejects when metadata is missing",
			path:      "/v1/messages",
			userAgent: claudeUA,
			body:      `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are Claude Code, Anthropic's CLI for Claude."}],"messages":[{"role":"user","content":"hi"}]}`,
			headers:   map[string]string{"X-App": "cli", "anthropic-beta": "x", "anthropic-version": "2023-06-01"},
			want:      false,
		},
		{
			name:      "accepts the billing attribution block as the identity prompt",
			path:      "/v1/messages",
			userAgent: claudeUA,
			body:      `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"x-anthropic-billing-header cc_version=2.1.22; cc_entrypoint=cli; cch=abc;"}],"messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":"user_abc"}}`,
			headers:   map[string]string{"X-App": "cli", "anthropic-beta": "x", "anthropic-version": "2023-06-01"},
			want:      true,
		},
		{
			name:      "always passes a connectivity probe even without identity headers",
			path:      "/v1/messages",
			userAgent: claudeUA,
			body:      `{"model":"claude-sonnet-4-5","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`,
			want:      true,
		},
		{
			name:      "passes non-messages paths on the user agent alone",
			path:      "/v1/models",
			userAgent: claudeUA,
			body:      `{}`,
			want:      true,
		},
		{
			name:      "rejects non-messages paths for other clients",
			path:      "/v1/chat/completions",
			userAgent: "curl/8.0",
			body:      `{"model":"gpt-4"}`,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClaudeCodeRequest(t, tc.path, tc.userAgent, tc.body)
			for key, value := range tc.headers {
				c.Request.Header.Set(key, value)
			}
			assert.Equal(t, tc.want, IsClaudeCodeClient(c))
			// The decision is cached so retries do not re-parse the body.
			assert.Equal(t, tc.want, IsClaudeCodeClient(c))
		})
	}
}

func TestClaudeCodeIdentityPromptSimilarity(t *testing.T) {
	// A real request carries a longer identity block than the template; the
	// Dice threshold has to keep accepting it.
	padded := claudeCodeIdentityPrompt + " You are an interactive CLI tool that helps users with software engineering tasks."
	assert.GreaterOrEqual(t, claudeCodePromptSimilarity(padded), claudeCodeSystemPromptThreshold)

	// Unrelated prose must stay below the threshold.
	assert.Less(t, claudeCodePromptSimilarity("Translate the following document into French."), claudeCodeSystemPromptThreshold)

	// Ordinary assistant prompts share enough wording with the summarizer
	// template that fuzzy matching alone would accept them, so they are
	// prefix-matched instead and must be refused here.
	for _, generic := range []string{
		"You are a helpful assistant.",
		"You are a helpful AI assistant.",
		"You are a helpful assistant for coding questions.",
	} {
		assert.Less(t, claudeCodePromptSimilarity(generic), claudeCodeSystemPromptThreshold, generic)
	}

	// The official generic prompts themselves still match.
	for _, official := range claudeCodeGenericPrompts {
		assert.GreaterOrEqual(t, claudeCodePromptSimilarity(official), claudeCodeSystemPromptThreshold, official)
	}
}
