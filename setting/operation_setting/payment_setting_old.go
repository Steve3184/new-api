/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation_setting

import (
	"maps"

	"github.com/QuantumNous/new-api/common"
)

var PayAddress = ""
var CustomCallbackAddress = ""
var EpayId = ""
var EpayKey = ""
var Price = 7.3
var MinTopUp = 1
var USDExchangeRate = 7.3

var PayMethods = []map[string]string{
	{
		"name": "支付宝",
		"icon": "SiAlipay",
		"type": "alipay",
	},
	{
		"name": "微信",
		"icon": "SiWechat",
		"type": "wxpay",
	},
	{
		"name":      "自定义1",
		"icon":      "LuCreditCard",
		"type":      "custom1",
		"min_topup": "50",
	},
}

func UpdatePayMethodsByJsonString(jsonString string) error {
	PayMethods = make([]map[string]string, 0)
	return common.Unmarshal([]byte(jsonString), &PayMethods)
}

func PayMethods2JsonString() string {
	jsonBytes, err := common.Marshal(PayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func GetGlobalPayMethod(method string) map[string]string {
	for _, payMethod := range PayMethods {
		if payMethod["type"] == method {
			return maps.Clone(payMethod)
		}
	}
	return nil
}

func ContainsPayMethod(method string) bool {
	if len(EpayGateways) == 0 {
		for _, payMethod := range PayMethods {
			if payMethod["type"] == method {
				return true
			}
		}
	}
	for _, gateway := range GetEpayGateways() {
		for _, payMethod := range gateway.PayMethods {
			if payMethod["type"] == method {
				return true
			}
		}
	}
	return false
}

func GetPayMethod(method string) map[string]string {
	return GetPayMethodForGateway(method, "")
}

func GetPayMethodForGateway(method, gatewayID string) map[string]string {
	if len(EpayGateways) == 0 {
		if gatewayID != "" && gatewayID != "default" {
			return nil
		}
		for _, payMethod := range PayMethods {
			if payMethod["type"] == method {
				copy := make(map[string]string, len(payMethod)+1)
				for key, value := range payMethod {
					copy[key] = value
				}
				copy["gateway"] = "default"
				return copy
			}
		}
	}
	var matched map[string]string
	for _, gateway := range GetEpayGateways() {
		if gatewayID != "" && gateway.ID != gatewayID {
			continue
		}
		for _, payMethod := range gateway.PayMethods {
			if payMethod["type"] == method {
				if gatewayID == "" && matched != nil {
					return nil
				}
				copy := make(map[string]string, len(payMethod))
				for key, value := range payMethod {
					copy[key] = value
				}
				copy["gateway"] = gateway.ID
				if gatewayID != "" {
					return copy
				}
				matched = copy
			}
		}
	}
	return matched
}
