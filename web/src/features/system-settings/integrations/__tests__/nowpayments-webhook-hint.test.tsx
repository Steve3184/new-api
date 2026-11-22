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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ComponentProps } from 'react'
import { describe, expect, it } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { PaymentSettingsSection } from '../payment-settings-section'

type PaymentProps = ComponentProps<typeof PaymentSettingsSection>
type PaymentValues = PaymentProps['defaultValues']

const defaults: PaymentValues = {
  PayAddress: 'https://pay.example.com',
  EpayId: '1001',
  EpayKey: '',
  EpaySignType: 'MD5',
  EpayMerchantPrivateKey: '',
  EpayPlatformPublicKey: '',
  Price: 7.3,
  MinTopUp: 1,
  CustomCallbackAddress: '',
  PayMethods: '[]',
  AmountOptions: '[]',
  AmountDiscount: '{}',
  StripeApiSecret: '',
  StripeWebhookSecret: '',
  StripePriceId: '',
  StripeUnitPrice: 8,
  StripeMinTopUp: 1,
  StripePromotionCodesEnabled: false,
  CreemApiKey: '',
  CreemWebhookSecret: '',
  CreemTestMode: false,
  CreemProducts: '[]',
  EpayGateways: '[]',
  RedemptionPurchaseEnabled: false,
  MoneroEnabled: false,
  MoneroWalletRPCURL: '',
  MoneroWalletRPCUsername: '',
  MoneroWalletRPCPassword: '',
  MoneroNetwork: 'mainnet',
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

function PaymentFixture(props: { values: PaymentValues }) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <SettingsPageProvider actionsContainer={actions}>
      <div ref={setActions} />
      <PaymentSettingsSection
        defaultValues={props.values}
        waffoDefaultValues={{
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
        }}
        waffoPancakeDefaultValues={{
          WaffoPancakeMerchantID: '',
          WaffoPancakePrivateKey: '',
          WaffoPancakeReturnURL: '',
          WaffoPancakeUseConfiguredProductPrice: false,
          WaffoPancakeUSDToCurrencyRate: 0,
        }}
        complianceDefaults={{
          confirmed: true,
          termsVersion: 'v1',
          confirmedAt: 1,
          confirmedBy: 1,
        }}
      />
    </SettingsPageProvider>
  )
}

async function openNowPaymentsTab() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <PaymentFixture values={defaults} />
    </QueryClientProvider>
  )
  await userEvent
    .setup()
    .click(screen.getByRole('tab', { name: 'NOWPayments' }))
}

describe('NOWPayments webhook hint', () => {
  it('shows the IPN endpoint the gateway calls back, like the other payment gateways', async () => {
    await openNowPaymentsTab()

    expect(screen.getByText('Webhook Configuration:')).toBeVisible()
    expect(
      screen.getByText('<ServerAddress>/api/nowpayments/webhook')
    ).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'NOWPayments Dashboard' })
    ).toHaveAttribute('href', 'https://account.nowpayments.io/')
  })
})
