package service

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// Claude Code client detection, ported from sub2api's ClaudeCodeValidator.
//
// This is a usage-policy gate, not a security boundary: the User-Agent, the
// anthropic headers and the system prompt are all client-controlled, so a
// caller that decides to impersonate Claude Code can pass. The gate exists to
// stop keys from being pointed at unrelated clients by mistake.
//
// Requests that Claude Code sends to check connectivity must never be rejected,
// otherwise its desktop app shows a warning: probes carry max_tokens=1 and a
// bare user message without the Claude Code system prompt.

const claudeCodeContextKey = "claude_code_client"

// claudeCodeUAPattern matches the official CLI only, case-insensitively.
var claudeCodeUAPattern = regexp.MustCompile(`(?i)^claude-cli/\d+\.\d+\.\d+`)

// claudeCodeSystemPromptThreshold is the Dice similarity needed for a system
// block to count as the Claude Code identity prompt, matching sub2api and
// claude-relay-service.
const claudeCodeSystemPromptThreshold = 0.5

// claudeCodeIdentityPrompts are the official identity prompts Claude Code
// sends. Only these take part in fuzzy matching: the shorter generic templates
// below would otherwise match ordinary assistant prompts.
var claudeCodeIdentityPrompts = []string{
	"You are Claude Code, Anthropic's CLI for Claude.",
	"You are a Claude agent, built on Anthropic's Claude Agent SDK.",
	"You are Claude Code, Anthropic's CLI for Claude, running within the Claude Agent SDK.",
	"You are a file search specialist for Claude Code, Anthropic's official CLI for Claude.",
}

// claudeCodeGenericPrompts are official prompts that are too short or too
// generic to fuzzy match, so they are compared as an exact prefix instead.
var claudeCodeGenericPrompts = []string{
	"You are a helpful AI assistant tasked with summarizing conversations.",
	"You are an interactive CLI tool that helps users",
}

const (
	// The auto-mode security monitor classifier is identified by its fixed
	// prefix, a length floor and its section markers rather than exact prose,
	// so wording changes upstream do not break detection.
	claudeCodeSecurityMonitorPrefix = "You are a security monitor for autonomous AI coding agents."
	claudeCodeSecurityMonitorMinLen = 10_000

	// Claude Code injects a billing attribution block as the first system
	// block. The format is more stable than the identity prose.
	claudeCodeBillingHeaderPrefix = "x-anthropic-billing-header"
	claudeCodeEntrypointMarker    = "cc_entrypoint="
)

var claudeCodeSecurityMonitorMarkers = []string{
	"## Threat Model",
	"- `<transcript>`:",
	"## HARD BLOCK",
	"## SOFT BLOCK",
	"## Classification Process",
	"## Output Format",
	"<block>yes</block>",
	"<block>no</block>",
}

// IsClaudeCodeClient reports whether the request comes from a Claude Code
// client. The result is cached on the context so retries do not re-parse.
func IsClaudeCodeClient(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if cached, ok := c.Get(claudeCodeContextKey); ok {
		if value, ok := cached.(bool); ok {
			return value
		}
	}
	result := detectClaudeCodeClient(c)
	c.Set(claudeCodeContextKey, result)
	return result
}

func detectClaudeCodeClient(c *gin.Context) bool {
	// Step 1: the User-Agent is required, and it is the cheap rejection that
	// keeps ordinary traffic from paying for a body parse.
	if !claudeCodeUAPattern.MatchString(c.GetHeader("User-Agent")) {
		return false
	}
	// Step 2: only the Messages endpoint carries the identity prompt.
	if !strings.Contains(c.Request.URL.Path, "messages") {
		return true
	}
	if strings.HasSuffix(c.Request.URL.Path, "/messages/count_tokens") {
		return true
	}

	body, err := common.GetBodyStorage(c)
	if err != nil {
		return false
	}
	raw, err := body.Bytes()
	if err != nil {
		return false
	}
	if _, err := body.Seek(0, 0); err != nil {
		return false
	}
	var payload map[string]any
	if common.Unmarshal(raw, &payload) != nil {
		return false
	}

	// Step 3: a probe asks for a single output token and proves the endpoint
	// works; it carries no identity prompt and must always pass.
	if isClaudeCodeProbe(payload) {
		return true
	}

	// Step 4: the remaining Messages traffic must look like Claude Code.
	if !hasClaudeCodeSystemPrompt(payload) {
		return false
	}
	if c.GetHeader("X-App") == "" ||
		c.GetHeader("anthropic-beta") == "" ||
		c.GetHeader("anthropic-version") == "" {
		return false
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		return false
	}
	userID, ok := metadata["user_id"].(string)
	return ok && userID != ""
}

