package console_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupAccessRulesFiltersUnknownGroups(t *testing.T) {
	previous := ratio_setting.GetGroupRatioCopy()
	t.Cleanup(func() {
		data, err := common.Marshal(previous)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(data)))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"free":1,"go":1}`))

	normalized, err := NormalizeGroupAccessRules(`[
		{"group":"missing","conditions":[{"type":"balance","min_quota":10}]},
		{"group":"free","logic":"OR","rules":[
			{"conditions":[{"type":"oauth","providers":[" GitHub ","github"]}]},
			{"logic":"and","conditions":[{"type":"github_days","days":90}]}
		]}
	]`)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"group":"free","logic":"or","rules":[{"logic":"and","conditions":[{"type":"oauth","providers":["github"]}]},{"logic":"and","conditions":[{"type":"github_registration_days","days":90}]}]}]`, normalized)
}

func TestNormalizeGroupAccessRulesRejectsInvalidCondition(t *testing.T) {
	_, err := NormalizeGroupAccessRules(`[{"group":"default","conditions":[{"type":"unknown"}]}]`)
	assert.Error(t, err)
}
