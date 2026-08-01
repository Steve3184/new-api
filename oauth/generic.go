package oauth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

// AuthStyle defines how to send client credentials
const (
	AuthStyleAutoDetect = 0 // Auto-detect based on server response
	AuthStyleInParams   = 1 // Send client_id and client_secret as POST parameters
	AuthStyleInHeader   = 2 // Send as Basic Auth header
)

// GenericOAuthProvider implements OAuth for custom/generic OAuth providers
type GenericOAuthProvider struct {
	config *model.CustomOAuthProvider
}

type accessPolicy struct {
	Logic      string            `json:"logic"`
	Conditions []accessCondition `json:"conditions"`
	Groups     []accessPolicy    `json:"groups"`
}

type accessCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type accessPolicyFailure struct {
	Field    string
	Op       string
	Expected any
	Current  any
}

type oauthJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

var signingMethodES256K = &jwt.SigningMethodECDSA{
	Name:      "ES256K",
	Hash:      crypto.SHA256,
	KeySize:   32,
	CurveBits: 256,
}

func init() {
	jwt.RegisterSigningMethod(signingMethodES256K.Alg(), func() jwt.SigningMethod {
		return signingMethodES256K
	})
}

var supportedAccessPolicyOps = []string{
	"eq",
	"ne",
	"gt",
	"gte",
	"lt",
	"lte",
	"in",
	"not_in",
	"contains",
	"not_contains",
	"exists",
	"not_exists",
}

// NewGenericOAuthProvider creates a new generic OAuth provider from config
func NewGenericOAuthProvider(config *model.CustomOAuthProvider) *GenericOAuthProvider {
	return &GenericOAuthProvider{config: config}
}

func (p *GenericOAuthProvider) GetName() string {
	return p.config.Name
}

func (p *GenericOAuthProvider) IsEnabled() bool {
	return p.config.Enabled
}

func (p *GenericOAuthProvider) GetConfig() *model.CustomOAuthProvider {
	return p.config
}

