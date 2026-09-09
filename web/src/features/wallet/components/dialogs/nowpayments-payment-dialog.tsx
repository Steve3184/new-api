import { Copy01Icon, CopyCheckIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { QRCodeSVG } from 'qrcode.react'
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { useNowPaymentsPaymentStatus } from '../../hooks'
import {
  getCryptoCurrencyInfo,
  getCryptoPaymentQrValue,
} from '../../lib/crypto'
import type { NowPaymentsInvoice } from '../../types'
import { CryptoCurrencyIcon } from '../crypto-currency-icon'

interface NowPaymentsPaymentDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  invoice: NowPaymentsInvoice | null
  onPaymentSuccess: () => void
}

export function NowPaymentsPaymentDialog(props: NowPaymentsPaymentDialogProps) {
  const { t } = useTranslation()
  const settledPaymentRef = useRef<string | null>(null)
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const paymentId = props.invoice?.payment_id
  const onOpenChange = props.onOpenChange
  const onPaymentSuccess = props.onPaymentSuccess
  const { data: paymentStatus, isError: paymentStatusError } =
    useNowPaymentsPaymentStatus(paymentId, props.open)

  useEffect(() => {
    if (
      !paymentId ||
      paymentStatus?.status !== 'success' ||
      settledPaymentRef.current === paymentId
    ) {
      return
    }
    settledPaymentRef.current = paymentId
    onOpenChange(false)
    toast.success(t('Cryptocurrency payment credited successfully'))
    onPaymentSuccess()
  }, [onOpenChange, onPaymentSuccess, paymentId, paymentStatus?.status, t])

  const copyValue = useCallback(
    async (value: string, successMessage: string, errorMessage: string) => {
      if (await copyToClipboard(value)) {
        toast.success(successMessage)
        return
      }
      toast.error(errorMessage)
    },
    [copyToClipboard]
  )

  if (!props.invoice) return null
  const currencyInfo = getCryptoCurrencyInfo(props.invoice.pay_currency)
  const qrValue = getCryptoPaymentQrValue(
    props.invoice.pay_currency,
    props.invoice.pay_address,
    props.invoice.pay_amount
  )
  const expiresAt = new Date(props.invoice.expires_at * 1000).toLocaleString()
  let statusText = t('Waiting for payment confirmation')
  if (paymentStatusError) {
    statusText = t(
      'Unable to fetch payment status; keep the payment address open and try again.'
    )
  } else if (
    paymentStatus?.status === 'expired' ||
    paymentStatus?.status === 'failed'
  ) {
    statusText = t('Payment expired or failed')
  } else if (paymentStatus?.status === 'success') {
    statusText = t('Payment confirmed')
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Pay with NOWPayments')}
      contentClassName='sm:max-w-lg'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      <div className='bg-muted/50 space-y-2 rounded-lg p-3 text-sm'>
        <div className='flex items-center justify-between gap-4'>
          <span className='text-muted-foreground'>{t('You need to pay:')}</span>
          <span className='flex items-center gap-2 text-right font-semibold'>
            <CryptoCurrencyIcon
              currency={props.invoice.pay_currency}
              className='h-4 w-4'
            />
            <span>
              {props.invoice.pay_amount} {currencyInfo.symbol}
            </span>
            {currencyInfo.network && (
              <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-xs font-medium'>
                {currencyInfo.network}
              </span>
            )}
          </span>
        </div>
        <div className='flex items-center justify-between gap-4'>
          <span className='text-muted-foreground'>{t('Price amount')}</span>
          <span>
            {props.invoice.price_amount}{' '}
            {props.invoice.price_currency.toUpperCase()}
          </span>
        </div>
        {currencyInfo.network && (
          <div className='flex items-center justify-between gap-4'>
            <span className='text-muted-foreground'>{t('Network')}</span>
            <span className='font-medium'>{currencyInfo.network}</span>
          </div>
        )}
      </div>
      <Alert>
        <AlertDescription>{statusText}</AlertDescription>
      </Alert>
      {qrValue && (
        <div className='space-y-2'>
          <div className='flex items-center gap-2 text-sm font-medium'>
            <CryptoCurrencyIcon
              currency={props.invoice.pay_currency}
              className='h-4 w-4'
            />
            <span>{t('Scan QR Code')}</span>
            {currencyInfo.network && (
              <span className='text-muted-foreground text-xs font-normal'>
                {currencyInfo.network}
              </span>
            )}
          </div>
          <div
            className='flex justify-center rounded-lg border bg-white p-4 dark:bg-white'
            role='img'
            aria-label={t('Scan QR Code')}
          >
            <QRCodeSVG value={qrValue} size={180} level='M' includeMargin />
          </div>
        </div>
      )}
      <div className='space-y-2'>
        <div className='text-sm font-medium'>{t('Payment address')}</div>
        <InputGroup>
          <InputGroupInput
            readOnly
            value={props.invoice.pay_address}
            className='font-mono text-xs'
          />
          <InputGroupAddon align='inline-end'>
            <InputGroupButton
              type='button'
              variant='outline'
              size='icon-sm'
              onClick={() =>
                void copyValue(
                  props.invoice?.pay_address || '',
                  t('Address copied'),
                  t('Unable to copy address')
                )
              }
              aria-label={t('Copy address')}
            >
              <HugeiconsIcon
                icon={
                  copiedText === props.invoice.pay_address
                    ? CopyCheckIcon
                    : Copy01Icon
                }
                strokeWidth={2}
                aria-hidden='true'
              />
            </InputGroupButton>
          </InputGroupAddon>
        </InputGroup>
      </div>
      {props.invoice.payin_extra_id && (
        <div className='space-y-2'>
          <div className='text-sm font-medium'>
            {t('Payment memo or destination tag')}
          </div>
          <InputGroup>
            <InputGroupInput
              readOnly
              value={props.invoice.payin_extra_id}
              className='font-mono text-xs'
            />
            <InputGroupAddon align='inline-end'>
              <InputGroupButton
                type='button'
                variant='outline'
                size='icon-sm'
                onClick={() =>
                  void copyValue(
                    props.invoice?.payin_extra_id || '',
                    t('Payment memo copied'),
                    t('Unable to copy payment memo')
                  )
                }
                aria-label={t('Copy payment memo or destination tag')}
              >
                <HugeiconsIcon
                  icon={
                    copiedText === props.invoice.payin_extra_id
                      ? CopyCheckIcon
                      : Copy01Icon
                  }
                  strokeWidth={2}
                  aria-hidden='true'
                />
              </InputGroupButton>
            </InputGroupAddon>
          </InputGroup>
        </div>
      )}
      <p className='text-muted-foreground text-xs'>
        {t(
          'Send the exact amount to the address above. Payment is credited automatically after gateway confirmation.'
        )}{' '}
        {t('Invoice expires at {{time}}.', { time: expiresAt })}
      </p>
    </Dialog>
  )
}
