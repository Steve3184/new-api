package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildSelfUserDataIncludesGitHubCreatedAt(t *testing.T) {
	const createdAt int64 = 1_646_880_000

	data := buildSelfUserData(&model.User{
		Id:              7,
		Username:        "github-user",
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		GitHubId:        "12345",
		GitHubCreatedAt: createdAt,
	})

	assert.Equal(t, "12345", data["github_id"])
	assert.Equal(t, createdAt, data["github_created_at"])
}
