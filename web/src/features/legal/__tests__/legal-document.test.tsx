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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { LegalDocument } from '../legal-document'

let legalBackgroundEnabled = true

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    appearance: { legalBackgroundEnabled },
  }),
}))

vi.mock('@/components/layout', () => ({
  PublicLayout: (props: {
    backgroundMode?: string
    children: React.ReactNode
    showMainContainer?: boolean
  }) => (
    <div
      data-testid='public-layout'
      data-background-mode={props.backgroundMode ?? 'none'}
      data-main-container={String(props.showMainContainer ?? true)}
    >
      {props.children}
    </div>
  ),
}))

function renderDocument() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <LegalDocument
        title='Privacy Policy'
        queryKey='privacy-policy-test'
        fetchDocument={async () => ({ success: true, data: 'Policy body' })}
        emptyMessage='Missing'
      />
    </QueryClientProvider>
  )
}

describe('legal document appearance', () => {
  beforeEach(() => {
    legalBackgroundEnabled = true
  })

  test('requests the hero background and adds top spacing when enabled', async () => {
    renderDocument()

    await screen.findByText('Policy body')
    expect(screen.getByTestId('public-layout')).toHaveAttribute(
      'data-background-mode',
      'hero'
    )
    expect(screen.getByRole('article')).toHaveClass('pt-20')
  })

  test('uses the normal document spacing when the background is disabled', async () => {
    legalBackgroundEnabled = false
    renderDocument()

    await screen.findByText('Policy body')
    expect(screen.getByTestId('public-layout')).toHaveAttribute(
      'data-background-mode',
      'none'
    )
    expect(screen.getByRole('article')).not.toHaveClass('pt-20')
  })
})
