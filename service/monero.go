package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

const (
	moneroAtomicUnits          = int32(12)
	moneroInvoiceExpiry        = 3 * time.Hour
	moneroMonitorInterval      = 30 * time.Second
	moneroAddressAuditInterval = 24 * time.Hour
	defaultMoneroUSDPriceURL   = "https://api.coingecko.com/api/v3/simple/price?ids=monero&vs_currencies=usd"
)

var (
	moneroUSDPriceURL             = defaultMoneroUSDPriceURL
	moneroSubaddressCreationMutex sync.Mutex
)

type moneroRPCClient struct {
	endpoint string
	username string
	password string
	client   *http.Client
}

type moneroRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type moneroRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type moneroAddressResult struct {
	Address      string      `json:"address"`
	AddressIndex json.Number `json:"address_index"`
}

type moneroAddressListResult struct {
	Addresses []struct {
		Address string `json:"address"`
	} `json:"addresses"`
}

type moneroBalanceResult struct {
	PerSubaddress []struct {
		AddressIndex    json.Number `json:"address_index"`
		Balance         json.Number `json:"balance"`
		UnlockedBalance json.Number `json:"unlocked_balance"`
	} `json:"per_subaddress"`
}

type moneroSubaddressBalance struct {
	Balance         decimal.Decimal
	UnlockedBalance decimal.Decimal
}

type moneroTransferResult struct {
	In []struct {
		Amount          json.Number `json:"amount"`
		Confirmations   json.Number `json:"confirmations"`
		DoubleSpendSeen bool        `json:"double_spend_seen"`
		Locked          bool        `json:"locked"`
		TxID            string      `json:"txid"`
	} `json:"in"`
	Pool []struct {
		Amount          json.Number `json:"amount"`
		Confirmations   json.Number `json:"confirmations"`
		DoubleSpendSeen bool        `json:"double_spend_seen"`
		TxID            string      `json:"txid"`
	} `json:"pool"`
	Out []struct {
		Confirmations   json.Number `json:"confirmations"`
		DoubleSpendSeen bool        `json:"double_spend_seen"`
		TxID            string      `json:"txid"`
		Destinations    []struct {
			Address string      `json:"address"`
			Amount  json.Number `json:"amount"`
		} `json:"destinations"`
	} `json:"out"`
}

type MoneroInvoice struct {
	Address       string `json:"address"`
	QuotaAmount   int64  `json:"quota_amount"`
	AmountXMR     string `json:"amount_xmr"`
	AmountAtomic  string `json:"amount_atomic"`
	QuoteUSD      string `json:"quote_usd"`
	USDPerXMR     string `json:"usd_per_xmr"`
	Network       string `json:"network"`
	Confirmations int    `json:"confirmations"`
	ExpiresAt     int64  `json:"expires_at"`
}

type MoneroInvoicePaymentStatus struct {
	Status                string `json:"status"`
	TransactionDetected   bool   `json:"transaction_detected"`
	Confirmations         int    `json:"confirmations"`
	RequiredConfirmations int    `json:"required_confirmations"`
}

// MoneroAddressAuditResult is recorded on the scheduled task. It reports
// which terminal invoice addresses have no locked outputs, but intentionally
// does not reuse, delete, or move funds from any address.
type MoneroAddressAuditResult struct {
	Candidates    int `json:"candidates"`
	FullyUnlocked int `json:"fully_unlocked"`
	Locked        int `json:"locked"`
	NotReported   int `json:"not_reported"`
}

func isMoneroGatewayConfigured() bool {
	return setting.MoneroEnabled &&
		strings.TrimSpace(setting.MoneroWalletRPCURL) != "" &&
		setting.IsValidMoneroNetwork(setting.MoneroNetwork) &&
		setting.MoneroConfirmations >= 1
}

func IsMoneroTopUpEnabled() bool {
	return operation_setting.IsPaymentComplianceConfirmed() && isMoneroGatewayConfigured()
}

