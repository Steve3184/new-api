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
export interface SupportConversation {
  id: number
  user_id: number
  title: string
  status: string
  last_message_at: number
  unread_user: number
  unread_admin: number
  created_at: number
  updated_at: number
  username?: string
  display_name?: string
}

export type SupportMessageKind =
  | 'text'
  | 'image'
  | 'order_quote'
  | 'quota_grant'
  | 'subscription_grant'

export interface SupportMessage {
  id: number
  conversation_id: number
  sender_id: number
  sender_role: number
  kind: SupportMessageKind | string
  content: string
  image_data?: string
  image_mime?: string
  image_size?: number
  order_type?: 'topup' | 'subscription' | string
  order_id?: number
  order_trade_no?: string
  order_status?: string
  order_provider?: string
  order_amount?: number
  order_money?: number
  order_plan_id?: number
  order_plan_title?: string
  grant_quota?: number
  grant_plan_id?: number
  grant_plan_title?: string
  created_at: number
}

export interface SupportConversationPayload {
  conversation: SupportConversation
  messages: SupportMessage[]
}

export interface SupportUnreadCount {
  count: number
}

export interface SupportOrderQuote {
  order_type: 'topup' | 'subscription' | string
  order_id: number
  trade_no: string
  status: string
  provider: string
  amount?: number
  money?: number
  plan_id?: number
  plan_title?: string
  created_at: number
  can_complete: boolean
}

export interface SupportConversationPage {
  items: SupportConversation[]
  total: number
  page: number
  page_size: number
}
