package dto

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
)

const (
	// SimulatedRemoteCompactV2Prefix marks compaction payloads produced by the
	// gateway. Native provider payloads remain opaque and are never decoded.
	SimulatedRemoteCompactV2Prefix   = "rcv2-sim/v1:"
	maxSimulatedRemoteCompactV2Bytes = 1 << 20
)

type simulatedRemoteCompactV2Payload struct {
	Version int    `json:"version"`
	Summary string `json:"summary"`
}

// EncodeSimulatedRemoteCompactV2 packages a generated handoff summary in the
// opaque encrypted_content field expected by Codex Remote Compact V2.
func EncodeSimulatedRemoteCompactV2(summary string) (string, error) {
	if strings.TrimSpace(summary) == "" {
		return "", fmt.Errorf("simulated remote compact v2 summary is empty")
	}
	if len(summary) > maxSimulatedRemoteCompactV2Bytes {
		return "", fmt.Errorf("simulated remote compact v2 summary is too large")
	}

	payload, err := kitutil.Marshal(simulatedRemoteCompactV2Payload{
		Version: 1,
		Summary: summary,
	})
	if err != nil {
		return "", fmt.Errorf("marshal simulated remote compact v2 payload: %w", err)
	}
	return SimulatedRemoteCompactV2Prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeSimulatedRemoteCompactV2 returns simulated=false for native provider
// payloads so callers can preserve their opaque encrypted_content unchanged.
func DecodeSimulatedRemoteCompactV2(encryptedContent string) (summary string, simulated bool, err error) {
	if !strings.HasPrefix(encryptedContent, SimulatedRemoteCompactV2Prefix) {
		return "", false, nil
	}

	simulated = true
	encoded := strings.TrimPrefix(encryptedContent, SimulatedRemoteCompactV2Prefix)
	if encoded == "" || len(encoded) > maxSimulatedRemoteCompactV2Bytes*2 {
		return "", true, fmt.Errorf("invalid simulated remote compact v2 payload")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", true, fmt.Errorf("decode simulated remote compact v2 payload: %w", err)
	}
	if len(payloadBytes) > maxSimulatedRemoteCompactV2Bytes {
		return "", true, fmt.Errorf("simulated remote compact v2 payload is too large")
	}

	var payload simulatedRemoteCompactV2Payload
	if err := kitutil.Unmarshal(payloadBytes, &payload); err != nil {
		return "", true, fmt.Errorf("unmarshal simulated remote compact v2 payload: %w", err)
	}
	if payload.Version != 1 || strings.TrimSpace(payload.Summary) == "" {
		return "", true, fmt.Errorf("invalid simulated remote compact v2 payload")
	}
	if len(payload.Summary) > maxSimulatedRemoteCompactV2Bytes {
		return "", true, fmt.Errorf("simulated remote compact v2 summary is too large")
	}

	return payload.Summary, true, nil
}
