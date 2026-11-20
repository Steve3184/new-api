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
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { purchaseSubscriptionExtraCredits } from '../../../api'
import {
  clearPendingExtraCreditsPurchase,
  EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
  resetExtraCreditsPageForTests,
  resolvePendingExtraCreditsPurchase,
  readPendingExtraCreditsPurchase,
  savePendingExtraCreditsPurchase,
} from '../../../lib/extra-credits-request'
import { subscriptionPlanSchema } from '../../../types'
import { SubscriptionExtraCreditsDialog } from '../subscription-extra-credits-dialog'

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('../../../api', () => ({
  purchaseSubscriptionExtraCredits: vi.fn(),
}))

const plan = subscriptionPlanSchema.parse({
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
  allow_extra_credits: true,
  extra_credits_rate: 2,
})

const subscription = {
  id: 9,
  user_id: 1,
  plan_id: 1,
  status: 'active',
  start_time: 0,
  end_time: Math.floor(Date.now() / 1000) + 86400,
  amount_total: 1000,
  amount_used: 0,
  extra_credits_total: 0,
  extra_credits_used: 0,
}

function renderDialog(
  overrides: Partial<
    React.ComponentProps<typeof SubscriptionExtraCreditsDialog>
  > = {}
) {
  const onOpenChange = vi.fn()
  const onPurchased = vi.fn()
  render(
    <SubscriptionExtraCreditsDialog
      open
      onOpenChange={onOpenChange}
      subscription={subscription}
      plan={plan}
      userQuota={10_000_000}
      onPurchased={onPurchased}
      {...overrides}
    />
  )
  return { onOpenChange, onPurchased }
}

function dialogElement(
  overrides: Partial<
    React.ComponentProps<typeof SubscriptionExtraCreditsDialog>
  > = {}
) {
  return (
    <SubscriptionExtraCreditsDialog
      open
      onOpenChange={vi.fn()}
      subscription={subscription}
      plan={plan}
      userQuota={10_000_000}
      onPurchased={vi.fn()}
      {...overrides}
    />
  )
}

function renderDialogUnmountable() {
  return render(dialogElement())
}

function setUser(id: number) {
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id, username: `user${id}`, role: 1 },
    },
  }))
}

async function buy(amount?: string) {
  const user = userEvent.setup()
  const dialog = await screen.findByRole('dialog')
  if (amount !== undefined) {
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'Amount (USD)' }),
      amount
    )
  }
  await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
  await user.click(
    within(await screen.findByRole('alertdialog')).getByRole('button', {
      name: 'Continue',
    })
  )
}

const callBody = (index: number) =>
  vi.mocked(purchaseSubscriptionExtraCredits).mock.calls[index][1]

beforeEach(() => {
  resetExtraCreditsPageForTests()
  const held = new Map<string, Promise<void>>()
  Object.defineProperty(navigator, 'locks', {
    configurable: true,
    value: {
      request: async (
        name: string,
        options: { ifAvailable?: boolean },
        callback: (lock: object | null) => Promise<unknown>
      ) => {
        if (options.ifAvailable && held.has(name)) return callback(null)
        while (held.has(name)) await held.get(name)
        let release!: () => void
        held.set(
          name,
          new Promise<void>((resolve) => {
            release = resolve
          })
        )
        try {
          return await callback({ name })
        } finally {
          held.delete(name)
          release()
        }
      },
    },
  })
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...state.config.currency,
        quotaDisplayType: 'USD',
        quotaPerUnit: 500000,
        usdExchangeRate: 1,
      },
    },
  }))
  window.localStorage.clear()
  window.sessionStorage.clear()
  setUser(1)
  vi.mocked(toast.success).mockReset()
  vi.mocked(toast.error).mockReset()
  vi.mocked(purchaseSubscriptionExtraCredits).mockReset()
  vi.mocked(purchaseSubscriptionExtraCredits).mockImplementation(
    async (subscriptionId, body) => ({
      success: true,
      data: {
        id: 1,
        user_id: useAuthStore.getState().auth.user?.id || 0,
        subscription_id: subscriptionId,
        request_id: body.request_id,
        wallet_quota: body.wallet_quota,
        credits: 2_000_000,
        rate: body.expected_rate,
        created_at: 0,
      },
    })
  )
})

