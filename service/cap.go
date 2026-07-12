package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type capConfigResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// SyncCapDifficulty updates a Cap Standalone site key. The difficulty must be
// enforced by Cap when it signs the challenge, not by a browser-controlled field.
func SyncCapDifficulty(ctx context.Context, serverURL, apiKey, siteKey string, difficulty int) error {
	baseURL := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if baseURL == "" || strings.TrimSpace(apiKey) == "" || strings.TrimSpace(siteKey) == "" {
		return nil
	}

	payload, err := common.Marshal(map[string]int{"difficulty": difficulty})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/server/keys/%s/config", baseURL, url.PathEscape(siteKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to update Cap difficulty: %w", err)
	}
	defer resp.Body.Close()

	var result capConfigResponse
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return fmt.Errorf("invalid Cap configuration response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.Success {
		if result.Error == "" {
			result.Error = resp.Status
		}
		return fmt.Errorf("Cap rejected difficulty update: %s", result.Error)
	}
	return nil
}
