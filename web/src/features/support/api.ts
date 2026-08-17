/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import type {
  SupportConversationPage,
  SupportConversationPayload,
  SupportMessage,
  SupportOrderQuote,
} from './types'

export async function getSupportConversation(): Promise<
  { success: boolean; message?: string; data?: SupportConversationPayload }
> {
  const res = await api.get('/api/support/conversation')
  return res.data
}

export async function getSupportOrders(): Promise<{
  success: boolean
  message?: string
  data?: SupportOrderQuote[]
}> {
  const res = await api.get('/api/support/orders')
  return res.data
}

export async function sendSupportMessage(
  payload: {
    content?: string
    kind?: string
    order_type?: string
    order_id?: number
    image?: File | null
  }
): Promise<{ success: boolean; message?: string; data?: SupportMessage }> {
  const body = new FormData()
  body.append('kind', payload.kind || 'text')
  body.append('content', payload.content || '')
  if (payload.order_type) body.append('order_type', payload.order_type)
  if (payload.order_id) body.append('order_id', String(payload.order_id))
  if (payload.image) body.append('image', payload.image)
  const res = await api.post('/api/support/messages', body)
  return res.data
}

export async function getAdminSupportConversations(): Promise<{
  success: boolean
  message?: string
  data?: SupportConversationPage
}> {
  const res = await api.get('/api/support/admin/conversations', {
    params: { p: 1, page_size: 100 },
  })
  return res.data
}

export async function getAdminSupportConversation(
  id: number
): Promise<{ success: boolean; message?: string; data?: SupportConversationPayload }> {
  const res = await api.get(`/api/support/admin/conversations/${id}`)
  return res.data
}

export async function sendAdminSupportMessage(
  id: number,
  payload: {
    content?: string
    kind?: string
    image?: File | null
  }
): Promise<{ success: boolean; message?: string; data?: SupportMessage }> {
  const body = new FormData()
  body.append('kind', payload.kind || 'text')
  body.append('content', payload.content || '')
  if (payload.image) body.append('image', payload.image)
  const res = await api.post(
    `/api/support/admin/conversations/${id}/messages`,
    body
  )
  return res.data
}

export async function grantSupportQuota(
  id: number,
  quota: number,
  note: string
): Promise<{ success: boolean; message?: string; data?: SupportMessage }> {
  const res = await api.post(`/api/support/admin/conversations/${id}/grant-quota`, {
    quota,
    note,
  })
  return res.data
}

export async function grantSupportSubscription(
  id: number,
  planId: number,
  note: string
): Promise<{ success: boolean; message?: string; data?: SupportMessage }> {
  const res = await api.post(
    `/api/support/admin/conversations/${id}/grant-subscription`,
    { plan_id: planId, note }
  )
  return res.data
}

export async function completeSupportOrder(
  messageId: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(`/api/support/admin/messages/${messageId}/complete-order`)
  return res.data
}