describe('buy extra credits dialog', () => {
  test.each([false, true])(
    'rechecks the wallet balance at final confirmation except for an exact retry (retry=%s)',
    async (retry) => {
      const body = {
        request_id: 'unconfirmed-retry-12345',
        wallet_quota: 1_000_000,
        expected_rate: 2,
      }
      if (retry) expect(savePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
      const view = render(dialogElement())
      const user = userEvent.setup()
      const dialog = await screen.findByRole('dialog')
      if (!retry) await user.type(within(dialog).getByRole('spinbutton'), '2')
      await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
      view.rerender(dialogElement({ userQuota: 0 }))
      const confirm = within(await screen.findByRole('alertdialog')).getByRole(
        'button',
        { name: 'Continue' }
      )
      await user.click(confirm)
      if (retry) expect(callBody(0)).toEqual(body)
      else {
        expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
        expect(confirm).toBeDisabled()
      }
    }
  )

  test.each([false, true])(
    'a failed session mirror write keeps only purchases that could have been sent (retry=%s)',
    async (retry) => {
      const body = {
        request_id: 'already-sent-1234567',
        wallet_quota: 1_000_000,
        expected_rate: 2,
      }
      expect(savePendingExtraCreditsPurchase(1, 10, body)).toBe(true)
      if (retry) expect(savePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
      const set = Storage.prototype.setItem
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (
        this: Storage,
        key,
        value
      ) {
        if (this === sessionStorage) throw new Error('session storage full')
        set.call(this, key, value)
      })
      renderDialog()
      await buy(retry ? undefined : '2')
      expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
      expect(readPendingExtraCreditsPurchase(1, 10)).toEqual(body)
      const input = within(screen.getByRole('dialog')).getByRole('spinbutton')
      if (retry) {
        expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(body)
        expect(input).toBeDisabled()
      } else {
        expect(readPendingExtraCreditsPurchase(1, 9)).toBeNull()
        expect(input).toBeEnabled()
      }
    }
  )

  test('requires confirmation again when another tab replaces the pending purchase', async () => {
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByRole('spinbutton'), '2')
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    const replacement = {
      request_id: 'other-tab-purchase-123456',
      wallet_quota: 10_000_000,
      expected_rate: 2,
    }
    // Another tab leaves an uncertain $20 purchase while this tab confirms $2.
    expect(savePendingExtraCreditsPurchase(1, 9, replacement)).toBe(true)
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
    expect(
      within(screen.getByRole('dialog')).getByRole('spinbutton')
    ).toHaveValue(20)
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    await buy()
    expect(callBody(0)).toEqual(replacement)
  })

  test('persists a cleared pending purchase again before replaying its confirmed body', async () => {
    const body = {
      request_id: 'stale-tab-purchase-123456',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    expect(savePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
    renderDialog()
    const user = userEvent.setup()
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    expect(clearPendingExtraCreditsPurchase(1, 9, body.request_id)).toBe(true)
    let savedAtSend: unknown
    vi.mocked(purchaseSubscriptionExtraCredits).mockImplementationOnce(
      async () => {
        savedAtSend = readPendingExtraCreditsPurchase(1, 9)
        throw new Error('Network Error')
      }
    )
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    expect(savedAtSend).toEqual(body)
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(body)
  })

  test('releases the purchase lock before waiting for balance refresh', async () => {
    let finishRefresh!: () => void
    const onPurchased = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finishRefresh = resolve
        })
    )
    const first = render(dialogElement({ onPurchased }))
    await buy('2')
    await waitFor(() => expect(onPurchased).toHaveBeenCalledTimes(1))
    first.unmount()
    renderDialog({ subscription: { ...subscription, id: 10 } })
    try {
      await buy('2')
      await waitFor(() =>
        expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
      )
    } finally {
      finishRefresh()
    }
  })

  test('explains unsupported browsers and disables purchase before confirmation', async () => {
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: undefined,
    })
    renderDialog()
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByRole('spinbutton'), '2')
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    expect(
      within(dialog).getByText(
        'Purchases are unavailable because this browser cannot coordinate tabs. Keep any pending purchase in this browser until its status can be checked.'
      )
    ).toBeVisible()
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
  })

  test('keeps the dialog usable when reading browser lock capability throws', async () => {
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      get() {
        throw new DOMException('Blocked', 'SecurityError')
      },
    })
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    expect(
      within(dialog).getByText(
        'Purchases are unavailable because this browser cannot coordinate tabs. Keep any pending purchase in this browser until its status can be checked.'
      )
    ).toBeVisible()
  })

  test('replays the first submitted body if storage disappears before its network response', async () => {
    let fail!: (error: Error) => void
    vi.mocked(purchaseSubscriptionExtraCredits).mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          fail = reject
        })
    )
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    const original = callBody(0)
    act(() => {
      window.localStorage.clear()
      window.sessionStorage.clear()
      fail(new Error('Network Error'))
    })
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    await buy()
    expect(callBody(1)).toEqual(original)
  })

  test('retains the original retry when another tab clears storage without a receipt', async () => {
    const body = {
      request_id: 'unknown-cleared-purchase-123456',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    expect(savePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    )
    act(() => {
      window.localStorage.clear()
      window.dispatchEvent(new StorageEvent('storage', { key: null }))
    })
    await navigator.locks.request(
      'extra-credits-pending-store',
      {},
      async () => {}
    )
    expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    expect(
      within(dialog).queryByText(
        'The earlier purchase was resolved in another tab. Enter an amount only if you want to make a new purchase.'
      )
    ).not.toBeInTheDocument()
    await buy()
    expect(callBody(0)).toEqual(body)
  })

  test('disables the purchase if the final lock request is rejected after reading succeeded', async () => {
    renderDialog()
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByRole('spinbutton'), '2')
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    vi.spyOn(navigator.locks, 'request').mockRejectedValueOnce(
      new DOMException('Blocked', 'SecurityError')
    )
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    await waitFor(() =>
      expect(
        within(dialog).getByText(
          'Purchases are unavailable because this browser cannot coordinate tabs. Keep any pending purchase in this browser until its status can be checked.'
        )
      ).toBeVisible()
    )
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
  })

  test('refuses a stale subscription belonging to another account', async () => {
    setUser(2)
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    const user = userEvent.setup()
    if (within(dialog).queryByRole('spinbutton')) {
      await user.type(within(dialog).getByRole('spinbutton'), '2')
    }
    await navigator.locks.request(
      'extra-credits-pending-store',
      {},
      async () => {}
    )
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
  })

  test('refreshes pending state after another tab completes its purchase', async () => {
    const body = {
      request_id: 'completed-other-tab-123456',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    expect(savePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    expect(resolvePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
    act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'new-api:subscription-extra-credits-pending',
        })
      )
    )
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toBeEnabled()
    )
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
  })

  test('disables a fresh purchase after the authoritative subscription list removes it', async () => {
    const view = render(dialogElement())
    const user = userEvent.setup()
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByRole('spinbutton'), '2')
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeEnabled()
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    view.rerender(dialogElement({ newPurchaseAvailable: false }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
  })

  test('keeps the retry body when a storage event arrives during a temporary read error', async () => {
    const body = {
      request_id: 'temporary-read-error-123456',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    savePendingExtraCreditsPurchase(1, 9, body)
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    )
    const read = vi
      .spyOn(Storage.prototype, 'getItem')
      .mockImplementation(() => {
        throw new Error('temporarily blocked')
      })
    act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'new-api:subscription-extra-credits-pending',
        })
      )
    )
    await navigator.locks.request(
      'extra-credits-pending-store',
      {},
      async () => {}
    )
    expect(
      within(dialog).queryByText(
        'The earlier purchase was resolved in another tab. Enter an amount only if you want to make a new purchase.'
      )
    ).not.toBeInTheDocument()
    expect(within(dialog).getByRole('spinbutton')).toHaveValue(2)
    read.mockRestore()
    act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'new-api:subscription-extra-credits-pending',
        })
      )
    )
    await waitFor(() =>
      expect(
        within(dialog).getByRole('button', { name: 'Continue' })
      ).toBeEnabled()
    )
    await buy()
    expect(callBody(0)).toEqual(body)
  })

  test('refreshes a new purchase completed in another tab without a cached pending body', async () => {
    const { onPurchased } = renderDialog()
    const dialog = await screen.findByRole('dialog')
    await navigator.locks.request(
      'extra-credits-pending-store',
      {},
      async () => {}
    )
    const body = {
      request_id: 'unseen-purchase-completed-1234',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    act(() => {
      savePendingExtraCreditsPurchase(1, 9, body)
      resolvePendingExtraCreditsPurchase(1, 9, body)
    })
    await waitFor(() => expect(onPurchased).toHaveBeenCalledTimes(1))
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
  })

  test.each([false, true])(
    'observes a purchase followed by a refusal in one locked read (initial=%s)',
    async (initial) => {
      const onPurchased = vi.fn()
      if (!initial) renderDialog({ onPurchased })
      let release!: () => void
      const gate = new Promise<void>((resolve) => {
        release = resolve
      })
      const write = navigator.locks.request(
        'extra-credits-pending-store',
        {},
        async () => {
          await gate
          const records = ['purchased', 'rejected'].map((status, i) => {
            const request_id = `other-tab-completed-${i}`
            return [
              `1:9:${request_id}`,
              {
                request_id,
                wallet_quota: 100,
                expected_rate: 2,
                status,
                resolved_at: i,
              },
            ]
          })
          window.localStorage.setItem(
            EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
            JSON.stringify(Object.fromEntries(records))
          )
          window.dispatchEvent(
            new StorageEvent('storage', {
              key: EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
            })
          )
        }
      )
      if (initial) renderDialog({ onPurchased })
      await act(async () => {
        release()
        await write
      })
      expect(onPurchased).toHaveBeenCalledTimes(1)
    }
  )

  test('does not announce purchased balance refresh after another tab definitively rejects an order', async () => {
    const body = {
      request_id: 'rejected-other-tab-123456',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    savePendingExtraCreditsPurchase(1, 9, body)
    const { onPurchased } = renderDialog()
    const dialog = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    )
    act(() => {
      resolvePendingExtraCreditsPurchase(1, 9, body, 'rejected')
    })
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toHaveValue(null)
    )
    expect(onPurchased).not.toHaveBeenCalled()
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
  })

  test('switching away after shared storage disappears still recovers the original body', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    const view = renderDialogUnmountable()
    await buy('2')
    const original = callBody(0)
    act(() => window.localStorage.clear())
    view.rerender(dialogElement({ subscription: { ...subscription, id: 10 } }))
    view.rerender(dialogElement())
    await buy()
    expect(callBody(1)).toEqual(original)
  })

  test('a queued refresh before completion still clears the old intent and refreshes balance', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    const { onPurchased } = renderDialog()
    await buy('2')
    const body = callBody(0)
    const dialog = screen.getByRole('dialog')
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    )
    let release!: () => void
    let entered!: () => void
    const started = new Promise<void>((resolve) => {
      entered = resolve
    })
    const other = navigator.locks.request(
      'extra-credits-pending-store',
      {},
      async () => {
        entered()
        await new Promise<void>((resolve) => {
          release = resolve
        })
        expect(resolvePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
      }
    )
    await started
    const user = userEvent.setup()
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    release()
    await other
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toHaveValue(null)
    )
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    await waitFor(() => expect(onPurchased).toHaveBeenCalledTimes(1))
    expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
  })

  test('clears the old typed amount when another tab settles the uncertain purchase', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    const body = callBody(0)
    const dialog = screen.getByRole('dialog')
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toBeDisabled()
    )
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(body)
    // A different browser document emits storage, without the local event.
    expect(resolvePendingExtraCreditsPurchase(1, 9, body)).toBe(true)
    act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'new-api:subscription-extra-credits-pending',
        })
      )
    )
    await waitFor(() =>
      expect(within(dialog).getByRole('spinbutton')).toHaveValue(null)
    )
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    expect(
      within(dialog).getByText(
        'The earlier purchase was resolved in another tab. Enter an amount only if you want to make a new purchase.'
      )
    ).toBeVisible()
  })

  test('explains when the browser refuses an actual purchase lock request', async () => {
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: vi
          .fn()
          .mockRejectedValue(new DOMException('Blocked', 'SecurityError')),
      },
    })
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(
        within(dialog).getByText(
          'Purchases are unavailable because this browser cannot coordinate tabs. Keep any pending purchase in this browser until its status can be checked.'
        )
      ).toBeVisible()
    )
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
  })

  test('does not submit when pending storage cannot be written', async () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage full')
    })
    renderDialog()
    await buy('2')
    expect(purchaseSubscriptionExtraCredits).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalled()
  })

  test('never resends a confirmed purchase when persistent cleanup fails', async () => {
    const remove = vi
      .spyOn(Storage.prototype, 'removeItem')
      .mockImplementation(() => {
        throw new Error('storage blocked')
      })
    const { onOpenChange, onPurchased } = renderDialog()
    await buy('2')
    await waitFor(() => expect(onPurchased).toHaveBeenCalledTimes(1))
    expect(toast.success).toHaveBeenCalledWith('Extra credits purchased')
    expect(toast.error).not.toHaveBeenCalledWith('Request failed')
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(readPendingExtraCreditsPurchase(1, 9)).not.toBeNull()
    remove.mockRestore()
    expect(
      within(screen.getByRole('dialog')).getByRole('button', {
        name: 'Continue',
      })
    ).toBeDisabled()
    expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
  })

  test('retains the pending body after an indeterminate response', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockResolvedValueOnce(
      '<html>gateway error</html>' as never
    )
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    await buy()
    expect(callBody(1)).toEqual(callBody(0))
  })

  test('allows replay after the plan stops selling extra credits', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    const view = renderDialogUnmountable()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    view.rerender(
      dialogElement({ plan: { ...plan, allow_extra_credits: false } })
    )
    await buy()
    expect(callBody(1)).toEqual(callBody(0))
  })
  test('states the exchange rate in the buyer currency on both sides', async () => {
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    // Rate 2 read as raw quota would be meaningless to a buyer whose wallet
    // and credits are both shown in the site currency.
    expect(
      within(dialog).getByText('1 USD spent = 2 USD extra credits')
    ).toBeInTheDocument()
  })

  test('previews the credits granted by the configured rate before charging', async () => {
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'Amount (USD)' }),
      '2'
    )
    // $2 * 500000 = 1,000,000 wallet quota; rate 2 -> 2,000,000 credits.
    expect(await within(dialog).findByText('$2')).toBeInTheDocument()
    expect(
      within(dialog).getByText('Extra credits you receive')
    ).toBeInTheDocument()
    expect(within(dialog).getByText('$4')).toBeInTheDocument()
  })

  test('purchases with the exact raw wallet quota and persists one request id across retries', async () => {
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'Amount (USD)' }),
      '2'
    )
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    const confirm = await screen.findByRole('alertdialog')
    await user.click(within(confirm).getByRole('button', { name: 'Continue' }))
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    const [subId, body] = vi.mocked(purchaseSubscriptionExtraCredits).mock
      .calls[0]
    expect(subId).toBe(9)
    expect(body.wallet_quota).toBe(1_000_000)
    expect(typeof body.request_id).toBe('string')
    expect(body.request_id.length).toBeGreaterThanOrEqual(16)
  })

  test('a generic failure without a known message key keeps the pending id for the retry', async () => {
    // A gateway error page can answer 200 with success:false and no
    // message_key. That says nothing about whether the first attempt charged,
    // so the id must survive and the retry must replay it.
    vi.mocked(purchaseSubscriptionExtraCredits).mockResolvedValueOnce({
      success: false,
      message: 'Service temporarily unavailable',
    } as never)
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    expect(toast.error).toHaveBeenCalled()
    const dialog = screen.getByRole('dialog')
    const input = within(dialog).getByRole('spinbutton', {
      name: 'Amount (USD)',
    })
    expect(input).toBeDisabled()
    expect(input).toHaveValue(2)
    expect(
      window.localStorage.getItem('new-api:subscription-extra-credits-pending')
    ).not.toBeNull()

    await buy()
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
    )
    // The retry replays the same id, and only now does it report success.
    expect(callBody(1)).toEqual(callBody(0))
    expect(toast.success).toHaveBeenCalledWith('Extra credits purchased')
  })

  test('a success receipt that does not confirm the promised credits keeps the pending id', async () => {
    // $2 at rate 2 promises 2,000,000 credits; a receipt reporting 0 (or a
    // different user) does not prove the purchase was granted as shown.
    vi.mocked(purchaseSubscriptionExtraCredits).mockImplementationOnce(
      async (subscriptionId, body) => ({
        success: true,
        data: {
          id: 1,
          user_id: 1,
          subscription_id: subscriptionId,
          request_id: body.request_id,
          wallet_quota: body.wallet_quota,
          credits: 0,
          rate: body.expected_rate,
          created_at: 0,
        },
      })
    )
    const { onOpenChange } = renderDialog()
    await buy('2')
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    expect(toast.success).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    await buy()
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
    )
    expect(callBody(1)).toEqual(callBody(0))
  })

  test('a definite rejection clears the pending purchase so the next attempt gets a new id', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockResolvedValueOnce({
      success: false,
      message:
        'Extra credits purchase request conflicts with an earlier purchase',
      message_key:
        'Extra credits purchase request conflicts with an earlier purchase',
    } as never)
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    const dialog = screen.getByRole('dialog')
    expect(
      within(dialog).getByRole('spinbutton', { name: 'Amount (USD)' })
    ).toBeEnabled()
    expect(
      within(dialog).queryByText(/resolved in another tab/)
    ).not.toBeInTheDocument()
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
    )
    expect(callBody(1).request_id).not.toBe(callBody(0).request_id)
  })

  test('sends the displayed plan rate as expected_rate', async () => {
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    expect(callBody(0)).toMatchObject({
      wallet_quota: 1_000_000,
      expected_rate: 2,
    })
  })

  test('after a network failure and reload, the exact stored body is retried and the amount is locked', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    const first = renderDialogUnmountable()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    first.unmount()

    resetExtraCreditsPageForTests()
    renderDialog()
    const dialog = await screen.findByRole('dialog')
    const input = within(dialog).getByRole('spinbutton', {
      name: 'Amount (USD)',
    })
    expect(input).toHaveValue(2)
    expect(input).toBeDisabled()
    await buy()
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
    )
    expect(callBody(1)).toEqual(callBody(0))
  })

  test('switching to another subscription and back reuses the pending body', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    const view = renderDialogUnmountable()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    view.rerender(dialogElement({ subscription: { ...subscription, id: 10 } }))
    const other = within(screen.getByRole('dialog')).getByRole('spinbutton', {
      name: 'Amount (USD)',
    })
    expect(other).toBeEnabled()
    expect(other).toHaveValue(null)

    view.rerender(dialogElement())
    await waitFor(() =>
      expect(
        within(screen.getByRole('dialog')).getByRole('spinbutton', {
          name: 'Amount (USD)',
        })
      ).toBeDisabled()
    )
    await buy()
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
    )
    expect(vi.mocked(purchaseSubscriptionExtraCredits).mock.calls[1][0]).toBe(9)
    expect(callBody(1)).toEqual(callBody(0))
  })

  test('a different signed-in user never inherits another user pending purchase', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockRejectedValueOnce(
      new Error('Network Error')
    )
    const first = renderDialogUnmountable()
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(1)
    )
    first.unmount()

    setUser(2)
    renderDialog({ subscription: { ...subscription, user_id: 2, id: 10 } })
    const input = within(await screen.findByRole('dialog')).getByRole(
      'spinbutton',
      { name: 'Amount (USD)' }
    )
    expect(input).toBeEnabled()
    expect(input).toHaveValue(null)
    await buy('2')
    await waitFor(() =>
      expect(purchaseSubscriptionExtraCredits).toHaveBeenCalledTimes(2)
    )
    expect(callBody(1).request_id).not.toBe(callBody(0).request_id)
  })

  test('a changed exchange rate shows the reload message and clears the pending body', async () => {
    vi.mocked(purchaseSubscriptionExtraCredits).mockResolvedValueOnce({
      success: false,
      message: 'Extra credits exchange rate changed; reload and try again',
      message_key: 'Extra credits exchange rate changed; reload and try again',
    } as never)
    renderDialog()
    await buy('2')
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Extra credits exchange rate changed; reload and try again'
      )
    )
    expect(
      window.localStorage.getItem('new-api:subscription-extra-credits-pending')
    ).toBeNull()
  })

  test('a failing refresh after a confirmed purchase does not report the purchase as failed', async () => {
    const onPurchased = vi.fn().mockRejectedValue(new Error('refresh down'))
    renderDialog({ onPurchased })
    await buy('2')
    await waitFor(() => expect(onPurchased).toHaveBeenCalledTimes(1))
    expect(toast.success).toHaveBeenCalledWith('Extra credits purchased')
    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
    expect(toast.error).not.toHaveBeenCalledWith('Request failed')
    expect(
      window.localStorage.getItem('new-api:subscription-extra-credits-pending')
    ).toBeNull()
  })

  test('refreshes the wallet and subscription only after a successful purchase', async () => {
    const user = userEvent.setup()
    const { onOpenChange, onPurchased } = renderDialog()
    const dialog = await screen.findByRole('dialog')
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'Amount (USD)' }),
      '2'
    )
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Continue',
      })
    )
    await waitFor(() => expect(onPurchased).toHaveBeenCalledTimes(1))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('blocks the purchase when the amount is invalid or exceeds the wallet balance', async () => {
    const user = userEvent.setup()
    renderDialog({ userQuota: 500_000 })
    const dialog = await screen.findByRole('dialog')
    const confirmButton = within(dialog).getByRole('button', {
      name: 'Continue',
    })
    expect(confirmButton).toBeDisabled()
    // $2 costs 1,000,000 quota but only 500,000 is available.
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'Amount (USD)' }),
      '2'
    )
    expect(
      await within(dialog).findByText('Insufficient balance')
    ).toBeVisible()
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
  })

  test('does not purchase when the plan no longer sells extra credits', async () => {
    renderDialog({ plan: { ...plan, allow_extra_credits: false } })
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText('Extra credits are not enabled for this plan')
    ).toBeInTheDocument()
    expect(
      within(dialog).getByRole('button', { name: 'Continue' })
    ).toBeDisabled()
  })
})
