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
import { createInstance, type i18n } from 'i18next'
import { useState } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { BasicAuthSection } from '../basic-auth-section'

const defaults = {
  PasswordLoginEnabled: true,
  UserBannedMessage: '',
  PasswordRegisterEnabled: true,
  EmailVerificationEnabled: true,
  RegisterEnabled: true,
  EmailDomainRestrictionEnabled: false,
  EmailDomainRewardRestrictionEnabled: false,
  EmailAliasRestrictionEnabled: false,
  EmailDomainWhitelist: 'gmail.com',
}

let testI18n: i18n

function Fixture(props: { enabledKey: string }) {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  )
  return (
    <I18nextProvider i18n={testI18n}>
      <QueryClientProvider client={client}>
        <div ref={setContainer} />
        <SettingsPageProvider actionsContainer={container}>
          <BasicAuthSection
            defaultValues={{ ...defaults, [props.enabledKey]: true }}
          />
        </SettingsPageProvider>
      </QueryClientProvider>
    </I18nextProvider>
  )
}

beforeEach(async () => {
  testI18n = createInstance()
  await testI18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

const restrictionKey = 'EmailDomainRestrictionEnabled'
const rewardKey = 'EmailDomainRewardRestrictionEnabled'

describe('Email domain restriction switches', () => {
  it.each([
    [restrictionKey, rewardKey, 'Email Domain Reward Restriction'],
    [rewardKey, restrictionKey, 'Email Domain Restriction'],
  ])(
    'with %s on, enabling %s turns the other off and saves the disable first',
    async (enabledKey, requestedKey, requestedLabel) => {
      const put = vi
        .spyOn(api, 'put')
        .mockResolvedValue({ data: { success: true, message: '' } })
      const user = userEvent.setup()
      render(<Fixture enabledKey={enabledKey} />)

      const requested = screen.getByRole('switch', { name: requestedLabel })
      await user.click(requested)

      expect(requested).toBeChecked()
      expect(
        screen.getByRole('switch', {
          name:
            enabledKey === restrictionKey
              ? 'Email Domain Restriction'
              : 'Email Domain Reward Restriction',
        })
      ).not.toBeChecked()

      await user.click(screen.getByRole('button', { name: 'Save Changes' }))

      await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
      expect(put.mock.calls.map((call) => call[1])).toEqual([
        { key: enabledKey, value: false },
        { key: requestedKey, value: true },
      ])
    }
  )
})
