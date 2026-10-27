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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import { sendSMTPTestEmail } from '../../api'
import { EmailSettingsSection } from '../email-settings-section'

vi.mock('../../api', () => ({
  sendSMTPTestEmail: vi.fn(),
  updateSystemOption: vi.fn(),
}))

const defaultValues = {
  QuotaRemindEnabled: true,
  QuotaRemindThreshold: '1000',
  SMTPServer: 'smtp.example.com',
  SMTPPort: '587',
  SMTPAccount: 'sender@example.com',
  SMTPFrom: 'sender@example.com',
  SMTPToken: '',
  SMTPSSLEnabled: false,
  SMTPStartTLSEnabled: true,
  SMTPInsecureSkipVerify: false,
  SMTPForceAuthLogin: false,
  EmailProvider: 'smtp' as const,
  ResendApiKey: '',
  ResendKeyConfigured: false,
  ResendFrom: '',
  EmailVerificationTemplate: '<p>{{.Code}}</p>',
  EmailPasswordResetTemplate: '',
  EmailBindingTemplate: '',
  EmailSecurityNotificationTemplate: '',
  EmailNotificationTemplate: '',
}

describe('SMTP email settings', () => {
  beforeEach(() => vi.clearAllMocks())
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'SMTP Host': 'SMTP Host',
      'Test recipient email': 'Test recipient email',
      'Send test email': 'Send test email',
    })
  })

  test('sends only to a valid recipient while the SMTP form is saved', async () => {
    const user = userEvent.setup()
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    vi.mocked(sendSMTPTestEmail).mockResolvedValue({
      success: true,
      message: '',
    })

    render(
      <QueryClientProvider client={queryClient}>
        <EmailSettingsSection defaultValues={defaultValues} />
      </QueryClientProvider>
    )

    const recipient = screen.getByLabelText('Test recipient email')
    const sendButton = screen.getByRole('button', { name: 'Send test email' })
    const recipientDescription = screen.getByText(
      'Save SMTP settings before sending a test email'
    )
    expect(recipient).toHaveAttribute('id', 'smtp-test-recipient')
    expect(sendButton).toBeDisabled()
    expect(sendButton.parentElement).not.toContainElement(recipientDescription)

    await user.type(recipient, 'admin@example.com')
    expect(sendButton).toBeEnabled()

    await user.click(sendButton)
    await waitFor(() => {
      expect(sendSMTPTestEmail).toHaveBeenCalledTimes(1)
    })
    expect(vi.mocked(sendSMTPTestEmail).mock.calls[0]?.[0]).toEqual({
      email: 'admin@example.com',
      type: 'smtp',
    })

    fireEvent.change(screen.getByLabelText('SMTP Host'), {
      target: { value: 'smtp.changed.example.com' },
    })
    expect(sendButton).toBeDisabled()

    queryClient.clear()
  })

  test('selects a security event and sends a sample without changing the settings form', async () => {
    const user = userEvent.setup()
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    vi.mocked(sendSMTPTestEmail).mockResolvedValue({
      success: true,
      message: '',
    })
    render(
      <QueryClientProvider client={queryClient}>
        <EmailSettingsSection defaultValues={defaultValues} />
      </QueryClientProvider>
    )
    await user.type(
      screen.getByLabelText('Test recipient email'),
      'admin@example.com'
    )
    await user.click(screen.getByRole('combobox', { name: 'Email type' }))
    await user.click(
      screen.getByRole('option', { name: 'Account security notification' })
    )
    const event = screen.getByRole('combobox', { name: 'Security event' })
    await user.click(event)
    await user.click(screen.getByRole('option', { name: 'Passkey registered' }))
    await user.click(screen.getByRole('button', { name: 'Send test email' }))
    await waitFor(() =>
      expect(sendSMTPTestEmail).toHaveBeenCalledWith(
        {
          email: 'admin@example.com',
          type: 'security',
          security_event: 'passkey_registered',
        },
        expect.anything()
      )
    )
    queryClient.clear()
  })
})
