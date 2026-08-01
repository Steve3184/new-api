package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClearBindingAcceptsUserFieldNameFromAdminClient(t *testing.T) {
	truncateTables(t)

	user := User{
		Username: "github-unbind",
		Password: "password",
		GitHubId: "github-user-id",
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, user.ClearBinding("github_id"))

	var cleared User
	require.NoError(t, DB.First(&cleared, user.Id).Error)
	assert.Empty(t, cleared.GitHubId)
}
