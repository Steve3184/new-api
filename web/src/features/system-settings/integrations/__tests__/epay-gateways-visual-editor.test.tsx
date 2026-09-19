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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { Dialog } from '@/components/dialog'

import { updateSystemOption } from '../../api'
import { parseEpayGateways, serializeEpayGateways } from '../epay-gateways'
import { PaymentMethodDialog } from '../payment-method-dialog'
import { PaymentSettingsSection } from '../payment-settings-section'

vi.mock('../../api', () => ({
  confirmPaymentCompliance: vi.fn(),
  updateSystemOption: vi.fn(),
}))

const epayGateways = JSON.stringify([
  {
    id: 'primary',
    name: 'Primary',
    address: 'https://pay.example.com',
    merchant_id: '10001',
    key: '***',
    enabled: true,
    pay_methods: [
      {
        name: 'Alipay',
        type: 'alipay',
        fee: '0',
        fee_rate: '0',
      },
    ],
  },
])

const paymentDefaults = {
  PayAddress: '',
  EpayId: '',
  EpayKey: '',
  EpayGateways: epayGateways,
  Price: 1,
  MinTopUp: 1,
  CustomCallbackAddress: '',
  PayMethods: '[]',
  AmountOptions: '[]',
  AmountDiscount: '{}',
  RedemptionPurchaseEnabled: false,
  StripeApiSecret: '',
  StripeWebhookSecret: '',
  StripePriceId: '',
  StripeUnitPrice: 1,
  StripeMinTopUp: 1,
  StripePromotionCodesEnabled: false,
  CreemApiKey: '',
  CreemWebhookSecret: '',
  CreemTestMode: false,
  CreemProducts: '[]',
  MoneroEnabled: false,
  MoneroWalletRPCURL: '',
  MoneroWalletRPCUsername: '',
  MoneroWalletRPCPassword: '',
  MoneroNetwork: 'mainnet' as const,
  MoneroConfirmations: 1,
  MoneroMaxSubaddresses: 10000,
  MoneroUSDToCurrencyRate: 0,
  NowPaymentsEnabled: false,
  NowPaymentsAPIKey: '',
  NowPaymentsIPNSecret: '',
  NowPaymentsAPIBaseURL: 'https://api.nowpayments.io',
  NowPaymentsPayCurrencies: 'btc',
  NowPaymentsMinTopUp: 1,
  NowPaymentsUSDToCurrencyRate: 0,
  NowPaymentsPaymentExpirationMins: 60,
  PaymentAnnouncement: '',
}

const waffoDefaults = {
  WaffoEnabled: false,
  WaffoApiKey: '',
  WaffoPrivateKey: '',
  WaffoPublicCert: '',
  WaffoSandboxPublicCert: '',
  WaffoSandboxApiKey: '',
  WaffoSandboxPrivateKey: '',
  WaffoSandbox: false,
  WaffoMerchantId: '',
  WaffoCurrency: 'USD',
  WaffoUnitPrice: 1,
  WaffoMinTopUp: 1,
  WaffoNotifyUrl: '',
  WaffoReturnUrl: '',
  WaffoPayMethods: '[]',
}

const waffoPancakeDefaults = {
  WaffoPancakeMerchantID: '',
  WaffoPancakePrivateKey: '',
  WaffoPancakeReturnURL: '',
  WaffoPancakeUseConfiguredProductPrice: false,
  WaffoPancakeUSDToCurrencyRate: 0,
}

