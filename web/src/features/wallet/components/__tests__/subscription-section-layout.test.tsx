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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, describe, expect, test, vi } from 'vitest'

import {
  EXTRA_CREDITS_PENDING_STORAGE_KEY,
  resetExtraCreditsPageForTests,
  resolvePendingExtraCreditsPurchase,
} from '@/features/subscriptions/lib/extra-credits-request'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { SubscriptionSection } from '../subscription-section'

const plan = {
  id: 1,
  title: 'Pro',
  price_amount: 10,
  duration_unit: 'month',
  duration_value: 1,
  quota_reset_period: 'never',
  enabled: true,
  sort_order: 0,
  max_purchase_per_user: 0,
  total_amount: 1000,
}

function mockSubscriptionApi(
  plans: unknown[],
  self: {
    subscriptions?: unknown[]
    all_subscriptions?: unknown[]
  } = {}
) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/subscription/plans'
          ? plans
          : {
              billing_preference: 'subscription_first',
              subscriptions: self.subscriptions ?? [],
              all_subscriptions: self.all_subscriptions ?? [],
            },
    },
  }))
}

function renderSection() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <SubscriptionSection
        topupInfo={null}
        recharge={<section aria-label='Recharge'>recharge</section>}
      />
    </QueryClientProvider>
  )
}

afterEach(() => {
  resetExtraCreditsPageForTests()
  window.localStorage.clear()
  window.sessionStorage.clear()
  vi.restoreAllMocks()
})

describe('wallet subscription section layout', () => {
  test.each([
    'success',
    'refresh failure',
    'blocked reads',
    'late first snapshot',
    'mismatched prior result',
  ])(
    'refreshes a still-open purchase or reports its balance refresh failure (%s)',
    async (mode) => {
      const refreshFails = mode === 'refresh failure'
      let releaseFirst!: () => void
      const firstResponse = new Promise<void>((resolve) => {
        releaseFirst = resolve
      })
      let delayNextSelf = false
      window.localStorage.clear()
      useAuthStore.setState((state) => ({
        auth: { ...state.auth, user: { id: 1, username: 'user1', role: 1 } },
      }))
      Object.defineProperty(navigator, 'locks', {
        configurable: true,
        value: {
          request: async (
            _name: string,
            _options: unknown,
            callback: (lock: object) => Promise<unknown>
          ) => callback({}),
        },
      })
      const sub = {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'active',
        start_time: 0,
        end_time: Math.floor(Date.now() / 1000) + 3600,
        amount_total: 1000,
        amount_used: 0,
        extra_credits_total: 0,
        extra_credits_used: 0,
      }
      let purchased = false
      const refreshWallet = vi.fn()
      const notify = vi.spyOn(toast, 'error')
      vi.spyOn(api, 'get').mockImplementation(async (url) => {
        if (refreshFails && purchased && url !== '/api/subscription/plans') {
          throw new Error('subscription refresh unavailable')
        }
        const response = {
          data: {
            success: true,
            data:
              url === '/api/subscription/plans'
                ? [
                    {
                      plan: {
                        ...plan,
                        allow_extra_credits: true,
                        extra_credits_rate: 2,
                      },
                    },
                  ]
                : {
                    billing_preference: 'subscription_first',
                    subscriptions: [
                      {
                        subscription: {
                          ...sub,
                          extra_credits_total: purchased ? 2_000_000 : 0,
                        },
                      },
                    ],
                    all_subscriptions: [
                      {
                        subscription: {
                          ...sub,
                          extra_credits_total: purchased ? 2_000_000 : 0,
                        },
                      },
                    ],
                  },
          },
        }
        if (delayNextSelf && url === '/api/subscription/self') {
          delayNextSelf = false
          await firstResponse
        }
        return response
      })
      vi.spyOn(api, 'post').mockImplementation(async (_url, body) => {
        const request = body as { request_id: string; wallet_quota: number }
        purchased = true
        if (mode === 'blocked reads') {
          vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
            throw new Error('storage read blocked')
          })
        }
        return {
          data: {
            success: true,
            data: {
              id: 1,
              user_id: 1,
              subscription_id: 9,
              request_id: request.request_id,
              wallet_quota: request.wallet_quota,
              credits: 2_000_000,
              rate: 2,
              created_at: 0,
            },
          },
        }
      })
      vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
        throw new Error('storage blocked')
      })
      if (mode === 'mismatched prior result') {
        const body = {
          request_id: 'prior-unknown-order',
          wallet_quota: 1_000_000,
          expected_rate: 2,
        }
        localStorage.setItem(
          EXTRA_CREDITS_PENDING_STORAGE_KEY,
          JSON.stringify({ '1:9': body })
        )
        localStorage.setItem(
          `${EXTRA_CREDITS_PENDING_STORAGE_KEY}:resolved`,
          JSON.stringify({
            [`1:9:${body.request_id}`]: {
              ...body,
              expected_rate: 3,
              status: 'rejected',
              resolved_at: 1,
            },
          })
        )
      }
      render(
        <QueryClientProvider client={new QueryClient()}>
          <SubscriptionSection
            topupInfo={null}
            userQuota={10_000_000}
            onPurchaseSuccess={refreshWallet}
          />
        </QueryClientProvider>
      )
      const user = userEvent.setup()
      await user.click(
        await screen.findByRole('button', { name: 'Buy Extra Credits' })
      )
      const dialog = await screen.findByRole('dialog')
      if (!refreshFails) {
        delayNextSelf = mode === 'late first snapshot'
        await act(async () => {
          resolvePendingExtraCreditsPurchase(1, 10, {
            request_id: 'external-order-sub-10',
            wallet_quota: 100,
            expected_rate: 2,
          })
        })
        expect(refreshWallet).toHaveBeenCalledTimes(1)
        refreshWallet.mockClear()
      }
      if (mode !== 'mismatched prior result') {
        await user.type(within(dialog).getByRole('spinbutton'), '2')
      }
      await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
      await user.click(
        within(await screen.findByRole('alertdialog')).getByRole('button', {
          name: 'Continue',
        })
      )
      if (refreshFails) {
        await waitFor(() =>
          expect(notify).toHaveBeenCalledWith(
            'Extra credits were purchased, but the balance could not be refreshed. Reload the page to see it.'
          )
        )
      } else {
        await act(async () => releaseFirst())
        await waitFor(() =>
          expect(
            within(dialog).getByText('Remaining extra credits').parentElement
          ).toHaveTextContent('$4')
        )
      }
      expect(screen.getByRole('dialog')).toBeVisible()
      expect(refreshWallet).toHaveBeenCalledTimes(1)
      if (mode === 'blocked reads') {
        expect(
          within(dialog).queryByText(/earlier purchase of this amount/)
        ).not.toBeInTheDocument()
      }
    }
  )

  test('can replay a stored purchase after the subscription row disappears', async () => {
    window.localStorage.clear()
    window.sessionStorage.clear()
    useAuthStore.setState((state) => ({
      auth: { ...state.auth, user: { id: 1, username: 'user1', role: 1 } },
    }))
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: async (
          _name: string,
          _options: unknown,
          callback: (lock: object) => Promise<unknown>
        ) => callback({}),
      },
    })
    const body = {
      request_id: 'removed-subscription-recovery',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    window.localStorage.setItem(
      EXTRA_CREDITS_PENDING_STORAGE_KEY,
      JSON.stringify({ '1:10': {}, '1:9': body })
    )
    mockSubscriptionApi([])
    const post = vi
      .spyOn(api, 'post')
      .mockRejectedValue(new Error('Network Error'))
    renderSection()
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Retry purchase · #9' })
    )
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        '/api/subscription/self/9/extra-credits',
        body
      )
    )
  })

  test('puts recharge beside my subscriptions and the plans in their own row', async () => {
    mockSubscriptionApi([{ plan }])
    renderSection()
    const plansHeading = await screen.findByText('Subscription Plans')
    const recharge = screen.getByRole('region', { name: 'Recharge' })
    const topRow = recharge.parentElement
    expect(topRow).toContainElement(screen.getByText('My Subscriptions'))
    expect(topRow).not.toContainElement(plansHeading)
    expect(screen.getByText('Pro')).toBeInTheDocument()
  })

  test('keeps the recharge card mounted and alone when no plan is sold and the user has no subscription', async () => {
    mockSubscriptionApi([])
    renderSection()
    const recharge = screen.getByRole('region', { name: 'Recharge' })
    await vi.waitFor(() =>
      expect(api.get).toHaveBeenCalledWith('/api/subscription/self')
    )
    await vi.waitFor(() =>
      expect(recharge.parentElement?.children).toHaveLength(1)
    )
    // The card loaded while subscriptions were fetched stays mounted.
    expect(recharge).toBeInTheDocument()
    expect(screen.queryByText('My Subscriptions')).not.toBeInTheDocument()
    expect(screen.queryByText('Subscription Plans')).not.toBeInTheDocument()
  })
})

