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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import type { TopupRecord } from '../types'

interface CompleteOrderDialogProps {
  record: TopupRecord | null
  completing: boolean
  onClose: () => void
  onConfirm: (tradeNo: string, receivedAmount: string) => Promise<boolean>
}

export function CompleteOrderDialog(props: CompleteOrderDialogProps) {
  const { t } = useTranslation()
  const [receivedAmount, setReceivedAmount] = useState('')
  const crypto = props.record?.crypto_payment

  const close = () => {
    setReceivedAmount('')
    props.onClose()
  }

  const confirm = async () => {
    if (!props.record) return
    const success = await props.onConfirm(props.record.trade_no, receivedAmount)
    if (success) close()
  }

  return (
    <ConfirmDialog
      open={props.record !== null}
      onOpenChange={(open) => !open && close()}
      title={t('Complete Order')}
      desc={
        crypto
          ? t(
              'The gateway is checked first and the confirmed amount is credited. Enter the received amount only if the gateway has no confirmed payment.'
            )
          : t(
              'Are you sure you want to manually complete this order? The user will be credited with the corresponding quota.'
            )
      }
      confirmText={props.completing ? t('Processing...') : t('Confirm')}
      isLoading={props.completing}
      handleConfirm={() => void confirm()}
    >
      {crypto && (
        <div className='space-y-2'>
          <Label htmlFor='received-amount'>
            {t('Received amount ({{currency}})', {
              currency: crypto.pay_currency.toUpperCase(),
            })}
          </Label>
          <Input
            id='received-amount'
            inputMode='decimal'
            value={receivedAmount}
            placeholder={crypto.pay_amount}
            onChange={(event) => setReceivedAmount(event.target.value)}
          />
        </div>
      )}
    </ConfirmDialog>
  )
}
