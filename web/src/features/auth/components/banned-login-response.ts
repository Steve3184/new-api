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
type UnknownRecord = Record<string, unknown>

function asRecord(value: unknown): UnknownRecord | null {
  return value && typeof value === 'object' ? (value as UnknownRecord) : null
}

export type BannedLoginResponse = {
  html: string
}

export function getBannedLoginResponse(
  value: unknown
): BannedLoginResponse | null {
  const source = asRecord(value)
  const response = asRecord(source?.response)
  const payload = asRecord(response?.data) ?? source
  const data = asRecord(payload?.data)

  if (data?.banned !== true) {
    return null
  }

  return {
    html: typeof data.html === 'string' ? data.html : '',
  }
}
