package setting

import "strings"

// NOWPayments hosted payment configuration. The gateway is opt-in and only
// exposes currencies explicitly enabled by the operator. Currency values are
// passed to NOWPayments unchanged (for example, usdtbsc).
var (
	NowPaymentsEnabled               = false
	NowPaymentsAPIKey                = ""
	NowPaymentsIPNSecret             = ""
	NowPaymentsAPIBaseURL            = "https://api.nowpayments.io"
	NowPaymentsPayCurrencies         = "btc,usdtbsc"
	NowPaymentsMinTopUp              = 1
	NowPaymentsUSDToCurrencyRate     = 0.0
	NowPaymentsPaymentExpirationMins = 60
)

func GetNowPaymentsPayCurrencies() []string {
	raw := strings.Split(NowPaymentsPayCurrencies, ",")
	result := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, value := range raw {
		currency := strings.ToLower(strings.TrimSpace(value))
		if currency == "" {
			continue
		}
		if _, exists := seen[currency]; exists {
			continue
		}
		seen[currency] = struct{}{}
		result = append(result, currency)
	}
	return result
}

func IsValidNowPaymentsCurrency(value string) bool {
	currency := strings.ToLower(strings.TrimSpace(value))
	for _, enabled := range GetNowPaymentsPayCurrencies() {
		if currency == enabled {
			return true
		}
	}
	return false
}
