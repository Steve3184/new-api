package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

const nowPaymentsPriceCurrency = "usd"

type NowPaymentsInvoice struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	PayAddress    string `json:"pay_address"`
	PayinExtraID  string `json:"payin_extra_id,omitempty"`
	PayAmount     string `json:"pay_amount"`
	PayCurrency   string `json:"pay_currency"`
	PriceAmount   string `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	Status        string `json:"status"`
	ExpiresAt     int64  `json:"expires_at"`
}

type NowPaymentsPaymentStatus struct {
	PaymentID    string `json:"payment_id"`
	OrderID      string `json:"order_id"`
	Status       string `json:"status"`
	PayCurrency  string `json:"pay_currency"`
	PayAmount    string `json:"pay_amount"`
	PayAddress   string `json:"pay_address"`
	PayinExtraID string `json:"payin_extra_id,omitempty"`
	ExpiresAt    int64  `json:"expires_at"`
}

type NowPaymentsWebhook struct {
	PaymentID     json.RawMessage `json:"payment_id"`
	PaymentStatus string          `json:"payment_status"`
	PayAddress    string          `json:"pay_address"`
	PayinExtraID  json.RawMessage `json:"payin_extra_id"`
	PriceAmount   json.RawMessage `json:"price_amount"`
	PriceCurrency string          `json:"price_currency"`
	PayAmount     json.RawMessage `json:"pay_amount"`
	ActuallyPaid  json.RawMessage `json:"actually_paid"`
	PayCurrency   string          `json:"pay_currency"`
	OrderID       string          `json:"order_id"`
}

type nowPaymentsCreateRequest struct {
	PriceAmount      json.Number `json:"price_amount"`
	PriceCurrency    string      `json:"price_currency"`
	PayCurrency      string      `json:"pay_currency"`
	OrderID          string      `json:"order_id"`
	OrderDescription string      `json:"order_description"`
	IPNCallbackURL   string      `json:"ipn_callback_url"`
}

type nowPaymentsCreateResponse struct {
	PaymentID              json.RawMessage `json:"payment_id"`
	PaymentStatus          string          `json:"payment_status"`
	PayAddress             string          `json:"pay_address"`
	PayinExtraID           json.RawMessage `json:"payin_extra_id"`
	PriceAmount            json.RawMessage `json:"price_amount"`
	PriceCurrency          string          `json:"price_currency"`
	PayAmount              json.RawMessage `json:"pay_amount"`
	PayCurrency            string          `json:"pay_currency"`
	OrderID                string          `json:"order_id"`
	ExpirationEstimateDate string          `json:"expiration_estimate_date"`
	ValidUntil             string          `json:"valid_until"`
}

func IsNowPaymentsTopUpEnabled() bool {
	return operation_setting.IsPaymentComplianceConfirmed() &&
		setting.NowPaymentsEnabled &&
		strings.TrimSpace(setting.NowPaymentsAPIKey) != "" &&
		strings.TrimSpace(setting.NowPaymentsIPNSecret) != "" &&
		len(setting.GetNowPaymentsPayCurrencies()) > 0
}

func nowPaymentsAPIEndpoint(path string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(setting.NowPaymentsAPIBaseURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return "", errors.New("NOWPayments API base URL must be an HTTP or HTTPS URL")
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func nowPaymentsCallbackURL() (string, error) {
	base, err := url.Parse(strings.TrimSpace(GetCallbackAddress()))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return "", errors.New("configure an HTTP or HTTPS callback address before enabling NOWPayments")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/nowpayments/webhook"
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func nowPaymentsDecimalString(value json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(value))
	if text == "" || text == "null" {
		return "", errors.New("NOWPayments returned an empty decimal value")
	}
	if strings.HasPrefix(text, `"`) {
		var decoded string
		if err := common.Unmarshal(value, &decoded); err != nil {
			return "", err
		}
		text = strings.TrimSpace(decoded)
	}
	parsed, err := decimal.NewFromString(text)
	if err != nil || parsed.IsNegative() {
		return "", errors.New("NOWPayments returned an invalid decimal value")
	}
	return parsed.String(), nil
}

