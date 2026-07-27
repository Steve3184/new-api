package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectWaffoPancakeConfiguredProductPrice_UsesTheConfiguredCNYPrice(t *testing.T) {
	price, err := selectWaffoPancakeConfiguredProductPrice([]WaffoPancakeConfiguredProductPrice{
		{Currency: "CNY", Amount: "1.00", TaxCategory: "saas"},
	})

	require.NoError(t, err)
	require.Equal(t, "CNY", price.Currency)
	require.Equal(t, "1.00", price.Amount)
	require.Equal(t, "saas", price.TaxCategory)
}
