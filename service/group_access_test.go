package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupAccessRuleEvaluatesOAuthOrGitHubAgeAndBalance(t *testing.T) {
	user := &groupAccessUser{
		user: &model.User{
			Quota:           500,
			GitHubCreatedAt: time.Now().Add(-91 * 24 * time.Hour).Unix(),
		},
		providers: map[string]struct{}{"github": {}},
	}
	rule := console_setting.GroupAccessRule{
		Logic: "or",
		Rules: []console_setting.GroupAccessRule{
			{Conditions: []console_setting.GroupAccessCondition{{Type: "oauth", Providers: []string{"linuxdo"}}}},
			{
				Logic: "and",
				Conditions: []console_setting.GroupAccessCondition{
					{Type: "oauth", Providers: []string{"github"}},
					{Type: "github_registration_days", Days: 90},
				},
			},
		},
	}

	require.True(t, user.evaluateRule(rule))
	user.user.GitHubCreatedAt = time.Now().Add(-89 * 24 * time.Hour).Unix()
	assert.False(t, user.evaluateRule(rule))
	user.user.GitHubCreatedAt = time.Now().Add(-10 * 24 * time.Hour).Unix()
	assert.False(t, user.evaluateRule(rule))
	user.providers["linuxdo"] = struct{}{}
	assert.True(t, user.evaluateRule(rule))

	user.providers = map[string]struct{}{}
	user.user.GitHubCreatedAt = 0
	assert.False(t, user.evaluateCondition(console_setting.GroupAccessCondition{Type: "github_registration_days", Days: 0}))
	assert.True(t, (&groupAccessUser{user: &model.User{Quota: 500}}).evaluateCondition(console_setting.GroupAccessCondition{Type: "balance", MinQuota: 500}))
}
