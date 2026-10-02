/* Copyright (C) 2023-2026 QuantumNous */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { UserProfile } from '../../types'
import { DisplayNameForm } from '../display-name-form'

const profile: UserProfile = {
  id: 1,
  username: 'alice',
  display_name: 'Alice',
  role: 1,
  group: 'default',
  quota: 100,
  used_quota: 0,
  request_count: 0,
  status: 1,
  aff_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  created_time: 0,
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

function renderEditor() {
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: profile as typeof state.auth.user },
  }))
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const refresh = vi.fn()
  render(
    <QueryClientProvider client={client}>
      <DisplayNameForm profile={profile} onProfileUpdate={refresh} />
    </QueryClientProvider>
  )
  return { refresh }
}

test('saving sends only the trimmed display name and updates the user menu after success', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const { refresh } = renderEditor()
  expect(
    screen.getByRole('button', { name: 'Save display name' })
  ).toBeDisabled()
  fireEvent.change(screen.getByRole('textbox', { name: 'Display Name' }), {
    target: { value: '  新的显示名  ' },
  })
  await userEvent.click(
    screen.getByRole('button', { name: 'Save display name' })
  )
  await waitFor(() => expect(refresh).toHaveBeenCalledOnce())
  expect(put).toHaveBeenCalledWith(
    '/api/user/self',
    { display_name: '新的显示名' },
    { acceptAuthRotation: false }
  )
  expect(useAuthStore.getState().auth.user).toMatchObject({
    username: 'alice',
    display_name: '新的显示名',
    role: 1,
  })
  expect(screen.getByRole('textbox', { name: 'Display Name' })).toHaveValue(
    '新的显示名'
  )
  expect(
    screen.getByRole('button', { name: 'Save display name' })
  ).toBeDisabled()
})

test.each(['   ', '名'.repeat(21)])(
  'invalid name %s blocks submission and shows an accessible error',
  async (name) => {
    const put = vi.spyOn(api, 'put')
    renderEditor()
    const input = screen.getByRole('textbox', { name: 'Display Name' })
    fireEvent.change(input, { target: { value: name } })
    await userEvent.click(
      screen.getByRole('button', { name: 'Save display name' })
    )
    expect(
      await screen.findByText('Display name must contain 1 to 20 characters.')
    ).toBeVisible()
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(put).not.toHaveBeenCalled()
  }
)

test('twenty Unicode characters are accepted without counting emoji as two characters', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  renderEditor()
  fireEvent.change(screen.getByRole('textbox', { name: 'Display Name' }), {
    target: { value: '🐱'.repeat(20) },
  })
  await userEvent.click(
    screen.getByRole('button', { name: 'Save display name' })
  )
  await waitFor(() => expect(put).toHaveBeenCalled())
})

test('pending save disables editing and failure preserves the draft and current user', async () => {
  let finish!: (value: unknown) => void
  vi.spyOn(api, 'put').mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve
      })
  )
  const { refresh } = renderEditor()
  const input = screen.getByRole('textbox', { name: 'Display Name' })
  fireEvent.change(input, { target: { value: 'New name' } })
  await userEvent.click(
    screen.getByRole('button', { name: 'Save display name' })
  )
  await waitFor(() => expect(input).toBeDisabled())
  expect(
    screen.getByRole('button', { name: 'Save display name' })
  ).toBeDisabled()
  finish({ data: { success: false, message: 'Save failed' } })
  await waitFor(() => expect(input).toBeEnabled())
  expect(input).toHaveValue('New name')
  expect(refresh).not.toHaveBeenCalled()
  expect(useAuthStore.getState().auth.user?.display_name).toBe('Alice')
})
