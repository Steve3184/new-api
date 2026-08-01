package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCustomOAuthProviderRequiresProfileEndpointOrDiscovery(t *testing.T) {
	provider := &CustomOAuthProvider{
		Name:                  "Custom",
		Slug:                  "custom",
		ClientId:              "client-id",
		AuthorizationEndpoint: "https://provider.example.com/authorize",
		TokenEndpoint:         "https://provider.example.com/token",
	}
	assert.Error(t, validateCustomOAuthProvider(provider))

	provider.WellKnown = "https://provider.example.com/.well-known/openid-configuration"
	assert.NoError(t, validateCustomOAuthProvider(provider))
}
