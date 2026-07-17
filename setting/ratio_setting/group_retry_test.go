package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRetryTimes(t *testing.T) {
	previous := GroupRetryTimes2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRetryTimesByJSONString(previous))
	})

	require.NoError(t, UpdateGroupRetryTimesByJSONString(`{"no_retry":0,"custom":4}`))

	assert.Equal(t, 0, GetGroupRetryTimes("no_retry", 3))
	assert.Equal(t, 4, GetGroupRetryTimes("custom", 3))
	assert.Equal(t, 3, GetGroupRetryTimes("inherited", 3))
	require.Error(t, CheckGroupRetryTimes(`{"invalid":-1}`))
	require.Error(t, CheckGroupRetryTimes(`{"invalid":11}`))
	require.Error(t, CheckGroupRetryTimes(`{"":1}`))
}