describe('wallet billing preference with only expired extra credits', () => {
  const now = Math.floor(Date.now() / 1000)
  const expiredWithExtras = {
    subscription: {
      id: 9,
      user_id: 1,
      plan_id: 1,
      status: 'expired',
      start_time: 0,
      end_time: now - 86400,
      amount_total: 1000,
      amount_used: 1000,
      extra_credits_total: 2_000_000,
      extra_credits_used: 500_000,
    },
  }

  test('keeps subscription-first selectable and saves it while unspent extras remain after expiry', async () => {
    // No active subscription; only an expired one that still holds paid extras.
    mockSubscriptionApi([{ plan }], { all_subscriptions: [expiredWithExtras] })
    const put = vi.spyOn(api, 'put').mockResolvedValue({
      data: {
        success: true,
        data: { billing_preference: 'subscription_only' },
      },
    })
    renderSection()
    const user = userEvent.setup()
    await screen.findByText('My Subscriptions')

    await user.click(screen.getByRole('combobox'))
    const option = await screen.findByRole('option', {
      name: 'Subscription Only',
    })
    expect(option).not.toHaveAttribute('aria-disabled', 'true')
    await user.click(option)

    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/subscription/self/preference', {
        billing_preference: 'subscription_only',
      })
    )
    // The extras are spendable, so the wallet-fallback warning is not shown.
    expect(
      screen.queryByText(/but no active subscription/)
    ).not.toBeInTheDocument()
  })

  test('disables the subscription preferences once the expired extras are fully spent', async () => {
    mockSubscriptionApi([{ plan }], {
      all_subscriptions: [
        {
          subscription: {
            ...expiredWithExtras.subscription,
            extra_credits_used: 2_000_000,
          },
        },
      ],
    })
    renderSection()
    const user = userEvent.setup()
    await screen.findByText('My Subscriptions')

    await user.click(screen.getByRole('combobox'))
    expect(
      await screen.findByRole('option', { name: /Subscription Only/ })
    ).toHaveAttribute('aria-disabled', 'true')
  })
})
