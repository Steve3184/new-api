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

For commercial licensing, please contact support@quantumnous.com
*/
import { getCurrencyDisplay } from '@/lib/currency'

import type { SubscriptionPlan, UserSubscription } from '../types'

/** Largest raw quota amount the backend accepts (common.MaxWalletQuota). */
export const MAX_EXTRA_CREDITS_WALLET_QUOTA = 2 ** 53 - 1

/** Smallest exchange rate the backend accepts for an enabled plan. */
export const MIN_EXTRA_CREDITS_RATE = 0.000001
/** Largest exchange rate the backend accepts. */
export const MAX_EXTRA_CREDITS_RATE = 1_000_000
/** The rate column is decimal(18,6); finer precision is rejected. */
export const EXTRA_CREDITS_RATE_DECIMALS = 6

/**
 * Mirrors the backend's rate validation: finite, inside the accepted range and
 * representable in six decimals, so the client never promises a conversion the
 * server would refuse.
 */
export function isValidExtraCreditsRate(rate: unknown): rate is number {
  return (
    typeof rate === 'number' &&
    Number.isFinite(rate) &&
    rate >= MIN_EXTRA_CREDITS_RATE &&
    rate <= MAX_EXTRA_CREDITS_RATE &&
    Number(rate.toFixed(EXTRA_CREDITS_RATE_DECIMALS)) === rate
  )
}

const RATE_TEXT_SCALE = 1_000_000n

/**
 * The rate an administrator typed, validated from its exact decimal text.
 *
 * Validating the parsed number is not enough: `Number.parseFloat` maps
 * `1000000.00000000001` to exactly `1000000`, so the extra decimals that make
 * the value invalid never reach the check the server applies. The text is
 * therefore compared as an exact fraction — range first, then the six-decimal
 * column the rate is stored in — and converted only once it is known to be
 * representable. Returns null for anything the backend would refuse, including
 * non-decimal text and non-strings.
 */
export function parseExtraCreditsRateText(text: unknown): number | null {
  if (typeof text !== 'string' || text.length > 64) return null
  const pieces = /^(?:\+)?(\d*)(?:\.(\d*))?(?:e([+-]?\d+))?$/i.exec(text.trim())
  if (
    !pieces ||
    Math.abs(Number(pieces[3] || 0) - (pieces[2]?.length || 0)) > 64
  ) {
    return null
  }
  const value = parseExactDecimal(text)
  if (!value || value.num <= 0n) return null
  const scaled = value.num * RATE_TEXT_SCALE
  if (scaled < value.den) return null // Below the smallest accepted rate.
  if (value.num > BigInt(MAX_EXTRA_CREDITS_RATE) * value.den) return null
  // decimal(18,6): a rate with more than six decimals cannot be stored.
  if (scaled % value.den !== 0n) return null
  const rate = Number(scaled / value.den) / Number(RATE_TEXT_SCALE)
  return isValidExtraCreditsRate(rate) ? rate : null
}

/** True when `text` is a rate the backend would accept. */
export function isValidExtraCreditsRateText(text: unknown): boolean {
  return parseExtraCreditsRateText(text) !== null
}

export type ExtraCreditsConversion = {
  /** The display amount the user typed, parsed to raw site quota units. */
  walletQuota: number
  /**
   * Subscription credit quota units the purchase grants, using the same
   * floor-rounding the backend applies (`decimal(walletQuota) * rate` floored).
   */
  credits: number
}

/**
 * A plan can sell extra credits only when the administrator enabled the toggle
 * and configured a strictly positive, finite exchange rate. Old plans default
 * to disabled with a zero rate, so this stays false for them.
 */
export function planAllowsExtraCredits(
  plan: Pick<SubscriptionPlan, 'allow_extra_credits' | 'extra_credits_rate'>
): boolean {
  return (
    plan.allow_extra_credits === true &&
    isValidExtraCreditsRate(Number(plan.extra_credits_rate || 0))
  )
}

/**
 * Whether the user may buy extra credits for a subscription right now.
 *
 * Only an active subscription that has not run out of time qualifies, and the
 * plan must still sell extra credits. Extras left on a subscription when it
 * ends are refunded to the wallet, so expired, cancelled and upgraded rows are
 * never topped up.
 */
export function canBuyExtraCredits(
  subscription: Pick<
    UserSubscription,
    'status' | 'end_time' | 'extra_credits_can_purchase'
  >,
  plan:
    | Pick<SubscriptionPlan, 'allow_extra_credits' | 'extra_credits_rate'>
    | undefined,
  nowSeconds: number
): boolean {
  if (!plan || !planAllowsExtraCredits(plan)) return false
  // The server derives this from the same status and group checks the
  // purchase applies (including groups changed after expiry); trust it.
  if (subscription.extra_credits_can_purchase === false) return false
  if (subscription.status !== 'active') return false
  // A subscription that ran out of time is not sold more credits; whatever
  // it still holds is refunded to the wallet.
  return !(subscription.end_time > 0 && subscription.end_time <= nowSeconds)
}

