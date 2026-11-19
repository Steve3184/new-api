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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  PLAN_FORM_DEFAULTS,
  formValuesToPlanPayload,
} from '../../lib/plan-form'
import { subscriptionPlanSchema, type PlanRecord } from '../../types'
import { SubscriptionsMutateDrawer } from '../subscriptions-mutate-drawer'
import { SubscriptionsProvider } from '../subscriptions-provider'

const AUTO_CREATE_KEY = 'Automatically create API key'

let client: QueryClient

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/group' ? ['default', 'vip'] : [],
    },
  }))
})

afterEach(() => {
  client.clear()
})

function renderDrawer(currentRow?: PlanRecord) {
  return render(
    <QueryClientProvider client={client}>
      <SubscriptionsProvider>
        <SubscriptionsMutateDrawer
          open
          currentRow={currentRow}
          onOpenChange={vi.fn()}
        />
      </SubscriptionsProvider>
    </QueryClientProvider>
  )
}

function planRecord(overrides: Record<string, unknown>): PlanRecord {
  return {
    plan: subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 7,
      title: 'Pro',
      ...overrides,
    }),
  }
}

describe('plan automatic API key setting', () => {
  test('a bound plan shows its enabled switch and explains automatic subscription-group activation', async () => {
    renderDrawer(
      planRecord({ subscription_group: 'vip', auto_create_key: true })
    )

    const toggle = await screen.findByRole('switch', { name: AUTO_CREATE_KEY })
    expect(toggle).toBeChecked()
    expect(toggle).not.toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByText(
        'Binding a plan makes its group a subscription group automatically. After a purchase, a key for that group is created only if the subscriber has no usable key covering it; the key limit still applies.'
      )
    ).toBeVisible()
  })

  test('a new unbound plan shows the switch off and disabled with binding guidance', async () => {
    renderDrawer()

    const toggle = await screen.findByRole('switch', { name: AUTO_CREATE_KEY })
    expect(toggle).not.toBeChecked()
    expect(toggle).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByText('Bind a subscription group to enable this option.')
    ).toBeVisible()
  })

  test('the group binding picker keeps its legacy and group options', async () => {
    const user = userEvent.setup()
    renderDrawer(planRecord({ subscription_group: 'vip' }))

    const group = await screen.findByRole('combobox', {
      name: 'Subscription group',
    })
    expect(group).toHaveTextContent('vip')
    expect(
      screen.getByText(
        'Leave unbound to keep legacy billing. A bound group becomes a subscription group automatically and is covered only by plans bound to it.'
      )
    ).toBeVisible()
    await user.click(group)
    expect(
      await screen.findByRole('option', { name: 'Legacy subscription' })
    ).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'default' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'vip' })).toBeInTheDocument()
  })

  test('unbinding a plan that created keys disables the switch and saves false', async () => {
    const user = userEvent.setup()
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true, data: {} } })
    renderDrawer(
      planRecord({ subscription_group: 'vip', auto_create_key: true })
    )

    await user.click(
      await screen.findByRole('combobox', { name: 'Subscription group' })
    )
    await user.click(
      await screen.findByRole('option', { name: 'Legacy subscription' })
    )
    const toggle = screen.getByRole('switch', { name: AUTO_CREATE_KEY })
    expect(toggle).not.toBeChecked()
    expect(toggle).toHaveAttribute('aria-disabled', 'true')

    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(put).toHaveBeenCalled())
    expect(put.mock.calls[0][1]).toMatchObject({
      plan: { subscription_group: '', auto_create_key: false },
    })
  })
})
