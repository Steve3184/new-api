package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pancakePriceTransport func(*http.Request) (*http.Response, error)

func (f pancakePriceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGetWaffoPancakeConfiguredProductPriceUsesStringIDContract(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	originalClient, originalMerchant, originalKey := http.DefaultClient, setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey
	t.Cleanup(func() {
		http.DefaultClient = originalClient
		setting.WaffoPancakeMerchantID = originalMerchant
		setting.WaffoPancakePrivateKey = originalKey
	})
	setting.WaffoPancakeMerchantID = "MER_6ffxDzzGwPDRc9PAdW5Ov8"
	setting.WaffoPancakePrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	http.DefaultClient = &http.Client{Transport: pancakePriceTransport(func(request *http.Request) (*http.Response, error) {
		var body struct {
			Query     string            `json:"query"`
			Variables map[string]string `json:"variables"`
		}
		require.NoError(t, common.DecodeJson(request.Body, &body))
		assert.Equal(t, "PROD_configured", body.Variables["id"])
		payload := `{"data":{"onetimeProduct":{"prices":[{"currency":"USD","priceInfo":{"amount":"29.99","taxCategory":"saas"}}]}}}`
		if !strings.Contains(body.Query, "$id: String!") {
			payload = `{"errors":[{"message":"Variable id must have type String!"}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
	})}
	price, err := GetWaffoPancakeConfiguredProductPrice(context.Background(), "PROD_configured")
	require.NoError(t, err)
	assert.Equal(t, "29.99", price.Amount)
	assert.Equal(t, "USD", price.Currency)
}

func TestSelectWaffoPancakeConfiguredProductPrice_UsesTheConfiguredCNYPrice(t *testing.T) {
	price, err := selectWaffoPancakeConfiguredProductPrice([]WaffoPancakeConfiguredProductPrice{
		{Currency: "CNY", Amount: "1.00", TaxCategory: "saas"},
	})

	require.NoError(t, err)
	require.Equal(t, "CNY", price.Currency)
	require.Equal(t, "1.00", price.Amount)
	require.Equal(t, "saas", price.TaxCategory)
}
