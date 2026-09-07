import i18next from 'i18next'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { isApiSuccess, requestNowPaymentsPayment } from '../api'
import type { NowPaymentsInvoice } from '../types'

export function useNowPaymentsPayment() {
  const [processing, setProcessing] = useState(false)

  const createInvoice = useCallback(
    async (amount: number, payCurrency: string) => {
      try {
        setProcessing(true)
        const response = await requestNowPaymentsPayment(
          Math.floor(amount),
          payCurrency
        )
        if (!isApiSuccess(response) || !response.data) {
          toast.error(response.message || i18next.t('Payment request failed'))
          return null
        }
        return response.data as NowPaymentsInvoice
      } catch {
        toast.error(i18next.t('Payment request failed'))
        return null
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  return { processing, createInvoice }
}
