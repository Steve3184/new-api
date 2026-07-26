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
)

func IsValidMoneroNetwork(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case MoneroNetworkMainnet, MoneroNetworkTestnet, MoneroNetworkStagenet:
		return true
	default:
		return false
	}
}