func (p *GenericOAuthProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logger.LogDebug(ctx, "[OAuth-Generic-%s] ExchangeToken: code=%s...", p.config.Slug, code[:min(len(code), 10)])

	redirectUri := fmt.Sprintf("%s/oauth/%s", system_setting.ServerAddress, p.config.Slug)
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", redirectUri)

	// Determine auth style
	authStyle := p.config.AuthStyle
	if authStyle == AuthStyleAutoDetect {
		// Default to params style for most OAuth servers
		authStyle = AuthStyleInParams
	}

	var req *http.Request
	var err error

	if authStyle == AuthStyleInParams {
		values.Set("client_id", p.config.ClientId)
		values.Set("client_secret", p.config.ClientSecret)
	}

	req, err = http.NewRequestWithContext(ctx, "POST", p.config.TokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if authStyle == AuthStyleInHeader {
		// Basic Auth
		credentials := base64.StdEncoding.EncodeToString([]byte(p.config.ClientId + ":" + p.config.ClientSecret))
		req.Header.Set("Authorization", "Basic "+credentials)
	}

	logger.LogDebug(ctx, "[OAuth-Generic-%s] ExchangeToken: token_endpoint=%s, redirect_uri=%s, auth_style=%d",
		p.config.Slug, p.config.TokenEndpoint, redirectUri, authStyle)

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] ExchangeToken error: %s", p.config.Slug, err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": p.config.Name}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-Generic-%s] ExchangeToken response status: %d", p.config.Slug, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] ExchangeToken read body error: %s", p.config.Slug, err.Error()))
		return nil, err
	}

	bodyStr := string(body)
	logger.LogDebug(ctx, "[OAuth-Generic-%s] ExchangeToken response body: %s", p.config.Slug, bodyStr[:min(len(bodyStr), 500)])

	// Try to parse as JSON first
	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		IDToken      string `json:"id_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := common.Unmarshal(body, &tokenResponse); err != nil {
		// Try to parse as URL-encoded (some OAuth servers like GitHub return this format)
		parsedValues, parseErr := url.ParseQuery(bodyStr)
		if parseErr != nil {
			logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] ExchangeToken parse error: %s", p.config.Slug, err.Error()))
			return nil, err
		}
		tokenResponse.AccessToken = parsedValues.Get("access_token")
		tokenResponse.TokenType = parsedValues.Get("token_type")
		tokenResponse.Scope = parsedValues.Get("scope")
	}

	if tokenResponse.Error != "" {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] ExchangeToken OAuth error: %s - %s",
			p.config.Slug, tokenResponse.Error, tokenResponse.ErrorDesc))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": p.config.Name}, tokenResponse.ErrorDesc)
	}

	if tokenResponse.AccessToken == "" {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] ExchangeToken failed: empty access token", p.config.Slug))
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": p.config.Name})
	}

	logger.LogDebug(ctx, "[OAuth-Generic-%s] ExchangeToken success: scope=%s", p.config.Slug, tokenResponse.Scope)

	return &OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		TokenType:    tokenResponse.TokenType,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn:    tokenResponse.ExpiresIn,
		Scope:        tokenResponse.Scope,
		IDToken:      tokenResponse.IDToken,
	}, nil
}

func (p *GenericOAuthProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	bodyStr, err := p.fetchUserInfo(ctx, token)
	if err != nil {
		return nil, err
	}

	logger.LogDebug(ctx, "[OAuth-Generic-%s] GetUserInfo response body: %s", p.config.Slug, bodyStr[:min(len(bodyStr), 500)])

	// Extract fields using gjson (supports JSONPath-like syntax)
	userId := gjson.Get(bodyStr, p.config.UserIdField).String()
	username := gjson.Get(bodyStr, p.config.UsernameField).String()
	displayName := gjson.Get(bodyStr, p.config.DisplayNameField).String()
	email := gjson.Get(bodyStr, p.config.EmailField).String()

	// If user ID field returns a number, convert it
	if userId == "" {
		// Try to get as number
		userIdNum := gjson.Get(bodyStr, p.config.UserIdField)
		if userIdNum.Exists() {
			userId = userIdNum.Raw
			// Remove quotes if present
			userId = strings.Trim(userId, "\"")
		}
	}

	if userId == "" {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] GetUserInfo failed: empty user ID (field: %s)", p.config.Slug, p.config.UserIdField))
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.config.Name})
	}

	logger.LogDebug(ctx, "[OAuth-Generic-%s] GetUserInfo success: id=%s, username=%s, name=%s, email=%s",
		p.config.Slug, userId, username, displayName, email)

	policyRaw := strings.TrimSpace(p.config.AccessPolicy)
	if policyRaw != "" {
		policy, err := parseAccessPolicy(policyRaw)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] invalid access policy: %s", p.config.Slug, err.Error()))
			return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, nil, "invalid access policy configuration")
		}
		allowed, failure := evaluateAccessPolicy(bodyStr, policy)
		if !allowed {
			message := renderAccessDeniedMessage(p.config.AccessDeniedMessage, p.config.Name, bodyStr, failure)
			logger.LogWarn(ctx, fmt.Sprintf("[OAuth-Generic-%s] access denied by policy: field=%s op=%s expected=%v current=%v",
				p.config.Slug, failure.Field, failure.Op, failure.Expected, failure.Current))
			return nil, &AccessDeniedError{Message: message}
		}
	}

	return &OAuthUser{
		ProviderUserID: userId,
		Username:       username,
		DisplayName:    displayName,
		Email:          email,
		Extra: map[string]any{
			"provider": p.config.Slug,
		},
	}, nil
}

func (p *GenericOAuthProvider) fetchUserInfo(ctx context.Context, token *OAuthToken) (string, error) {
	if strings.TrimSpace(p.config.UserInfoEndpoint) == "" {
		claims, err := p.verifyIDToken(ctx, token.IDToken)
		if err != nil {
			return "", err
		}
		payload, err := common.Marshal(claims)
		if err != nil {
			return "", err
		}
		return string(payload), nil
	}

	logger.LogDebug(ctx, "[OAuth-Generic-%s] GetUserInfo: fetching user info from %s", p.config.Slug, p.config.UserInfoEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.UserInfoEndpoint, nil)
	if err != nil {
		return "", err
	}
	tokenType := normalizeAuthorizationTokenType(token.TokenType)
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, token.AccessToken))
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] GetUserInfo error: %s", p.config.Slug, err.Error()))
		return "", NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": p.config.Name}, err.Error())
	}
	defer res.Body.Close()
	logger.LogDebug(ctx, "[OAuth-Generic-%s] GetUserInfo response status: %d", p.config.Slug, res.StatusCode)
	if res.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] GetUserInfo failed: status=%d", p.config.Slug, res.StatusCode))
		return "", NewOAuthError(i18n.MsgOAuthGetUserErr, nil)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Generic-%s] GetUserInfo read body error: %s", p.config.Slug, err.Error()))
		return "", err
	}
	return string(body), nil
}

func (p *GenericOAuthProvider) verifyIDToken(ctx context.Context, rawToken string) (jwt.MapClaims, error) {
	if rawToken == "" || strings.TrimSpace(p.config.WellKnown) == "" {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.config.Name})
	}

	discoveryBody, err := fetchOAuthJSON(ctx, p.config.WellKnown)
	if err != nil {
		return nil, err
	}
	jwksURL := gjson.Get(string(discoveryBody), "jwks_uri").String()
	issuer := gjson.Get(string(discoveryBody), "issuer").String()
	if jwksURL == "" || issuer == "" {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.config.Name})
	}
	jwksBody, err := fetchOAuthJSON(ctx, jwksURL)
	if err != nil {
		return nil, err
	}
	var jwks struct {
		Keys []oauthJWK `json:"keys"`
	}
	if err := common.Unmarshal(jwksBody, &jwks); err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		for _, key := range jwks.Keys {
			if key.Kid == kid || (kid == "" && len(jwks.Keys) == 1) {
				return jwkPublicKey(key)
			}
		}
		return nil, fmt.Errorf("OAuth signing key not found")
	},
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "ES256K", "EdDSA"}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.config.Name}, err.Error())
	}
	if !idTokenAudienceMatches(claims, p.config.ClientId) {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.config.Name})
	}
	return claims, nil
}

func idTokenAudienceMatches(claims jwt.MapClaims, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	match := func(value any) bool {
		switch audience := value.(type) {
		case string:
			return audience == expected
		case float64:
			expectedAudience, err := strconv.ParseInt(expected, 10, 64)
			return err == nil && audience == float64(expectedAudience)
		case stdjson.Number:
			return audience.String() == expected
		default:
			return false
		}
	}
	switch audience := claims["aud"].(type) {
	case []any:
		for _, value := range audience {
			if match(value) {
				return true
			}
		}
		return false
	default:
		return match(audience)
	}
}

func fetchOAuthJSON(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, nil, err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, nil)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func jwkPublicKey(key oauthJWK) (any, error) {
	decode := func(value string) ([]byte, error) {
		return base64.RawURLEncoding.DecodeString(value)
	}
	switch key.Kty {
	case "RSA":
		n, err := decode(key.N)
		if err != nil {
			return nil, err
		}
		e, err := decode(key.E)
		if err != nil {
			return nil, err
		}
		exponent := 0
		for _, value := range e {
			exponent = exponent<<8 | int(value)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, nil
	case "EC":
		var curve elliptic.Curve
		switch key.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		case "secp256k1":
			curve = secp256k1.S256()
		default:
			return nil, fmt.Errorf("unsupported OAuth curve %s", key.Crv)
		}
		x, err := decode(key.X)
		if err != nil {
			return nil, err
		}
		y, err := decode(key.Y)
		if err != nil {
			return nil, err
		}
		publicKey := &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if !curve.IsOnCurve(publicKey.X, publicKey.Y) {
			return nil, errors.New("OAuth EC public key is not on the configured curve")
		}
		return publicKey, nil
	case "OKP":
		if key.Crv != "Ed25519" {
			return nil, fmt.Errorf("unsupported OAuth key curve %s", key.Crv)
		}
		x, err := decode(key.X)
		if err != nil {
			return nil, err
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid OAuth Ed25519 public key length")
		}
		return ed25519.PublicKey(x), nil
	default:
		return nil, fmt.Errorf("unsupported OAuth key type %s", key.Kty)
	}
}

func (p *GenericOAuthProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsProviderUserIdTaken(p.config.Id, providerUserID)
}

func (p *GenericOAuthProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	foundUser, err := model.GetUserByOAuthBinding(p.config.Id, providerUserID)
	if err != nil {
		return err
	}
	*user = *foundUser
	return nil
}

func (p *GenericOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	// For generic providers, we store the binding in user_oauth_bindings table
	// This is handled separately in the OAuth controller
}

func (p *GenericOAuthProvider) GetProviderPrefix() string {
	return p.config.Slug + "_"
}

// GetProviderId returns the provider ID for binding purposes
func (p *GenericOAuthProvider) GetProviderId() int {
	return p.config.Id
}

func normalizeAuthorizationTokenType(tokenType string) string {
	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" || strings.EqualFold(tokenType, "Bearer") {
		return "Bearer"
	}
	return tokenType
}

// IsGenericProvider returns true for generic providers
func (p *GenericOAuthProvider) IsGenericProvider() bool {
	return true
}

func parseAccessPolicy(raw string) (*accessPolicy, error) {
	var policy accessPolicy
	if err := common.UnmarshalJsonStr(raw, &policy); err != nil {
		return nil, err
	}
	if err := validateAccessPolicy(&policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

func validateAccessPolicy(policy *accessPolicy) error {
	if policy == nil {
		return errors.New("policy is nil")
	}

	logic := strings.ToLower(strings.TrimSpace(policy.Logic))
	if logic == "" {
		logic = "and"
	}
	if !lo.Contains([]string{"and", "or"}, logic) {
		return fmt.Errorf("unsupported policy logic: %s", logic)
	}
	policy.Logic = logic

	if len(policy.Conditions) == 0 && len(policy.Groups) == 0 {
		return errors.New("policy requires at least one condition or group")
	}

	for index := range policy.Conditions {
		if err := validateAccessCondition(&policy.Conditions[index], index); err != nil {
			return err
		}
	}

	for index := range policy.Groups {
		if err := validateAccessPolicy(&policy.Groups[index]); err != nil {
			return fmt.Errorf("invalid policy group[%d]: %w", index, err)
		}
	}

	return nil
}

func validateAccessCondition(condition *accessCondition, index int) error {
	if condition == nil {
		return fmt.Errorf("condition[%d] is nil", index)
	}

	condition.Field = strings.TrimSpace(condition.Field)
	if condition.Field == "" {
		return fmt.Errorf("condition[%d].field is required", index)
	}

	condition.Op = normalizePolicyOp(condition.Op)
	if !lo.Contains(supportedAccessPolicyOps, condition.Op) {
		return fmt.Errorf("condition[%d].op is unsupported: %s", index, condition.Op)
	}

	if lo.Contains([]string{"in", "not_in"}, condition.Op) {
		if _, ok := condition.Value.([]any); !ok {
			return fmt.Errorf("condition[%d].value must be an array for op %s", index, condition.Op)
		}
	}

	return nil
}

func evaluateAccessPolicy(body string, policy *accessPolicy) (bool, *accessPolicyFailure) {
	if policy == nil {
		return true, nil
	}

	logic := strings.ToLower(strings.TrimSpace(policy.Logic))
	if logic == "" {
		logic = "and"
	}

	hasAny := len(policy.Conditions) > 0 || len(policy.Groups) > 0
	if !hasAny {
		return true, nil
	}

	if logic == "or" {
		var firstFailure *accessPolicyFailure
		for _, cond := range policy.Conditions {
			ok, failure := evaluateAccessCondition(body, cond)
			if ok {
				return true, nil
			}
			if firstFailure == nil {
				firstFailure = failure
			}
		}
		for _, group := range policy.Groups {
			ok, failure := evaluateAccessPolicy(body, &group)
			if ok {
				return true, nil
			}
			if firstFailure == nil {
				firstFailure = failure
			}
		}
		return false, firstFailure
	}

	for _, cond := range policy.Conditions {
		ok, failure := evaluateAccessCondition(body, cond)
		if !ok {
			return false, failure
		}
	}
	for _, group := range policy.Groups {
		ok, failure := evaluateAccessPolicy(body, &group)
		if !ok {
			return false, failure
		}
	}
	return true, nil
}

func evaluateAccessCondition(body string, cond accessCondition) (bool, *accessPolicyFailure) {
	path := cond.Field
	op := cond.Op
	result := gjson.Get(body, path)
	current := gjsonResultToValue(result)
	failure := &accessPolicyFailure{
		Field:    path,
		Op:       op,
		Expected: cond.Value,
		Current:  current,
	}

	switch op {
	case "exists":
		return result.Exists(), failure
	case "not_exists":
		return !result.Exists(), failure
	case "eq":
		return compareAny(current, cond.Value) == 0, failure
	case "ne":
		return compareAny(current, cond.Value) != 0, failure
	case "gt":
		return compareAny(current, cond.Value) > 0, failure
	case "gte":
		return compareAny(current, cond.Value) >= 0, failure
	case "lt":
		return compareAny(current, cond.Value) < 0, failure
	case "lte":
		return compareAny(current, cond.Value) <= 0, failure
	case "in":
		return valueInSlice(current, cond.Value), failure
	case "not_in":
		return !valueInSlice(current, cond.Value), failure
	case "contains":
		return containsValue(current, cond.Value), failure
	case "not_contains":
		return !containsValue(current, cond.Value), failure
	default:
		return false, failure
	}
}

func normalizePolicyOp(op string) string {
	return strings.ToLower(strings.TrimSpace(op))
}

func gjsonResultToValue(result gjson.Result) any {
	if !result.Exists() {
		return nil
	}
	if result.IsArray() {
		arr := result.Array()
		values := make([]any, 0, len(arr))
		for _, item := range arr {
			values = append(values, gjsonResultToValue(item))
		}
		return values
	}
	switch result.Type {
	case gjson.Null:
		return nil
	case gjson.True:
		return true
	case gjson.False:
		return false
	case gjson.Number:
		return result.Num
	case gjson.String:
		return result.String()
	case gjson.JSON:
		var data any
		if err := common.UnmarshalJsonStr(result.Raw, &data); err == nil {
			return data
		}
		return result.Raw
	default:
		return result.Value()
	}
}

func compareAny(left any, right any) int {
	if lf, ok := toFloat(left); ok {
		if rf, ok2 := toFloat(right); ok2 {
			switch {
			case lf < rf:
				return -1
			case lf > rf:
				return 1
			default:
				return 0
			}
		}
	}

	ls := strings.TrimSpace(fmt.Sprint(left))
	rs := strings.TrimSpace(fmt.Sprint(right))
	switch {
	case ls < rs:
		return -1
	case ls > rs:
		return 1
	default:
		return 0
	}
}

func toFloat(v any) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case stdjson.Number:
		n, err := value.Float64()
		if err == nil {
			return n, true
		}
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func valueInSlice(current any, expected any) bool {
	list, ok := expected.([]any)
	if !ok {
		return false
	}
	return lo.ContainsBy(list, func(item any) bool {
		return compareAny(current, item) == 0
	})
}

func containsValue(current any, expected any) bool {
	switch value := current.(type) {
	case string:
		target := strings.TrimSpace(fmt.Sprint(expected))
		return strings.Contains(value, target)
	case []any:
		return lo.ContainsBy(value, func(item any) bool {
			return compareAny(item, expected) == 0
		})
	}
	return false
}

func renderAccessDeniedMessage(template string, providerName string, body string, failure *accessPolicyFailure) string {
	defaultMessage := "Access denied: your account does not meet this provider's access requirements."
	message := strings.TrimSpace(template)
	if message == "" {
		return defaultMessage
	}

	if failure == nil {
		failure = &accessPolicyFailure{}
	}

	replacements := map[string]string{
		"{{provider}}": providerName,
		"{{field}}":    failure.Field,
		"{{op}}":       failure.Op,
		"{{required}}": fmt.Sprint(failure.Expected),
		"{{current}}":  fmt.Sprint(failure.Current),
	}

	for key, value := range replacements {
		message = strings.ReplaceAll(message, key, value)
	}

	currentPattern := regexp.MustCompile(`\{\{current\.([^}]+)\}\}`)
	message = currentPattern.ReplaceAllStringFunc(message, func(token string) string {
		match := currentPattern.FindStringSubmatch(token)
		if len(match) != 2 {
			return ""
		}
		path := strings.TrimSpace(match[1])
		if path == "" {
			return ""
		}
		return strings.TrimSpace(gjson.Get(body, path).String())
	})

	requiredPattern := regexp.MustCompile(`\{\{required\.([^}]+)\}\}`)
	message = requiredPattern.ReplaceAllStringFunc(message, func(token string) string {
		match := requiredPattern.FindStringSubmatch(token)
		if len(match) != 2 {
			return ""
		}
		path := strings.TrimSpace(match[1])
		if failure.Field == path {
			return fmt.Sprint(failure.Expected)
		}
		return ""
	})

	return strings.TrimSpace(message)
}
