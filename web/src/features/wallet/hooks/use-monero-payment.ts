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
import i18next from 'i18next'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { isApiSuccess, requestMoneroPayment } from '../api'
import type { MoneroInvoice } from '../types'

export function useMoneroPayment() {
  const [processing, setProcessing] = useState(false)

  const createInvoice = useCallback(async (amount: number) => {
    try {
      setProcessing(true)
      const response = await requestMoneroPayment(Math.floor(amount))
      if (!isApiSuccess(response) || !response.data) {
        toast.error(response.message || i18next.t('Payment request failed'))
        return null
      }
      return response.data as MoneroInvoice
    } catch {
      toast.error(i18next.t('Payment request failed'))
      return null
    } finally {
      setProcessing(false)
    }
  }, [])

  return { processing, createInvoice }
}
