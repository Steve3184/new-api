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
import { describe, expect, test } from 'vitest'

import en from '../locales/en.json'
import fr from '../locales/fr.json'
import ja from '../locales/ja.json'
import ru from '../locales/ru.json'
import vi from '../locales/vi.json'
import zhTW from '../locales/zh-TW.json'
import zh from '../locales/zh.json'

const locales = { en, zh, 'zh-TW': zhTW, fr, ja, ru, vi }
const planKeys = [
  'Automatically create API key',
  'Binding a plan makes its group a subscription group automatically. After a purchase, a key for that group is created only if the subscriber has no usable key covering it; the key limit still applies.',
  'Bind a subscription group to enable this option.',
  'Leave unbound to keep legacy billing. A bound group becomes a subscription group automatically and is covered only by plans bound to it.',
] as const
const retiredGroupSettingKeys = [
  'Automatically create API key: {{group}}',
  'Bind a subscription plan before enabling its group. Other groups keep their existing billing rules.',
  'Leave unbound to keep legacy billing. A bound plan only covers its group once that group is enabled for subscriptions.',
  'Subscription group: {{group}}',
  'Subscription groups',
] as const

describe('plan-derived subscription group translations', () => {
  test.each(Object.entries(locales))(
    'provides the plan automatic key copy in the %s translation namespace',
    (_locale, resource) => {
      const translation = resource.translation as Record<string, string>
      for (const key of planKeys) {
        expect(translation[key]).toBeTruthy()
      }
    }
  )

  test('translates every plan automatic key message for Chinese', () => {
    const translation = zh.translation as Record<string, string>
    for (const key of planKeys) {
      expect(translation[key]).not.toBe(key)
    }
  })

  test.each(Object.entries(locales))(
    'drops retired group-settings subscription copy from %s',
    (_locale, resource) => {
      for (const key of retiredGroupSettingKeys) {
        expect(resource.translation).not.toHaveProperty([key])
      }
    }
  )
})
