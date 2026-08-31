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
import {
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Loader2,
  RefreshCw,
  RotateCcw,
  Ticket,
} from 'lucide-react'
import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  calculateRedemptionPurchaseAmount,
  getUserRedemptions,
  isApiSuccess,
  refundUserRedemption,
  requestRedemptionPurchase,
} from '../api'
import {
  calculatePresetPricing,
  formatCurrency,
  getDiscountLabel,
  getPaymentIcon,
  getPaymentMethodMinTopup,
  submitPaymentForm,
} from '../lib'
import type {
  MoneroInvoice,
  PaymentMethod,
  PresetAmount,
  TopupInfo,
  UserRedemption,
} from '../types'
import { PaymentConfirmDialog } from './dialogs/payment-confirm-dialog'

const MAX_PURCHASE_COUNT = 100
const PAGE_SIZE = 20

interface RedemptionPurchaseCardProps {
  topupInfo: TopupInfo | null
  presetAmounts: PresetAmount[]
  priceRatio?: number
  usdExchangeRate?: number
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

function getMaskedCode(code: string): string {
  return `${code.slice(0, 6)}${'*'.repeat(Math.max(0, code.length - 12))}${code.slice(-6)}`
}

export function RedemptionPurchaseCard({
  topupInfo,
  presetAmounts,
  priceRatio = 1,
  usdExchangeRate = 1,
  onMoneroInvoice,
  onRefreshUser,
}: RedemptionPurchaseCardProps) {
  const { t } = useTranslation()
  const purchaseMethods = useMemo(() => {
    const allowed = topupInfo?.redemption_purchase_methods ?? []
    return allowed
      .map(
        (type) =>
          topupInfo?.pay_methods?.find((method) => method.type === type) ??
          getFallbackPaymentMethod(type)
      )
      .filter((method) => method.type)
  }, [topupInfo?.pay_methods, topupInfo?.redemption_purchase_methods])

  const standardPurchaseMethods = useMemo(
    () => purchaseMethods.filter((method) => method.type !== 'waffo'),
    [purchaseMethods]
  )
  const waffoPurchaseEnabled = purchaseMethods.some(
    (method) => method.type === 'waffo'
  )
  const waffoPayMethods = topupInfo?.waffo_pay_methods ?? []
  const initialMethod = ''
  const minimumAmount = Math.max(1, topupInfo?.min_topup ?? 1)
  const [unitAmountText, setUnitAmountText] = useState(String(minimumAmount))
  const [quantityText, setQuantityText] = useState('1')
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [paymentMethod, setPaymentMethod] = useState(initialMethod)
  const [waffoMethodIndex, setWaffoMethodIndex] = useState<string | null>(
    waffoPayMethods.length > 0 ? '0' : null
  )
  const [paymentAmount, setPaymentAmount] = useState<string | null>(null)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [codes, setCodes] = useState<UserRedemption[]>([])
  const [codesPage, setCodesPage] = useState(1)
  const [codesTotal, setCodesTotal] = useState(0)
  const [codesLoading, setCodesLoading] = useState(false)
  const [refundingId, setRefundingId] = useState<number | null>(null)
  const initializedAmountRef = useRef(false)

  const unitAmount = Number.parseInt(unitAmountText, 10) || 0
  const quantity = Number.parseInt(quantityText, 10) || 0
  const selectedMethod = purchaseMethods.find(
    (method) => method.type === paymentMethod
  )
  const isWaffo = paymentMethod === 'waffo'
  const selectedWaffoMethod =
    isWaffo && waffoMethodIndex !== null
      ? waffoPayMethods[Number.parseInt(waffoMethodIndex, 10)]
      : undefined
  const selectedPaymentMethod = selectedWaffoMethod
    ? {
        type: 'waffo',
        name: selectedWaffoMethod.name,
        icon: selectedWaffoMethod.icon,
      }
    : selectedMethod
  const totalAmount = unitAmount > 0 && quantity > 0 ? unitAmount * quantity : 0

  useEffect(() => {
    if (!purchaseMethods.some((method) => method.type === paymentMethod)) {
      setPaymentMethod(initialMethod)
    }
  }, [initialMethod, paymentMethod, purchaseMethods])

  useEffect(() => {
    if (waffoPurchaseEnabled && waffoPayMethods.length > 0) {
      setWaffoMethodIndex((current) => current ?? '0')
    }
  }, [waffoPayMethods.length, waffoPurchaseEnabled])

  useEffect(() => {
    if (initializedAmountRef.current || purchaseMethods.length === 0) {
      return
    }
    initializedAmountRef.current = true
    const firstMethodMinimum = getPaymentMethodMinTopup(
      purchaseMethods[0],
      topupInfo
    )
    const defaultAmount = Math.max(minimumAmount, firstMethodMinimum)
    setUnitAmountText((current) =>
      current === '1' || current === String(minimumAmount)
        ? String(defaultAmount)
        : current
    )
  }, [minimumAmount, purchaseMethods, topupInfo])

  const loadCodes = useCallback(
    async (page: number) => {
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
    },
    [t]
  )

  useEffect(() => {
    if (topupInfo?.enable_redemption_purchase) {
      void loadCodes(1)
    }
  }, [loadCodes, topupInfo?.enable_redemption_purchase])

  const buildRequest = useCallback(
    (methodType = paymentMethod, methodIndex = waffoMethodIndex) => ({
      unit_amount: unitAmount,
      quantity,
      payment_method: methodType,
      ...(methodType === 'waffo' && methodIndex !== null
        ? { pay_method_index: Number.parseInt(methodIndex, 10) }
        : {}),
    }),
    [paymentMethod, quantity, unitAmount, waffoMethodIndex]
  )

  const calculatePurchaseAmount = useCallback(
    async (methodType: string, methodIndex: string | null) => {
      if (
        !methodType ||
        !Number.isSafeInteger(unitAmount) ||
        !Number.isSafeInteger(quantity) ||
        unitAmount <= 0 ||
        quantity <= 0 ||
        quantity > MAX_PURCHASE_COUNT
      ) {
        setPaymentAmount(null)
        return null
      }

      setCalculating(true)
      try {
        const response = await calculateRedemptionPurchaseAmount(
          buildRequest(methodType, methodIndex)
        )
        if (!isApiSuccess(response) || !response.data?.trim()) {
          setPaymentAmount(null)
          return null
        }
        setPaymentAmount(response.data)
        return response.data
      } catch {
        setPaymentAmount(null)
        return null
      } finally {
        setCalculating(false)
      }
    },
    [buildRequest, quantity, unitAmount]
  )

  useEffect(() => {
    if (!paymentMethod || unitAmount <= 0 || quantity <= 0) {
      setPaymentAmount(null)
      return
    }
    const timer = window.setTimeout(() => {
      void calculatePurchaseAmount(paymentMethod, waffoMethodIndex)
    }, 250)
    return () => window.clearTimeout(timer)
  }, [
    calculatePurchaseAmount,
    paymentMethod,
    quantity,
    unitAmount,
    waffoMethodIndex,
  ])

  const handlePurchase = async (
    methodType = paymentMethod,
    methodIndex = waffoMethodIndex
  ) => {
    const method =
      methodType === 'waffo' && selectedWaffoMethod
        ? {
            type: 'waffo',
            name: selectedWaffoMethod.name,
            icon: selectedWaffoMethod.icon,
          }
        : purchaseMethods.find((item) => item.type === methodType)

    if (
      !Number.isSafeInteger(unitAmount) ||
      !Number.isSafeInteger(quantity) ||
      unitAmount <= 0 ||
      quantity <= 0 ||
      quantity > MAX_PURCHASE_COUNT
    ) {
      toast.error(
        t('Enter a valid denomination and a quantity from 1 to {{max}}.', {
          max: MAX_PURCHASE_COUNT,
        })
      )
      return
    }
    if (!methodType || !method) {
      toast.error(t('Select a payment method'))
      return
    }

    const minimum = getPaymentMethodMinTopup(method, topupInfo)
    if (totalAmount < minimum) {
      toast.error(t('Minimum topup amount: {{amount}}', { amount: minimum }))
      return
    }

    setProcessing(true)
    try {
      const response = await requestRedemptionPurchase(
        buildRequest(methodType, methodIndex)
      )
      if (!isApiSuccess(response)) {
        toast.error(
          getResponseDataString(response.data) ||
            response.message ||
            t('Payment request failed')
        )
        return
      }

      if (methodType === 'monero' && isRecord(response.data)) {
        setConfirmDialogOpen(false)
        onMoneroInvoice(response.data as unknown as MoneroInvoice)
        return
      }

      if (methodType === 'stripe' && isRecord(response.data)) {
        const payLink = response.data.pay_link
        if (typeof payLink === 'string' && payLink) {
          window.open(payLink, '_blank', 'noopener,noreferrer')
          setConfirmDialogOpen(false)
          toast.success(t('Redirecting to payment page...'))
          return
        }
      }

      if (methodType === 'waffo' && isRecord(response.data)) {
        const paymentUrl = response.data.payment_url
        if (typeof paymentUrl === 'string' && paymentUrl) {
          window.open(paymentUrl, '_blank', 'noopener,noreferrer')
          setConfirmDialogOpen(false)
          toast.success(t('Redirecting to payment page...'))
          return
        }
      }

      if (methodType === 'waffo_pancake' && isRecord(response.data)) {
        const checkoutUrl = response.data.checkout_url
        if (typeof checkoutUrl === 'string' && checkoutUrl) {
          window.open(checkoutUrl, '_blank', 'noopener,noreferrer')
          setConfirmDialogOpen(false)
          toast.success(t('Redirecting to payment page...'))
          return
        }
      }

      if (response.url && isRecord(response.data)) {
        setConfirmDialogOpen(false)
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

  const handlePaymentMethodSelect = async (
    method: PaymentMethod,
    methodIndex?: number
  ) => {
    const nextMethodIndex =
      methodIndex === undefined ? waffoMethodIndex : String(methodIndex)
    setPaymentMethod(method.type)
    if (methodIndex !== undefined) {
      setWaffoMethodIndex(nextMethodIndex)
    }

    const minimum = getPaymentMethodMinTopup(method, topupInfo)
    if (totalAmount < minimum) {
      toast.error(t('Minimum topup amount: {{amount}}', { amount: minimum }))
      return
    }

    if (method.type === 'monero') {
      await handlePurchase(method.type, nextMethodIndex)
      return
    }

    const calculatedAmount = await calculatePurchaseAmount(
      method.type,
      nextMethodIndex
    )
    if (!calculatedAmount) {
      toast.error(t('Payment request failed'))
      return
    }
    setConfirmDialogOpen(true)
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
    code.status === 1 &&
    code.refunded_time === 0 &&
    code.purchase_trade_no !== ''

  let paymentAmountDisplay: ReactNode = '-'
  if (calculating) {
    paymentAmountDisplay = <Skeleton className='h-5 w-20 sm:ml-auto' />
  } else if (paymentAmount) {
    paymentAmountDisplay = formatCurrency(Number(paymentAmount))
  } else if (paymentMethod === 'monero') {
    paymentAmountDisplay = t('Shown in invoice')
  }

  let codesContent: ReactNode
  if (codesLoading && codes.length === 0) {
    codesContent = (
      <div className='space-y-2'>
        {[1, 2, 3].map((key) => (
          <Skeleton key={key} className='h-16 w-full' />
        ))}
      </div>
    )
  } else if (codes.length === 0) {
    codesContent = (
      <div className='text-muted-foreground rounded-md border border-dashed px-4 py-8 text-center text-sm'>
        {t('You do not own any purchased redemption codes yet.')}
      </div>
    )
  } else {
    codesContent = (
      <div className='divide-y rounded-md border'>
        {codes.map((code) => {
          const status = getStatus(code, t)
          return (
            <div
              key={code.id}
              className='flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between'
            >
              <div className='min-w-0 space-y-2'>
                <div className='flex min-w-0 items-center gap-2'>
                  <MaskedValueDisplay
                    label={t('Full Code')}
                    fullValue={code.key}
                    maskedValue={getMaskedCode(code.key)}
                    copyTooltip={t('Copy code')}
                    copyAriaLabel={t('Copy redemption code')}
                  />
                  <StatusBadge
                    label={status.label}
                    variant={status.variant}
                    copyable={false}
                  />
                </div>
                <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
                  <span>{formatQuota(code.quota)}</span>
                  <span>
                    {t('Created')} {formatTimestampToDate(code.created_time)}
                  </span>
                  {code.redeemed_time > 0 && (
                    <span>
                      {t('Redeemed')}{' '}
                      {formatTimestampToDate(code.redeemed_time)}
                    </span>
                  )}
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
                  {refundingId === code.id ? (
                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  ) : (
                    <RotateCcw className='mr-2 h-4 w-4' />
                  )}
                  {t('Refund to balance')}
                </Button>
              )}
            </div>
          )
        })}
      </div>
    )
  }

  if (!topupInfo?.enable_redemption_purchase) return null

  const renderPaymentMethodButton = (method: PaymentMethod) => {
    const minimum = getPaymentMethodMinTopup(method, topupInfo)
    const belowMinimum = totalAmount < minimum
    const disabledReason = belowMinimum
      ? t('Minimum topup amount: {{amount}}', { amount: minimum })
      : undefined
    const button = (
      <Button
        key={method.type}
        type='button'
        variant='outline'
        onClick={() => void handlePaymentMethodSelect(method)}
        disabled={belowMinimum || calculating || processing}
        title={disabledReason}
        aria-label={
          disabledReason ? `${method.name}. ${disabledReason}` : method.name
        }
        className={cn(
          'min-h-14 min-w-0 justify-start gap-2 rounded-lg px-3 py-2 text-left',
          paymentMethod === method.type &&
            'border-foreground bg-foreground/5 dark:border-foreground dark:bg-foreground/10'
        )}
      >
        {calculating && paymentMethod === method.type ? (
          <Loader2 className='h-4 w-4 animate-spin' />
        ) : (
          getPaymentIcon(method.type, 'h-4 w-4', method.icon, method.name)
        )}
        <span className='flex min-w-0 flex-col items-start gap-0.5'>
          <span className='max-w-full truncate'>{method.name}</span>
          {belowMinimum && (
            <span className='text-muted-foreground max-w-full truncate text-[11px] leading-4 font-normal'>
              {t('Minimum:')} {minimum}
            </span>
          )}
        </span>
      </Button>
    )

    if (!belowMinimum) return button

    return (
      <TooltipProvider key={method.type}>
        <Tooltip>
          <TooltipTrigger render={button} />
          <TooltipContent>{disabledReason}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    )
  }

  return (
    <TitledCard
      title={t('Purchase Redemption Codes')}
      description={t(
        'Pay with a configured payment method and receive codes in your wallet.'
      )}
      icon={<Ticket className='h-4 w-4' />}
      iconTone='warning'
      disableHoverEffect
      action={
        <div className='flex justify-end sm:block'>
          <Button
            type='button'
            variant='outline'
            size='icon'
            onClick={() => void loadCodes(codesPage)}
            disabled={codesLoading}
            aria-label={t('Refresh redemption codes')}
            title={t('Refresh redemption codes')}
          >
            <RefreshCw
              className={codesLoading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'}
            />
          </Button>
        </div>
      }
      contentClassName='space-y-5'
    >
      <div className='grid gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'>
        <div className='min-w-0 space-y-5'>
          {purchaseMethods.length === 0 ? (
            <Alert>
              <AlertDescription>
                {t(
                  'No external payment methods are configured for redemption purchases.'
                )}
              </AlertDescription>
            </Alert>
          ) : (
            <>
              <div className='space-y-2.5 sm:space-y-3'>
                <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                  {t('Amount')}
                </Label>
                {presetAmounts.length > 0 && (
                  <div className='grid grid-cols-2 gap-1.5 sm:gap-3'>
                    {presetAmounts.map((preset) => {
                      const discount =
                        preset.discount ||
                        topupInfo?.discount?.[preset.value] ||
                        1
                      const {
                        displayValue,
                        actualPrice,
                        savedAmount,
                        hasDiscount,
                      } = calculatePresetPricing(
                        preset.value,
                        priceRatio,
                        discount,
                        usdExchangeRate
                      )
                      return (
                        <Button
                          key={preset.value}
                          type='button'
                          variant='outline'
                          className={cn(
                            'flex min-h-16 flex-col items-start rounded-lg px-3 py-2.5 text-left whitespace-normal sm:min-h-[72px] sm:p-4',
                            selectedPreset === preset.value
                              ? 'border-foreground bg-foreground/5 dark:border-foreground dark:bg-foreground/10'
                              : 'border-muted'
                          )}
                          onClick={() => {
                            setUnitAmountText(String(preset.value))
                            setSelectedPreset(preset.value)
                          }}
                        >
                          <div className='flex w-full items-center justify-between'>
                            <div className='text-base font-semibold sm:text-lg'>
                              {formatNumber(displayValue)}
                            </div>
                            {hasDiscount && (
                              <div className='text-xs font-medium text-green-600'>
                                {getDiscountLabel(discount)}
                              </div>
                            )}
                          </div>
                          <div className='text-muted-foreground mt-1.5 w-full text-xs sm:mt-2'>
                            {t('Pay')} {formatCurrency(actualPrice)}
                            {hasDiscount && savedAmount > 0 && (
                              <span className='text-green-600'>
                                {' '}
                                • {t('Save amount')}{' '}
                                {formatCurrency(savedAmount)}
                              </span>
                            )}
                          </div>
                        </Button>
                      )
                    })}
                  </div>
                )}

                <div className='grid grid-cols-[minmax(0,1fr)_minmax(96px,0.5fr)] gap-2 sm:grid-cols-[minmax(0,1fr)_120px]'>
                  <div className='space-y-2'>
                    <Label htmlFor='redemption-unit-amount'>
                      {t('Code denomination')}
                    </Label>
                    <Input
                      id='redemption-unit-amount'
                      type='number'
                      min={1}
                      step={1}
                      value={unitAmountText}
                      onChange={(event) => {
                        setUnitAmountText(event.target.value)
                        setSelectedPreset(null)
                      }}
                      placeholder={t('For example, 5')}
                      className='h-9 text-base sm:h-10 sm:text-lg'
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
                      className='h-9 sm:h-10'
                    />
                  </div>
                </div>
              </div>

              <div className='space-y-2.5 sm:space-y-3'>
                <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                  {t('Payment Method')}
                </Label>
                {standardPurchaseMethods.length > 0 && (
                  <div className='grid grid-cols-2 gap-1.5 sm:gap-3 lg:grid-cols-3'>
                    {standardPurchaseMethods.map(renderPaymentMethodButton)}
                  </div>
                )}

                {waffoPurchaseEnabled && waffoPayMethods.length > 0 && (
                  <div className='space-y-2.5 sm:space-y-3'>
                    <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                      {t('Waffo Payment')}
                    </Label>
                    <div className='grid grid-cols-2 gap-1.5 sm:gap-3 lg:grid-cols-3'>
                      {waffoPayMethods.map((method, index) => {
                        const methodKey = [
                          method.name,
                          method.icon,
                          method.payMethodType,
                          method.payMethodName,
                        ]
                          .filter(Boolean)
                          .join('-')
                        const waffoPaymentMethod: PaymentMethod = {
                          name: method.name,
                          type: 'waffo',
                          icon: method.icon,
                        }
                        const minimum = getPaymentMethodMinTopup(
                          waffoPaymentMethod,
                          topupInfo
                        )
                        const belowMinimum = totalAmount < minimum
                        const disabledReason = belowMinimum
                          ? t('Minimum topup amount: {{amount}}', {
                              amount: minimum,
                            })
                          : undefined
                        const isSelected =
                          paymentMethod === 'waffo' &&
                          waffoMethodIndex === String(index)
                        const button = (
                          <Button
                            key={methodKey}
                            type='button'
                            variant='outline'
                            onClick={() =>
                              void handlePaymentMethodSelect(
                                waffoPaymentMethod,
                                index
                              )
                            }
                            disabled={belowMinimum || calculating || processing}
                            title={disabledReason}
                            aria-label={
                              disabledReason
                                ? `${method.name}. ${disabledReason}`
                                : method.name
                            }
                            className={cn(
                              'min-h-14 min-w-0 justify-start gap-2 rounded-lg px-3 py-2 text-left',
                              isSelected &&
                                'border-foreground bg-foreground/5 dark:border-foreground dark:bg-foreground/10'
                            )}
                          >
                            {calculating && isSelected ? (
                              <Loader2 className='h-4 w-4 animate-spin' />
                            ) : (
                              getPaymentIcon(
                                waffoPaymentMethod.type,
                                'h-4 w-4',
                                method.icon,
                                method.name
                              )
                            )}
                            <span className='flex min-w-0 flex-col items-start gap-0.5'>
                              <span className='max-w-full truncate'>
                                {method.name}
                              </span>
                              {belowMinimum && (
                                <span className='text-muted-foreground max-w-full truncate text-[11px] leading-4 font-normal'>
                                  {t('Minimum:')} {minimum}
                                </span>
                              )}
                            </span>
                          </Button>
                        )

                        return belowMinimum ? (
                          <TooltipProvider key={methodKey}>
                            <Tooltip>
                              <TooltipTrigger render={button} />
                              <TooltipContent>{disabledReason}</TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        ) : (
                          button
                        )
                      })}
                    </div>
                  </div>
                )}

                {standardPurchaseMethods.length === 0 &&
                  (!waffoPurchaseEnabled || waffoPayMethods.length === 0) && (
                    <Alert>
                      <AlertDescription>
                        {t(
                          'No payment methods available. Please contact administrator.'
                        )}
                      </AlertDescription>
                    </Alert>
                  )}
              </div>

              <div className='bg-muted/20 flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
                <div className='text-sm'>
                  <div className='text-muted-foreground'>
                    {t('Purchase total')}
                  </div>
                  <div className='font-semibold'>
                    {totalAmount > 0
                      ? `${formatCurrency(totalAmount)} · ${quantity} ${t('codes')}`
                      : '-'}
                  </div>
                </div>
                <div className='text-left text-sm sm:text-right'>
                  <div className='text-muted-foreground'>
                    {t('Amount to pay:')}
                  </div>
                  <div className='font-semibold'>{paymentAmountDisplay}</div>
                </div>
              </div>
            </>
          )}

          <p className='text-muted-foreground flex items-start gap-2 text-xs'>
            <ExternalLink className='mt-0.5 h-3.5 w-3.5 shrink-0' />
            {t(
              'Wallet balance cannot be used to buy codes. Payment discounts configured by the administrator are applied automatically.'
            )}
          </p>
        </div>

        <div className='min-w-0 space-y-3 border-t pt-5 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6'>
          <div className='flex items-center justify-between gap-3'>
            <div>
              <h3 className='text-sm font-semibold'>
                {t('Your Redemption Codes')}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {t('Unused purchased codes can be refunded to your balance.')}
              </p>
            </div>
            {codesTotal > 0 && (
              <span className='text-muted-foreground text-xs'>
                {codesTotal}
              </span>
            )}
          </div>

          {codesContent}

          {codesTotal > PAGE_SIZE && (
            <div className='flex items-center justify-end gap-2'>
              <Button
                type='button'
                variant='outline'
                size='icon'
                onClick={() => void loadCodes(codesPage - 1)}
                disabled={codesPage <= 1 || codesLoading}
                aria-label={t('Previous page')}
                title={t('Previous page')}
              >
                <ChevronLeft className='h-4 w-4' />
              </Button>
              <span className='text-muted-foreground text-xs'>
                {codesPage} / {totalPages}
              </span>
              <Button
                type='button'
                variant='outline'
                size='icon'
                onClick={() => void loadCodes(codesPage + 1)}
                disabled={codesPage >= totalPages || codesLoading}
                aria-label={t('Next page')}
                title={t('Next page')}
              >
                <ChevronRight className='h-4 w-4' />
              </Button>
            </div>
          )}
        </div>
      </div>

      <PaymentConfirmDialog
        open={confirmDialogOpen}
        onOpenChange={setConfirmDialogOpen}
        onConfirm={() => void handlePurchase()}
        topupAmount={totalAmount}
        paymentAmount={Number(paymentAmount) || 0}
        paymentMethod={selectedPaymentMethod}
        calculating={calculating}
        processing={processing}
        discountRate={topupInfo?.discount?.[totalAmount] ?? 1}
        usdExchangeRate={usdExchangeRate}
        amountLabel={t('Purchase total')}
        amountDisplay={
          totalAmount > 0
            ? `${formatCurrency(totalAmount)} · ${quantity} ${t('codes')}`
            : '-'
        }
      />
    </TitledCard>
  )
}
