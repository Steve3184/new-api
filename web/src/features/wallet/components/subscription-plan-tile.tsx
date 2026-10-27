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
import { ArrowUpCircle, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  applyUsageTier,
  describePlan,
  formatBillingPeriod,
  formatMultiplier,
  getUsageTierOptions,
} from '@/features/subscriptions/lib'
import type {
  SubscriptionPlan,
  UserSubscription,
} from '@/features/subscriptions/types'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { cn } from '@/lib/utils'

type SubscriptionPlanTileProps = {
  plan: SubscriptionPlan
  recommended?: boolean
  purchaseCount: number
  activeCount: number
  /** The user's active subscription to this plan, if any. */
  currentSubscription?: UserSubscription
  /** An active subscription of another plan that may upgrade to this one. */
  upgradeSource?: UserSubscription
  onSubscribe: (plan: SubscriptionPlan, multiplier: number) => void
  onUpgrade: (
    subscription: UserSubscription,
    plan: SubscriptionPlan,
    multiplier: number
  ) => void
}

/** A purchasable plan with its usage tier switcher, price and benefits. */
export function SubscriptionPlanTile(props: SubscriptionPlanTileProps) {
  const { t } = useTranslation()
  const tiers = getUsageTierOptions(props.plan)
  const currentMultiplier = props.currentSubscription
    ? Math.max(1, Number(props.currentSubscription.multiplier || 1))
    : 1
  const [multiplier, setMultiplier] = useState(
    tiers.some((tier) => tier.multiplier === currentMultiplier)
      ? currentMultiplier
      : 1
  )
  const view = applyUsageTier(props.plan, multiplier)
  const description = describePlan(view, t)
  const price = formatBillingCurrencyFromUSD(Number(view.price_amount || 0), {
    digitsLarge: 2,
    digitsSmall: 2,
    abbreviate: false,
  })

  const purchaseLimit = Number(props.plan.max_purchase_per_user || 0)
  const activeLimit = Number(props.plan.max_active_per_user || 0)
  const purchaseLimitReached =
    purchaseLimit > 0 && props.purchaseCount >= purchaseLimit
  const activeLimitReached = activeLimit > 0 && props.activeCount >= activeLimit
  const canSwitchTier =
    props.plan.allow_tier_switch && props.plan.allow_balance_pay !== false

  let action = (
    <Button
      variant={props.recommended ? 'default' : 'outline'}
      className='w-full'
      onClick={() => props.onSubscribe(view, multiplier)}
    >
      {t('Subscribe Now')}
    </Button>
  )
  if (props.currentSubscription && multiplier === currentMultiplier) {
    action = (
      <Button variant='outline' className='w-full' disabled>
        {t('Your current plan')}
      </Button>
    )
  } else if (
    props.currentSubscription &&
    multiplier > currentMultiplier &&
    canSwitchTier
  ) {
    const subscription = props.currentSubscription
    action = (
      <Button
        className='w-full'
        onClick={() => props.onUpgrade(subscription, props.plan, multiplier)}
      >
        <ArrowUpCircle />
        {t('Upgrade to {{tier}}', { tier: formatMultiplier(multiplier) })}
      </Button>
    )
  } else if (!props.currentSubscription && props.upgradeSource) {
    const subscription = props.upgradeSource
    action = (
      <Button
        className='w-full'
        onClick={() => props.onUpgrade(subscription, props.plan, multiplier)}
      >
        <ArrowUpCircle />
        {t('Upgrade to {{plan}}', { plan: props.plan.title })}
      </Button>
    )
  } else if (purchaseLimitReached || activeLimitReached) {
    action = (
      <Tooltip>
        <TooltipTrigger render={<div />}>
          <Button variant='outline' className='w-full' disabled>
            {t('Limit Reached')}
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          {activeLimitReached
            ? `${t('Active subscription limit reached')} (${props.activeCount}/${activeLimit})`
            : `${t('Purchase limit reached')} (${props.purchaseCount}/${purchaseLimit})`}
        </TooltipContent>
      </Tooltip>
    )
  }

  return (
    <Card
      data-card-hover='false'
      className={cn(
        'h-full gap-0 py-0',
        props.recommended && 'border-primary/60 bg-primary/[0.03] shadow-sm'
      )}
    >
      <CardContent className='flex h-full flex-col gap-4 p-4 sm:p-5'>
        <div className='flex min-h-7 flex-wrap items-center justify-between gap-2'>
          <div className='flex min-w-0 items-center gap-2'>
            <h4 className='truncate font-semibold'>
              {props.plan.title || t('Subscription Plans')}
            </h4>
            {props.recommended && (
              <StatusBadge variant='info' copyable={false} className='shrink-0'>
                <Sparkles className='h-3 w-3' />
                {t('Recommended')}
              </StatusBadge>
            )}
          </div>
          {tiers.length > 1 && (
            <ToggleGroup
              value={[String(multiplier)]}
              onValueChange={(values) => {
                const next = Number(values[0])
                if (Number.isInteger(next) && next > 0) setMultiplier(next)
              }}
              variant='outline'
              size='sm'
              aria-label={t('Usage Tier')}
            >
              {tiers.map((tier) => (
                <ToggleGroupItem
                  key={tier.multiplier}
                  value={String(tier.multiplier)}
                  className='px-2.5 tabular-nums'
                >
                  {formatMultiplier(tier.multiplier)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          )}
        </div>

        {props.plan.subtitle && (
          <p className='text-muted-foreground -mt-2 text-sm'>
            {props.plan.subtitle}
          </p>
        )}

        <div className='flex items-baseline gap-1.5'>
          <span className='text-3xl font-semibold tracking-tight tabular-nums'>
            {price}
          </span>
          <span className='text-muted-foreground text-sm'>
            {formatBillingPeriod(view, t)}
          </span>
        </div>

        {action}

        <ul className='flex-1 space-y-2.5'>
          {description.features.map((feature) => (
            <li key={feature.key} className='flex items-start gap-2.5 text-sm'>
              <feature.icon
                className='text-primary mt-0.5 h-4 w-4 shrink-0'
                aria-hidden='true'
              />
              <span>{feature.label}</span>
            </li>
          ))}
        </ul>

        {description.notes.length > 0 && (
          <p className='text-muted-foreground text-xs leading-relaxed'>
            {description.notes.join(' · ')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