func nowPaymentsOptionalDecimalString(value json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(value))
	if text == "" || text == "null" {
		return "0", nil
	}
	return nowPaymentsDecimalString(value)
}

func nowPaymentsIDString(value json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(value))
	if text == "" || text == "null" {
		return "", errors.New("NOWPayments returned no payment id")
	}
	if strings.HasPrefix(text, `"`) {
		var decoded string
		if err := common.Unmarshal(value, &decoded); err != nil {
			return "", err
		}
		text = strings.TrimSpace(decoded)
	}
	if text == "" || len(text) > 64 {
		return "", errors.New("NOWPayments returned an invalid payment id")
	}
	return text, nil
}

// nowPaymentsOptionalExtraID accepts the memo/destination-tag value returned
// by NOWPayments. Some assets return this field as a number even though the
// public API documents it as a string.
func nowPaymentsOptionalExtraID(value json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(value))
	if text == "" || text == "null" {
		return "", nil
	}
	switch common.GetJsonType(value) {
	case "string":
		var decoded string
		if err := common.Unmarshal(value, &decoded); err != nil {
			return "", err
		}
		text = strings.TrimSpace(decoded)
	case "number":
		var number json.Number
		if err := common.Unmarshal(value, &number); err != nil {
			return "", err
		}
		text = strings.TrimSpace(number.String())
	default:
		return "", errors.New("NOWPayments returned an invalid payment memo or tag")
	}
	if len(text) > 255 {
		return "", errors.New("NOWPayments payment memo or tag is too long")
	}
	return text, nil
}

func nowPaymentsExpiration(now int64, providerDates ...string) (int64, error) {
	minutes := setting.NowPaymentsPaymentExpirationMins
	if minutes < 5 || minutes > 1440 {
		minutes = 60
	}
	expiresAt := now + int64(minutes)*60
	for _, rawDate := range providerDates {
		rawDate = strings.TrimSpace(rawDate)
		if rawDate == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, rawDate)
		if err != nil {
			return 0, errors.New("NOWPayments returned an invalid expiration date")
		}
		providerExpiry := parsed.Unix()
		if providerExpiry > 0 && providerExpiry < expiresAt {
			expiresAt = providerExpiry
		}
	}
	return expiresAt, nil
}

func createNowPaymentsPayment(ctx context.Context, request nowPaymentsCreateRequest) (*NowPaymentsInvoice, error) {
	endpoint, err := nowPaymentsAPIEndpoint("/v1/payment")
	if err != nil {
		return nil, err
	}
	payload, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(setting.NowPaymentsAPIKey))
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("NOWPayments returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var response nowPaymentsCreateResponse
	if err := common.DecodeJson(resp.Body, &response); err != nil {
		return nil, err
	}
	paymentID, err := nowPaymentsIDString(response.PaymentID)
	if err != nil {
		return nil, err
	}
	priceAmount, err := nowPaymentsDecimalString(response.PriceAmount)
	if err != nil {
		return nil, err
	}
	payAmount, err := nowPaymentsDecimalString(response.PayAmount)
	if err != nil {
		return nil, err
	}
	payinExtraID, err := nowPaymentsOptionalExtraID(response.PayinExtraID)
	if err != nil {
		return nil, err
	}
	if response.OrderID != request.OrderID || !strings.EqualFold(response.PriceCurrency, request.PriceCurrency) || !strings.EqualFold(response.PayCurrency, request.PayCurrency) {
		return nil, errors.New("NOWPayments returned mismatched payment details")
	}
	requestedPrice, err := decimal.NewFromString(request.PriceAmount.String())
	if err != nil {
		return nil, err
	}
	returnedPrice, err := decimal.NewFromString(priceAmount)
	if err != nil || !returnedPrice.Equal(requestedPrice) {
		return nil, errors.New("NOWPayments returned a mismatched price amount")
	}
	if strings.TrimSpace(response.PayAddress) == "" {
		return nil, errors.New("NOWPayments returned no payment address")
	}
	now := common.GetTimestamp()
	expiresAt, err := nowPaymentsExpiration(now, response.ExpirationEstimateDate, response.ValidUntil)
	if err != nil {
		return nil, err
	}
	return &NowPaymentsInvoice{
		PaymentID:     paymentID,
		OrderID:       response.OrderID,
		PayAddress:    response.PayAddress,
		PayinExtraID:  payinExtraID,
		PayAmount:     payAmount,
		PayCurrency:   strings.ToLower(response.PayCurrency),
		PriceAmount:   priceAmount,
		PriceCurrency: strings.ToLower(response.PriceCurrency),
		Status:        response.PaymentStatus,
		ExpiresAt:     expiresAt,
	}, nil
}