func newMoneroRPCClient() (*moneroRPCClient, error) {
	rawEndpoint := strings.TrimSpace(setting.MoneroWalletRPCURL)
	if rawEndpoint == "" {
		return nil, errors.New("monero wallet RPC URL is not configured")
	}
	endpoint, err := url.Parse(rawEndpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return nil, errors.New("monero wallet RPC URL must be an HTTP or HTTPS URL")
	}
	if endpoint.Path == "" || endpoint.Path == "/" {
		endpoint.Path = "/json_rpc"
	}
	return &moneroRPCClient{
		endpoint: endpoint.String(),
		username: setting.MoneroWalletRPCUsername,
		password: setting.MoneroWalletRPCPassword,
		client:   &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (client *moneroRPCClient) call(ctx context.Context, method string, params any, target any) error {
	payload, err := common.Marshal(moneroRPCRequest{
		JSONRPC: "2.0",
		ID:      "new-api-monero",
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized && (client.username != "" || client.password != "") {
		challenge := resp.Header.Values("WWW-Authenticate")
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		authorization, err := moneroDigestAuthorization(challenge, client.username, client.password, req.Method, req.URL.RequestURI())
		if err != nil {
			return err
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authorization)
		resp, err = client.client.Do(req)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("monero wallet RPC returned HTTP %d", resp.StatusCode)
	}
	var rpcResponse moneroRPCResponse
	if err := common.DecodeJson(resp.Body, &rpcResponse); err != nil {
		return err
	}
	if rpcResponse.Error != nil {
		return fmt.Errorf("monero wallet RPC %s failed: %s", method, rpcResponse.Error.Message)
	}
	if len(rpcResponse.Result) == 0 {
		return fmt.Errorf("monero wallet RPC %s returned no result", method)
	}
	return common.Unmarshal(rpcResponse.Result, target)
}

// moneroDigestAuthorization implements the Digest MD5 authentication used by
// monero-wallet-rpc. Wallet RPC rejects HTTP Basic authentication even on a
// loopback-only endpoint.
func moneroDigestAuthorization(challenges []string, username, password, method, requestURI string) (string, error) {
	var values map[string]string
	for _, challenge := range challenges {
		candidate, ok := moneroDigestParameters(challenge)
		if !ok {
			continue
		}
		algorithm := strings.ToUpper(candidate["algorithm"])
		if algorithm == "" || algorithm == "MD5" {
			values = candidate
			break
		}
	}
	if values == nil {
		return "", errors.New("monero wallet RPC did not provide an MD5 digest challenge")
	}
	realm, nonce := values["realm"], values["nonce"]
	if realm == "" || nonce == "" {
		return "", errors.New("monero wallet RPC provided an invalid digest challenge")
	}
	if requestURI == "" {
		requestURI = "/"
	}
	hash := func(value string) string {
		return fmt.Sprintf("%x", md5.Sum([]byte(value)))
	}
	ha1 := hash(username + ":" + realm + ":" + password)
	ha2 := hash(method + ":" + requestURI)
	responseParts := []string{ha1, nonce}
	headerParts := []string{
		fmt.Sprintf("username=%q", username),
		fmt.Sprintf("realm=%q", realm),
		fmt.Sprintf("nonce=%q", nonce),
		fmt.Sprintf("uri=%q", requestURI),
	}
	if qop := strings.TrimSpace(values["qop"]); qop != "" {
		supportsAuth := false
		for _, value := range strings.Split(qop, ",") {
			if strings.EqualFold(strings.TrimSpace(value), "auth") {
				supportsAuth = true
				break
			}
		}
		if !supportsAuth {
			return "", fmt.Errorf("monero wallet RPC digest qop %q is unsupported", qop)
		}
		cnonceBytes := make([]byte, 16)
		if _, err := rand.Read(cnonceBytes); err != nil {
			return "", fmt.Errorf("generate monero wallet RPC digest nonce: %w", err)
		}
		cnonce := fmt.Sprintf("%x", cnonceBytes)
		const nonceCount = "00000001"
		responseParts = append(responseParts, nonceCount, cnonce, "auth")
		headerParts = append(headerParts,
			"algorithm=MD5",
			"qop=auth",
			"nc="+nonceCount,
			fmt.Sprintf("cnonce=%q", cnonce),
		)
	} else {
		headerParts = append(headerParts, "algorithm=MD5")
	}
	responseParts = append(responseParts, ha2)
	headerParts = append(headerParts, fmt.Sprintf("response=%q", hash(strings.Join(responseParts, ":"))))
	return "Digest " + strings.Join(headerParts, ", "), nil
}

func moneroDigestParameters(challenge string) (map[string]string, bool) {
	challenge = strings.TrimSpace(challenge)
	if len(challenge) < len("Digest ") || !strings.EqualFold(challenge[:len("Digest ")], "Digest ") {
		return nil, false
	}
	parameters := make(map[string]string)
	for remaining := strings.TrimSpace(challenge[len("Digest "):]); remaining != ""; {
		remaining = strings.TrimLeft(remaining, " ,")
		equals := strings.IndexByte(remaining, '=')
		if equals <= 0 {
			return nil, false
		}
		key := strings.ToLower(strings.TrimSpace(remaining[:equals]))
		remaining = strings.TrimSpace(remaining[equals+1:])
		if remaining == "" {
			return nil, false
		}
		var value string
		if remaining[0] == '"' {
			remaining = remaining[1:]
			end := strings.IndexByte(remaining, '"')
			if end < 0 {
				return nil, false
			}
			value = remaining[:end]
			remaining = remaining[end+1:]
		} else {
			end := strings.IndexByte(remaining, ',')
			if end < 0 {
				value = remaining
				remaining = ""
			} else {
				value = remaining[:end]
				remaining = remaining[end+1:]
			}
		}
		parameters[key] = strings.TrimSpace(value)
	}
	return parameters, true
}

func (client *moneroRPCClient) createAddress(ctx context.Context, label string) (string, int, error) {
	var result moneroAddressResult
	if err := client.call(ctx, "create_address", map[string]any{
		"account_index": 0,
		"label":         label,
	}, &result); err != nil {
		return "", 0, err
	}
	addressIndex, err := result.AddressIndex.Int64()
	if err != nil || addressIndex < 0 || addressIndex > math.MaxInt32 {
		return "", 0, errors.New("monero wallet RPC returned an invalid address index")
	}
	if !isMoneroAddressForNetwork(result.Address, setting.MoneroNetwork) {
		return "", 0, fmt.Errorf("monero wallet RPC returned an address outside configured %s", setting.MoneroNetwork)
	}
	return result.Address, int(addressIndex), nil
}

func (client *moneroRPCClient) subaddressCount(ctx context.Context) (int, error) {
	var result moneroAddressListResult
	if err := client.call(ctx, "get_address", map[string]any{
		"account_index": 0,
	}, &result); err != nil {
		return 0, err
	}
	return len(result.Addresses), nil
}

func (client *moneroRPCClient) createInvoiceAddress(ctx context.Context, label string) (string, int, error) {
	moneroSubaddressCreationMutex.Lock()
	defer moneroSubaddressCreationMutex.Unlock()

	limit := setting.MoneroMaxSubaddresses
	if limit < 1 {
		return "", 0, errors.New("monero subaddress limit must be positive")
	}
	count, err := client.subaddressCount(ctx)
	if err != nil {
		return "", 0, err
	}
	if count >= limit {
		return "", 0, fmt.Errorf("monero wallet subaddress limit (%d) has been reached", limit)
	}
	return client.createAddress(ctx, label)
}

func (client *moneroRPCClient) subaddressBalances(ctx context.Context) (map[int]moneroSubaddressBalance, error) {
	var result moneroBalanceResult
	if err := client.call(ctx, "get_balance", map[string]any{
		"account_index":   0,
		"strict_balances": true,
	}, &result); err != nil {
		return nil, err
	}

	balances := make(map[int]moneroSubaddressBalance, len(result.PerSubaddress))
	for _, entry := range result.PerSubaddress {
		addressIndex, indexErr := entry.AddressIndex.Int64()
		balance, balanceErr := decimal.NewFromString(entry.Balance.String())
		unlockedBalance, unlockedErr := decimal.NewFromString(entry.UnlockedBalance.String())
		if indexErr != nil || addressIndex < 0 || addressIndex > math.MaxInt32 || balanceErr != nil || unlockedErr != nil || balance.IsNegative() || unlockedBalance.IsNegative() || unlockedBalance.GreaterThan(balance) {
			return nil, errors.New("monero wallet RPC returned an invalid subaddress balance")
		}
		balances[int(addressIndex)] = moneroSubaddressBalance{
			Balance:         balance,
			UnlockedBalance: unlockedBalance,
		}
	}
	return balances, nil
}

func (client *moneroRPCClient) incomingTransfers(ctx context.Context, addressIndex int) ([]moneroTransferResult, error) {
	var result moneroTransferResult
	if err := client.call(ctx, "get_transfers", map[string]any{
		"in":              true,
		"pool":            true,
		"account_index":   0,
		"subaddr_indices": []int{addressIndex},
	}, &result); err != nil {
		return nil, err
	}
	return []moneroTransferResult{result}, nil
}

func GetMoneroInvoicePaymentStatus(ctx context.Context, userID int, address string) (*MoneroInvoicePaymentStatus, error) {
	payment, err := model.GetMoneroPaymentByAddressAndUser(strings.TrimSpace(address), userID)
	if err != nil {
		return nil, err
	}
	status := &MoneroInvoicePaymentStatus{
		Status:                payment.Status,
		RequiredConfirmations: setting.MoneroConfirmations,
	}
	if payment.Status != model.MoneroPaymentStatusPending || !isMoneroGatewayConfigured() {
		return status, nil
	}
	rpc, err := newMoneroRPCClient()
	if err != nil {
		return nil, err
	}
	transferResults, err := rpc.incomingTransfers(ctx, payment.AddressIndex)
	if err != nil {
		return nil, err
	}
	for _, transferResult := range transferResults {
		for _, transfer := range transferResult.In {
			amount, amountErr := decimal.NewFromString(transfer.Amount.String())
			if amountErr != nil || !amount.IsPositive() || transfer.DoubleSpendSeen {
				continue
			}
			status.TransactionDetected = true
			if confirmations, confirmationsErr := transfer.Confirmations.Int64(); confirmationsErr == nil && confirmations > int64(status.Confirmations) {
				if confirmations > math.MaxInt32 {
					status.Confirmations = math.MaxInt32
					continue
				}
				status.Confirmations = int(confirmations)
			}
		}
		for _, transfer := range transferResult.Pool {
			amount, amountErr := decimal.NewFromString(transfer.Amount.String())
			if amountErr == nil && amount.IsPositive() && !transfer.DoubleSpendSeen {
				status.TransactionDetected = true
			}
		}
	}
	return status, nil
}

func (client *moneroRPCClient) outgoingTransfers(ctx context.Context) ([]moneroTransferResult, error) {
	var result moneroTransferResult
	if err := client.call(ctx, "get_transfers", map[string]any{
		"out":           true,
		"pool":          false,
		"account_index": 0,
	}, &result); err != nil {
		return nil, err
	}
	return []moneroTransferResult{result}, nil
}

func isMoneroAddressForNetwork(address, network string) bool {
	if address == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(network)) {
	case setting.MoneroNetworkMainnet:
		return strings.HasPrefix(address, "4") || strings.HasPrefix(address, "8")
	case setting.MoneroNetworkTestnet:
		return strings.HasPrefix(address, "9") || strings.HasPrefix(address, "B")
	case setting.MoneroNetworkStagenet:
		return strings.HasPrefix(address, "5") || strings.HasPrefix(address, "7")
	default:
		return false
	}
}

func getMoneroUSDPrice(ctx context.Context) (decimal.Decimal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, moneroUSDPriceURL, nil)
	if err != nil {
		return decimal.Zero, err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return decimal.Zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decimal.Zero, fmt.Errorf("monero USD price endpoint returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Monero struct {
			USD json.Number `json:"usd"`
		} `json:"monero"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return decimal.Zero, err
	}
	price, err := decimal.NewFromString(payload.Monero.USD.String())
	if err != nil || !price.IsPositive() {
		return decimal.Zero, errors.New("monero USD price endpoint returned an invalid price")
	}
	return price, nil
}

func moneroQuoteUSD(amount int64, group string) (decimal.Decimal, error) {
	if amount <= 0 {
		return decimal.Zero, errors.New("monero topup amount must be positive")
	}
	if math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
		return decimal.Zero, errors.New("quota per USD must be positive")
	}
	quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	quote := decimal.NewFromInt(amount)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeTokens:
		quote = quote.Div(quotaPerUnit)
	default:
		currencyRate := setting.MoneroUSDToCurrencyRate
		if currencyRate == 0 {
			currencyRate = operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
		}
		if math.IsNaN(currencyRate) || math.IsInf(currencyRate, 0) || currencyRate <= 0 {
			return decimal.Zero, errors.New("monero USD to system currency rate must be positive")
		}
		quote = quote.Div(decimal.NewFromFloat(currencyRate))
	}
	groupRatio := common.GetTopupGroupRatio(group)
	if groupRatio == 0 {
		groupRatio = 1
	}
	if math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio <= 0 {
		return decimal.Zero, errors.New("topup group ratio must be positive")
	}
	quote = quote.Mul(decimal.NewFromFloat(groupRatio))
	if discount, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if math.IsNaN(discount) || math.IsInf(discount, 0) || discount <= 0 {
			return decimal.Zero, errors.New("topup discount must be positive")
		}
		quote = quote.Mul(decimal.NewFromFloat(discount))
	}
	if !quote.IsPositive() {
		return decimal.Zero, errors.New("monero quote must be positive")
	}
	return quote, nil
}

func CreateMoneroInvoice(ctx context.Context, userID int, amount int64) (*MoneroInvoice, error) {
	if !IsMoneroTopUpEnabled() {
		return nil, errors.New("monero topup is not configured")
	}
	if amount < int64(operation_setting.MinTopUp) {
		return nil, fmt.Errorf("minimum topup is %d", operation_setting.MinTopUp)
	}
	user, err := model.GetUserById(userID, true)
	if err != nil {
		return nil, err
	}
	quoteUSD, err := moneroQuoteUSD(amount, user.Group)
	if err != nil {
		return nil, err
	}
	usdPerXMR, err := getMoneroUSDPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get monero USD price: %w", err)
	}
	expectedAtomic := quoteUSD.Div(usdPerXMR).Shift(moneroAtomicUnits).Ceil()
	if !expectedAtomic.IsPositive() || expectedAtomic.GreaterThan(decimal.NewFromUint64(math.MaxUint64)) {
		return nil, errors.New("monero quote is outside the supported range")
	}
	// Freeze the conversion back into internal quota as part of the invoice.
	// For currency displays, quoteUSD has already incorporated the configured
	// USD-to-system-currency rate; for token displays, amount is already
	// internal quota. This guarantees a payment of the quoted amount credits
	// the amount the user requested, even if rates change before confirmation.
	quotaPerUSD := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		quotaPerUSD = quotaPerUSD.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	quotaPerUSD = quotaPerUSD.Div(quoteUSD)
	rpc, err := newMoneroRPCClient()
	if err != nil {
		return nil, err
	}
	tradeNo, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return nil, err
	}
	tradeNo = "monero-" + tradeNo
	seenAddresses := make(map[string]struct{})
	var address string
	var payment *model.MoneroPayment
	for {
		var addressIndex int
		address, addressIndex, err = rpc.createInvoiceAddress(ctx, tradeNo)
		if err != nil {
			return nil, err
		}
		if _, seen := seenAddresses[address]; seen {
			return nil, errors.New("monero wallet RPC repeatedly returned the same invoice address")
		}
		seenAddresses[address] = struct{}{}

		now := common.GetTimestamp()
		candidatePayment := &model.MoneroPayment{
			Address:        address,
			AccountIndex:   0,
			AddressIndex:   addressIndex,
			Network:        strings.ToLower(setting.MoneroNetwork),
			ExpectedAtomic: expectedAtomic.StringFixed(0),
			QuoteUSD:       quoteUSD.StringFixed(12),
			USDPerXMR:      usdPerXMR.StringFixed(12),
			QuotaPerUSD:    quotaPerUSD.StringFixed(12),
			Status:         model.MoneroPaymentStatusPending,
			ExpiresAt:      now + int64(moneroInvoiceExpiry/time.Second),
			CreateTime:     now,
		}
		topUp := &model.TopUp{
			UserId:          userID,
			Amount:          amount,
			Money:           quoteUSD.InexactFloat64(),
			TradeNo:         tradeNo,
			PaymentMethod:   model.PaymentMethodMonero,
			PaymentProvider: model.PaymentProviderMonero,
			CreateTime:      now,
			Status:          common.TopUpStatusPending,
		}
		if err := model.CreateMoneroPaymentInvoice(topUp, candidatePayment); err != nil {
			if errors.Is(err, model.ErrMoneroPaymentAddressConflict) {
				logger.LogWarn(ctx, fmt.Sprintf("Monero wallet RPC returned an existing invoice address address_index=%d", addressIndex))
				continue
			}
			return nil, err
		}
		payment = candidatePayment
		break
	}
	return &MoneroInvoice{
		Address:       address,
		QuotaAmount:   amount,
		AmountXMR:     expectedAtomic.Shift(-moneroAtomicUnits).StringFixed(12),
		AmountAtomic:  expectedAtomic.StringFixed(0),
		QuoteUSD:      quoteUSD.StringFixed(12),
		USDPerXMR:     usdPerXMR.StringFixed(12),
		Network:       payment.Network,
		Confirmations: setting.MoneroConfirmations,
		ExpiresAt:     payment.ExpiresAt,
	}, nil
}

func MonitorMoneroPayments(ctx context.Context) error {
	if !IsMoneroTopUpEnabled() {
		return nil
	}
	now := common.GetTimestamp()
	if err := model.ExpirePendingMoneroPayments(now); err != nil {
		return err
	}
	payments, err := model.ListPendingMoneroPayments(strings.ToLower(setting.MoneroNetwork))
	if err != nil {
		return err
	}
	if len(payments) == 0 {
		return nil
	}
	rpc, err := newMoneroRPCClient()
	if err != nil {
		return err
	}
	outgoingTransferResults, outgoingTransfersErr := rpc.outgoingTransfers(ctx)
	if outgoingTransfersErr != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Monero outgoing payment scan failed error=%q", outgoingTransfersErr.Error()))
	}
	for _, payment := range payments {
		if payment.ExpiresAt > 0 && payment.ExpiresAt <= now {
			continue
		}
		transferResults, err := rpc.incomingTransfers(ctx, payment.AddressIndex)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("Monero payment scan failed payment_id=%d address_index=%d error=%q", payment.ID, payment.AddressIndex, err.Error()))
			continue
		}
		received := decimal.Zero
		transactionIDs := make([]string, 0)
		incomingTransactionIDs := make(map[string]struct{})
		for _, transferResult := range transferResults {
			for _, transfer := range transferResult.In {
				confirmations, confirmationsErr := transfer.Confirmations.Int64()
				atomicAmount, amountErr := decimal.NewFromString(transfer.Amount.String())
				if confirmationsErr != nil || amountErr != nil || !atomicAmount.IsPositive() || transfer.DoubleSpendSeen || confirmations < int64(setting.MoneroConfirmations) {
					continue
				}
				received = received.Add(atomicAmount)
				if transfer.TxID != "" {
					transactionIDs = append(transactionIDs, transfer.TxID)
					incomingTransactionIDs[transfer.TxID] = struct{}{}
				}
			}
		}
		for _, transferResult := range outgoingTransferResults {
			for _, transfer := range transferResult.Out {
				confirmations, confirmationsErr := transfer.Confirmations.Int64()
				if confirmationsErr != nil || transfer.DoubleSpendSeen || confirmations < int64(setting.MoneroConfirmations) {
					continue
				}
				if _, alreadyReceived := incomingTransactionIDs[transfer.TxID]; transfer.TxID != "" && alreadyReceived {
					continue
				}
				matchedPayment := false
				for _, destination := range transfer.Destinations {
					if destination.Address != payment.Address {
						continue
					}
					atomicAmount, amountErr := decimal.NewFromString(destination.Amount.String())
					if amountErr != nil || !atomicAmount.IsPositive() {
						continue
					}
					received = received.Add(atomicAmount)
					matchedPayment = true
				}
				if matchedPayment && transfer.TxID != "" {
					transactionIDs = append(transactionIDs, transfer.TxID)
				}
			}
		}
		if received.IsZero() {
			continue
		}
		sort.Strings(transactionIDs)
		if _, quota, err := model.SettleMoneroPayment(payment.ID, received.StringFixed(0), transactionIDs); err != nil {
			if !strings.Contains(err.Error(), "below the expected amount") {
				logger.LogWarn(ctx, fmt.Sprintf("Monero payment settlement failed payment_id=%d error=%q", payment.ID, err.Error()))
			}
			continue
		} else if quota > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("Monero payment settled payment_id=%d quota=%d", payment.ID, quota))
		}
	}
	return nil
}

// AuditMoneroTerminalAddresses verifies that terminal invoices do not have
// locked Monero outputs. Monero wallet RPC cannot delete individual
// subaddresses, and reusing one would make late payments unsafe to attribute,
// so this task deliberately has no destructive side effects.
func AuditMoneroTerminalAddresses(ctx context.Context) (*MoneroAddressAuditResult, error) {
	payments, err := model.ListTerminalMoneroPaymentAddressAuditCandidates(strings.ToLower(setting.MoneroNetwork))
	if err != nil {
		return nil, err
	}
	result := &MoneroAddressAuditResult{Candidates: len(payments)}
	if len(payments) == 0 {
		return result, nil
	}

	rpc, err := newMoneroRPCClient()
	if err != nil {
		return nil, err
	}
	balances, err := rpc.subaddressBalances(ctx)
	if err != nil {
		return nil, err
	}
	for _, payment := range payments {
		balance, ok := balances[payment.AddressIndex]
		if !ok {
			result.NotReported++
			continue
		}
		if balance.Balance.Equal(balance.UnlockedBalance) {
			result.FullyUnlocked++
			continue
		}
		result.Locked++
	}
	return result, nil
}

type moneroPaymentMonitorHandler struct{}

func (moneroPaymentMonitorHandler) Type() string {
	return model.SystemTaskTypeMoneroPaymentMonitor
}

func (moneroPaymentMonitorHandler) Enabled() bool {
	return IsMoneroTopUpEnabled()
}

func (moneroPaymentMonitorHandler) Interval() time.Duration {
	return moneroMonitorInterval
}

func (moneroPaymentMonitorHandler) NewPayload() any {
	return nil
}

func (moneroPaymentMonitorHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	if err := MonitorMoneroPayments(ctx); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Monero payment monitor finish failed task_id=%s error=%q", task.TaskID, err.Error()))
	}
}

type moneroAddressAuditHandler struct{}

func (moneroAddressAuditHandler) Type() string {
	return model.SystemTaskTypeMoneroAddressAudit
}

func (moneroAddressAuditHandler) Enabled() bool {
	return IsMoneroTopUpEnabled()
}

func (moneroAddressAuditHandler) Interval() time.Duration {
	return moneroAddressAuditInterval
}

func (moneroAddressAuditHandler) NewPayload() any {
	return nil
}

func (moneroAddressAuditHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := AuditMoneroTerminalAddresses(ctx)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf("Monero terminal subaddress audit candidates=%d fully_unlocked=%d locked=%d not_reported=%d", result.Candidates, result.FullyUnlocked, result.Locked, result.NotReported))
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Monero address audit finish failed task_id=%s error=%q", task.TaskID, err.Error()))
	}
}

func init() {
	RegisterSystemTaskHandler(moneroPaymentMonitorHandler{})
	RegisterSystemTaskHandler(moneroAddressAuditHandler{})
}
