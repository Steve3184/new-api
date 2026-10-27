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
import { ArrowUpCircle } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { updateSubscriptionRenewal } from '@/features/subscriptions/api'
import {
  formatMultiplier,
  getUsageTierOptions,
  isActiveSubscription,
} from '@/features/subscriptions/lib'
import type {
  SubscriptionPlan,
  UserSubscription,
} from '@/features/subscriptions/types'
import { formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

type MySubscriptionItemProps = {
  subscription: UserSubscription
  /** The public plan of the subscription; absent when it is no longer sold. */
  plan?: SubscriptionPlan
  canUpgrade: boolean
  onUpgrade: (subscription: UserSubscription) => void
  onChanged: () => void | Promise<void>
}

function usagePercent(used: number, limit: number): number {
  if (limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((used / limit) * 100)))
}

/** One of the user's subscriptions with usage, renewal and upgrade controls. */
export function MySubscriptionItem(props: MySubscriptionItemProps) {
  const { t } = useTranslation()
  const [saving, setSaving] = useState(false)
  const sub = props.subscription
  const now = Date.now() / 1000
  const isActive = isActiveSubscription(sub, now)
  const multiplier = Math.max(1, Number(sub.multiplier || 1))
  const tiers = props.plan ? getUsageTierOptions(props.plan) : []
  const showMultiplier = multiplier > 1 || tiers.length > 1
  const renewMultiplier = Number(sub.renew_multiplier || 0) || multiplier
  const canAutoRenew =
    isActive &&
    !!props.plan?.allow_auto_renew &&
    props.plan.allow_balance_pay !== false
  const canPickRenewTier =
    canAutoRenew &&
    !!sub.auto_renew &&
    !!props.plan?.allow_tier_switch &&
    tiers.length > 1

  const saveRenewal = async (autoRenew: boolean, nextMultiplier: number) => {
    setSaving(true)
    try {
      const result = await updateSubscriptionRenewal(sub.id, {
        auto_renew: autoRenew,
        renew_multiplier: nextMultiplier,
      })
      if (!result.success) {
        handleServerError(result, t('Update failed'))
        return
      }
      toast.success(t('Updated successfully'))
      await props.onChanged()
    } catch (error) {
      handleServerError(error, t('Update failed'))
    } finally {
      setSaving(false)
    }
  }

  let status: { label: string; variant: 'neutral' | 'success' } = {
    label: t('Expired'),
    variant: 'neutral',
  }
  if (isActive) status = { label: t('Active'), variant: 'success' }
  else if (sub.status === 'cancelled') {
    status = { label: t('Cancelled'), variant: 'neutral' }
  } else if (sub.status === 'upgraded') {
    status = { label: t('Upgraded'), variant: 'neutral' }
  }

  let endLabel = t('Expired at')
  if (isActive) endLabel = sub.auto_renew ? t('Renews on') : t('Until')
  else if (sub.status === 'cancelled') endLabel = t('Cancelled at')
  else if (sub.status === 'upgraded') endLabel = t('Upgraded at')

  const total = Number(sub.amount_total || 0)
  const used = Number(sub.amount_used || 0)
  const usages = [
    {
      key: 'total',
      label: t('Total Quota'),
      limit: total,
      used,
      next: sub.next_reset_time ?? 0,
    },
    {
      key: 'five-hour',
      label: t('5-hour Quota Limit'),
      limit: Number(sub.five_hour_limit || 0),
      used: Number(sub.five_hour_used || 0),
      next: sub.five_hour_next_reset_time ?? 0,
    },
    {
      key: 'weekly',
      label: t('Weekly Quota Limit'),
      limit: Number(sub.weekly_limit || 0),
      used: Number(sub.weekly_used || 0),
      next: sub.weekly_next_reset_time ?? 0,
    },
    {
      key: 'monthly',
      label: t('Monthly Quota Limit'),
      limit: Number(sub.monthly_limit || 0),
      used: Number(sub.monthly_used || 0),
      next: sub.monthly_next_reset_time ?? 0,
    },
  ].filter((usage) => usage.limit > 0)

  return (
    <div className='bg-background space-y-2.5 rounded-lg border p-3 text-xs'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex min-w-0 items-center gap-2'>
          <span className='truncate text-sm font-medium'>
            {props.plan?.title || `${t('Subscription')} #${sub.id}`}
          </span>
          {showMultiplier && (
            <span className='bg-primary/10 text-primary rounded px-1.5 py-0.5 font-medium tabular-nums'>
              {formatMultiplier(multiplier)}
            </span>
          )}
          <StatusBadge
            label={status.label}
            variant={status.variant}
            copyable={false}
          />
        </div>
        {isActive && (
          <span className='text-muted-foreground'>
            {t('{{count}} days remaining', {
              count: Math.max(0, Math.ceil((sub.end_time - now) / 86400)),
            })}
          </span>
        )}
      </div>

      <div className='text-muted-foreground'>
        {endLabel} {new Date(sub.end_time * 1000).toLocaleString()}
        <span className='text-muted-foreground/50 ml-2'>#{sub.id}</span>
      </div>

      {total < 0 && (
        <div className='text-muted-foreground'>
          {t('Plan benefits only; usage is billed to your wallet')}
        </div>
      )}
      {total === 0 && (
        <div className='text-muted-foreground'>
          {t('Total Quota')}: {t('Unlimited')}
        </div>
      )}
      {usages.map((usage) => {
        const percent = usagePercent(usage.used, usage.limit)
        return (
          <div key={usage.key} className='space-y-1'>
            <div className='text-muted-foreground flex flex-wrap justify-between gap-x-2'>
              <span>{usage.label}</span>
              <Tooltip>
                <TooltipTrigger
                  render={<span className='cursor-help tabular-nums' />}
                >
                  {formatQuota(usage.used)} / {formatQuota(usage.limit)} ·{' '}
                  {percent}%
                </TooltipTrigger>
                <TooltipContent>
                  {t('Remaining')}{' '}
                  {formatQuota(Math.max(0, usage.limit - usage.used))}
                  {isActive && usage.next > 0 && (
                    <>
                      {' · '}
                      {t('Next reset')}:{' '}
                      {new Date(usage.next * 1000).toLocaleString()}
                    </>
                  )}
                </TooltipContent>
              </Tooltip>
            </div>
            {isActive && <Progress value={percent} className='h-1.5' />}
          </div>
        )
      })}

      {isActive && (canAutoRenew || props.canUpgrade) && (
        <div className='flex flex-wrap items-center justify-between gap-2 border-t pt-2.5'>
          {canAutoRenew ? (
            <div className='flex flex-wrap items-center gap-2'>
              <Switch
                id={`subscription-renew-${sub.id}`}
                checked={!!sub.auto_renew}
                disabled={saving}
                onCheckedChange={(checked) =>
                  void saveRenewal(checked, checked ? renewMultiplier : 0)
                }
              />
              <Label
                htmlFor={`subscription-renew-${sub.id}`}
                className='text-xs'
              >
                {t('Auto-renew from balance')}
              </Label>
              {canPickRenewTier && (
                <Select
                  items={tiers.map((tier) => ({
                    value: String(tier.multiplier),
                    label: formatMultiplier(tier.multiplier),
                  }))}
                  value={String(renewMultiplier)}
                  disabled={saving}
                  onValueChange={(value) =>
                    value !== null && void saveRenewal(true, Number(value))
                  }
                >
                  <SelectTrigger
                    size='sm'
                    className='h-7 text-xs'
                    aria-label={t('Next renewal tier')}
                  >
                    <SelectValue>
                      {t('Next renewal: {{tier}}', {
                        tier: formatMultiplier(renewMultiplier),
                      })}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {tiers.map((tier) => (
                        <SelectItem
                          key={tier.multiplier}
                          value={String(tier.multiplier)}
                        >
                          {formatMultiplier(tier.multiplier)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              )}
            </div>
          ) : (
            <span />
          )}
          {props.canUpgrade && (
            <Button
              variant='outline'
              size='sm'
              className='h-7 text-xs'
              onClick={() => props.onUpgrade(sub)}
            >
              <ArrowUpCircle />
              {t('Upgrade')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