// isClaudeCodeProbe matches the connectivity check Claude Code sends: one
// output token, and at most a single user message.
func isClaudeCodeProbe(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	oneToken := false
	switch maxTokens := payload["max_tokens"].(type) {
	case float64:
		oneToken = maxTokens == 1
	case int:
		oneToken = maxTokens == 1
	case int64:
		oneToken = maxTokens == 1
	}
	if !oneToken {
		return false
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 1 {
		return false
	}
	message, ok := messages[0].(map[string]any)
	if !ok {
		return false
	}
	role, _ := message["role"].(string)
	return role == "user"
}

func hasClaudeCodeSystemPrompt(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	if _, ok := payload["model"].(string); !ok {
		return false
	}
	entries, ok := payload["system"].([]any)
	if !ok {
		return false
	}
	for _, entry := range entries {
		block, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		text, ok := block["text"].(string)
		if !ok || text == "" {
			continue
		}
		if isClaudeCodeSecurityMonitorPrompt(text) {
			return true
		}
		if strings.HasPrefix(text, claudeCodeBillingHeaderPrefix) &&
			strings.Contains(text, claudeCodeEntrypointMarker) {
			return true
		}
		if claudeCodePromptSimilarity(text) >= claudeCodeSystemPromptThreshold {
			return true
		}
	}
	return false
}

func isClaudeCodeSecurityMonitorPrompt(text string) bool {
	if len(text) < claudeCodeSecurityMonitorMinLen ||
		!strings.HasPrefix(text, claudeCodeSecurityMonitorPrefix) {
		return false
	}
	for _, marker := range claudeCodeSecurityMonitorMarkers {
		if !strings.Contains(text, marker) {
			return false
		}
	}
	return true
}

func claudeCodePromptSimilarity(text string) float64 {
	normalizedText := normalizeClaudeCodePrompt(text)
	if normalizedText == "" {
		return 0
	}
	lowerText := strings.ToLower(normalizedText)
	for _, generic := range claudeCodeGenericPrompts {
		if strings.HasPrefix(lowerText, strings.ToLower(normalizeClaudeCodePrompt(generic))) {
			return 1
		}
	}
	best := 0.0
	for _, template := range claudeCodeIdentityPrompts {
		if score := diceCoefficient(lowerText, strings.ToLower(normalizeClaudeCodePrompt(template))); score > best {
			best = score
		}
	}
	return best
}

func normalizeClaudeCodePrompt(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// diceCoefficient is the Sørensen–Dice coefficient over character bigrams.
func diceCoefficient(a, b string) float64 {
	if a == b {
		return 1
	}
	bigramsA := characterBigrams(a)
	bigramsB := characterBigrams(b)
	if len(bigramsA) == 0 || len(bigramsB) == 0 {
		return 0
	}
	intersection := 0
	for bigram, countA := range bigramsA {
		if countB, exists := bigramsB[bigram]; exists {
			intersection += min(countA, countB)
		}
	}
	totalA := 0
	for _, count := range bigramsA {
		totalA += count
	}
	totalB := 0
	for _, count := range bigramsB {
		totalB += count
	}
	return float64(2*intersection) / float64(totalA+totalB)
}

func characterBigrams(text string) map[string]int {
	runes := []rune(strings.ToLower(text))
	if len(runes) < 2 {
		return nil
	}
	bigrams := make(map[string]int, len(runes)-1)
	for i := range len(runes) - 1 {
		bigrams[string(runes[i:i+2])]++
	}
	return bigrams
}
