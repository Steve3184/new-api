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
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

export interface ReferralRules {
  enabled: boolean
  percent: number
  min_paid_cny: number
  max_orders: number
  delay_hours: number
  currency_rates: string
}

export interface ReferralReward {
  id: number
  inviter_id: number
  invitee_id: number
  trade_no: string
  paid_cny: number
  percent: number
  quota: number
  status: 'pending' | 'credited' | 'reversed'
  created_at: number
  available_at: number
  credited_at: number
  reason: string
}

export interface ReferralData {
  invited_users: number
  items: ReferralReward[]
  total: number
  summary: { status: ReferralReward['status']; quota: number; count: number }[]
  rules: ReferralRules
}

export async function getReferralRewards(
  admin: boolean,
  params: Record<string, string | number>
) {
  const response = await api.get<{
    success: boolean
    message: string
    data: ReferralData
  }>(admin ? '/api/user/referral-rewards' : '/api/user/self/referral-rewards', {
    params,
  })
  return requireServerSuccess(response.data).data
}

export async function reverseReferralReward(id: number, reason: string) {
  const response = await api.post(`/api/user/referral-rewards/${id}/reverse`, {
    reason,
  })
  return requireServerSuccess(response.data)
}