func nowPaymentsTopUpQuote(amount int64, group string) (decimal.Decimal, int, error) {
	if amount <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
		return decimal.Zero, 0, errors.New("invalid NOWPayments top-up amount")
	}
	creditedQuota := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		creditedQuota = creditedQuota.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	} else {
		quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		creditedQuota = decimal.NewFromInt(creditedQuota.Div(quotaPerUnit).IntPart()).Mul(quotaPerUnit)
	}
	quota, err := common.WalletQuotaFromDecimalStrict(creditedQuota)
	if err != nil || quota <= 0 {
		return decimal.Zero, 0, model.ErrInvalidTopUpQuota
	}

	quote := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		quote = quote.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	} else {
		rate := setting.NowPaymentsUSDToCurrencyRate
		if rate == 0 {
			rate = operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
		}
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
			return decimal.Zero, 0, errors.New("NOWPayments USD to system currency rate must be positive")
		}
		quote = quote.Div(decimal.NewFromFloat(rate))
	}
	groupRatio := common.GetTopupGroupRatio(group)
	if groupRatio == 0 {
		groupRatio = 1
	}
	if math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio <= 0 {
		return decimal.Zero, 0, errors.New("top-up group ratio must be positive")
	}
	quote = quote.Mul(decimal.NewFromFloat(groupRatio))
	if discount, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if math.IsNaN(discount) || math.IsInf(discount, 0) || discount <= 0 {
			return decimal.Zero, 0, errors.New("top-up discount must be positive")
		}
		quote = quote.Mul(decimal.NewFromFloat(discount))
	}
	if !quote.IsPositive() {
		return decimal.Zero, 0, errors.New("NOWPayments quote must be positive")
	}
	return quote, quota, nil
}

