package operation_setting

import (
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type EpayGateway struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Address    string              `json:"address"`
	MerchantID string              `json:"merchant_id"`
	Key        string              `json:"key"`
	Enabled    bool                `json:"enabled"`
	PayMethods []map[string]string `json:"pay_methods"`
}

var EpayGateways = []EpayGateway{}

type PaymentSetting struct {
	AmountOptions             []int           `json:"amount_options"`
	AmountDiscount            map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠
	RedemptionPurchaseEnabled bool            `json:"redemption_purchase_enabled"`

	ComplianceConfirmed    bool   `json:"compliance_confirmed"`
	ComplianceTermsVersion string `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64  `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int    `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string `json:"compliance_confirmed_ip"`

	// 易支付签名方式：MD5 使用 EpayKey；RSA 使用商户私钥签名请求、平台公钥验证回调。
	EpaySignType           string `json:"epay_sign_type"`
	EpayMerchantPrivateKey string `json:"epay_merchant_private_key"`
	EpayPlatformPublicKey  string `json:"epay_platform_public_key"`
}

const CurrentComplianceTermsVersion = "v1"

const (
	EpaySignTypeMD5 = "MD5"
	EpaySignTypeRSA = "RSA"

	EpaySignTypeOptionKey           = "payment_setting.epay_sign_type"
	EpayMerchantPrivateKeyOptionKey = "payment_setting.epay_merchant_private_key"
	EpayPlatformPublicKeyOptionKey  = "payment_setting.epay_platform_public_key"
)

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:             []int{10, 20, 50, 100, 200, 500},
	AmountDiscount:            map[int]float64{},
	RedemptionPurchaseEnabled: false,
	// 没有保存过签名方式的站点按 MD5 运行，升级后行为不变。
	EpaySignType: EpaySignTypeMD5,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}

func UpdateEpayGatewaysByJsonString(jsonString string) error {
	if err := ValidateEpayGatewaysJSON(jsonString); err != nil {
		return err
	}
	var gateways []EpayGateway
	if err := common.Unmarshal([]byte(jsonString), &gateways); err != nil {
		return err
	}
	EpayGateways = gateways
	return nil
}

func ValidateEpayGatewaysJSON(jsonString string) error {
	var gateways []EpayGateway
	if err := common.Unmarshal([]byte(jsonString), &gateways); err != nil {
		return err
	}
	ids := make(map[string]struct{}, len(gateways))
	for _, gateway := range gateways {
		id := strings.TrimSpace(gateway.ID)
		if id == "" || len(id) > 100 {
			return errors.New("Epay gateway id is required and must not exceed 100 characters")
		}
		if _, exists := ids[id]; exists {
			return errors.New("Epay gateway ids must be unique")
		}
		ids[id] = struct{}{}
		address, err := url.Parse(strings.TrimSpace(gateway.Address))
		if err != nil || (address.Scheme != "http" && address.Scheme != "https") || address.Host == "" {
			return errors.New("Epay gateway address must be a valid HTTP URL")
		}
		if strings.TrimSpace(gateway.MerchantID) == "" || (paymentSetting.EpaySignType != EpaySignTypeRSA && strings.TrimSpace(gateway.Key) == "") {
			return errors.New("Epay gateway merchant id and key are required")
		}
		methodTypes := make(map[string]struct{}, len(gateway.PayMethods))
		for _, method := range gateway.PayMethods {
			methodType := strings.TrimSpace(method["type"])
			if methodType == "" {
				return errors.New("Epay payment method type is required")
			}
			if _, exists := methodTypes[methodType]; exists {
				return errors.New("Epay payment method types must be unique within a gateway")
			}
			methodTypes[methodType] = struct{}{}
			for _, field := range []string{"fee", "fee_rate"} {
				if strings.TrimSpace(method[field]) == "" {
					continue
				}
				value, err := strconv.ParseFloat(method[field], 64)
				if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
					return errors.New("Epay payment fees must be finite non-negative numbers")
				}
			}
		}
	}
	return nil
}

func EpayGateways2JsonString() string {
	jsonBytes, err := common.Marshal(EpayGateways)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func GetEpayGateways() []EpayGateway {
	if len(EpayGateways) > 0 {
		result := make([]EpayGateway, 0, len(EpayGateways))
		for _, gateway := range EpayGateways {
			if !gateway.Enabled {
				continue
			}
			result = append(result, gateway)
		}
		return result
	}
	if PayAddress == "" || EpayId == "" || (paymentSetting.EpaySignType != EpaySignTypeRSA && EpayKey == "") {
		return nil
	}
	return []EpayGateway{{
		ID: "default", Name: "Epay", Address: PayAddress, MerchantID: EpayId,
		Key: EpayKey, Enabled: true, PayMethods: PayMethods,
	}}
}

func GetEpayGateway(id string) *EpayGateway {
	if id != "" && len(EpayGateways) > 0 {
		for i := range EpayGateways {
			if EpayGateways[i].ID == id {
				gateway := EpayGateways[i]
				return &gateway
			}
		}
		return nil
	}
	gateways := GetEpayGateways()
	for i := range gateways {
		if gateways[i].ID == id || id == "" {
			gateway := gateways[i]
			return &gateway
		}
	}
	return nil
}

// ValidateEpaySignOption checks a change to the Epay signing settings against
// the saved ones. While RSA is active, both keys must parse; under MD5 only a
// newly entered key is checked.
func ValidateEpaySignOption(key, value string) error {
	if key != EpaySignTypeOptionKey && key != EpayMerchantPrivateKeyOptionKey && key != EpayPlatformPublicKeyOptionKey {
		return nil
	}
	// Saved values are written under OptionMapRWMutex by the option update path.
	common.OptionMapRWMutex.RLock()
	signType := paymentSetting.EpaySignType
	privateKey := paymentSetting.EpayMerchantPrivateKey
	publicKey := paymentSetting.EpayPlatformPublicKey
	common.OptionMapRWMutex.RUnlock()
	switch key {
	case EpaySignTypeOptionKey:
		if value != EpaySignTypeMD5 && value != EpaySignTypeRSA {
			return common.NewMessage("Epay signature type must be MD5 or RSA")
		}
		signType = value
	case EpayMerchantPrivateKeyOptionKey:
		privateKey = value
	case EpayPlatformPublicKeyOptionKey:
		publicKey = value
	}
	rsaActive := signType == EpaySignTypeRSA
	if rsaActive || (key == EpayMerchantPrivateKeyOptionKey && strings.TrimSpace(value) != "") {
		if _, err := epay.ParseRSAPrivateKey(privateKey); err != nil {
			return common.NewMessage("The Epay merchant private key is missing or invalid")
		}
	}
	if rsaActive || (key == EpayPlatformPublicKeyOptionKey && strings.TrimSpace(value) != "") {
		if _, err := epay.ParseRSAPublicKey(publicKey); err != nil {
			return common.NewMessage("The Epay platform public key is missing or invalid")
		}
	}
	return nil
}

// MigrateLegacyEpayGateway converts the old single-gateway settings into the
// first provider once, while leaving the legacy fields untouched for rollback.
func MigrateLegacyEpayGateway() bool {
	if len(EpayGateways) > 0 || PayAddress == "" || EpayId == "" || (paymentSetting.EpaySignType != EpaySignTypeRSA && EpayKey == "") {
		return false
	}
	EpayGateways = []EpayGateway{{
		ID: "default", Name: "Epay", Address: PayAddress, MerchantID: EpayId,
		Key: EpayKey, Enabled: true, PayMethods: PayMethods,
	}}
	return true
}
