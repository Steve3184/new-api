/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, test } from 'vitest'

import { filterChatGroups, getGroupFallback } from './playground-option-utils'

describe('chat group filtering', () => {
  test('does not expose the virtual auto group in the chat selector', () => {
    const groups = [
      { label: 'Default', value: 'default', ratio: 1 },
      { label: 'Auto', value: 'auto', ratio: 1 },
      { label: 'Premium', value: 'premium', ratio: 1 },
    ]

    expect(filterChatGroups(groups).map((group) => group.value)).toEqual([
      'default',
      'premium',
    ])
  })

  test('falls back from a persisted auto group to a selectable chat group', () => {
    const groups = filterChatGroups([
      { label: 'Default', value: 'default', ratio: 1 },
      { label: 'Auto', value: 'auto', ratio: 1 },
    ])

    expect(getGroupFallback(groups, 'auto')).toBe('default')
  })
})
