package setting

// Waffo Pancake hosted checkout configuration. Gateway is enabled once
// MerchantID + PrivateKey + ProductID are populated (no separate Enabled
// flag, matching Stripe / Creem). StoreID + ProductID are operator-bound
// via SaveWaffoPancakeConfig.
var (
	WaffoPancakeMerchantID string
	WaffoPancakePrivateKey string
	WaffoPancakeReturnURL  string
	// WaffoPancakeUseConfiguredProductPrice uses the selected Pancake product's
	// configured price as the unit price for wallet top-ups.
	WaffoPancakeUseConfiguredProductPrice bool
	// WaffoPancakeUSDToCurrencyRate optionally overrides the legacy per-unit
	// USD price for wallet top-ups. It means 1 USD = X system currency units.
	// Zero keeps the legacy WaffoPancakeUnitPrice calculation for existing
	// installations.
	WaffoPancakeUSDToCurrencyRate float64
	WaffoPancakeUnitPrice         float64 = 1.0
	WaffoPancakeMinTopUp          int     = 1
	WaffoPancakeStoreID           string
	WaffoPancakeProductID         string
)
