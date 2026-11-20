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
import { Coins } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { toIntlLocale } from '@/i18n/languages'
import { getCurrencyLabel } from '@/lib/currency'
import {
  formatNumber,
  formatQuota,
  getEditableQuotaStep,
  quotaUnitsToEditableAmount,
} from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

import { purchaseSubscriptionExtraCredits } from '../../api'
import { usePendingExtraCreditsPurchase } from '../../hooks/use-pending-extra-credits-purchase'
import {
  canBuyExtraCredits,
  convertExtraCreditsAmount,
  extraCreditsFromWalletQuota,
  planAllowsExtraCredits,
  remainingExtraCredits,
  type ExtraCreditsConversion,
} from '../../lib'
import {
  resolvePendingExtraCreditsPurchase,
  sameBody,
  readExtraCreditsPurchaseState,
  savePendingExtraCreditsPurchase,
  type ExtraCreditsPurchasePayload,
} from '../../lib/extra-credits-request'
import type { SubscriptionPlan, UserSubscription } from '../../types'

type SubscriptionExtraCreditsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  subscription: UserSubscription | null
  /** The plan as currently sold; its rate is re-checked by the server. */
  plan?: SubscriptionPlan
  newPurchaseAvailable?: boolean
  recoverySubscriptionId?: number
  userQuota?: number
  /** Called after a successful purchase to refetch wallet and subscriptions. */
  onPurchased?: () => void | Promise<void>
}

/**
 * `message_key`s the purchase endpoint returns when it definitively refused
 * the order before charging. Any other failure leaves the outcome unknown.
 */
const EXTRA_CREDITS_REJECTION_KEYS: ReadonlySet<string> = new Set([
  'Invalid extra credits amount',
  'Invalid extra credits exchange rate',
  'Extra credits exchange rate changed; reload and try again',
  'Subscription not found',
  'Cannot purchase extra credits for this subscription',
  'Extra credits are not enabled for this plan',
  'Subscription still has unspent extra credits',
  'Insufficient balance',
  'Extra credits purchase request conflicts with an earlier purchase',
])

function newRequestId(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  // Fallback for non-secure contexts; still >= 16 chars as the backend needs.
  const random = Array.from({ length: 4 }, () =>
    Math.random().toString(36).slice(2, 10)
  ).join('')
  return `${Date.now().toString(36)}-${random}`
}

/** Preview of a stored purchase, using its own amount and rate. */
function pendingConversion(
  pending: ExtraCreditsPurchasePayload
): ExtraCreditsConversion | null {
  const credits = extraCreditsFromWalletQuota(
    pending.wallet_quota,
    pending.expected_rate
  )
  return credits === null
    ? null
    : { walletQuota: pending.wallet_quota, credits }
}

/**
 * Spends wallet balance on extra credits for one subscription. Extra credits
 * sit in their own balance next to the plan quota: the plan quota is used
 * first, then extra credits, and unspent extras survive the subscription's
 * expiry. The amount is entered in the site display currency and sent to the
 * server as exact raw quota units; the granted credits are previewed with the
 * same floor rounding the server applies.
 *
 * Every attempt is saved to localStorage (per signed-in user and
 * subscription) before it is sent, and only removed once the server answers.
 * If the network drops, the tab reloads or the user switches subscriptions,
 * the next attempt replays that exact body, so an earlier charge is returned
 * instead of repeated.
 */
