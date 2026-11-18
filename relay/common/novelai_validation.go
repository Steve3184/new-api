package common

import (
	"encoding/json"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

const NovelAIMaxSteps = 28

// ValidateNovelAISteps checks OpenAI image fields and NovelAI's nested native
// parameters object when a channel has enabled its 28-step limit.
func ValidateNovelAISteps(body []byte, enabled bool) error {
	if !enabled {
		return nil
	}
	var request map[string]json.RawMessage
	if err := common.Unmarshal(body, &request); err != nil {
		return fmt.Errorf("invalid image request: %w", err)
	}
	var stepsToValidate []json.RawMessage
	if steps, exists := request["steps"]; exists {
		stepsToValidate = append(stepsToValidate, steps)
	}
	if parametersRaw, exists := request["parameters"]; exists {
		var parameters map[string]json.RawMessage
		if err := common.Unmarshal(parametersRaw, &parameters); err != nil {
			return fmt.Errorf("invalid image parameters: %w", err)
		}
		if steps, exists := parameters["steps"]; exists {
			stepsToValidate = append(stepsToValidate, steps)
		}
	}
	for _, steps := range stepsToValidate {
		if string(steps) == "null" {
			continue
		}
		var value int
		if err := common.Unmarshal(steps, &value); err != nil {
			return fmt.Errorf("steps must be an integer no greater than %d", NovelAIMaxSteps)
		}
		if value > NovelAIMaxSteps {
			return fmt.Errorf("steps must be no greater than %d", NovelAIMaxSteps)
		}
		if value < 0 {
			return fmt.Errorf("steps must be non-negative and no greater than %d", NovelAIMaxSteps)
		}
	}
	return nil
}
