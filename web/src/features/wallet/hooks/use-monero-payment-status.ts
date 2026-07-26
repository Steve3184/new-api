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
import { useQuery } from '@tanstack/react-query'

import { getMoneroPaymentStatus, isApiSuccess } from '../api'

export function useMoneroPaymentStatus(
  address: string | undefined,
  open: boolean
) {
  return useQuery({
    queryKey: ['monero-payment-status', address],
    queryFn: async () => {
      if (!address) {
        throw new Error('Monero payment address is required')
      }
      const response = await getMoneroPaymentStatus(address)
      if (!isApiSuccess(response) || !response.data) {
        throw new Error(
          response.message || 'Monero payment status is unavailable'
        )
      }
      return response.data
    },
    enabled: open && Boolean(address),
    refetchInterval: (query) =>
      query.state.data?.status === 'pending' ? 5000 : false,
  })
}
