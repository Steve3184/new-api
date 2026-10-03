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
import { CalendarClock, Crown, Package, WalletCards } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { GroupBadge } from '@/components/group-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { NowPaymentsCurrencyDialog } from '@/features/wallet/components/dialogs/nowpayments-currency-dialog'
import { PaymentMethodCard } from '@/features/wallet/components/payment-method-card'
import { submitPaymentForm } from '@/features/wallet/lib/payment'
import type { NowPaymentsInvoice } from '@/features/wallet/types'
import { useSystemConfig } from '@/hooks/use-system-config'
import { toIntlLocale } from '@/i18n/languages'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatQuota, formatNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'

import {
  quoteSubscriptionPayment,
  paySubscriptionStripe,
  paySubscriptionCreem,
  paySubscriptionEpay,
  paySubscriptionWaffoPancake,
  paySubscriptionBalance,
  paySubscriptionNowPayments,
} from '../../api'
import { formatDuration, formatResetPeriod } from '../../lib'
import type { PlanRecord } from '../../types'

interface PaymentMethod {
  type: string
  name?: string
  gateway?: string
  fee?: number
  fee_rate?: number
  icon?: string
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  enableStripe?: boolean
  enableCreem?: boolean
  enableWaffoPancake?: boolean
  enableNowPayments?: boolean
  nowPaymentsCurrencies?: string[]
  enableOnlineTopUp?: boolean
  epayMethods?: PaymentMethod[]
  paymentMethods?: PaymentMethod[]
  purchaseLimit?: number
  purchaseCount?: number
  activePurchaseLimit?: number
  activePurchaseCount?: number
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
  onNowPaymentsInvoice?: (invoice: NowPaymentsInvoice) => void
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t, i18n } = useTranslation()
  const { currency } = useSystemConfig()
  const [paying, setPaying] = useState(false)
  const [selectedPayment, setSelectedPayment] = useState('balance')
  const [selectedNowPaymentsCurrency, setSelectedNowPaymentsCurrency] =
    useState('')
  const [nowPaymentsCurrencyDialogOpen, setNowPaymentsCurrencyDialogOpen] =
    useState(false)

  useEffect(() => {
    if (!props.open) return
    const currentPlan = props.plan?.plan
    if (currentPlan?.allow_balance_pay !== false) setSelectedPayment('balance')
    else if (props.enableStripe) setSelectedPayment('stripe')
    else if (props.enableCreem) setSelectedPayment('creem')
    else if (props.enableWaffoPancake) setSelectedPayment('waffo_pancake')
    else if (props.enableNowPayments) setSelectedPayment('nowpayments')
    else if (props.enableOnlineTopUp && props.epayMethods?.length) {
      setSelectedPayment('epay-0')
    } else setSelectedPayment('')
  }, [
    props.open,
    props.plan?.plan,
    props.enableStripe,
    props.enableCreem,
    props.enableWaffoPancake,
    props.enableNowPayments,
    props.enableOnlineTopUp,
    props.epayMethods,
  ])

  useEffect(() => {
    const currencies = props.nowPaymentsCurrencies || []
    if (props.open && currencies.length > 0) {
      if (!currencies.includes(selectedNowPaymentsCurrency)) {
        setSelectedNowPaymentsCurrency(currencies[0])
      }
    } else if (!props.open) {
      setSelectedNowPaymentsCurrency('')
      setNowPaymentsCurrencyDialogOpen(false)
    }
  }, [props.open, props.nowPaymentsCurrencies, selectedNowPaymentsCurrency])

  const selectedEpayMethodConfig = selectedPayment.startsWith('epay-')
    ? (props.epayMethods || [])[Number.parseInt(selectedPayment.slice(5), 10)]
    : undefined
  const quoteMethod = selectedEpayMethodConfig?.type || selectedPayment
  const paymentQuote = useQuery({
    queryKey: [
      'subscription-payment-quote',
      props.plan?.plan.id,
      quoteMethod,
      selectedEpayMethodConfig?.gateway,
    ],
    queryFn: async () => {
      if (!props.plan) throw new Error(t('Payment request failed'))
      const result = await quoteSubscriptionPayment({
        plan_id: props.plan.plan.id,
        payment_method: quoteMethod,
        epay_gateway: selectedEpayMethodConfig?.gateway,
      })
      requireServerSuccess(result)
      if (!result.data) throw new Error(t('Payment request failed'))
      return result.data
    },
    enabled:
      props.open &&
      !!props.plan &&
      !!quoteMethod &&
      selectedPayment !== 'balance',
    retry: false,
    staleTime: 0,
  })
  const plan = props.plan?.plan
  if (!plan) return null

  const hasStripe = props.enableStripe
  const hasCreem = props.enableCreem
  const hasWaffoPancake = props.enableWaffoPancake
  const nowPaymentsCurrencies = props.nowPaymentsCurrencies || []
  const hasNowPayments =
    props.enableNowPayments && nowPaymentsCurrencies.length > 0
  const hasEpay =
    props.enableOnlineTopUp && (props.epayMethods || []).length > 0
  const totalAmount = Number(plan.total_amount || 0)
  const price = formatBillingCurrencyFromUSD(Number(plan.price_amount || 0), {
    digitsLarge: 2,
    digitsSmall: 2,
    abbreviate: false,
  })
  const quotaPerUnit =
    currency?.quotaPerUnit && currency.quotaPerUnit > 0
      ? currency.quotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const balanceCost = Math.max(
    0,
    Math.ceil(Number(plan.price_amount || 0) * quotaPerUnit)
  )
  const userQuota = Math.max(0, Number(props.userQuota || 0))
  const allowBalancePay = plan.allow_balance_pay !== false
  const insufficientBalance = userQuota < balanceCost
  const limitReached =
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)
  const activeLimitReached =
    (props.activePurchaseLimit || 0) > 0 &&
    (props.activePurchaseCount || 0) >= (props.activePurchaseLimit || 0)
  const blocked = limitReached || activeLimitReached

  const handlePayStripe = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionStripe({ plan_id: plan.id })
      if (res.message === 'success' && res.data?.pay_link) {
        window.open(res.data.pay_link, '_blank')
        toast.success(t('Payment page opened'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayCreem = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionCreem({ plan_id: plan.id })
      if (res.message === 'success' && res.data?.checkout_url) {
        window.open(res.data.checkout_url, '_blank')
        toast.success(t('Payment page opened'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  // In-tab redirect (not window.open) — user-gesture context is lost
  // across the await, so a popup would be blocked. Same as the wallet hook.
  const handlePayWaffoPancake = async () => {
    setPaying(true)
    try {
      const res = await paySubscriptionWaffoPancake({ plan_id: plan.id })
      if (res.message === 'success' && res.data?.checkout_url) {
        toast.success(t('Redirecting to payment page...'))
        window.location.href = res.data.checkout_url
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayEpay = async () => {
    if (!selectedEpayMethodConfig) {
      toast.error(t('Please select a payment method'))
      return
    }
    setPaying(true)
    try {
      const res = await paySubscriptionEpay({
        plan_id: plan.id,
        payment_method: selectedEpayMethodConfig.type,
        epay_gateway: selectedEpayMethodConfig.gateway,
      })
      if (res.message === 'success' && res.url) {
        submitPaymentForm(res.url, res.data || {})
        toast.success(t('Payment initiated'))
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayBalance = async () => {
    if (!allowBalancePay) {
      toast.error(t('This plan does not allow balance redemption'))
      return
    }
    setPaying(true)
    try {
      const res = await paySubscriptionBalance({ plan_id: plan.id })
      if (res.success) {
        toast.success(t('Subscription purchased successfully'))
        void props.onPurchaseSuccess?.()
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  const handlePayNowPayments = async () => {
    if (!selectedNowPaymentsCurrency) {
      toast.error(t('Please select a cryptocurrency'))
      return
    }
    setPaying(true)
    try {
      const res = await paySubscriptionNowPayments({
        plan_id: plan.id,
        pay_currency: selectedNowPaymentsCurrency,
      })
      if (res.success && res.data) {
        setNowPaymentsCurrencyDialogOpen(false)
        props.onNowPaymentsInvoice?.(res.data)
        props.onOpenChange(false)
      } else {
        toast.error(
          res.message && res.message !== 'success'
            ? res.message
            : t('Payment request failed')
        )
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setPaying(false)
    }
  }

  let payableLabel = '—'
  if (selectedPayment === 'balance') payableLabel = price
  else if (
    paymentQuote.data &&
    !paymentQuote.isFetching &&
    !paymentQuote.isError
  ) {
    payableLabel = `${paymentQuote.data.currency} ${formatNumber(paymentQuote.data.amount, toIntlLocale(i18n.resolvedLanguage || i18n.language))}`
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={
          <>
            <Crown className='h-5 w-5' />
            {t('Purchase Subscription')}
          </>
        }
        contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
        titleClassName='flex items-center gap-2'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footerClassName='flex-row items-center justify-between gap-3'
        footer={
          <>
            <div className='min-w-0 text-sm'>
              <div className='text-muted-foreground text-xs'>
                {t('You Pay')}
              </div>
              <div className='font-semibold tabular-nums'>{payableLabel}</div>
            </div>
            <Button
              disabled={
                blocked ||
                paying ||
                !selectedPayment ||
                (selectedPayment !== 'balance' &&
                  (!paymentQuote.data ||
                    paymentQuote.isFetching ||
                    paymentQuote.isError)) ||
                (selectedPayment === 'balance' &&
                  (!allowBalancePay || insufficientBalance))
              }
              onClick={() => {
                if (selectedPayment === 'balance') void handlePayBalance()
                else if (selectedPayment === 'stripe') void handlePayStripe()
                else if (selectedPayment === 'creem') void handlePayCreem()
                else if (selectedPayment === 'waffo_pancake') {
                  void handlePayWaffoPancake()
                } else if (selectedPayment === 'nowpayments') {
                  setNowPaymentsCurrencyDialogOpen(true)
                } else void handlePayEpay()
              }}
            >
              {t('Pay')}
            </Button>
          </>
        }
      >
        <div className='space-y-3 sm:space-y-4'>
          <div className='bg-muted/50 space-y-2.5 rounded-lg border p-3 sm:space-y-3 sm:p-4'>
            <div className='flex justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Plan Name')}
              </span>
              <span className='max-w-[200px] truncate text-sm font-medium'>
                {plan.title}
              </span>
            </div>
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Validity Period')}
              </span>
              <span className='flex items-center gap-1 text-sm'>
                <CalendarClock className='h-3.5 w-3.5' />
                {formatDuration(plan, t)}
              </span>
            </div>
            {formatResetPeriod(plan, t) !== t('No Reset') && (
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Reset Period')}
                </span>
                <span className='text-sm'>{formatResetPeriod(plan, t)}</span>
              </div>
            )}
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Quota per Billing Period')}
              </span>
              <span className='flex items-center gap-1 text-sm'>
                <Package className='h-3.5 w-3.5' />
                {totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')}
              </span>
            </div>
            {plan.upgrade_group && (
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Upgrade Group')}
                </span>
                <GroupBadge group={plan.upgrade_group} />
              </div>
            )}
            <Separator />
            <div className='flex items-center justify-between'>
              <span className='text-sm font-medium'>{t('Amount Due')}</span>
              <span className='text-primary text-lg font-bold'>{price}</span>
            </div>
          </div>

          {blocked && (
            <Alert variant='destructive'>
              <AlertDescription>
                {activeLimitReached
                  ? t('Active subscription limit reached')
                  : t('Purchase limit reached')}{' '}
                (
                {activeLimitReached
                  ? props.activePurchaseCount
                  : props.purchaseCount}
                /
                {activeLimitReached
                  ? props.activePurchaseLimit
                  : props.purchaseLimit}
                )
              </AlertDescription>
            </Alert>
          )}

          <div className='space-y-3'>
            <p className='text-muted-foreground text-xs'>
              {t('Select payment method')}
            </p>
            <div className='grid grid-cols-2 gap-2'>
              {allowBalancePay && (
                <PaymentMethodCard
                  name={t('Balance')}
                  type='balance'
                  iconNode={<WalletCards className='h-4 w-4' />}
                  selected={selectedPayment === 'balance'}
                  onClick={() => setSelectedPayment('balance')}
                  disabled={blocked || paying}
                  description={formatQuota(userQuota)}
                />
              )}
              {hasStripe && (
                <PaymentMethodCard
                  name={
                    props.paymentMethods?.find((m) => m.type === 'stripe')
                      ?.name || 'Stripe'
                  }
                  type='stripe'
                  icon={
                    props.paymentMethods?.find((m) => m.type === 'stripe')?.icon
                  }
                  selected={selectedPayment === 'stripe'}
                  onClick={() => setSelectedPayment('stripe')}
                  disabled={blocked || paying}
                />
              )}
              {hasCreem && (
                <PaymentMethodCard
                  name={
                    props.paymentMethods?.find((m) => m.type === 'creem')
                      ?.name || 'Creem'
                  }
                  type='creem'
                  icon={
                    props.paymentMethods?.find((m) => m.type === 'creem')?.icon
                  }
                  selected={selectedPayment === 'creem'}
                  onClick={() => setSelectedPayment('creem')}
                  disabled={blocked || paying}
                />
              )}
              {hasWaffoPancake && (
                <PaymentMethodCard
                  name={
                    props.paymentMethods?.find(
                      (m) => m.type === 'waffo_pancake'
                    )?.name || 'Waffo Pancake'
                  }
                  type='waffo_pancake'
                  icon={
                    props.paymentMethods?.find(
                      (m) => m.type === 'waffo_pancake'
                    )?.icon
                  }
                  selected={selectedPayment === 'waffo_pancake'}
                  onClick={() => setSelectedPayment('waffo_pancake')}
                  disabled={blocked || paying}
                />
              )}
              {hasNowPayments && (
                <PaymentMethodCard
                  name={
                    props.paymentMethods?.find((m) => m.type === 'nowpayments')
                      ?.name || 'NOWPayments'
                  }
                  type='nowpayments'
                  icon={
                    props.paymentMethods?.find((m) => m.type === 'nowpayments')
                      ?.icon
                  }
                  selected={selectedPayment === 'nowpayments'}
                  onClick={() => setSelectedPayment('nowpayments')}
                  disabled={blocked || paying}
                />
              )}
              {hasEpay &&
                (props.epayMethods || []).map((m, index) => (
                  <PaymentMethodCard
                    key={`${m.gateway || 'epay'}-${m.type}`}
                    name={m.name || m.type}
                    type={m.type}
                    icon={m.icon}
                    selected={selectedPayment === `epay-${index}`}
                    onClick={() => {
                      setSelectedPayment(`epay-${index}`)
                    }}
                    disabled={blocked || paying}
                  />
                ))}
            </div>
          </div>
          {selectedPayment !== 'balance' && paymentQuote.isError && (
            <Alert variant='destructive'>
              <AlertDescription>
                {t(
                  'Unable to calculate payment amount. Please choose another payment method.'
                )}
              </AlertDescription>
            </Alert>
          )}
          {selectedPayment === 'balance' && (
            <div className='flex flex-col gap-2 rounded-md border p-3'>
              <div className='flex items-center justify-between gap-2 text-xs'>
                <span className='text-muted-foreground'>{t('Required')}</span>
                <span>{formatQuota(balanceCost)}</span>
              </div>
              <div className='flex items-center justify-between gap-2 text-xs'>
                <span className='text-muted-foreground'>{t('Available')}</span>
                <span>{formatQuota(userQuota)}</span>
              </div>
              {selectedPayment === 'balance' && insufficientBalance && (
                <Alert variant='destructive'>
                  <AlertDescription>
                    {t('Insufficient balance')}
                  </AlertDescription>
                </Alert>
              )}
            </div>
          )}
        </div>
      </Dialog>
      <NowPaymentsCurrencyDialog
        open={nowPaymentsCurrencyDialogOpen}
        onOpenChange={setNowPaymentsCurrencyDialogOpen}
        currencies={nowPaymentsCurrencies}
        selectedCurrency={selectedNowPaymentsCurrency}
        onSelectedCurrencyChange={setSelectedNowPaymentsCurrency}
        onConfirm={handlePayNowPayments}
        loading={paying}
      />
    </>
  )
}
