import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
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

test('a scoped plan displays its binding and can switch back to legacy settings', async () => {
  const user = userEvent.setup()
  const network = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/group' ? ['default', 'vip'] : [],
    },
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  try {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 1,
      title: 'Pro',
      subscription_group: 'vip',
    })
    render(
      <QueryClientProvider client={client}>
        <SubscriptionsProvider>
          <SubscriptionsMutateDrawer
            open
            currentRow={{ plan }}
            onOpenChange={vi.fn()}
          />
        </SubscriptionsProvider>
      </QueryClientProvider>
    )
    const group = await screen.findByRole('combobox', {
      name: 'Subscription group',
    })
    expect(group).toHaveTextContent('vip')
    expect(
      screen.queryByRole('combobox', { name: 'Upgrade Group' })
    ).not.toBeInTheDocument()
    await user.click(group)
    await user.click(
      await screen.findByRole('option', { name: 'Legacy subscription' })
    )
    expect(
      await screen.findByRole('combobox', { name: 'Upgrade Group' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('switch', { name: 'Enable wallet-only groups' })
    ).toBeInTheDocument()
  } finally {
    client.clear()
    network.mockRestore()
  }
})
