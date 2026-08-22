package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRefreshGitHubCreatedAtUsesConfiguredTokenAndUpdatesLegacyUser(t *testing.T) {
	previousBaseURL := githubAPIBaseURL
	previousToken := common.GitHubAPIToken
	previousDB := model.DB
	t.Cleanup(func() {
		githubAPIBaseURL = previousBaseURL
		common.GitHubAPIToken = previousToken
		model.DB = previousDB
	})

	createdAt := time.Date(2012, time.March, 14, 9, 26, 53, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/user/12345", r.URL.Path)
		assert.Equal(t, "Bearer github-api-token", r.Header.Get("Authorization"))
		assert.Equal(t, "new-api", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":12345,"login":"legacy-user","created_at":"2012-03-14T09:26:53Z"}`))
	}))
	defer server.Close()
	githubAPIBaseURL = server.URL
	common.GitHubAPIToken = "github-api-token"

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db

	user := &model.User{
		Username: "legacy-github-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		GitHubId: "12345",
	}
	require.NoError(t, db.Create(user).Error)

	require.NoError(t, RefreshGitHubCreatedAt(context.Background(), user))
	assert.Equal(t, createdAt.Unix(), user.GitHubCreatedAt)

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, createdAt.Unix(), stored.GitHubCreatedAt)
}

func TestGitHubProviderGetUserCreatedAtUsesUsersEndpointForLegacyUsername(t *testing.T) {
	previousBaseURL := githubAPIBaseURL
	previousToken := common.GitHubAPIToken
	t.Cleanup(func() {
		githubAPIBaseURL = previousBaseURL
		common.GitHubAPIToken = previousToken
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/legacy-user", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":12345,"login":"legacy-user","created_at":"2018-06-01T00:00:00Z"}`))
	}))
	defer server.Close()
	githubAPIBaseURL = server.URL
	common.GitHubAPIToken = ""

	createdAt, err := (&GitHubProvider{}).GetUserCreatedAt(context.Background(), "legacy-user")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2018, time.June, 1, 0, 0, 0, 0, time.UTC).Unix(), createdAt)
}
