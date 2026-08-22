/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Loader2,
  RefreshCw,
  RotateCcw,
  Ticket,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import {
  calculateRedemptionPurchaseAmount,
  getUserRedemptions,
  isApiSuccess,
  refundUserRedemption,
  requestRedemptionPurchase,
} from '../api'
import { formatCurrency, getPaymentIcon, submitPaymentForm } from '../lib'
import type {
  MoneroInvoice,
  PaymentMethod,
  TopupInfo,
  UserRedemption,
} from '../types'

const MAX_PURCHASE_COUNT = 100
const PAGE_SIZE = 20

interface RedemptionPurchaseCardProps {
  topupInfo: TopupInfo | null
  onMoneroInvoice: (invoice: MoneroInvoice) => void
  onRefreshUser: () => Promise<void> | void
}

function getFallbackPaymentMethod(type: string): PaymentMethod {
  const labels: Record<string, string> = {
    stripe: 'Stripe',
    waffo: 'Waffo',
    waffo_pancake: 'Waffo Pancake',
    monero: 'Monero',
  }
  return { type, name: labels[type] || type }
}

function getStatus(code: UserRedemption, t: (key: string) => string) {
  if (code.refunded_time > 0) {
    return { label: t('Refunded'), variant: 'warning' as const }
  }
  if (code.status === 3) {
    return { label: t('Used'), variant: 'neutral' as const }
  }
  if (code.status === 1) {
    return { label: t('Unused'), variant: 'success' as const }
  }
  return { label: t('Disabled'), variant: 'neutral' as const }
}

