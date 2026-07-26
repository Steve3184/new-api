package setting

import "strings"

const (
	MoneroNetworkMainnet  = "mainnet"
	MoneroNetworkTestnet  = "testnet"
	MoneroNetworkStagenet = "stagenet"
)

var (
	// MoneroEnabled keeps the gateway opt-in even when a wallet RPC endpoint
	// has been configured for maintenance or testing.
	MoneroEnabled = false
	// MoneroWalletRPCURL is the monero-wallet-rpc JSON-RPC endpoint. The wallet
	// process may use a remote daemon, so new-api never needs to run monerod.
	MoneroWalletRPCURL      = ""
	MoneroWalletRPCUsername = ""
	MoneroWalletRPCPassword = ""
	MoneroNetwork           = MoneroNetworkMainnet
	MoneroConfirmations     = 1
	// MoneroMaxSubaddresses limits the total number of addresses in account 0,
	// including the primary address. It prevents a long-running invoice flow
	// from growing the wallet's subaddress list without bound.
	MoneroMaxSubaddresses = 10000
	// MoneroUSDToCurrencyRate optionally overrides the system display rate
	// when converting a wallet top-up amount to USD for an XMR invoice. Zero
	// keeps the system display rate for backward compatibility.
	MoneroUSDToCurrencyRate = 0.0
)

func IsValidMoneroNetwork(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case MoneroNetworkMainnet, MoneroNetworkTestnet, MoneroNetworkStagenet:
		return true
	default:
		return false
	}
}