/** Remaining spendable extra credits on a subscription, never negative. */
export function remainingExtraCredits(
  subscription: Pick<
    UserSubscription,
    'extra_credits_total' | 'extra_credits_used'
  >
): number {
  const total = Number(subscription.extra_credits_total || 0)
  const used = Number(subscription.extra_credits_used || 0)
  return Math.max(0, total - used)
}

/** True when the subscription has any extra credits, spent or not. */
export function hasExtraCredits(
  subscription: Pick<
    UserSubscription,
    'extra_credits_total' | 'extra_credits_used'
  >
): boolean {
  return Number(subscription.extra_credits_total || 0) > 0
}

type Fraction = { num: bigint; den: bigint }

/** Exact value of a plain or exponent decimal string, or null. */
function parseExactDecimal(text: string): Fraction | null {
  const match = /^\+?(\d*)(?:\.(\d*))?(?:e([+-]?\d+))?$/i.exec(text.trim())
  if (!match || (match[1] === '' && !match[2])) return null
  const fraction = match[2] || ''
  const exponent = Number(match[3] || 0) - fraction.length
  if (!Number.isSafeInteger(exponent) || Math.abs(exponent) > 400) return null
  let num = BigInt((match[1] || '0') + fraction)
  let den = 1n
  if (exponent >= 0) num *= 10n ** BigInt(exponent)
  else den = 10n ** BigInt(-exponent)
  return { num, den }
}

/**
 * Raw wallet quota for a typed display amount, computed exactly from the
 * decimal text and the configured currency (`quotaPerUnit`, exchange rate),
 * rounded half up like `parseQuotaFromDollars`. Null when not representable.
 */
export function parseExtraCreditsWalletQuota(amount: string): number | null {
  const value = parseExactDecimal(amount)
  if (!value || value.num <= 0n) return null
  const { config, meta } = getCurrencyDisplay()
  let num = value.num
  let den = value.den
  if (meta.kind === 'tokens' && num % den !== 0n) return null
  if (meta.kind !== 'tokens') {
    const perUnit = parseExactDecimal(String(config.quotaPerUnit))
    const exchange =
      meta.exchangeRate > 0
        ? parseExactDecimal(String(meta.exchangeRate))
        : null
    if (!perUnit) return null
    num *= perUnit.num
    den *= perUnit.den
    if (exchange && exchange.num > 0n) {
      num *= exchange.den
      den *= exchange.num
    }
  }
  const quota = (2n * num + den) / (2n * den)
  if (quota <= 0n || quota > BigInt(MAX_EXTRA_CREDITS_WALLET_QUOTA)) return null
  return Number(quota)
}

/**
 * Turn a display amount (configured currency) the user entered into the raw
 * wallet quota debited and the subscription credits granted. Returns null for
 * anything the backend would reject. The amount stays a decimal string until
 * converted, so no precision is lost to floating point.
 *
 * Credits are floored to match the backend's `decimal.Mul(...).Floor()`.
 */
export function convertExtraCreditsAmount(
  displayAmount: string | number,
  rate: number
): ExtraCreditsConversion | null {
  if (!isValidExtraCreditsRate(rate)) return null
  if (typeof displayAmount === 'number' && !Number.isFinite(displayAmount)) {
    return null
  }
  const walletQuota = parseExtraCreditsWalletQuota(String(displayAmount))
  if (walletQuota === null) return null
  const credits = extraCreditsFromWalletQuota(walletQuota, rate)
  if (credits === null) return null
  return { walletQuota, credits }
}

/**
 * Credits an exact raw wallet quota grants at `rate`, floored like the
 * backend, or null when the result is unusable. Replaying a stored pending
 * purchase previews with this so the amount and rate stay the ones the server
 * will actually charge.
 */
export function extraCreditsFromWalletQuota(
  walletQuota: number,
  rate: number
): number | null {
  if (!Number.isSafeInteger(walletQuota) || walletQuota <= 0) return null
  if (walletQuota > MAX_EXTRA_CREDITS_WALLET_QUOTA) return null
  if (!isValidExtraCreditsRate(rate)) return null

  const credits = Number(
    (BigInt(walletQuota) * BigInt(Math.round(rate * 1_000_000))) / 1_000_000n
  )
  return Number.isSafeInteger(credits) && credits > 0 ? credits : null
}
