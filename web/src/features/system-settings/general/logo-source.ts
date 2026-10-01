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
import { t } from 'i18next'
import { z } from 'zod'

export const logoSourceSchema = z
  .string()
  .trim()
  .refine(
    (source) => {
      if (!source) return true
      if (
        source.startsWith('//') ||
        /[\\\s]/.test(source) ||
        [...source].some((char) => char.charCodeAt(0) < 32)
      ) {
        return false
      }
      try {
        const url = new URL(source, 'https://logo-source.invalid/')
        return (
          ['http:', 'https:'].includes(url.protocol) &&
          !url.username &&
          !url.password
        )
      } catch {
        return false
      }
    },
    { error: () => t('Enter an HTTP(S) URL or relative image path') }
  )
  .optional()
