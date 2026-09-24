package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemOneRequestValidatePreservesJSONValues(t *testing.T) {
	request := SystemOneRequest{
		Model: "jev-1.13",
		State: json.RawMessage(`{"ticket":{"status":"failed"},"attempts":2}`),
		Questions: map[string]SystemOneQuestion{
			"route": {
				Type:         "choice",
				Instructions: json.RawMessage(`{"prompt":"Choose a team"}`),
				Criteria:     json.RawMessage(`{"billing":"Payment issues","support":"Other issues"}`),
			},
			"priority": {
				Type:         "score",
				Instructions: json.RawMessage(`"Rate urgency"`),
				Criteria:     json.RawMessage(`["low","high"]`),
			},
			"urgent": {
				Type:         "noul",
				Instructions: json.RawMessage(`"Is this urgent?"`),
			},
		},
	}

	require.NoError(t, request.Validate())
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &got))
	assert.JSONEq(t, `{"ticket":{"status":"failed"},"attempts":2}`, string(got["state"]))
	assert.JSONEq(t, `{"route":{"type":"choice","instructions":{"prompt":"Choose a team"},"criteria":{"billing":"Payment issues","support":"Other issues"}},"priority":{"type":"score","instructions":"Rate urgency","criteria":["low","high"]},"urgent":{"type":"noul","instructions":"Is this urgent?"}}`, string(got["questions"]))
}

func TestSystemOneRequestValidateRejectsInvalidRequiredFieldsAndCriteria(t *testing.T) {
	valid := func() SystemOneRequest {
		return SystemOneRequest{
			Model: "jev-1.13",
			State: json.RawMessage(`"payment failed"`),
			Questions: map[string]SystemOneQuestion{
				"urgent": {Type: "noul", Instructions: json.RawMessage(`"Is this urgent?"`)},
			},
		}
	}

	tests := []struct {
		name   string
		change func(*SystemOneRequest)
	}{
		{"blank model", func(r *SystemOneRequest) { r.Model = "  " }},
		{"missing state", func(r *SystemOneRequest) { r.State = nil }},
		{"null state", func(r *SystemOneRequest) { r.State = json.RawMessage(`null`) }},
		{"empty questions", func(r *SystemOneRequest) { r.Questions = nil }},
		{"missing question type", func(r *SystemOneRequest) { q := r.Questions["urgent"]; q.Type = ""; r.Questions["urgent"] = q }},
		{"missing instructions", func(r *SystemOneRequest) { q := r.Questions["urgent"]; q.Instructions = nil; r.Questions["urgent"] = q }},
		{"invalid choice criteria", func(r *SystemOneRequest) {
			q := r.Questions["urgent"]
			q.Type = "choice"
			q.Criteria = json.RawMessage(`[]`)
			r.Questions["urgent"] = q
		}},
		{"invalid score criteria", func(r *SystemOneRequest) {
			q := r.Questions["urgent"]
			q.Type = "score"
			q.Criteria = json.RawMessage(`{"low":"Low"}`)
			r.Questions["urgent"] = q
		}},
		{"unknown question type", func(r *SystemOneRequest) { q := r.Questions["urgent"]; q.Type = "binary"; r.Questions["urgent"] = q }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := valid()
			test.change(&request)
			assert.Error(t, request.Validate())
		})
	}
}

func TestSystemOneQuestionNoulCriteriaIsOptional(t *testing.T) {
	request := SystemOneRequest{
		Model: "jev-1.13-free",
		State: json.RawMessage(`"payment failed"`),
		Questions: map[string]SystemOneQuestion{
			"urgent": {Type: "noul", Instructions: json.RawMessage(`"Is this urgent?"`)},
		},
	}
	assert.NoError(t, request.Validate())
}