func CreateNowPaymentsTopUp(ctx context.Context, userID int, amount int64, payCurrency string) (*NowPaymentsInvoice, error) {
	if !IsNowPaymentsTopUpEnabled() {
		return nil, errors.New("NOWPayments top-up is not configured")
	}
	payCurrency = strings.ToLower(strings.TrimSpace(payCurrency))
	if !setting.IsValidNowPaymentsCurrency(payCurrency) {
		return nil, errors.New("selected NOWPayments currency is not enabled")
	}
	minimum := int64(setting.NowPaymentsMinTopUp)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		converted, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(minimum).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
		if err != nil {
			return nil, model.ErrInvalidTopUpQuota
		}
		minimum = int64(converted)
	}
	if amount < minimum {
		return nil, fmt.Errorf("minimum NOWPayments top-up is %d", minimum)
	}
	user, err := model.GetUserById(userID, true)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	quoteUSD, creditedQuota, err := nowPaymentsTopUpQuote(amount, user.Group)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateTopUpQuotaCapacity(userID, creditedQuota); err != nil {
		return nil, err
	}
	tradeID, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return nil, err
	}
	tradeNo := "nowpayments-" + tradeID
	callbackURL, err := nowPaymentsCallbackURL()
	if err != nil {
		return nil, err
	}
	invoice, err := createNowPaymentsPayment(ctx, nowPaymentsCreateRequest{
		PriceAmount:      json.Number(quoteUSD.StringFixed(8)),
		PriceCurrency:    nowPaymentsPriceCurrency,
		PayCurrency:      payCurrency,
		OrderID:          tradeNo,
		OrderDescription: fmt.Sprintf("Wallet top-up %d", amount),
		IPNCallbackURL:   callbackURL,
	})
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	topUp := &model.TopUp{
		UserId:          userID,
		Amount:          amount,
		Money:           quoteUSD.InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodNowPayments,
		PaymentProvider: model.PaymentProviderNowPayments,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	payment := &model.NowPaymentsPayment{
		PaymentID:     invoice.PaymentID,
		OrderID:       tradeNo,
		PayCurrency:   invoice.PayCurrency,
		PriceCurrency: invoice.PriceCurrency,
		PriceAmount:   invoice.PriceAmount,
		PayAmount:     invoice.PayAmount,
		CreditedQuota: creditedQuota,
		PayAddress:    invoice.PayAddress,
		PayinExtraID:  invoice.PayinExtraID,
		GatewayStatus: invoice.Status,
		Status:        model.NowPaymentsPaymentStatusPending,
		ExpiresAt:     invoice.ExpiresAt,
		CreateTime:    now,
	}
	if err := model.CreateNowPaymentsTopUp(topUp, payment); err != nil {
		return nil, err
	}
	return invoice, nil
}

func CreateNowPaymentsSubscription(ctx context.Context, userID, planID int, payCurrency string) (*NowPaymentsInvoice, error) {
	if !IsNowPaymentsTopUpEnabled() {
		return nil, errors.New("NOWPayments is not configured")
	}
	payCurrency = strings.ToLower(strings.TrimSpace(payCurrency))
	if !setting.IsValidNowPaymentsCurrency(payCurrency) {
		return nil, errors.New("selected NOWPayments currency is not enabled")
	}
	plan, err := model.GetSubscriptionPlanById(planID)
	if err != nil || plan == nil {
		return nil, errors.New("subscription plan not found")
	}
	if !plan.Enabled || plan.PriceAmount <= 0 || math.IsNaN(plan.PriceAmount) || math.IsInf(plan.PriceAmount, 0) {
		return nil, errors.New("subscription plan is unavailable")
	}
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userID, plan.Id)
		if err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("subscription purchase limit reached")
		}
	}
	priceUSD := decimal.NewFromFloat(plan.PriceAmount)
	if !strings.EqualFold(strings.TrimSpace(plan.Currency), "USD") {
		rate := setting.NowPaymentsUSDToCurrencyRate
		if rate == 0 {
			rate = operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
		}
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return nil, errors.New("NOWPayments USD to system currency rate must be positive")
		}
		priceUSD = priceUSD.Div(decimal.NewFromFloat(rate))
	}
	tradeID, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return nil, err
	}
	tradeNo := "nowpayments-sub-" + tradeID
	callbackURL, err := nowPaymentsCallbackURL()
	if err != nil {
		return nil, err
	}
	invoice, err := createNowPaymentsPayment(ctx, nowPaymentsCreateRequest{
		PriceAmount:      json.Number(priceUSD.StringFixed(8)),
		PriceCurrency:    nowPaymentsPriceCurrency,
		PayCurrency:      payCurrency,
		OrderID:          tradeNo,
		OrderDescription: "Subscription: " + plan.Title,
		IPNCallbackURL:   callbackURL,
	})
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	order := &model.SubscriptionOrder{
		UserId:          userID,
		PlanId:          plan.Id,
		Money:           priceUSD.InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodNowPayments,
		PaymentProvider: model.PaymentProviderNowPayments,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	payment := &model.NowPaymentsPayment{
		PaymentID:     invoice.PaymentID,
		OrderID:       tradeNo,
		PayCurrency:   invoice.PayCurrency,
		PriceCurrency: invoice.PriceCurrency,
		PriceAmount:   invoice.PriceAmount,
		PayAmount:     invoice.PayAmount,
		PayAddress:    invoice.PayAddress,
		PayinExtraID:  invoice.PayinExtraID,
		GatewayStatus: invoice.Status,
		Status:        model.NowPaymentsPaymentStatusPending,
		ExpiresAt:     invoice.ExpiresAt,
		CreateTime:    now,
	}
	if err := model.CreateNowPaymentsSubscription(order, payment); err != nil {
		return nil, err
	}
	return invoice, nil
}

