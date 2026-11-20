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
import { z } from 'zod'

// ============================================================================
// Subscription Plan Schema & Types
// ============================================================================

export const subscriptionPlanSchema = z.object({
  id: z.number(),
  title: z.string(),
  subtitle: z.string().optional(),
  price_amount: z.number(),
  currency: z.string().default('USD'),
  duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
  duration_value: z.number(),
  custom_seconds: z.number().optional(),
  quota_reset_period: z.enum(['never', 'daily', 'weekly', 'monthly', 'custom']),
  quota_reset_custom_seconds: z.number().optional(),
  enabled: z.boolean(),
  sort_order: z.number(),
  allow_balance_pay: z.boolean().optional().default(true),
  allow_wallet_overflow: z.boolean().optional().default(true),
  wallet_only_groups_enabled: z.boolean().default(false),
  wallet_only_groups_mode: z
    .enum(['blacklist', 'whitelist'])
    .default('blacklist'),
  wallet_only_groups: z.string().optional().default(''),
  rate_limit_groups: z.string().optional().default('[]'),
  max_purchase_per_user: z.number(),
  max_active_per_user: z.number().optional().default(0),
  total_amount: z.number(),
  five_hour_limit: z.number().optional(),
  weekly_limit: z.number().optional(),
  monthly_limit: z.number().optional(),
  subscription_group: z.string().optional(),
  auto_create_key: z.boolean().optional().default(false),
  upgrade_group: z.string().optional(),
  downgrade_group: z.string().optional(),
  stripe_price_id: z.string().optional(),
  creem_product_id: z.string().optional(),
  waffo_pancake_product_id: z.string().optional(),
  usage_tiers: z.string().optional().default('[]'),
  allow_tier_switch: z.boolean().optional().default(false),
  upgrade_plan_ids: z.string().optional().default(''),
  allow_auto_renew: z.boolean().optional().default(false),
  // Extra credits are opt-in per plan; plans saved before this field existed
  // parse as disabled with no rate.
  allow_extra_credits: z.boolean().optional().default(false),
  extra_credits_rate: z.number().optional().default(0),
})

export type SubscriptionPlan = z.infer<typeof subscriptionPlanSchema>

export interface PlanRecord {
  plan: SubscriptionPlan
}

// ============================================================================
// User Subscription Schema & Types
// ============================================================================

export const userSubscriptionSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  plan_id: z.number(),
  status: z.string(),
  source: z.string().optional(),
  start_time: z.number(),
  end_time: z.number(),
  amount_total: z.number(),
  amount_used: z.number(),
  next_reset_time: z.number().optional(),
  five_hour_limit: z.number().optional(),
  five_hour_used: z.number().optional(),
  five_hour_next_reset_time: z.number().optional(),
  weekly_limit: z.number().optional(),
  weekly_used: z.number().optional(),
  weekly_next_reset_time: z.number().optional(),
  monthly_limit: z.number().optional(),
  monthly_used: z.number().optional(),
  monthly_next_reset_time: z.number().optional(),
  multiplier: z.number().optional(),
  price_amount: z.number().nullable().optional(),
  auto_renew: z.boolean().optional(),
  renew_multiplier: z.number().optional(),
  /** Purchased extra credits in raw quota units; they survive expiry. */
  extra_credits_total: z.number().optional(),
  extra_credits_used: z.number().optional(),
  /** Server-derived: whether a new extra-credits purchase would be accepted. */
  extra_credits_can_purchase: z.boolean().optional(),
})

export type UserSubscription = z.infer<typeof userSubscriptionSchema>

export interface UserSubscriptionRecord {
  subscription: UserSubscription
}

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PlanPayload {
  plan: Partial<SubscriptionPlan>
}

export interface SubscriptionPayRequest {
  plan_id: number
  multiplier?: number
  payment_method?: string
  auto_renew?: boolean
}

/** A purchasable usage multiplier of a plan; the plan itself is 1x. */
export interface SubscriptionUsageTier {
  multiplier: number
  price_amount: number
  stripe_price_id?: string
  creem_product_id?: string
  waffo_pancake_product_id?: string
}

export interface SubscriptionUpgradeRequest {
  subscription_id: number
  plan_id: number
  multiplier: number
}

export interface SubscriptionUpgradeQuote {
  subscription_id: number
  plan_id: number
  multiplier: number
  price_quota: number
  credit_quota: number
  charge_quota: number
}

export interface SubscriptionRenewalRequest {
  auto_renew: boolean
  renew_multiplier: number
}

export interface SubscriptionExtraCreditsRequest {
  /** Raw site wallet quota units debited from the user's balance. */
  wallet_quota: number
  /** Client-generated id reused on every retry so one purchase charges once. */
  request_id: string
  /**
   * The plan rate this purchase was previewed with. The server refuses the
   * purchase when the plan's current rate differs, instead of granting
   * credits at a rate the user never saw.
   */
  expected_rate: number
}

export interface SubscriptionExtraCreditsPurchase {
  id: number
  user_id: number
  subscription_id: number
  request_id: string
  wallet_quota: number
  /** Subscription credit quota units granted. */
  credits: number
  rate: number
  created_at: number
}

export interface SubscriptionPayResponse {
  success: boolean
  message?: string
  data?: {
    // Stripe-style hosted checkout link.
    pay_link?: string
    // Waffo Pancake / Creem hosted checkout URL.
    checkout_url?: string
    // Pancake-only: order metadata + self-service buyer session token,
    // surfaced for future flows (refund / cancel from new-api's own UI).
    session_id?: string
    expires_at?: number | string
    order_id?: string
    token?: string
    token_expires_at?: number | string
  }
  url?: string
}

export interface CreateUserSubscriptionRequest {
  plan_id: number
  multiplier?: number
  auto_renew?: boolean
}

export interface ResetUserSubscriptionsRequest {
  plan_id: number
  advance_reset_time: boolean
  reset_total: boolean
  reset_five_hour: boolean
  reset_weekly: boolean
}

export interface ResetPlanSubscriptionsRequest {
  advance_reset_time: boolean
  reset_total: boolean
  reset_five_hour: boolean
  reset_weekly: boolean
}

export interface SubscriptionResetResult {
  plan_id: number
  matched_count: number
  reset_count: number
  user_count: number
  advance_reset_time: boolean
}

// ============================================================================
// Self Subscription Data (user-facing)
// ============================================================================

export interface SelfSubscriptionData {
  billing_preference: string
  subscriptions: UserSubscriptionRecord[]
  all_subscriptions: UserSubscriptionRecord[]
}

// ============================================================================
// Dialog Types
// ============================================================================

export type SubscriptionsDialogType =
  | 'create'
  | 'update'
  | 'toggle-status'
  | 'delete'
  | 'reset-subscriptions'
