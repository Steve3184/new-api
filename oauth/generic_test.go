package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericOAuthProviderUsesDistinctTokenAndUserInfoEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			assert.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, "authorization-code", r.Form.Get("code"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer"}`))
		case "/profile":
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"telegram-user","preferred_username":"telegram","name":"Telegram User"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewGenericOAuthProvider(&model.CustomOAuthProvider{
		Name:             "Custom",
		Slug:             "custom",
		ClientId:         "client-id",
		ClientSecret:     "client-secret",
		TokenEndpoint:    server.URL + "/token",
		UserInfoEndpoint: server.URL + "/profile",
		UserIdField:      "id",
		UsernameField:    "preferred_username",
		DisplayNameField: "name",
	})

	token, err := provider.ExchangeToken(context.Background(), "authorization-code", nil)
	require.NoError(t, err)
	user, err := provider.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "telegram-user", user.ProviderUserID)
	assert.Equal(t, "telegram", user.Username)
}

func TestGenericOAuthProviderReadsIDTokenWhenUserInfoEndpointIsEmpty(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q}`, server.URL, server.URL+"/jwks")
		case "/jwks":
			_, _ = fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":"test-key","n":%q,"e":"AQAB"}]}`,
				base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	idToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":                server.URL,
		"aud":                123456789,
		"exp":                time.Now().Add(time.Minute).Unix(),
		"sub":                "telegram-subject",
		"preferred_username": "telegram",
		"name":               "Telegram User",
	})
	idToken.Header["kid"] = "test-key"
	signedToken, err := idToken.SignedString(privateKey)
	require.NoError(t, err)

	provider := NewGenericOAuthProvider(&model.CustomOAuthProvider{
		Name:             "Telegram",
		Slug:             "telegram",
		ClientId:         "123456789",
		WellKnown:        server.URL + "/.well-known/openid-configuration",
		UserIdField:      "sub",
		UsernameField:    "preferred_username",
		DisplayNameField: "name",
	})

	user, err := provider.GetUserInfo(context.Background(), &OAuthToken{IDToken: signedToken})
	require.NoError(t, err)
	assert.Equal(t, "telegram-subject", user.ProviderUserID)
	assert.Equal(t, "telegram", user.Username)

	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	invalidToken, err := idToken.SignedString(wrongKey)
	require.NoError(t, err)
	_, err = provider.GetUserInfo(context.Background(), &OAuthToken{IDToken: invalidToken})
	require.Error(t, err)
}

func TestGenericOAuthProviderAcceptsES256KIDToken(t *testing.T) {
	privateKey, err := secp256k1.GeneratePrivateKey()
	require.NoError(t, err)
	publicKey := privateKey.PubKey().ToECDSA()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q}`, server.URL, server.URL+"/jwks")
		case "/jwks":
			_, _ = fmt.Fprintf(w, `{"keys":[{"kty":"EC","kid":"test-key","crv":"secp256k1","x":%q,"y":%q}]}`,
				base64.RawURLEncoding.EncodeToString(publicKey.X.FillBytes(make([]byte, 32))),
				base64.RawURLEncoding.EncodeToString(publicKey.Y.FillBytes(make([]byte, 32))))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	idToken := jwt.NewWithClaims(signingMethodES256K, jwt.MapClaims{
		"iss": server.URL,
		"aud": "client-id",
		"exp": time.Now().Add(time.Minute).Unix(),
		"sub": "telegram-subject",
	})
	idToken.Header["kid"] = "test-key"
	signedToken, err := idToken.SignedString(privateKey.ToECDSA())
	require.NoError(t, err)

	provider := NewGenericOAuthProvider(&model.CustomOAuthProvider{
		Name:        "Telegram",
		Slug:        "telegram",
		ClientId:    "client-id",
		WellKnown:   server.URL + "/.well-known/openid-configuration",
		UserIdField: "sub",
	})

	user, err := provider.GetUserInfo(context.Background(), &OAuthToken{IDToken: signedToken})
	require.NoError(t, err)
	assert.Equal(t, "telegram-subject", user.ProviderUserID)
}
