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
import type { TFunction } from 'i18next'
import {
  CalendarDays,
  CalendarRange,
  Gauge,
  Layers,
  type LucideIcon,
  Repeat,
  ShieldCheck,
  Timer,
  WalletCards,
  Zap,
} from 'lucide-react'

import { formatQuota } from '@/lib/format'

import type { SubscriptionPlan } from '../types'
import { formatResetPeriod } from './format'

export type PlanBenefit = {
  key: string
  icon: LucideIcon
  label: string
}

export type PlanDescription = {
  /** What the plan gives, in the order a buyer compares plans. */
  features: PlanBenefit[]
  /** Purchase rules and side effects shown as fine print. */
  notes: string[]
}

function splitList(value: string | undefined): string[] {
  return (value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function describeQuota(plan: SubscriptionPlan, t: TFunction): string {
  const total = Number(plan.total_amount || 0)
  if (total < 0) return t('Plan benefits only; usage is billed to your wallet')
  if (total === 0) return t('Unlimited usage during the plan')
  const quota = formatQuota(total)
  switch (plan.quota_reset_period) {
    case 'daily':
      return t('{{quota}} quota, refreshed every day', { quota })
    case 'weekly':
      return t('{{quota}} quota, refreshed every week', { quota })
    case 'monthly':
      return t('{{quota}} quota, refreshed every month', { quota })
    case 'custom':
      return t('{{quota}} quota, refreshed every {{period}}', {
        quota,
        period: formatResetPeriod(plan, t),
      })
    default:
      return t('{{quota}} quota for the whole plan period', { quota })
  }
}

function parseRateLimits(raw: string | undefined) {
  try {
    const parsed = JSON.parse(raw || '[]') as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (item): item is { group: string; rpm: number } =>
        typeof item === 'object' &&
        item !== null &&
        typeof (item as { group?: unknown }).group === 'string' &&
        typeof (item as { rpm?: unknown }).rpm === 'number'
    )
  } catch {
    return []
  }
}

/**
 * Builds the buyer-facing description of a plan (already sized for the
 * selected usage tier) from its configuration.
 */
export function describePlan(
  plan: SubscriptionPlan,
  t: TFunction
): PlanDescription {
  const total = Number(plan.total_amount || 0)
  const features: PlanBenefit[] = [
    { key: 'quota', icon: Gauge, label: describeQuota(plan, t) },
  ]
  if (total >= 0) {
    const caps = [
      {
        key: 'five-hour',
        icon: Timer,
        value: Number(plan.five_hour_limit || 0),
        label: (quota: string) => t('Up to {{quota}} every 5 hours', { quota }),
      },
      {
        key: 'weekly',
        icon: CalendarDays,
        value: Number(plan.weekly_limit || 0),
        label: (quota: string) => t('Up to {{quota}} per week', { quota }),
      },
      {
        key: 'monthly',
        icon: CalendarRange,
        value: Number(plan.monthly_limit || 0),
        label: (quota: string) => t('Up to {{quota}} per month', { quota }),
      },
    ]
    for (const cap of caps) {
      if (cap.value > 0) {
        features.push({
          key: cap.key,
          icon: cap.icon,
          label: cap.label(formatQuota(cap.value)),
        })
      }
    }
  }
  if (plan.upgrade_group) {
    features.push({
      key: 'group',
      icon: ShieldCheck,
      label: t('Access to the {{group}} group', { group: plan.upgrade_group }),
    })
  }
  for (const limit of parseRateLimits(plan.rate_limit_groups)) {
    features.push({
      key: `rpm-${limit.group}`,
      icon: Zap,
      label: t('{{rpm}} requests per minute in {{group}}', {
        rpm: limit.rpm,
        group: limit.group,
      }),
    })
  }
  if (plan.wallet_only_groups_enabled) {
    const groups = splitList(plan.wallet_only_groups)
    const list = groups.length > 0 ? groups.join(', ') : t('None')
    features.push({
      key: 'groups',
      icon: Layers,
      label:
        plan.wallet_only_groups_mode === 'whitelist'
          ? t('Covers requests in groups: {{groups}}', { groups: list })
          : t('Does not cover requests in groups: {{groups}}', {
              groups: list,
            }),
    })
  }
  if (total > 0) {
    features.push({
      key: 'overflow',
      icon: WalletCards,
      label:
        plan.allow_wallet_overflow === false
          ? t('Requests stop when the quota runs out')
          : t('Wallet balance covers usage beyond the quota'),
    })
  }
  if (plan.allow_auto_renew && plan.allow_balance_pay !== false) {
    features.push({
      key: 'auto-renew',
      icon: Repeat,
      label: t('Optional auto-renewal from your balance'),
    })
  }

  const notes: string[] = []
  const purchaseLimit = Number(plan.max_purchase_per_user || 0)
  if (purchaseLimit > 0) {
    notes.push(
      t('Limited to {{count}} purchases per account', { count: purchaseLimit })
    )
  }
  const activeLimit = Number(plan.max_active_per_user || 0)
  if (activeLimit > 0) {
    notes.push(
      t('Up to {{count}} active at the same time', { count: activeLimit })
    )
  }
  if (plan.downgrade_group) {
    notes.push(
      t('Your group changes to {{group}} when the plan ends', {
        group: plan.downgrade_group,
      })
    )
  }
  return { features, notes }
}
