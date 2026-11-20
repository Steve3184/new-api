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

// The backend sends these as `message_key`, so each must resolve in every
// locale or the web console falls back to raw English.
const backendKeys = [
  'Extra credits are not enabled for this plan',
  'Invalid extra credits amount',
  'Invalid extra credits exchange rate',
  'Extra credits purchase request conflicts with an earlier purchase',
  'Cannot purchase extra credits for this subscription',
  'Extra credits purchased',
  'Subscription still has unspent extra credits',
  'Extra credits exchange rate changed; reload and try again',
] as const

const uiKeys = [
  'Allow buying extra credits',
  'Extra credits exchange rate',
  'Extra credits are granted at this rate in the same currency as the plan quota: 1 {{currency}} of wallet balance grants this many {{currency}} of extra credits. They are stored separately from the plan quota and stay available after the subscription expires.',
  '1 {{currency}} spent = {{rate}} {{currency}} extra credits',
  'Buy Extra Credits',
  'Remaining extra credits',
  'Extra credits you receive',
  'Confirm extra credits purchase',
  'Extra credits: {{amount}}',
  'Unused extra credits stay after expiry',
  'Purchases are unavailable because this browser cannot coordinate tabs. Keep any pending purchase in this browser until its status can be checked.',
  'The purchase result could not be saved reliably in this browser. Check your balance before retrying in any tab.',
  'The pending purchase changed in another tab. Review the amount and confirm again.',
  'The earlier purchase was resolved in another tab. Enter an amount only if you want to make a new purchase.',
] as const

describe('extra credits translations', () => {
  test.each(Object.entries(locales))(
    'provides every backend extra-credits message in %s',
    (_locale, resource) => {
      const translation = resource.translation as Record<string, string>
      for (const key of backendKeys) {
        expect(translation[key]).toBeTruthy()
      }
    }
  )

  test.each(Object.entries(locales))(
    'provides the extra-credits interface copy in %s',
    (_locale, resource) => {
      const translation = resource.translation as Record<string, string>
      for (const key of uiKeys) {
        expect(translation[key]).toBeTruthy()
      }
    }
  )

  test.each(['zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi'] as const)(
    'actually translates the backend messages for %s instead of copying English',
    (locale) => {
      const translation = locales[locale].translation as Record<string, string>
      for (const key of backendKeys) {
        expect(translation[key]).not.toBe(key)
      }
    }
  )

  test.each(Object.entries(locales))(
    'keeps the interpolation placeholders intact in %s',
    (_locale, resource) => {
      const translation = resource.translation as Record<string, string>
      expect(translation['Extra credits: {{amount}}']).toContain('{{amount}}')
      const confirm =
        translation[
          '{{cost}} will be deducted from your wallet balance for {{credits}} of extra credits. This cannot be undone.'
        ]
      expect(confirm).toContain('{{cost}}')
      expect(confirm).toContain('{{credits}}')
    }
  )
})