export function SubscriptionExtraCreditsDialog(
  props: SubscriptionExtraCreditsDialogProps
) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const userId = useAuthStore((state) => state.auth.user?.id) || 0
  const subscriptionId =
    props.subscription?.id || props.recoverySubscriptionId || 0
  const [amount, setAmount] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmation, setConfirmation] = useState<{
    userId: number
    subscriptionId: number
    body: ExtraCreditsPurchasePayload
  } | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const ownResolved = useRef(new Set<string>())
  const [resolvedElsewhere, setResolvedElsewhere] = useState(false)
  const {
    pending,
    ready: pendingReady,
    locks,

    markUnavailable,
    refresh: refreshPending,
  } = usePendingExtraCreditsPurchase(
    userId,
    subscriptionId,
    props.open,
    (outcomes) => {
      const external = outcomes.filter(
        (outcome) =>
          !ownResolved.current.has(
            `${userId}:${subscriptionId}:${outcome.request_id}`
          )
      )
      if (!external.length) return
      setResolvedElsewhere(true)
      setAmount('')
      setConfirmOpen(false)
      setConfirmation(null)
      if (external.some((outcome) => outcome.status === 'purchased')) {
        void Promise.resolve()
          .then(() => props.onPurchased?.())
          .catch((error: unknown) =>
            handleServerError(error, undefined, {
              title: t(
                'Extra credits were purchased, but the balance could not be refreshed. Reload the page to see it.'
              ),
            })
          )
      }
    }
  )

  useEffect(() => {
    if (props.open) {
      setResolvedElsewhere(false)
      setAmount('')
      setConfirmOpen(false)
    }
  }, [props.open, userId, subscriptionId])

  const plan = props.plan
  const rate = Number(plan?.extra_credits_rate || 0)
  const owned =
    !!userId &&
    (props.subscription
      ? props.subscription.user_id === userId
      : !!props.recoverySubscriptionId)
  const enabled =
    owned &&
    props.newPurchaseAvailable !== false &&
    !!plan &&
    planAllowsExtraCredits(plan) &&
    !!props.subscription &&
    canBuyExtraCredits(props.subscription, plan, Date.now() / 1000)
  let conversion: ExtraCreditsConversion | null = null
  if (pending) {
    conversion = pendingConversion(pending)
  } else if (enabled) {
    conversion = convertExtraCreditsAmount(amount, rate)
  }
  const displayRate = pending ? pending.expected_rate : rate
  const inputValue = pending
    ? String(quotaUnitsToEditableAmount(pending.wallet_quota))
    : amount
  const userQuota = Math.max(0, Number(props.userQuota || 0))
  const latestQuota = useRef(userQuota)
  latestQuota.current = userQuota
  // A pending purchase may already have been charged, so the current balance
  // cannot decide whether retrying it is affordable; the server does.
  const insufficient =
    !pending && !!conversion && conversion.walletQuota > userQuota
  const showInvalid = !pending && amount.trim() !== '' && enabled && !conversion
  const lockSupported = !!locks
  const canContinue =
    lockSupported &&
    owned &&
    pendingReady &&
    (!!pending || enabled) &&
    !!userId &&
    !!conversion &&
    !insufficient &&
    !submitting
  const currencyLabel = getCurrencyLabel()
  const remaining = props.subscription
    ? remainingExtraCredits(props.subscription)
    : 0

  const settle = () => {
    setSubmitting(false)
    setConfirmOpen(false)
  }

  const submitPurchase = async () => {
    if (
      !owned ||
      !confirmation ||
      confirmation.userId !== userId ||
      confirmation.subscriptionId !== subscriptionId ||
      useAuthStore.getState().auth.user?.id !== confirmation.userId
    ) {
      settle()
      return false
    }
    // Send only the body shown in the confirmation. Another tab may have
    // replaced or cleared the pending transaction while this prompt was open.
    const body = confirmation.body
    const snapshot = readExtraCreditsPurchaseState(userId, subscriptionId)
    if (snapshot.outcomes[`${userId}:${subscriptionId}:${body.request_id}`]) {
      refreshPending()
      setAmount('')
      setConfirmation(null)
      settle()
      return false
    }
    if (
      !snapshot.pending &&
      (!enabled || body.wallet_quota > latestQuota.current)
    ) {
      settle()
      return false
    }
    const stored = snapshot.pending
    if (stored && !sameBody(stored, body)) {
      refreshPending()
      settle()
      toast.error(
        t(
          'The pending purchase changed in another tab. Review the amount and confirm again.'
        )
      )
      return false
    }
    // Persist even a cached retry: another tab may have removed it following a
    // definitive refusal. Storage failure must leave this attempt unsent.
    if (!savePendingExtraCreditsPurchase(userId, subscriptionId, body)) {
      refreshPending()
      settle()
      toast.error(t('Request failed'))
      return false
    }
    refreshPending()
    setSubmitting(true)
    let result: Awaited<ReturnType<typeof purchaseSubscriptionExtraCredits>>
    try {
      result = await purchaseSubscriptionExtraCredits(subscriptionId, {
        ...body,
      })
    } catch (error) {
      // The request may have reached the server: keep the stored body so the
      // retry is idempotent.
      handleServerError(error, t('Request failed'))
      settle()
      return
    }
    const receipt = result?.data
    const confirmed =
      result?.success === true &&
      receipt?.request_id === body.request_id &&
      receipt.subscription_id === subscriptionId &&
      receipt.wallet_quota === body.wallet_quota &&
      receipt.rate === body.expected_rate &&
      receipt.user_id === userId &&
      receipt.credits ===
        extraCreditsFromWalletQuota(body.wallet_quota, body.expected_rate)
    // Only a business refusal the purchase handler itself returns proves the
    // order was not charged. A generic success:false (gateway, proxy, outage)
    // says nothing about the first attempt, so its id must be kept.
    const messageKey = (result as { message_key?: unknown } | undefined)
      ?.message_key
    const rejected =
      result?.success === false &&
      typeof messageKey === 'string' &&
      EXTRA_CREDITS_REJECTION_KEYS.has(messageKey)
    if (!confirmed && !rejected) {
      toast.error(t('Request failed'))
      settle()
      return
    }
    ownResolved.current.add(`${userId}:${subscriptionId}:${body.request_id}`)
    setResolvedElsewhere(false)
    const cleared = resolvePendingExtraCreditsPurchase(
      userId,
      subscriptionId,
      body,
      confirmed ? 'purchased' : 'rejected'
    )
    setAmount('')
    setConfirmation(null)
    refreshPending()
    if (!cleared) {
      settle()
      toast.error(
        t(
          'The purchase result could not be saved reliably in this browser. Check your balance before retrying in any tab.'
        )
      )
      if (confirmed) {
        toast.success(t('Extra credits purchased'))
        return true
      }
      handleServerError(result, t('Request failed'))
      return
    }
    if (!confirmed) {
      handleServerError(result, t('Request failed'))
      settle()
      return
    }
    toast.success(t('Extra credits purchased'))
    settle()
    props.onOpenChange(false)
    return true
  }

  const handlePurchase = async () => {
    let purchased = false
    try {
      if (!locks) {
        toast.error(t('Request failed'))
        return
      }
      await locks.request(
        'extra-credits-pending-store',
        { ifAvailable: true },
        async (lock) => {
          if (!lock) {
            refreshPending()
            toast.error(t('Request failed'))
            return
          }
          purchased = (await submitPurchase()) === true
        }
      )
    } catch (error) {
      markUnavailable()
      handleServerError(error, t('Request failed'))
      settle()
    }
    // Balance refresh does not read or write the pending map and must not
    // block another purchase while its network requests finish.
    if (purchased) {
      try {
        await props.onPurchased?.()
      } catch (error) {
        handleServerError(error, undefined, {
          title: t(
            'Extra credits were purchased, but the balance could not be refreshed. Reload the page to see it.'
          ),
        })
      }
    }
  }

  const confirmationConversion = confirmation
    ? pendingConversion(confirmation.body)
    : null

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={
          <>
            <Coins className='h-5 w-5' aria-hidden='true' />
            {t('Buy Extra Credits')}
          </>
        }
        description={t(
          'Convert wallet balance into extra credits for this subscription. Plan quota is used first, then extra credits. Unused extra credits stay after expiry.'
        )}
        contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
        titleClassName='flex items-center gap-2'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <Button
            disabled={!canContinue}
            onClick={() => {
              if (!conversion) return
              setConfirmation({
                userId,
                subscriptionId,
                body: pending
                  ? { ...pending }
                  : {
                      request_id: newRequestId(),
                      wallet_quota: conversion.walletQuota,
                      expected_rate: rate,
                    },
              })
              setConfirmOpen(true)
            }}
          >
            {t('Continue')}
          </Button>
        }
      >
        {!lockSupported && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t(
                'Purchases are unavailable because this browser cannot coordinate tabs. Keep any pending purchase in this browser until its status can be checked.'
              )}
            </AlertDescription>
          </Alert>
        )}
        {!enabled && !pending ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {plan && planAllowsExtraCredits(plan)
                ? t('Cannot purchase extra credits for this subscription')
                : t('Extra credits are not enabled for this plan')}
            </AlertDescription>
          </Alert>
        ) : (
          <>
            <div className='bg-muted/50 space-y-1 rounded-lg border p-3 text-sm'>
              <div className='flex justify-between gap-2'>
                <span className='text-muted-foreground'>
                  {t('Exchange rate')}
                </span>
                <span className='tabular-nums'>
                  {t(
                    '1 {{currency}} spent = {{rate}} {{currency}} extra credits',
                    {
                      currency: currencyLabel,
                      rate: formatNumber(displayRate, locale),
                    }
                  )}
                </span>
              </div>
              {props.subscription && (
                <div className='flex justify-between gap-2'>
                  <span className='text-muted-foreground'>
                    {t('Remaining extra credits')}
                  </span>
                  <span className='tabular-nums'>{formatQuota(remaining)}</span>
                </div>
              )}
            </div>

            <div className='space-y-2'>
              <Label htmlFor='extra-credits-amount'>
                {t('Amount ({{currency}})', { currency: currencyLabel })}
              </Label>
              <Input
                id='extra-credits-amount'
                type='number'
                inputMode='decimal'
                min={0}
                step={getEditableQuotaStep()}
                value={inputValue}
                disabled={!!pending || submitting}
                aria-invalid={showInvalid || insufficient}
                onChange={(event) => setAmount(event.target.value)}
              />
            </div>

            {resolvedElsewhere && !pending && (
              <Alert>
                <AlertDescription>
                  {t(
                    'The earlier purchase was resolved in another tab. Enter an amount only if you want to make a new purchase.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            {pending && (
              <Alert>
                <AlertDescription>
                  {t(
                    'An earlier purchase of this amount may not have completed. Continue to retry it; you will not be charged twice.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            {conversion && (
              <div className='space-y-2 rounded-lg border p-3 text-sm'>
                <div className='flex justify-between gap-2'>
                  <span className='text-muted-foreground'>
                    {t('Due from balance')}
                  </span>
                  <span className='tabular-nums'>
                    {formatQuota(conversion.walletQuota)}
                  </span>
                </div>
                <Separator />
                <div className='flex justify-between gap-2 font-medium'>
                  <span>{t('Extra credits you receive')}</span>
                  <span className='tabular-nums'>
                    {formatQuota(conversion.credits)}
                  </span>
                </div>
                <div className='text-muted-foreground flex justify-between gap-2 text-xs'>
                  <span>{t('Available')}</span>
                  <span className='tabular-nums'>{formatQuota(userQuota)}</span>
                </div>
              </div>
            )}

            {showInvalid && (
              <Alert variant='destructive'>
                <AlertDescription>
                  {t('Invalid extra credits amount')}
                </AlertDescription>
              </Alert>
            )}
            {insufficient && (
              <Alert variant='destructive'>
                <AlertDescription>{t('Insufficient balance')}</AlertDescription>
              </Alert>
            )}
          </>
        )}
      </Dialog>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => {
          if (!submitting) setConfirmOpen(open)
        }}
        title={t('Confirm extra credits purchase')}
        desc={
          confirmationConversion
            ? t(
                '{{cost}} will be deducted from your wallet balance for {{credits}} of extra credits. This cannot be undone.',
                {
                  cost: formatQuota(confirmationConversion.walletQuota),
                  credits: formatQuota(confirmationConversion.credits),
                }
              )
            : ''
        }
        isLoading={submitting}
        disabled={
          !lockSupported ||
          !owned ||
          !confirmationConversion ||
          (!pending && confirmationConversion.walletQuota > userQuota) ||
          submitting ||
          confirmation?.userId !== userId ||
          confirmation?.subscriptionId !== subscriptionId
        }
        handleConfirm={() => void handlePurchase()}
      />
    </>
  )
}
