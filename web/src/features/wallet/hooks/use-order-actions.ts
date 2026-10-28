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
import i18next from 'i18next'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'

import { completeOrder, isApiSuccess, resumeTopupPayment } from '../api'
import { submitPaymentForm } from '../lib/payment'
import type { MoneroInvoice, NowPaymentsInvoice, TopupRecord } from '../types'

/**
 * Actions on a single billing order: resume an unpaid payment (any gateway)
 * and complete an order by hand (admin). Crypto invoices are surfaced as state
 * so the existing payment dialogs can be reused.
 */
export function useOrderActions(onChanged: () => void) {
  const [resumingTradeNo, setResumingTradeNo] = useState<string | null>(null)
  const [completing, setCompleting] = useState(false)
  const [nowPaymentsInvoice, setNowPaymentsInvoice] =
    useState<NowPaymentsInvoice | null>(null)
  const [moneroInvoice, setMoneroInvoice] = useState<MoneroInvoice | null>(null)

  const resume = useCallback(
    async (record: TopupRecord) => {
      setResumingTradeNo(record.trade_no)
      try {
        const response = await resumeTopupPayment(record.trade_no)
        if (!isApiSuccess(response) || !response.data) {
          handleServerError(response, i18next.t('Failed to continue payment'))
          onChanged()
          return
        }
        const payment = response.data
        if (payment.type === 'nowpayments') {
          setNowPaymentsInvoice(payment.invoice)
          return
        }
        if (payment.type === 'monero') {
          setMoneroInvoice(payment.invoice)
          return
        }
        if (payment.type === 'form') {
          submitPaymentForm(payment.url, payment.params)
          return
        }
        const target = new URL(payment.url)
        if (target.protocol !== 'https:' && target.protocol !== 'http:') {
          throw new Error(i18next.t('Failed to continue payment'))
        }
        window.open(target.href, '_blank', 'noopener,noreferrer')
      } catch (error) {
        handleServerError(error, i18next.t('Failed to continue payment'))
      } finally {
        setResumingTradeNo(null)
      }
    },
    [onChanged]
  )

  const complete = useCallback(
    async (tradeNo: string, receivedAmount: string) => {
      setCompleting(true)
      try {
        const response = await completeOrder({
          trade_no: tradeNo,
          received_amount: receivedAmount.trim() || undefined,
        })
        if (!isApiSuccess(response)) {
          handleServerError(response, i18next.t('Failed to complete order'))
          return false
        }
        toast.success(i18next.t('Order completed successfully'))
        onChanged()
        return true
      } catch (error) {
        handleServerError(error, i18next.t('Failed to complete order'))
        return false
      } finally {
        setCompleting(false)
      }
    },
    [onChanged]
  )

  return {
    resumingTradeNo,
    completing,
    nowPaymentsInvoice,
    moneroInvoice,
    closeNowPaymentsInvoice: () => setNowPaymentsInvoice(null),
    closeMoneroInvoice: () => setMoneroInvoice(null),
    resume,
    complete,
  }
}
