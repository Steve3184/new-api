import { useQuery } from '@tanstack/react-query'

import { getNowPaymentsPaymentStatus, isApiSuccess } from '../api'

export function useNowPaymentsPaymentStatus(
  paymentId: string | undefined,
  open: boolean
) {
  return useQuery({
    queryKey: ['nowpayments-payment-status', paymentId],
    queryFn: async () => {
      if (!paymentId) throw new Error('NOWPayments payment id is required')
      const response = await getNowPaymentsPaymentStatus(paymentId)
      if (!isApiSuccess(response) || !response.data) {
        throw new Error(
          response.message || 'NOWPayments payment status is unavailable'
        )
      }
      return response.data
    },
    enabled: open && Boolean(paymentId),
    refetchInterval: (query) =>
      query.state.data?.status === 'pending' ? 5000 : false,
  })
}
