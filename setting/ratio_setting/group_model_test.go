package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupModelMaps(t *testing.T) {
	previousDefaultModels := GroupDefaultModel2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupDefaultModelByJSONString(previousDefaultModels))
	})

	require.NoError(t, CheckGroupModelMap(`{"default":"gpt-5.6-sol"}`))
	require.NoError(t, UpdateGroupDefaultModelByJSONString(`{"default":"gpt-5.6-sol"}`))

	assert.Equal(t, "gpt-5.6-sol", GetGroupDefaultModel("default"))
	assert.Empty(t, GetGroupDefaultModel("missing"))
	require.Error(t, CheckGroupModelMap(`{"default":""}`))
}

func TestUpdateGroupDefaultModelIgnoresDeletedGroups(t *testing.T) {
	previousRatios := GroupRatio2JSONString()
	previousDefaultModels := GroupDefaultModel2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(previousRatios))
		require.NoError(t, UpdateGroupDefaultModelByJSONString(previousDefaultModels))
	})

	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1}`))
	require.NoError(t, UpdateGroupDefaultModelByJSONString(`{"default":"gpt-5.6-sol","deleted":"legacy-model"}`))

	assert.Equal(t, "gpt-5.6-sol", GetGroupDefaultModel("default"))
	assert.Empty(t, GetGroupDefaultModel("deleted"))
}