func GetNowPaymentsPaymentStatus(userID int, paymentID string) (*NowPaymentsPaymentStatus, error) {
	payment, err := model.GetNowPaymentsPaymentByIDAndUser(paymentID, userID)
	if err != nil {
		return nil, err
	}
	if payment.Status == model.NowPaymentsPaymentStatusPending {
		if payload, fetchErr := fetchNowPaymentsPayment(context.Background(), payment.PaymentID); fetchErr == nil {
			_ = HandleNowPaymentsWebhook(payload, "status-poll")
			if refreshed, reloadErr := model.GetNowPaymentsPaymentByIDAndUser(payment.PaymentID, userID); reloadErr == nil {
				payment = refreshed
			}
		}
	}
	return &NowPaymentsPaymentStatus{
		PaymentID:    payment.PaymentID,
		OrderID:      payment.OrderID,
		Status:       payment.Status,
		PayCurrency:  payment.PayCurrency,
		PayAmount:    payment.PayAmount,
		PayAddress:   payment.PayAddress,
		PayinExtraID: payment.PayinExtraID,
		ExpiresAt:    payment.ExpiresAt,
	}, nil
}

func fetchNowPaymentsPayment(ctx context.Context, paymentID string) ([]byte, error) {
	endpoint, err := nowPaymentsAPIEndpoint("/v1/payment/" + url.PathEscape(strings.TrimSpace(paymentID)))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", strings.TrimSpace(setting.NowPaymentsAPIKey))
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("NOWPayments status returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64*1024))
}

func canonicalNowPaymentsJSON(payload []byte) ([]byte, error) {
	return canonicalNowPaymentsValue(bytes.TrimSpace(payload))
}

// nowPaymentsJSONString undoes the extra escapes added by encoding/json for
// HTML and JSONP safety. NOWPayments signs JSON.stringify output, which keeps
// these characters literal.
func nowPaymentsJSONString(value string) ([]byte, error) {
	encoded, err := common.Marshal(value)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	for index := 0; index < len(encoded); {
		if encoded[index] != '\\' || index+1 >= len(encoded) {
			buffer.WriteByte(encoded[index])
			index++
			continue
		}
		if encoded[index+1] == '\\' {
			buffer.Write(encoded[index : index+2])
			index += 2
			continue
		}
		if encoded[index+1] == 'u' && index+6 <= len(encoded) {
			switch string(encoded[index+2 : index+6]) {
			case "003c":
				buffer.WriteByte('<')
				index += 6
				continue
			case "003e":
				buffer.WriteByte('>')
				index += 6
				continue
			case "0026":
				buffer.WriteByte('&')
				index += 6
				continue
			case "2028":
				buffer.WriteRune('\u2028')
				index += 6
				continue
			case "2029":
				buffer.WriteRune('\u2029')
				index += 6
				continue
			}
		}
		buffer.Write(encoded[index : index+2])
		index += 2
	}
	return buffer.Bytes(), nil
}

// nowPaymentsNumberString follows ECMAScript Number::toString formatting as
// used by JSON.stringify: fixed notation is used for [1e-6, 1e21), while
// other finite numbers use normalized scientific notation.
func nowPaymentsNumberString(value float64) string {
	if value == 0 {
		return "0"
	}
	abs := math.Abs(value)
	if abs >= 1e-6 && abs < 1e21 {
		raw := strconv.FormatFloat(value, 'g', -1, 64)
		parts := strings.SplitN(strings.ToLower(raw), "e", 2)
		if len(parts) != 2 {
			return raw
		}
		exponent, err := strconv.Atoi(parts[1])
		if err != nil {
			return raw
		}
		sign := ""
		mantissa := parts[0]
		if strings.HasPrefix(mantissa, "-") {
			sign = "-"
			mantissa = mantissa[1:]
		}
		decimalIndex := strings.IndexByte(mantissa, '.')
		if decimalIndex < 0 {
			decimalIndex = len(mantissa)
		}
		digits := strings.ReplaceAll(mantissa, ".", "")
		decimalIndex += exponent
		switch {
		case decimalIndex <= 0:
			return sign + "0." + strings.Repeat("0", -decimalIndex) + digits
		case decimalIndex >= len(digits):
			return sign + digits + strings.Repeat("0", decimalIndex-len(digits))
		default:
			return sign + digits[:decimalIndex] + "." + digits[decimalIndex:]
		}
	}
	raw := strconv.FormatFloat(value, 'e', -1, 64)
	parts := strings.SplitN(strings.ToLower(raw), "e", 2)
	if len(parts) != 2 {
		return raw
	}
	exponent, err := strconv.Atoi(parts[1])
	if err != nil {
		return raw
	}
	sign := ""
	if exponent >= 0 {
		sign = "+"
	}
	return parts[0] + "e" + sign + strconv.Itoa(exponent)
}

func canonicalNowPaymentsValue(value json.RawMessage) ([]byte, error) {
	switch common.GetJsonType(value) {
	case "object":
		var object map[string]json.RawMessage
		if err := common.Unmarshal(value, &object); err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var buffer bytes.Buffer
		buffer.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.WriteByte(',')
			}
			encodedKey, err := nowPaymentsJSONString(key)
			if err != nil {
				return nil, err
			}
			encodedValue, err := canonicalNowPaymentsValue(object[key])
			if err != nil {
				return nil, err
			}
			buffer.Write(encodedKey)
			buffer.WriteByte(':')
			buffer.Write(encodedValue)
		}
		buffer.WriteByte('}')
		return buffer.Bytes(), nil
	case "array":
		var values []json.RawMessage
		if err := common.Unmarshal(value, &values); err != nil {
			return nil, err
		}
		var buffer bytes.Buffer
		buffer.WriteByte('[')
		for index, item := range values {
			if index > 0 {
				buffer.WriteByte(',')
			}
			encodedItem, err := canonicalNowPaymentsValue(item)
			if err != nil {
				return nil, err
			}
			buffer.Write(encodedItem)
		}
		buffer.WriteByte(']')
		return buffer.Bytes(), nil
	case "number":
		var number float64
		if err := common.Unmarshal(value, &number); err != nil {
			return nil, err
		}
		return []byte(nowPaymentsNumberString(number)), nil
	case "string":
		var decoded string
		if err := common.Unmarshal(value, &decoded); err != nil {
			return nil, err
		}
		return nowPaymentsJSONString(decoded)
	case "boolean", "null":
		var decoded any
		if err := common.Unmarshal(value, &decoded); err != nil {
			return nil, err
		}
		return bytes.TrimSpace(value), nil
	default:
		var decoded any
		if err := common.Unmarshal(value, &decoded); err != nil {
			return nil, err
		}
		return common.Marshal(decoded)
	}
}

