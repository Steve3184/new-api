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
import { useQuery } from '@tanstack/react-query'
import { ArrowUpCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Separator } from '@/components/ui/separator'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import { quoteSubscriptionUpgrade, upgradeSubscription } from '../../api'
import {
  applyUsageTier,
  formatBillingPeriod,
  formatMultiplier,
  getUpgradeOptions,
  getUsageTierOptions,
} from '../../lib'
import type { SubscriptionPlan, UserSubscription } from '../../types'

type SubscriptionUpgradeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  subscription: UserSubscription | null
  plans: SubscriptionPlan[]
  /** Option preselected when the dialog opens, as `planId:multiplier`. */
  initialTarget?: string
  userQuota?: number
  onUpgraded?: () => void | Promise<void>
}

function optionKey(planId: number, multiplier: number): string {
  return `${planId}:${multiplier}`
}

/**
 * Moves an active subscription to a higher usage tier or another plan. The
 * new period starts now; the unused part of the current period is credited
 * and the difference is paid from the wallet balance.
 */
export function SubscriptionUpgradeDialog(
  props: SubscriptionUpgradeDialogProps
) {
  const { t } = useTranslation()
  const [selected, setSelected] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const options = props.subscription
    ? getUpgradeOptions(props.subscription, props.plans)
    : []
  const firstOptionKey = options[0]
    ? optionKey(options[0].plan.id, options[0].multiplier)
    : ''

  useEffect(() => {
    if (props.open) setSelected(props.initialTarget || firstOptionKey)
  }, [props.open, props.initialTarget, firstOptionKey])

  const target = options.find(
    (option) => optionKey(option.plan.id, option.multiplier) === selected
  )
  const subscriptionId = props.subscription?.id
  const quote = useQuery({
    queryKey: [
      'subscription-upgrade-quote',
      subscriptionId,
      target?.plan.id,
      target?.multiplier,
    ],
    queryFn: async () => {
      const result = requireServerSuccess(
        await quoteSubscriptionUpgrade({
          subscription_id: subscriptionId ?? 0,
          plan_id: target?.plan.id ?? 0,
          multiplier: target?.multiplier ?? 1,
        })
      )
      if (!result.data) throw new Error(t('Request failed'))
      return result.data
    },
    enabled: props.open && !!subscriptionId && !!target,
    retry: false,
    staleTime: 0,
    meta: { errorToast: false },
  })

  const sourcePlan = props.plans.find(
    (plan) => plan.id === props.subscription?.plan_id
  )
  const userQuota = Math.max(0, Number(props.userQuota || 0))
  const charge = quote.data?.charge_quota ?? 0
  const insufficient = !!quote.data && userQuota < charge

  const handleUpgrade = async () => {
    if (!subscriptionId || !target) return
    setSubmitting(true)
    try {
      const result = await upgradeSubscription({
        subscription_id: subscriptionId,
        plan_id: target.plan.id,
        multiplier: target.multiplier,
      })
      if (!result.success) {
        handleServerError(result, t('Upgrade failed'))
        return
      }
      toast.success(t('Subscription upgraded'))
      props.onOpenChange(false)
      await props.onUpgraded?.()
    } catch (error) {
      handleServerError(error, t('Upgrade failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          <ArrowUpCircle className='h-5 w-5' />
          {t('Upgrade Subscription')}
        </>
      }
      description={t(
        'The new plan starts now. Unused time on your current plan is credited toward it.'
      )}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-lg'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footerClassName='flex-row items-center justify-between gap-3'
      footer={
        <>
          <div className='min-w-0 text-sm'>
            <div className='text-muted-foreground text-xs'>{t('You Pay')}</div>
            <div className='font-semibold tabular-nums'>
              {quote.data ? formatQuota(charge) : '—'}
            </div>
          </div>
          <Button
            disabled={
              submitting ||
              !target ||
              !quote.data ||
              quote.isFetching ||
              insufficient
            }
            onClick={() => void handleUpgrade()}
          >
            {t('Upgrade')}
          </Button>
        </>
      }
    >
      {sourcePlan && props.subscription && (
        <div className='text-muted-foreground text-sm'>
          {t('Current plan')}:{' '}
          <span className='text-foreground font-medium'>
            {sourcePlan.title}
            {getUsageTierOptions(sourcePlan).length > 1 &&
              ` · ${formatMultiplier(props.subscription.multiplier)}`}
          </span>
        </div>
      )}

      {options.length === 0 ? (
        <Alert>
          <AlertDescription>
            {t('No upgrade is available for this subscription.')}
          </AlertDescription>
        </Alert>
      ) : (
        <RadioGroup
          value={selected}
          onValueChange={(value) => setSelected(String(value))}
          aria-label={t('Upgrade to')}
        >
          {options.map((option) => {
            const key = optionKey(option.plan.id, option.multiplier)
            const view = applyUsageTier(option.plan, option.multiplier)
            const hasTiers = getUsageTierOptions(option.plan).length > 1
            const id = `subscription-upgrade-${key.replace(':', '-')}`
            return (
              <Label
                key={key}
                htmlFor={id}
                className={cn(
                  'hover:border-primary/40 has-data-[checked]:border-primary has-data-[checked]:ring-primary/20 flex cursor-pointer items-center gap-3 rounded-lg border p-3 font-normal transition-all has-data-[checked]:ring-2'
                )}
              >
                <RadioGroupItem id={id} value={key} />
                <div className='min-w-0 flex-1'>
                  <div className='truncate text-sm font-medium'>
                    {option.plan.title}
                    {hasTiers && (
                      <span className='text-primary ml-1.5'>
                        {formatMultiplier(option.multiplier)}
                      </span>
                    )}
                  </div>
                  <div className='text-muted-foreground text-xs'>
                    {Number(view.total_amount || 0) > 0
                      ? t('{{quota}} quota per billing period', {
                          quota: formatQuota(Number(view.total_amount)),
                        })
                      : t('Unlimited usage during the plan')}
                  </div>
                </div>
                <div className='shrink-0 text-right text-sm tabular-nums'>
                  {formatBillingCurrencyFromUSD(Number(view.price_amount), {
                    digitsLarge: 2,
                    digitsSmall: 2,
                    abbreviate: false,
                  })}
                  <span className='text-muted-foreground ml-1 text-xs'>
                    {formatBillingPeriod(view, t)}
                  </span>
                </div>
              </Label>
            )
          })}
        </RadioGroup>
      )}

      {target && (
        <div className='bg-muted/50 space-y-2 rounded-lg border p-3 text-sm'>
          <div className='flex justify-between gap-2'>
            <span className='text-muted-foreground'>{t('New plan price')}</span>
            <span className='tabular-nums'>
              {quote.data ? formatQuota(quote.data.price_quota) : '—'}
            </span>
          </div>
          <div className='flex justify-between gap-2'>
            <span className='text-muted-foreground'>
              {t('Credit for unused time')}
            </span>
            <span className='tabular-nums'>
              {quote.data ? `- ${formatQuota(quote.data.credit_quota)}` : '—'}
            </span>
          </div>
          <Separator />
          <div className='flex justify-between gap-2 font-medium'>
            <span>{t('Due from balance')}</span>
            <span className='tabular-nums'>
              {quote.data ? formatQuota(charge) : '—'}
            </span>
          </div>
          <div className='text-muted-foreground flex justify-between gap-2 text-xs'>
            <span>{t('Available')}</span>
            <span className='tabular-nums'>{formatQuota(userQuota)}</span>
          </div>
        </div>
      )}

      {quote.isError && (
        <Alert variant='destructive'>
          <AlertDescription>
            {quote.error instanceof Error
              ? quote.error.message
              : t('Request failed')}
          </AlertDescription>
        </Alert>
      )}
      {insufficient && (
        <Alert variant='destructive'>
          <AlertDescription>{t('Insufficient balance')}</AlertDescription>
        </Alert>
      )}
    </Dialog>
  )
}
