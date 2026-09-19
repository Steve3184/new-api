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
const keys = [
  'Exclude upstream HTTP 400 errors',
  'Do not include upstream HTTP 400 responses in request success-rate metrics.',
] as const

describe('HTTP 400 metric exclusion translations', () => {
  test.each(Object.entries(locales))(
    'keeps both labels inside the %s translation namespace',
    (_locale, resource) => {
      for (const key of keys) {
        expect(resource.translation[key]).toBeTruthy()
        expect(resource).not.toHaveProperty(key)
      }
    }
  )
})
