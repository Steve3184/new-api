/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { Coins } from 'lucide-react'
import type { ComponentType, CSSProperties } from 'react'
import {
  SiAlgorand,
  SiBitcoincash,
  SiBitcoin,
  SiBnbchain,
  SiCardano,
  SiDogecoin,
  SiEthereum,
  SiLitecoin,
  SiMonero,
  SiPolkadot,
  SiPolygon,
  SiSolana,
  SiStellar,
  SiTether,
  SiTon,
  SiXrp,
} from 'react-icons/si'

export interface CryptoCurrencyInfo {
  code: string
  symbol: string
  network?: string
  Icon: ComponentType<{
    className?: string
    style?: CSSProperties
    'aria-hidden'?: boolean | 'true' | 'false'
  }>
  color: string
}

function normalizedCurrency(value: string): string {
  return value.trim().toLowerCase().replaceAll('_', '').replaceAll('-', '')
}

function stablecoinInfo(code: string): CryptoCurrencyInfo | null {
  if (!code.startsWith('usdt')) return null

  let network: string | undefined
  if (/(bsc|bep20|bnb)/.test(code)) {
    network = 'BSC'
  } else if (/(trc20|tron)/.test(code)) {
    network = 'TRON'
  } else if (/(erc20|ethereum|eth)/.test(code)) {
    network = 'Ethereum'
  } else if (code.includes('sol')) {
    network = 'Solana'
  }

  return {
    code,
    symbol: 'USDT',
    network,
    Icon: SiTether,
    color: '#26A17B',
  }
}

/**
 * Maps the gateway's currency IDs to a user-facing asset and network label.
 * NOWPayments uses IDs such as `usdtbsc`; the network suffix is intentionally
 * kept out of the displayed asset symbol.
 */
export function getCryptoCurrencyInfo(currency: string): CryptoCurrencyInfo {
  const code = normalizedCurrency(currency)
  const stablecoin = stablecoinInfo(code)
  if (stablecoin) return stablecoin

  const definitions: Record<string, Omit<CryptoCurrencyInfo, 'code'>> = {
    btc: {
      symbol: 'BTC',
      network: 'Bitcoin',
      Icon: SiBitcoin,
      color: '#F7931A',
    },
    bitcoin: {
      symbol: 'BTC',
      network: 'Bitcoin',
      Icon: SiBitcoin,
      color: '#F7931A',
    },
    bch: {
      symbol: 'BCH',
      network: 'Bitcoin Cash',
      Icon: SiBitcoincash,
      color: '#0AC18E',
    },
    bitcoincash: {
      symbol: 'BCH',
      network: 'Bitcoin Cash',
      Icon: SiBitcoincash,
      color: '#0AC18E',
    },
    eth: {
      symbol: 'ETH',
      network: 'Ethereum',
      Icon: SiEthereum,
      color: '#627EEA',
    },
    ethereum: {
      symbol: 'ETH',
      network: 'Ethereum',
      Icon: SiEthereum,
      color: '#627EEA',
    },
    bnb: {
      symbol: 'BNB',
      network: 'BSC',
      Icon: SiBnbchain,
      color: '#F3BA2F',
    },
    bnbbsc: {
      symbol: 'BNB',
      network: 'BSC',
      Icon: SiBnbchain,
      color: '#F3BA2F',
    },
    sol: {
      symbol: 'SOL',
      network: 'Solana',
      Icon: SiSolana,
      color: '#9945FF',
    },
    solana: {
      symbol: 'SOL',
      network: 'Solana',
      Icon: SiSolana,
      color: '#9945FF',
    },
    ltc: {
      symbol: 'LTC',
      network: 'Litecoin',
      Icon: SiLitecoin,
      color: '#345D9D',
    },
    litecoin: {
      symbol: 'LTC',
      network: 'Litecoin',
      Icon: SiLitecoin,
      color: '#345D9D',
    },
    doge: {
      symbol: 'DOGE',
      network: 'Dogecoin',
      Icon: SiDogecoin,
      color: '#C2A633',
    },
    dogecoin: {
      symbol: 'DOGE',
      network: 'Dogecoin',
      Icon: SiDogecoin,
      color: '#C2A633',
    },
    matic: {
      symbol: 'MATIC',
      network: 'Polygon',
      Icon: SiPolygon,
      color: '#8247E5',
    },
    polygon: {
      symbol: 'MATIC',
      network: 'Polygon',
      Icon: SiPolygon,
      color: '#8247E5',
    },
    pol: {
      symbol: 'POL',
      network: 'Polygon',
      Icon: SiPolygon,
      color: '#8247E5',
    },
    ton: {
      symbol: 'TON',
      network: 'TON',
      Icon: SiTon,
      color: '#0098EA',
    },
    xrp: {
      symbol: 'XRP',
      network: 'XRP Ledger',
      Icon: SiXrp,
      color: '#23292F',
    },
    ripple: {
      symbol: 'XRP',
      network: 'XRP Ledger',
      Icon: SiXrp,
      color: '#23292F',
    },
    ada: {
      symbol: 'ADA',
      network: 'Cardano',
      Icon: SiCardano,
      color: '#0033AD',
    },
    cardano: {
      symbol: 'ADA',
      network: 'Cardano',
      Icon: SiCardano,
      color: '#0033AD',
    },
    dot: {
      symbol: 'DOT',
      network: 'Polkadot',
      Icon: SiPolkadot,
      color: '#E6007A',
    },
    polkadot: {
      symbol: 'DOT',
      network: 'Polkadot',
      Icon: SiPolkadot,
      color: '#E6007A',
    },
    xlm: {
      symbol: 'XLM',
      network: 'Stellar',
      Icon: SiStellar,
      color: '#000000',
    },
    stellar: {
      symbol: 'XLM',
      network: 'Stellar',
      Icon: SiStellar,
      color: '#000000',
    },
    algo: {
      symbol: 'ALGO',
      network: 'Algorand',
      Icon: SiAlgorand,
      color: '#000000',
    },
    algorand: {
      symbol: 'ALGO',
      network: 'Algorand',
      Icon: SiAlgorand,
      color: '#000000',
    },
    xmr: {
      symbol: 'XMR',
      network: 'Monero',
      Icon: SiMonero,
      color: '#FF6600',
    },
  }

  const definition = definitions[code]
  if (definition) return { code, ...definition }

  return {
    code,
    symbol: code.toUpperCase() || 'CRYPTO',
    Icon: Coins,
    color: '#6B7280',
  }
}

export function getCryptoCurrencyLabel(currency: string): string {
  const info = getCryptoCurrencyInfo(currency)
  return info.network ? `${info.symbol} (${info.network})` : info.symbol
}

/**
 * Returns a wallet-compatible QR payload where a standard native-asset URI is
 * unambiguous. Token/network IDs fall back to the deposit address because a
 * token contract is not included in the NOWPayments invoice response.
 */
export function getCryptoPaymentQrValue(
  currency: string,
  address: string,
  amount: string
): string {
  const code = normalizedCurrency(currency)
  const trimmedAddress = address.trim()
  if (!trimmedAddress) return ''

  if (code === 'btc' || code === 'bitcoin') {
    return `bitcoin:${trimmedAddress}?amount=${encodeURIComponent(amount)}`
  }
  if (code === 'eth' || code === 'ethereum') {
    return `ethereum:${trimmedAddress}?value=${encodeURIComponent(amount)}`
  }
  return trimmedAddress
}
