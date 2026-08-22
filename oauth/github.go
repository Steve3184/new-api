package oauth

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("github", &GitHubProvider{})
}

// GitHubProvider implements OAuth for GitHub
type GitHubProvider struct{}

type gitHubOAuthResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type gitHubUser struct {
	Id        int64     `json:"id"`    // GitHub numeric ID (permanent, never changes)
	Login     string    `json:"login"` // GitHub username (can be changed by user)
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"` // GitHub account creation time
}

var githubAPIBaseURL = "https://api.github.com"

func (p *GitHubProvider) GetName() string {
	return "GitHub"
}

func (p *GitHubProvider) IsEnabled() bool {
	return common.GitHubOAuthEnabled
}

func (p *GitHubProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logger.LogDebug(ctx, "[OAuth-GitHub] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	values := map[string]string{
		"client_id":     common.GitHubClientId,
		"client_secret": common.GitHubClientSecret,
		"code":          code,
	}
	jsonData, err := common.Marshal(values)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "GitHub"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-GitHub] ExchangeToken response status: %d", res.StatusCode)

	var oAuthResponse gitHubOAuthResponse
	err = common.DecodeJson(res.Body, &oAuthResponse)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if oAuthResponse.AccessToken == "" {
		logger.LogError(ctx, "[OAuth-GitHub] ExchangeToken failed: empty access token")
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "GitHub"})
	}

	logger.LogDebug(ctx, "[OAuth-GitHub] ExchangeToken success: scope=%s", oAuthResponse.Scope)

	return &OAuthToken{
		AccessToken: oAuthResponse.AccessToken,
		TokenType:   oAuthResponse.TokenType,
		Scope:       oAuthResponse.Scope,
	}, nil
}

func (p *GitHubProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logger.LogDebug(ctx, "[OAuth-GitHub] GetUserInfo: fetching user info")

	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(githubAPIBaseURL, "/")+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "GitHub"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-GitHub] GetUserInfo response status: %d", res.StatusCode)

	// Check for non-200 status codes before attempting to decode
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		logger.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] GetUserInfo failed: status=%d, body=%s", res.StatusCode, bodyStr))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "GitHub"}, fmt.Sprintf("status %d", res.StatusCode))
	}

	var githubUser gitHubUser
	err = common.DecodeJson(res.Body, &githubUser)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-GitHub] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	if githubUser.Id == 0 || githubUser.Login == "" {
		logger.LogError(ctx, "[OAuth-GitHub] GetUserInfo failed: empty id or login field")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "GitHub"})
	}

	logger.LogDebug(ctx, "[OAuth-GitHub] GetUserInfo success: id=%d, login=%s, name=%s, email=%s",
		githubUser.Id, githubUser.Login, githubUser.Name, githubUser.Email)

	return &OAuthUser{
		ProviderUserID: strconv.FormatInt(githubUser.Id, 10), // Use numeric ID as primary identifier
		Username:       githubUser.Login,
		DisplayName:    githubUser.Name,
		Email:          githubUser.Email,
		RegisteredAt:   githubUser.CreatedAt.Unix(),
		Extra: map[string]any{
			"legacy_id": githubUser.Login, // Store login for migration from old accounts
		},
	}, nil
}

// GetUserCreatedAt fetches a GitHub account's creation timestamp for a bound
// user. Numeric GitHub IDs use the account-id endpoint, while legacy bindings
// that still contain a username use the public users endpoint.
func (p *GitHubProvider) GetUserCreatedAt(ctx context.Context, providerUserID string) (int64, error) {
	providerUserID = strings.TrimSpace(providerUserID)
	if providerUserID == "" {
		return 0, fmt.Errorf("GitHub id is empty")
	}

	endpoint := "/users/" + url.PathEscape(providerUserID)
	if _, err := strconv.ParseInt(providerUserID, 10, 64); err == nil {
		endpoint = "/user/" + url.PathEscape(providerUserID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(githubAPIBaseURL, "/")+endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "new-api")
	if token := strings.TrimSpace(common.GitHubAPIToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return 0, fmt.Errorf("GitHub API returned status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var githubUser gitHubUser
	if err := common.DecodeJson(res.Body, &githubUser); err != nil {
		return 0, err
	}
	if githubUser.CreatedAt.IsZero() {
		return 0, fmt.Errorf("GitHub API returned an empty created_at")
	}
	return githubUser.CreatedAt.Unix(), nil
}

// RefreshGitHubCreatedAt fills the cached GitHub registration time for legacy
// users. A failed refresh is returned to the caller so login flows can log it
// without blocking an otherwise valid login.
func RefreshGitHubCreatedAt(ctx context.Context, user *model.User) error {
	if user == nil || user.Id <= 0 || strings.TrimSpace(user.GitHubId) == "" || user.GitHubCreatedAt > 0 {
		return nil
	}

	createdAt, err := (&GitHubProvider{}).GetUserCreatedAt(ctx, user.GitHubId)
	if err != nil {
		return err
	}
	if err := model.UpdateUserGitHubBinding(user.Id, user.GitHubId, createdAt); err != nil {
		return err
	}
	user.GitHubCreatedAt = createdAt
	return nil
}

func (p *GitHubProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsGitHubIdAlreadyTaken(providerUserID)
}

func (p *GitHubProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.GitHubId = providerUserID
	return user.FillUserByGitHubId()
}

func (p *GitHubProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func (p *GitHubProvider) GetProviderPrefix() string {
	return "github_"
}

// ProviderUserIDColumn returns the users-table column storing this provider's user ID.
func (p *GitHubProvider) ProviderUserIDColumn() string {
	return "github_id"
}
