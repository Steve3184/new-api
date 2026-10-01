package operation_setting

import (
	"errors"
	"math"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ReferralSetting struct {
	Enabled       bool    `json:"enabled"`
	Percent       float64 `json:"percent"`
	MinPaidCNY    float64 `json:"min_paid_cny"`
	MaxOrders     int     `json:"max_orders"`
	DelayHours    int     `json:"delay_hours"`
	CurrencyRates string  `json:"currency_rates"`
}

type referralConfig struct {
	mu    sync.RWMutex
	value ReferralSetting
}

var referralSettings = referralConfig{value: ReferralSetting{
	Enabled: true, Percent: 5, MinPaidCNY: 20, MaxOrders: 10, DelayHours: 24,
	CurrencyRates: `{"CNY":1,"USD":7.3}`,
}}

func init() { config.GlobalConfig.Register("referral_setting", &referralSettings) }

func GetReferralSetting() ReferralSetting {
	referralSettings.mu.RLock()
	defer referralSettings.mu.RUnlock()
	return referralSettings.value
}

func (s *referralConfig) ExportConfigMap() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return config.ConfigToMap(s.value)
}

func (s *referralConfig) UpdateConfigMap(values map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range values {
		if err := ValidateReferralOption(key, value); err != nil {
			return err
		}
	}
	next := s.value
	if err := config.UpdateConfigFromMap(&next, values); err != nil {
		return err
	}
	s.value = next
	return nil
}

func ValidateReferralOption(key, value string) error {
	invalid := errors.New("invalid referral setting")
	switch key {
	case "enabled":
		if value != "true" && value != "false" {
			return invalid
		}
	case "percent", "min_paid_cny":
		n, err := strconv.ParseFloat(value, 64)
		limit := 100.0
		if key == "min_paid_cny" {
			limit = 1000000000
		}
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > limit {
			return invalid
		}
	case "max_orders", "delay_hours":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 87600 {
			return invalid
		}
	case "currency_rates":
		var rates map[string]float64
		if len(value) > 4096 || common.UnmarshalJsonStr(value, &rates) != nil || rates["CNY"] != 1 || rates["USD"] <= 0 {
			return invalid
		}
		for currency, rate := range rates {
			if len(currency) != 3 || rate <= 0 || rate > 1000000 || math.IsNaN(rate) || math.IsInf(rate, 0) {
				return invalid
			}
			for _, char := range currency {
				if char < 'A' || char > 'Z' {
					return invalid
				}
			}
		}
	default:
		return invalid
	}
	return nil
}
