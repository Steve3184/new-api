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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  PLAN_FORM_DEFAULTS,
  formValuesToPlanPayload,
} from '../../lib/plan-form'
import { subscriptionPlanSchema } from '../../types'
import { SubscriptionsMutateDrawer } from '../subscriptions-mutate-drawer'
import { SubscriptionsProvider } from '../subscriptions-provider'

function renderDrawer(
  props: React.ComponentProps<typeof SubscriptionsMutateDrawer>
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <SubscriptionsProvider>
        <SubscriptionsMutateDrawer {...props} />
      </SubscriptionsProvider>
    </QueryClientProvider>
  )
}

function mockGroups() {
  return vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/group' ? ['default', 'vip'] : [],
    },
  }))
}

test('a new plan keeps extra credits off unless the administrator turns it on', async () => {
  const user = userEvent.setup()
  const network = mockGroups()
  try {
    renderDrawer({ open: true, onOpenChange: vi.fn() })
    const toggle = await screen.findByRole('switch', {
      name: 'Allow buying extra credits',
    })
    // Old plans and new ones both default to disabled.
    expect(toggle).not.toBeChecked()
    // The rate input only appears once the toggle is on, so a disabled plan
    // never sends a rate.
    expect(
      screen.queryByRole('textbox', { name: 'Extra credits exchange rate' })
    ).not.toBeInTheDocument()

    await user.click(toggle)
    const rate = await screen.findByRole('textbox', {
      name: 'Extra credits exchange rate',
    })
    await user.clear(rate)
    await user.type(rate, '2.5')
    expect(toggle).toBeChecked()
    expect(rate).toHaveValue('2.5')
  } finally {
    network.mockRestore()
  }
})

test('editing a legacy plan leaves its disabled extra credits untouched', async () => {
  const network = mockGroups()
  try {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 7,
      title: 'Legacy',
      allow_extra_credits: false,
      extra_credits_rate: 0,
    })
    renderDrawer({ open: true, currentRow: { plan }, onOpenChange: vi.fn() })
    const toggle = await screen.findByRole('switch', {
      name: 'Allow buying extra credits',
    })
    expect(toggle).not.toBeChecked()
    expect(
      screen.queryByRole('textbox', { name: 'Extra credits exchange rate' })
    ).not.toBeInTheDocument()
  } finally {
    network.mockRestore()
  }
})

test('loading a plan with extra credits enabled prefills the rate for editing', async () => {
  const network = mockGroups()
  try {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 8,
      title: 'Pro',
      allow_extra_credits: true,
      extra_credits_rate: 3,
    })
    renderDrawer({ open: true, currentRow: { plan }, onOpenChange: vi.fn() })
    expect(
      await screen.findByRole('switch', { name: 'Allow buying extra credits' })
    ).toBeChecked()
    expect(
      await screen.findByRole('textbox', {
        name: 'Extra credits exchange rate',
      })
    ).toHaveValue('3')
  } finally {
    network.mockRestore()
  }
})

test('turning extra credits off keeps the saved rate so switching it back on restores it', async () => {
  const user = userEvent.setup()
  const network = mockGroups()
  try {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 9,
      title: 'Pro',
      allow_extra_credits: true,
      extra_credits_rate: 4,
    })
    const post = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true, data: {} } })
    renderDrawer({ open: true, currentRow: { plan }, onOpenChange: vi.fn() })
    const toggle = await screen.findByRole('switch', {
      name: 'Allow buying extra credits',
    })
    await user.click(toggle)
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
    const body = post.mock.calls[0][1] as {
      plan: { allow_extra_credits: boolean; extra_credits_rate: number }
    }
    expect(body.plan).toMatchObject({
      allow_extra_credits: false,
      extra_credits_rate: 4,
    })
    post.mockRestore()
  } finally {
    network.mockRestore()
  }
})

test('typing a rate keeps the exact text so six-decimal values are not rounded', async () => {
  const user = userEvent.setup()
  const network = mockGroups()
  try {
    renderDrawer({ open: true, onOpenChange: vi.fn() })
    await user.click(
      await screen.findByRole('switch', { name: 'Allow buying extra credits' })
    )
    const rate = await screen.findByRole('textbox', {
      name: 'Extra credits exchange rate',
    })
    await user.type(rate, '0.000001')
    expect(rate).toHaveValue('0.000001')
  } finally {
    network.mockRestore()
  }
})

test('a rate with more decimals than the column holds is rejected instead of being rounded into range', async () => {
  const user = userEvent.setup()
  const network = mockGroups()
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: {} } })
  try {
    renderDrawer({ open: true, onOpenChange: vi.fn() })
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Plan Title'), 'Over Limit')
    await user.click(
      within(dialog).getByRole('switch', { name: 'Allow buying extra credits' })
    )
    // 1000000.00000000001 parses to exactly 1000000 as a float, so only the
    // exact decimal text shows that it exceeds both the ceiling and six decimals.
    await user.type(
      within(dialog).getByRole('textbox', {
        name: 'Extra credits exchange rate',
      }),
      '1000000.00000000001'
    )
    await user.click(
      within(dialog).getByRole('button', { name: 'Save changes' })
    )
    expect(
      await within(dialog).findByText('Invalid extra credits exchange rate')
    ).toBeVisible()
    expect(post).not.toHaveBeenCalled()
  } finally {
    post.mockRestore()
    network.mockRestore()
  }
})

test('saves an enabled rate at the exact typed value', async () => {
  const user = userEvent.setup()
  const network = mockGroups()
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: {} } })
  try {
    renderDrawer({ open: true, onOpenChange: vi.fn() })
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Plan Title'), 'Micro')
    await user.click(
      within(dialog).getByRole('switch', { name: 'Allow buying extra credits' })
    )
    await user.type(
      within(dialog).getByRole('textbox', {
        name: 'Extra credits exchange rate',
      }),
      '2.5'
    )
    await user.click(
      within(dialog).getByRole('button', { name: 'Save changes' })
    )
    await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
    const body = post.mock.calls[0][1] as {
      plan: { allow_extra_credits: boolean; extra_credits_rate: number }
    }
    expect(body.plan).toMatchObject({
      allow_extra_credits: true,
      extra_credits_rate: 2.5,
    })
  } finally {
    post.mockRestore()
    network.mockRestore()
  }
})

test('describes the exchange rate in the currency the plan quota uses', async () => {
  const user = userEvent.setup()
  const network = mockGroups()
  try {
    renderDrawer({ open: true, onOpenChange: vi.fn() })
    await user.click(
      await screen.findByRole('switch', { name: 'Allow buying extra credits' })
    )
    const dialog = await screen.findByRole('dialog')
    // The rate is a multiplier on the wallet balance the buyer spends, and
    // that balance is shown in the site currency — not in raw quota units.
    expect(
      within(dialog).getByText(
        /1 USD of wallet balance grants this many USD of extra credits/
      )
    ).toBeInTheDocument()
  } finally {
    network.mockRestore()
  }
})
