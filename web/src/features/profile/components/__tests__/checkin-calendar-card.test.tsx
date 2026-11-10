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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { getCheckinStatus } from '../../api'
import { CheckinCalendarCard } from '../checkin-calendar-card'

vi.mock('../../api', () => ({
  getCheckinStatus: vi.fn(),
  performCheckin: vi.fn(),
}))

vi.mock('@/features/auth/hooks/use-captcha', () => ({
  captchaProviderName: () => 'Cloudflare Turnstile',
  useCaptcha: () => ({
    isCaptchaEnabled: false,
    skipsCaptcha: true,
    captchaIssue: undefined,
    captchaType: 'none',
    isTurnstileEnabled: false,
    isHCaptchaEnabled: false,
    isCapEnabled: false,
    turnstileSiteKey: '',
    hCaptchaSiteKey: '',
    capApiEndpoint: '',
    setCaptchaToken: vi.fn(),
    tokenQueryParam: 'turnstile',
  }),
}))

const checkinData = {
  enabled: true,
  min_user_quota: 100,
  min_used_quota: 200,
  daily_user_limit: 0,
  daily_quota_limit: 0,
  eligible: false,
  stats: {
    checked_in_today: false,
    total_checkins: 0,
    total_quota: 0,
    checkin_count: 0,
    records: [],
  },
  eligibility: {
    eligible: false,
    already_checked_in: false,
    balance_met: false,
    total_spend_met: false,
    daily_user_limit_met: true,
    daily_quota_limit_met: true,
    current_quota: 0,
    current_used_quota: 0,
    today_user_count: 0,
    today_quota_awarded: 0,
    daily_quota_remaining: 0,
  },
}

function renderCard() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <CheckinCalendarCard checkinEnabled />
    </QueryClientProvider>
  )
}

describe('check-in eligibility requirements', () => {
  beforeEach(() => {
    vi.mocked(getCheckinStatus).mockResolvedValue({
      success: true,
      data: checkinData,
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('does not repeat the summary requirement in the requirements list', async () => {
    renderCard()

    const summaryRequirement = await screen.findByText(
      /Balance must be greater than/
    )
    expect(summaryRequirement).toBeInTheDocument()
    expect(screen.getAllByText(/Balance must be greater than/)).toHaveLength(1)
    expect(screen.getByText(/Total spending must reach/)).toBeInTheDocument()
  })
})