describe('Epay gateway visual editor serialization', () => {
  test('preserves masked secrets and nested payment fees', () => {
    const gateways = parseEpayGateways(
      JSON.stringify([
        {
          id: 'primary',
          name: 'Primary',
          address: 'https://pay.example.com',
          merchant_id: '10001',
          key: '***',
          enabled: true,
          pay_methods: [
            {
              name: 'Alipay',
              type: 'alipay',
              fee: '0.30',
              fee_rate: '3',
            },
          ],
        },
      ])
    )

    expect(gateways).toHaveLength(1)
    expect(gateways[0].key).toBe('***')
    expect(gateways[0].pay_methods[0]).toMatchObject({
      type: 'alipay',
      fee: '0.30',
      fee_rate: '3',
    })
    expect(JSON.parse(serializeEpayGateways(gateways))).toEqual(gateways)
  })

  test('ignores malformed rows instead of replacing valid gateway data', () => {
    const gateways = parseEpayGateways(
      JSON.stringify([
        null,
        { id: 12 },
        {
          id: 'backup',
          name: 'Backup',
          address: 'https://backup.example.com',
          merchant_id: '20002',
          key: 'secret',
          enabled: false,
          pay_methods: [],
        },
      ])
    )

    expect(gateways.map((gateway) => gateway.id)).toEqual(['backup'])
  })

  test('saves a fee edited through the nested gateway editor', async () => {
    const user = userEvent.setup()
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    vi.mocked(updateSystemOption).mockResolvedValue({
      success: true,
      message: '',
    })

    render(
      <QueryClientProvider client={queryClient}>
        <PaymentSettingsSection
          defaultValues={paymentDefaults}
          waffoDefaultValues={waffoDefaults}
          waffoPancakeDefaultValues={waffoPancakeDefaults}
          complianceDefaults={{
            confirmed: true,
            termsVersion: 'v1',
            confirmedAt: 1,
            confirmedBy: 1,
          }}
        />
      </QueryClientProvider>
    )

    await user.click(screen.getByRole('tab', { name: 'Epay' }))
    await user.click(screen.getAllByRole('button', { name: 'Edit' })[0])

    const gatewayDialog = await screen.findByRole('dialog', {
      name: 'Edit Epay gateway',
    })
    await user.click(
      within(gatewayDialog).getAllByRole('button', { name: 'Edit' })[0]
    )

    const methodDialog = await screen.findByRole('dialog', {
      name: 'Edit payment method',
    })
    expect(gatewayDialog).toHaveAttribute('data-open')
    const feeRate = within(methodDialog).getByRole('spinbutton', {
      name: 'Payment fee rate (%)',
    })
    await user.clear(feeRate)
    await user.type(feeRate, '3')
    await user.click(
      within(methodDialog).getByRole('button', { name: 'Update' })
    )

    await waitFor(() => {
      expect(
        screen.queryByRole('dialog', { name: 'Edit payment method' })
      ).not.toBeInTheDocument()
    })
    expect(gatewayDialog).toHaveAttribute('data-open')
    expect(within(gatewayDialog).getAllByText('3%').length).toBeGreaterThan(0)
    await user.click(
      within(gatewayDialog).getByRole('button', { name: 'Update' })
    )
    await waitFor(() => {
      expect(
        screen.queryByRole('dialog', { name: 'Edit Epay gateway' })
      ).not.toBeInTheDocument()
    })

    const settingsForm = document.querySelector<HTMLFormElement>(
      'form[data-no-autosubmit="true"]'
    )
    if (!settingsForm) throw new Error('Payment settings form was not rendered')
    fireEvent.submit(settingsForm)

    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalledTimes(1)
    })
    const request = vi.mocked(updateSystemOption).mock.calls[0][0]
    expect(request.key).toBe('EpayGateways')
    expect(JSON.parse(String(request.value))[0].pay_methods[0]).toMatchObject({
      fee: '0',
      fee_rate: '3',
    })

    queryClient.clear()
  })

  test('places the nested payment method backdrop above the gateway dialog', () => {
    render(
      <Dialog open title='Gateway'>
        <PaymentMethodDialog
          open
          onOpenChange={() => undefined}
          onSave={() => undefined}
          editData={{ name: 'Alipay', type: 'alipay' }}
        />
      </Dialog>
    )

    const overlays = document.querySelectorAll('[data-slot="dialog-overlay"]')
    expect(overlays).toHaveLength(2)
    expect(overlays[1]).toHaveClass(
      'z-[60]',
      'supports-backdrop-filter:backdrop-blur-sm'
    )
    expect(
      screen.getByRole('dialog', { name: 'Edit payment method' })
    ).toHaveClass('z-[61]')
  })
})
