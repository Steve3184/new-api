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
*/
import { Check, Copy } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { useMoneroPaymentStatus } from '../../hooks'
import { getMoneroPaymentStatusMessage } from '../../lib/monero-payment-status'
import type { MoneroInvoice } from '../../types'

interface MoneroPaymentDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  invoice: MoneroInvoice | null
  onPaymentSuccess: () => void
}

export function MoneroPaymentDialog({
  open,
  onOpenChange,
  invoice,
  onPaymentSuccess,
}: MoneroPaymentDialogProps) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const settledInvoiceAddressRef = useRef<string | null>(null)
  const { data: paymentStatus } = useMoneroPaymentStatus(invoice?.address, open)

  useEffect(() => {
    if (
      !invoice?.address ||
      paymentStatus?.status !== 'success' ||
      settledInvoiceAddressRef.current === invoice.address
    ) {
      return
    }
    settledInvoiceAddressRef.current = invoice.address
    onOpenChange(false)
    toast.success(t('Monero payment credited successfully'))
    onPaymentSuccess()
  }, [
    invoice?.address,
    onOpenChange,
    onPaymentSuccess,
    paymentStatus?.status,
    t,
  ])

  if (!invoice) return null

  const paymentURI = `monero:${invoice.address}?tx_amount=${invoice.amount_xmr}`
  const expiresAt = new Date(invoice.expires_at * 1000).toLocaleString()
  const paymentStatusMessage = getMoneroPaymentStatusMessage(paymentStatus)

  const copyAddress = async () => {
    try {
      await navigator.clipboard.writeText(invoice.address)
      setCopied(true)
      toast.success(t('Address copied'))
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('Unable to copy address'))
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Pay with Monero')}
      contentClassName='sm:max-w-lg'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      <div className='bg-muted/50 space-y-2 rounded-lg p-3 text-sm'>
        <div className='flex items-center justify-between gap-4'>
          <span className='text-muted-foreground'>
            {t('You will receive:')}
          </span>
          <span className='font-semibold'>{invoice.quota_amount}</span>
        </div>
        <div className='flex items-center justify-between gap-4'>
          <span className='text-muted-foreground'>{t('You need to pay:')}</span>
          <span className='font-semibold'>{invoice.amount_xmr} XMR</span>
        </div>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'The displayed XMR amount excludes network fees. Send fees in addition to this amount.'
        )}
      </p>

      <Alert>
        <AlertDescription>
          {t(paymentStatusMessage.key, paymentStatusMessage.values)}
        </AlertDescription>
      </Alert>

      <div className='flex justify-center rounded-lg border bg-white p-4 dark:bg-white'>
        <QRCodeSVG value={paymentURI} size={180} level='M' includeMargin />
      </div>

      <div className='grid grid-cols-2 gap-3 text-sm'>
        <div className='rounded-md border p-3'>
          <div className='text-muted-foreground'>{t('Network')}</div>
          <div className='mt-1 font-medium capitalize'>{invoice.network}</div>
        </div>
        <div className='rounded-md border p-3'>
          <div className='text-muted-foreground'>
            {t('Required confirmations')}
          </div>
          <div className='mt-1 font-medium'>{invoice.confirmations}</div>
        </div>
      </div>

      <div className='space-y-2'>
        <div className='text-sm font-medium'>{t('Monero address')}</div>
        <div className='flex gap-2'>
          <Input
            readOnly
            value={invoice.address}
            className='font-mono text-xs'
          />
          <Button
            type='button'
            variant='outline'
            size='icon'
            onClick={copyAddress}
            aria-label={t('Copy address')}
          >
            {copied ? (
              <Check className='h-4 w-4' />
            ) : (
              <Copy className='h-4 w-4' />
            )}
          </Button>
        </div>
      </div>

      <p className='text-muted-foreground text-xs'>
        {t('Payment is credited automatically after confirmation.')}{' '}
        {t(
          'Credited quota is calculated from the XMR actually received at the locked invoice rate.'
        )}{' '}
        {t('Invoice expires at {{time}}.', { time: expiresAt })}
      </p>
    </Dialog>
  )
}
