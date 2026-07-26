# Monero payments

The Monero gateway creates one wallet subaddress per top-up invoice. It queries
`monero-wallet-rpc` for incoming transfers, waits for the configured number of
block confirmations, and then credits the user from the USD/XMR quote captured
when the invoice was created. The system quota conversion is also captured from
the administrator's existing `QuotaPerUnit` setting, so later rate changes do
not alter an existing invoice.

Invoices expire one hour after creation. After expiry, the monitor will not
credit a late payment to that invoice.

`new-api` does not need a local `monerod`. Run `monero-wallet-rpc` with a
trusted remote daemon instead. A Cake Wallet-compatible remote node is one
possible choice; choose a node and network appropriate for your deployment,
and do not expose wallet RPC to the public Internet.

For testnet, start wallet RPC in testnet mode and point it at a trusted testnet
daemon, for example:

```bash
monero-wallet-rpc \
  --testnet \
  --wallet-file /srv/monero/testnet.wallet \
  --daemon-address YOUR_TRUSTED_TESTNET_NODE:28081 \
  --trusted-daemon \
  --rpc-bind-ip 127.0.0.1 \
  --rpc-bind-port 18082 \
  --rpc-login USERNAME:STRONG_PASSWORD
```

In **System Settings → Billing → Payment Gateway → Monero**, configure the
wallet RPC URL (normally `http://127.0.0.1:18082/json_rpc`), credentials,
network, and confirmation count. Select **testnet** before enabling the
gateway. The application verifies the returned subaddress prefix against the
selected network and refuses a mismatched wallet.

The default exchange-rate source is CoinGecko's public Monero/USD endpoint.
Each invoice stores its USD/XMR rate, XMR atomic amount, confirmation target,
and USD-to-system-quota conversion snapshot. Overpayments are credited at the
same stored rate; underpayments remain pending until the full invoice amount is
received. The displayed XMR amount excludes network fees. Users pay any wallet
or network fee in addition to that amount, while credited quota is calculated
from the XMR amount actually received at the invoice's stored rate.

Selecting **Monero** on the wallet page creates the invoice immediately. The
dialog shows the requested quota, then the exact XMR principal to pay before
the wallet address and QR code.