function getResponseDataString(data: unknown): string | null {
  return typeof data === 'string' && data.trim() ? data : null
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export function RedemptionPurchaseCard({
  topupInfo,
  onMoneroInvoice,
  onRefreshUser,
}: RedemptionPurchaseCardProps) {
  const { t } = useTranslation()
  const purchaseMethods = useMemo(() => {
    const allowed = topupInfo?.redemption_purchase_methods ?? []
    return allowed.map((type) => {
      return (
        topupInfo?.pay_methods?.find((method) => method.type === type) ??
        getFallbackPaymentMethod(type)
      )
    })
  }, [topupInfo?.pay_methods, topupInfo?.redemption_purchase_methods])

  const initialMethod = purchaseMethods[0]?.type ?? ''
  const minimumAmount = Math.max(1, topupInfo?.min_topup ?? 1)
  const [unitAmountText, setUnitAmountText] = useState(String(minimumAmount))
  const [quantityText, setQuantityText] = useState('1')
  const [paymentMethod, setPaymentMethod] = useState(initialMethod)
  const [waffoMethodIndex, setWaffoMethodIndex] = useState('0')
  const [paymentAmount, setPaymentAmount] = useState<string | null>(null)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)
  const [codes, setCodes] = useState<UserRedemption[]>([])
  const [codesPage, setCodesPage] = useState(1)
  const [codesTotal, setCodesTotal] = useState(0)
  const [codesLoading, setCodesLoading] = useState(false)
  const [refundingId, setRefundingId] = useState<number | null>(null)

  const unitAmount = Number.parseInt(unitAmountText, 10) || 0
  const quantity = Number.parseInt(quantityText, 10) || 0
  const selectedMethod = purchaseMethods.find(
    (method) => method.type === paymentMethod
  )
  const waffoPayMethods = topupInfo?.waffo_pay_methods ?? []
  const isWaffo = paymentMethod === 'waffo'
  const totalAmount = unitAmount > 0 && quantity > 0 ? unitAmount * quantity : 0

  useEffect(() => {
    if (!purchaseMethods.some((method) => method.type === paymentMethod)) {
      setPaymentMethod(initialMethod)
    }
  }, [initialMethod, paymentMethod, purchaseMethods])

  const loadCodes = useCallback(async (page: number) => {
    setCodesLoading(true)
    try {
      const response = await getUserRedemptions(page, PAGE_SIZE)
      if (!isApiSuccess(response) || !response.data) {
        toast.error(response.message || t('Failed to load redemption codes'))
        return
      }
      setCodes(response.data.items ?? [])
      setCodesTotal(response.data.total ?? 0)
      setCodesPage(response.data.page || page)
    } catch {
      toast.error(t('Failed to load redemption codes'))
    } finally {
      setCodesLoading(false)
    }
  }, [t])

  useEffect(() => {
    if (topupInfo?.enable_redemption_purchase) {
      void loadCodes(1)
    }
  }, [loadCodes, topupInfo?.enable_redemption_purchase])

  useEffect(() => {
    if (!paymentMethod || unitAmount <= 0 || quantity <= 0) {
      setPaymentAmount(null)
      return
    }
    let active = true
    const timer = window.setTimeout(async () => {
      setCalculating(true)
      try {
        const response = await calculateRedemptionPurchaseAmount({
          unit_amount: unitAmount,
          quantity,
          payment_method: paymentMethod,
          ...(isWaffo && waffoPayMethods.length > 0
            ? { pay_method_index: Number(waffoMethodIndex) }
            : {}),
        })
        if (!active) return
        if (!isApiSuccess(response)) {
          setPaymentAmount(null)
          return
        }
        setPaymentAmount(response.data || null)
      } catch {
        if (active) setPaymentAmount(null)
      } finally {
        if (active) setCalculating(false)
      }
    }, 250)
    return () => {
      active = false
      window.clearTimeout(timer)
    }
  }, [
    isWaffo,
    paymentMethod,
    quantity,
    unitAmount,
    waffoMethodIndex,
    waffoPayMethods.length,
  ])

  const buildRequest = () => ({
    unit_amount: unitAmount,
    quantity,
    payment_method: paymentMethod,
    ...(isWaffo && waffoPayMethods.length > 0
      ? { pay_method_index: Number(waffoMethodIndex) }
      : {}),
  })

  const handlePurchase = async () => {
    if (unitAmount <= 0 || quantity <= 0 || quantity > MAX_PURCHASE_COUNT) {
      toast.error(
        t('Enter a valid denomination and a quantity from 1 to {{max}}.', {
          max: MAX_PURCHASE_COUNT,
        })
      )
      return
    }
    if (!paymentMethod) {
      toast.error(t('Select a payment method'))
      return
    }

    setProcessing(true)
    try {
      const response = await requestRedemptionPurchase(buildRequest())
      if (!isApiSuccess(response)) {
        toast.error(
          getResponseDataString(response.data) ||
            response.message ||
            t('Payment request failed')
        )
        return
      }

      if (paymentMethod === 'monero' && isRecord(response.data)) {
        onMoneroInvoice(response.data as unknown as MoneroInvoice)
        return
      }

      if (paymentMethod === 'stripe' && isRecord(response.data)) {
        const payLink = response.data.pay_link
        if (typeof payLink === 'string' && payLink) {
          window.open(payLink, '_blank', 'noopener,noreferrer')
          toast.success(t('Redirecting to payment page...'))
          return
        }
      }

      if (paymentMethod === 'waffo' && isRecord(response.data)) {
        const paymentUrl = response.data.payment_url
        if (typeof paymentUrl === 'string' && paymentUrl) {
          window.open(paymentUrl, '_blank', 'noopener,noreferrer')
          toast.success(t('Redirecting to payment page...'))
          return
        }
      }

      if (paymentMethod === 'waffo_pancake' && isRecord(response.data)) {
        const checkoutUrl = response.data.checkout_url
        if (typeof checkoutUrl === 'string' && checkoutUrl) {
          window.open(checkoutUrl, '_blank', 'noopener,noreferrer')
          toast.success(t('Redirecting to payment page...'))
          return
        }
      }

      if (response.url && isRecord(response.data)) {
        submitPaymentForm(response.url, response.data)
        toast.success(t('Redirecting to payment page...'))
        return
      }

      toast.error(t('Payment request failed'))
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      setProcessing(false)
      await onRefreshUser()
      void loadCodes(codesPage)
    }
  }

  const handleRefund = async (code: UserRedemption) => {
    setRefundingId(code.id)
    try {
      const response = await refundUserRedemption(code.id)
      if (!isApiSuccess(response)) {
        toast.error(response.message || t('Refund failed'))
        return
      }
      toast.success(t('Redemption code refunded to your balance'))
      await onRefreshUser()
      await loadCodes(codesPage)
    } catch {
      toast.error(t('Refund failed'))
    } finally {
      setRefundingId(null)
    }
  }

  const totalPages = Math.max(1, Math.ceil(codesTotal / PAGE_SIZE))
  const canRefund = (code: UserRedemption) =>
    code.status === 1 && code.refunded_time === 0 && code.purchase_trade_no !== ''

  if (!topupInfo?.enable_redemption_purchase) return null

  return (
    <TitledCard
      title={t('Purchase Redemption Codes')}
      description={t('Pay with a configured payment method and receive codes in your wallet.')}
      icon={<Ticket className='h-4 w-4' />}
      iconTone='warning'
      disableHoverEffect
      action={
        <Button
          type='button'
          variant='outline'
          size='icon'
          onClick={() => void loadCodes(codesPage)}
          disabled={codesLoading}
          aria-label={t('Refresh redemption codes')}
          title={t('Refresh redemption codes')}
        >
          <RefreshCw className={codesLoading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
        </Button>
      }
      contentClassName='space-y-5'
    >
      {purchaseMethods.length === 0 ? (
        <Alert>
          <AlertDescription>
            {t('No external payment methods are configured for redemption purchases.')}
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <div className='grid gap-3 sm:grid-cols-[minmax(0,1fr)_120px]'>
            <div className='space-y-2'>
              <Label htmlFor='redemption-unit-amount'>{t('Code denomination')}</Label>
              <Input
                id='redemption-unit-amount'
                type='number'
                min={1}
                step={1}
                value={unitAmountText}
                onChange={(event) => setUnitAmountText(event.target.value)}
                placeholder={t('For example, 5')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='redemption-quantity'>{t('Quantity')}</Label>
              <Input
                id='redemption-quantity'
                type='number'
                min={1}
                max={MAX_PURCHASE_COUNT}
                step={1}
                value={quantityText}
                onChange={(event) => setQuantityText(event.target.value)}
              />
            </div>
          </div>

          <div className='grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]'>
            <div className='space-y-2'>
              <Label>{t('Payment Method')}</Label>
              <Select value={paymentMethod} onValueChange={(value) => value && setPaymentMethod(value)}>
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Select a payment method')}>
                    {selectedMethod && (
                      <span className='flex items-center gap-2'>
                        {getPaymentIcon(selectedMethod.type, 'h-4 w-4', selectedMethod.icon, selectedMethod.name)}
                        {selectedMethod.name}
                      </span>
                    )}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {purchaseMethods.map((method) => (
                      <SelectItem key={method.type} value={method.type}>
                        <span className='flex items-center gap-2'>
                          {getPaymentIcon(method.type, 'h-4 w-4', method.icon, method.name)}
                          {method.name}
                        </span>
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            {isWaffo && waffoPayMethods.length > 0 ? (
              <div className='space-y-2'>
                <Label>{t('Waffo payment option')}</Label>
                <Select value={waffoMethodIndex} onValueChange={(value) => value && setWaffoMethodIndex(value)}>
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {waffoPayMethods.map((method, index) => (
                        <SelectItem key={`${method.name}-${index}`} value={String(index)}>
                          {method.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            ) : (
              <div className='bg-muted/30 flex min-h-10 items-center justify-between rounded-md border px-3 text-sm'>
                <span className='text-muted-foreground'>{t('Total codes')}</span>
                <span className='font-semibold'>{quantity > 0 ? quantity : 0}</span>
              </div>
            )}
          </div>

          <div className='flex flex-col gap-3 rounded-md border bg-muted/20 p-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='text-sm'>
              <div className='text-muted-foreground'>{t('Purchase total')}</div>
              <div className='font-semibold'>
                {totalAmount > 0 ? `${formatCurrency(totalAmount)} · ${quantity} ${t('codes')}` : '-'}
              </div>
            </div>
            <div className='flex items-center gap-3'>
              <div className='text-right text-sm'>
                <div className='text-muted-foreground'>{t('Amount to pay:')}</div>
                <div className='font-semibold'>
                  {calculating ? <Skeleton className='ml-auto h-5 w-20' /> : paymentAmount ? formatCurrency(Number(paymentAmount)) : paymentMethod === 'monero' ? t('Shown in invoice') : '-'}
                </div>
              </div>
              <Button type='button' onClick={() => void handlePurchase()} disabled={processing || calculating}>
                {processing && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {t('Buy Codes')}
              </Button>
            </div>
          </div>
        </>
      )}

      <div className='space-y-3 border-t pt-5'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <h3 className='text-sm font-semibold'>{t('Your Redemption Codes')}</h3>
            <p className='text-muted-foreground text-xs'>{t('Unused purchased codes can be refunded to your balance.')}</p>
          </div>
          {codesTotal > 0 && <span className='text-muted-foreground text-xs'>{codesTotal}</span>}
        </div>

        {codesLoading && codes.length === 0 ? (
          <div className='space-y-2'>
            {[1, 2, 3].map((key) => <Skeleton key={key} className='h-16 w-full' />)}
          </div>
        ) : codes.length === 0 ? (
          <div className='text-muted-foreground rounded-md border border-dashed px-4 py-8 text-center text-sm'>
            {t('You do not own any purchased redemption codes yet.')}
          </div>
        ) : (
          <div className='divide-y rounded-md border'>
            {codes.map((code) => {
              const status = getStatus(code, t)
              return (
                <div key={code.id} className='flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between'>
                  <div className='min-w-0 space-y-2'>
                    <div className='flex min-w-0 items-center gap-2'>
                      <MaskedValueDisplay
                        label={t('Full Code')}
                        fullValue={code.key}
                        maskedValue={`${code.key.slice(0, 6)}${'*'.repeat(Math.max(0, code.key.length - 12))}${code.key.slice(-6)}`}
                        copyTooltip={t('Copy code')}
                        copyAriaLabel={t('Copy redemption code')}
                      />
                      <StatusBadge label={status.label} variant={status.variant} copyable={false} />
                    </div>
                    <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
                      <span>{formatQuota(code.quota)}</span>
                      <span>{t('Created')} {formatTimestampToDate(code.created_time)}</span>
                      {code.redeemed_time > 0 && <span>{t('Redeemed')} {formatTimestampToDate(code.redeemed_time)}</span>}
                    </div>
                  </div>
                  {canRefund(code) && (
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={() => void handleRefund(code)}
                      disabled={refundingId === code.id}
                      className='shrink-0'
                    >
                      {refundingId === code.id ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : <RotateCcw className='mr-2 h-4 w-4' />}
                      {t('Refund to balance')}
                    </Button>
                  )}
                </div>
              )
            })}
          </div>
        )}

        {codesTotal > PAGE_SIZE && (
          <div className='flex items-center justify-end gap-2'>
            <Button type='button' variant='outline' size='icon' onClick={() => void loadCodes(codesPage - 1)} disabled={codesPage <= 1 || codesLoading} aria-label={t('Previous page')} title={t('Previous page')}>
              <ChevronLeft className='h-4 w-4' />
            </Button>
            <span className='text-muted-foreground text-xs'>{codesPage} / {totalPages}</span>
            <Button type='button' variant='outline' size='icon' onClick={() => void loadCodes(codesPage + 1)} disabled={codesPage >= totalPages || codesLoading} aria-label={t('Next page')} title={t('Next page')}>
              <ChevronRight className='h-4 w-4' />
            </Button>
          </div>
        )}
      </div>

      <p className='text-muted-foreground flex items-start gap-2 text-xs'>
        <ExternalLink className='mt-0.5 h-3.5 w-3.5 shrink-0' />
        {t('Wallet balance cannot be used to buy codes. Payment discounts configured by the administrator are applied automatically.')}
      </p>
    </TitledCard>
  )
}
