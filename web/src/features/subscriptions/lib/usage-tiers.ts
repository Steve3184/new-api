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
import type {
  SubscriptionPlan,
  SubscriptionUsageTier,
  UserSubscription,
} from '../types'

export const MAX_USAGE_MULTIPLIER = 100

/** Parses the extra usage tiers stored on a plan, ignoring malformed data. */
export function parseUsageTiers(
  raw: string | undefined
): SubscriptionUsageTier[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (tier): tier is SubscriptionUsageTier =>
          typeof tier === 'object' &&
          tier !== null &&
          Number.isInteger((tier as { multiplier?: unknown }).multiplier) &&
          typeof (tier as { price_amount?: unknown }).price_amount === 'number'
      )
      .filter((tier) => tier.multiplier > 1)
      .sort((a, b) => a.multiplier - b.multiplier)
  } catch {
    return []
  }
}

/** Every purchasable tier of a plan, starting with the 1x plan itself. */
export function getUsageTierOptions(
  plan: Partial<SubscriptionPlan>
): SubscriptionUsageTier[] {
  return [
    {
      multiplier: 1,
      price_amount: Number(plan.price_amount || 0),
      stripe_price_id: plan.stripe_price_id,
      creem_product_id: plan.creem_product_id,
      waffo_pancake_product_id: plan.waffo_pancake_product_id,
    },
    ...parseUsageTiers(plan.usage_tiers),
  ]
}

function scaleQuota(value: number | undefined, multiplier: number): number {
  const quota = Number(value || 0)
  return quota > 0 ? quota * multiplier : quota
}

/**
 * The plan as sold at a usage tier: the tier price and every quota limit
 * multiplied, matching the backend snapshot. Unknown tiers fall back to 1x.
 */
export function applyUsageTier(
  plan: SubscriptionPlan,
  multiplier: number
): SubscriptionPlan {
  const tier =
    getUsageTierOptions(plan).find((item) => item.multiplier === multiplier) ??
    getUsageTierOptions(plan)[0]
  return {
    ...plan,
    price_amount: tier.price_amount,
    stripe_price_id: tier.stripe_price_id,
    creem_product_id: tier.creem_product_id,
    waffo_pancake_product_id: tier.waffo_pancake_product_id,
    total_amount: scaleQuota(plan.total_amount, tier.multiplier),
    five_hour_limit: scaleQuota(plan.five_hour_limit, tier.multiplier),
    weekly_limit: scaleQuota(plan.weekly_limit, tier.multiplier),
    monthly_limit: scaleQuota(plan.monthly_limit, tier.multiplier),
  }
}

export function formatMultiplier(multiplier: number | undefined): string {
  return `${Math.max(1, Number(multiplier || 1))}x`
}

export function getUpgradePlanIds(plan: Partial<SubscriptionPlan>): number[] {
  return (plan.upgrade_plan_ids || '')
    .split(',')
    .map((value) => Number.parseInt(value.trim(), 10))
    .filter((id) => Number.isInteger(id) && id > 0 && id !== plan.id)
}

export type UpgradeOption = {
  plan: SubscriptionPlan
  multiplier: number
}

/**
 * Targets the subscription can be upgraded to right now: higher tiers of its
 * own plan when tier switching is allowed, and every tier of the configured
 * upgrade plans. Targets that cannot be paid with balance are left out.
 */
export function getUpgradeOptions(
  subscription: UserSubscription,
  plans: SubscriptionPlan[]
): UpgradeOption[] {
  const source = plans.find((plan) => plan.id === subscription.plan_id)
  if (!source) return []
  const current = Math.max(1, Number(subscription.multiplier || 1))
  const options: UpgradeOption[] = []
  if (source.allow_tier_switch && source.allow_balance_pay !== false) {
    for (const tier of getUsageTierOptions(source)) {
      if (tier.multiplier > current) {
        options.push({ plan: source, multiplier: tier.multiplier })
      }
    }
  }
  for (const id of getUpgradePlanIds(source)) {
    const target = plans.find((plan) => plan.id === id)
    if (!target || target.allow_balance_pay === false) continue
    for (const tier of getUsageTierOptions(target)) {
      options.push({ plan: target, multiplier: tier.multiplier })
    }
  }
  return options
}

export function isActiveSubscription(
  subscription: UserSubscription,
  nowSeconds: number
): boolean {
  return subscription.status === 'active' && subscription.end_time > nowSeconds
}
