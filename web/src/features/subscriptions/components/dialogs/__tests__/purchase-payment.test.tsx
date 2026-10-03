import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  paySubscriptionBalance,
  paySubscriptionEpay,
  quoteSubscriptionPayment,
} from '../../../api'
import { subscriptionPlanSchema } from '../../../types'
import { SubscriptionPurchaseDialog } from '../subscription-purchase-dialog'

vi.mock('../../../api', () => ({
  quoteSubscriptionPayment: vi.fn(),
  paySubscriptionBalance: vi.fn(),
  paySubscriptionEpay: vi.fn(),
  paySubscriptionStripe: vi.fn(),
  paySubscriptionCreem: vi.fn(),
  paySubscriptionWaffoPancake: vi.fn(),
  paySubscriptionNowPayments: vi.fn(),
}))

const plan = subscriptionPlanSchema.parse({
  id: 1,
  title: 'Monthly Plan',
  price_amount: 10,
  duration_unit: 'month',
  duration_value: 1,
  quota_reset_period: 'never',
  enabled: true,
  sort_order: 0,
  max_purchase_per_user: 0,
  max_active_per_user: 1,
  total_amount: 0,
})

function renderPurchase(
  overrides: Partial<
    React.ComponentProps<typeof SubscriptionPurchaseDialog>
  > = {}
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <SubscriptionPurchaseDialog
        open
        onOpenChange={vi.fn()}
        plan={{ plan }}
        userQuota={999999999}
        enableOnlineTopUp
        enableStripe
        enableWaffoPancake
        epayMethods={[
          { type: 'alipay', name: 'Alipay', gateway: 'custom', fee: 1 },
        ]}
        {...overrides}
      />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.mocked(quoteSubscriptionPayment).mockResolvedValue({
    success: true,
    data: { amount: 11, currency: 'USD' },
  })
  vi.mocked(paySubscriptionBalance).mockResolvedValue({ success: true })
  vi.mocked(paySubscriptionEpay).mockResolvedValue({
    success: false,
    message: 'Declined',
  })
})

describe('subscription payment selection', () => {
  test('puts balance first, selects cards without paying, and submits the selected gateway from one footer button', async () => {
    renderPurchase()
    const dialog = await screen.findByRole('dialog')
    const firstCard = within(dialog)
      .getAllByRole('button')
      .find((button) => button.hasAttribute('aria-pressed'))
    expect(firstCard).toHaveAccessibleName('Balance')
    expect(firstCard).toHaveAttribute('aria-pressed', 'true')
    expect(within(dialog).queryByRole('combobox')).not.toBeInTheDocument()
    expect(within(dialog).getAllByRole('button', { name: 'Pay' })).toHaveLength(
      1
    )
    fireEvent.click(within(dialog).getByRole('button', { name: 'Alipay' }))
    await waitFor(() =>
      expect(within(dialog).getByText('USD 11')).toBeInTheDocument()
    )
    expect(paySubscriptionEpay).not.toHaveBeenCalled()
    expect(quoteSubscriptionPayment).toHaveBeenCalledWith({
      plan_id: 1,
      payment_method: 'alipay',
      epay_gateway: 'custom',
    })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Pay' }))
    await waitFor(() =>
      expect(paySubscriptionEpay).toHaveBeenCalledWith({
        plan_id: 1,
        payment_method: 'alipay',
        epay_gateway: 'custom',
      })
    )
    expect(paySubscriptionBalance).not.toHaveBeenCalled()
  })

  test('shows product-capable payment cards without plan product IDs and supports keyboard selection', async () => {
    renderPurchase()
    const pancake = await screen.findByRole('button', { name: 'Waffo Pancake' })
    pancake.focus()
    await userEvent.keyboard('{Enter}')
    expect(pancake).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() =>
      expect(quoteSubscriptionPayment).toHaveBeenCalledWith({
        plan_id: 1,
        payment_method: 'waffo_pancake',
        epay_gateway: undefined,
      })
    )
  })

  test('disables payment while quoting and after a quote failure', async () => {
    let rejectQuote!: (error: Error) => void
    vi.mocked(quoteSubscriptionPayment).mockImplementation(
      () =>
        new Promise((_, reject) => {
          rejectQuote = reject
        })
    )
    renderPurchase()
    fireEvent.click(await screen.findByRole('button', { name: 'Stripe' }))
    const pay = screen.getByRole('button', { name: 'Pay' })
    expect(pay).toBeDisabled()
    await waitFor(() => expect(rejectQuote).toBeDefined())
    rejectQuote(new Error('Gateway unavailable'))
    await screen.findByText(
      'Unable to calculate payment amount. Please choose another payment method.'
    )
    expect(pay).toBeDisabled()
  })

  test('blocks a second active subscription while allowing expired history', async () => {
    const view = renderPurchase({
      activePurchaseLimit: 1,
      activePurchaseCount: 1,
    })
    expect(await screen.findByRole('button', { name: 'Pay' })).toBeDisabled()
    expect(
      screen.getByText(/Active subscription limit reached/)
    ).toBeInTheDocument()
    view.unmount()
    renderPurchase({
      activePurchaseLimit: 1,
      activePurchaseCount: 0,
      purchaseCount: 4,
    })
    expect(await screen.findByRole('button', { name: 'Pay' })).toBeEnabled()
  })

  test('requires sufficient balance but permits another payment method', async () => {
    renderPurchase({ userQuota: 0 })
    const pay = await screen.findByRole('button', { name: 'Pay' })
    expect(pay).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Alipay' }))
    await waitFor(() => expect(pay).toBeEnabled())
  })
})
