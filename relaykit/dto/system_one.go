package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
)

type SystemOneQuestion struct {
	Type         string                     `json:"type"`
	Instructions json.RawMessage            `json:"instructions"`
	Criteria     json.RawMessage            `json:"criteria,omitempty"`
	Extra        map[string]json.RawMessage `json:"-"`
}

func (q *SystemOneQuestion) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := kitutil.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, ok := fields["type"]; ok {
		if err := kitutil.Unmarshal(raw, &q.Type); err != nil {
			return err
		}
		delete(fields, "type")
	}
	if raw, ok := fields["instructions"]; ok {
		q.Instructions = append(q.Instructions[:0], raw...)
		delete(fields, "instructions")
	}
	if raw, ok := fields["criteria"]; ok {
		q.Criteria = append(q.Criteria[:0], raw...)
		delete(fields, "criteria")
	}
	q.Extra = fields
	return nil
}
func (q SystemOneQuestion) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(q.Extra)+3)
	for key, value := range q.Extra {
		fields[key] = value
	}
	typeValue, err := kitutil.Marshal(q.Type)
	if err != nil {
		return nil, err
	}
	fields["type"] = typeValue
	if len(q.Instructions) > 0 {
		fields["instructions"] = q.Instructions
	}
	if len(q.Criteria) > 0 {
		fields["criteria"] = q.Criteria
	}
	return kitutil.Marshal(fields)
}

type SystemOneRequest struct {
	BaseRequest
	Model     string                       `json:"model"`
	State     json.RawMessage              `json:"state"`
	Questions map[string]SystemOneQuestion `json:"questions"`
	Extra     map[string]json.RawMessage   `json:"-"`
}

func (r *SystemOneRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := kitutil.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, ok := fields["model"]; ok {
		if err := kitutil.Unmarshal(raw, &r.Model); err != nil {
			return err
		}
		delete(fields, "model")
	}
	if raw, ok := fields["state"]; ok {
		r.State = append(r.State[:0], raw...)
		delete(fields, "state")
	}
	if raw, ok := fields["questions"]; ok {
		if err := kitutil.Unmarshal(raw, &r.Questions); err != nil {
			return err
		}
		delete(fields, "questions")
	}
	r.Extra = fields
	return nil
}

func (r SystemOneRequest) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(r.Extra)+3)
	for key, value := range r.Extra {
		fields[key] = value
	}
	modelValue, err := kitutil.Marshal(r.Model)
	if err != nil {
		return nil, err
	}
	fields["model"] = modelValue
	if len(r.State) > 0 {
		fields["state"] = r.State
	}
	questionsValue, err := kitutil.Marshal(r.Questions)
	if err != nil {
		return nil, err
	}
	fields["questions"] = questionsValue
	return kitutil.Marshal(fields)
}

func (r *SystemOneRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

func (r *SystemOneRequest) GetTokenCountMeta() *types.TokenCountMeta {
	body, err := kitutil.Marshal(r)
	if err != nil {
		return &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer}
	}
	return &types.TokenCountMeta{CombineText: string(body), TokenType: types.TokenTypeTokenizer}
}

func (r *SystemOneRequest) Validate() error {
	if strings.TrimSpace(r.Model) == "" {
		return errors.New("model is required")
	}
	if !validJSONValue(r.State) || !validStateShape(r.State) {
		return errors.New("state is required and must be a JSON string, object, or array")
	}
	if len(r.Questions) == 0 {
		return errors.New("questions must be a non-empty object")
	}
	for id, question := range r.Questions {
		if strings.TrimSpace(id) == "" {
			return errors.New("question id must not be empty")
		}
		if question.Type != "noul" && question.Type != "choice" && question.Type != "score" {
			return fmt.Errorf("question %q has an invalid type", id)
		}
		if !validJSONValue(question.Instructions) {
			return fmt.Errorf("question %q instructions are required and must be valid JSON", id)
		}
		switch question.Type {
		case "choice":
			var criteria map[string]json.RawMessage
			if err := kitutil.Unmarshal(question.Criteria, &criteria); err != nil || len(criteria) == 0 || len(criteria) > 255 {
				return fmt.Errorf("question %q choice criteria must be a non-empty object with at most 255 options", id)
			}
			for _, description := range criteria {
				if len(bytes.TrimSpace(description)) == 0 || !kitutil.Valid(description) {
					return fmt.Errorf("question %q choice criteria values must be valid JSON", id)
				}
			}
		case "score":
			var criteria []json.RawMessage
			if err := kitutil.Unmarshal(question.Criteria, &criteria); err != nil || len(criteria) < 2 || len(criteria) > 10 {
				return fmt.Errorf("question %q score criteria must contain 2 to 10 levels", id)
			}
			for _, item := range criteria {
				if !validJSONValue(item) {
					return fmt.Errorf("question %q score criteria items must be valid non-null JSON", id)
				}
			}
		}
	}
	return nil
}

func validJSONValue(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && kitutil.Valid(trimmed)
}

func validStateShape(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return false
	}
	return trimmed[0] == '"' || trimmed[0] == '{' || trimmed[0] == '['
}