func VerifyNowPaymentsSignature(payload []byte, signature string) bool {
	canonical, err := canonicalNowPaymentsJSON(payload)
	if err != nil {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}
	mac := hmac.New(sha512.New, []byte(setting.NowPaymentsIPNSecret))
	_, _ = mac.Write(canonical)
	return hmac.Equal(provided, mac.Sum(nil))
}

func HandleNowPaymentsWebhook(payload []byte, callerIP string) error {
	var event NowPaymentsWebhook
	if err := common.Unmarshal(payload, &event); err != nil {
		return err
	}
	paymentID, err := nowPaymentsIDString(event.PaymentID)
	if err != nil {
		return err
	}
	payment, err := model.GetNowPaymentsPaymentByPaymentID(paymentID)
	if err != nil {
		return err
	}
	priceAmount, err := nowPaymentsDecimalString(event.PriceAmount)
	if err != nil {
		return err
	}
	payAmount, err := nowPaymentsDecimalString(event.PayAmount)
	if err != nil {
		return err
	}
	actuallyPaid, err := nowPaymentsOptionalDecimalString(event.ActuallyPaid)
	if err != nil {
		return err
	}
	payinExtraID, err := nowPaymentsOptionalExtraID(event.PayinExtraID)
	if err != nil {
		return err
	}
	storedPrice, err := decimal.NewFromString(payment.PriceAmount)
	if err != nil {
		return err
	}
	eventPrice, err := decimal.NewFromString(priceAmount)
	if err != nil {
		return err
	}
	storedPayAmount, err := decimal.NewFromString(payment.PayAmount)
	if err != nil {
		return err
	}
	eventPayAmount, err := decimal.NewFromString(payAmount)
	if err != nil {
		return err
	}
	eventActuallyPaid, err := decimal.NewFromString(actuallyPaid)
	if err != nil {
		return err
	}
	if event.OrderID != payment.OrderID || !strings.EqualFold(event.PayCurrency, payment.PayCurrency) ||
		!strings.EqualFold(event.PriceCurrency, payment.PriceCurrency) || !eventPrice.Equal(storedPrice) ||
		!eventPayAmount.Equal(storedPayAmount) || event.PayAddress != payment.PayAddress ||
		payinExtraID != payment.PayinExtraID {
		return errors.New("NOWPayments webhook details do not match the stored order")
	}
	canonical, err := canonicalNowPaymentsJSON(payload)
	if err != nil {
		return err
	}
	status := strings.ToLower(strings.TrimSpace(event.PaymentStatus))
	switch status {
	case "finished":
		if eventActuallyPaid.LessThan(storedPayAmount) {
			return errors.New("NOWPayments finished payment is below the quoted amount")
		}
		if payment.OrderType == model.NowPaymentsOrderTypeTopUp {
			_, err = model.SettleNowPaymentsTopUp(paymentID, status, actuallyPaid, string(canonical), callerIP)
			return err
		}
		if payment.OrderType != model.NowPaymentsOrderTypeSubscription {
			return errors.New("unknown NOWPayments order type")
		}
		paid, parseErr := decimal.NewFromString(actuallyPaid)
		if parseErr != nil || !paid.IsPositive() {
			return errors.New("NOWPayments subscription payment is empty")
		}
		return model.CompleteNowPaymentsSubscription(paymentID, status, actuallyPaid, string(canonical))
	case "expired":
		return model.FailNowPaymentsPayment(paymentID, status, actuallyPaid, string(canonical), model.NowPaymentsPaymentStatusExpired)
	case "failed", "refunded", "wrong_amount":
		return model.FailNowPaymentsPayment(paymentID, status, actuallyPaid, string(canonical), model.NowPaymentsPaymentStatusFailed)
	case "waiting", "confirming", "confirmed", "sending", "partially_paid":
		return model.UpdateNowPaymentsPendingState(paymentID, status, actuallyPaid, string(canonical))
	default:
		return fmt.Errorf("unsupported NOWPayments payment status %q", status)
	}
}

func ParseNowPaymentsWebhookPaymentID(payload []byte) string {
	var event struct {
		PaymentID json.RawMessage `json:"payment_id"`
	}
	if err := common.Unmarshal(payload, &event); err != nil {
		return ""
	}
	paymentID, _ := nowPaymentsIDString(event.PaymentID)
	return paymentID
}
