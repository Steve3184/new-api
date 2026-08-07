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

export type ErrorRewriteRuleDraft = {
  id: string
  statusCode: string
  message: string
}

export type ErrorRewriteRuleErrorCode =
  | 'invalid-status-code'
  | 'duplicate-status-code'
  | 'empty-message'

let nextRuleId = 0

export function createErrorRewriteRuleId() {
  nextRuleId += 1
  return `error-rewrite-${nextRuleId}`
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/** Parse the persisted option into editable, string-backed table rows. */
export function parseErrorRewriteRules(value: string): ErrorRewriteRuleDraft[] {
  if (!value.trim()) return []

  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return []
  }

  if (!Array.isArray(parsed)) return []

  return parsed.flatMap((item, index) => {
    if (!isRecord(item)) return []

    const statusCode = item.status_code
    const message = item.message
    return [
      {
        id: `error-rewrite-${index}`,
        statusCode:
          typeof statusCode === 'number' || typeof statusCode === 'string'
            ? String(statusCode)
            : '',
        message: typeof message === 'string' ? message : '',
      },
    ]
  })
}

/** Serialize table rows to the backend's stable JSON array shape. */
export function serializeErrorRewriteRules(
  rules: ErrorRewriteRuleDraft[]
): string {
  return JSON.stringify(
    rules.map((rule) => ({
      status_code: Number(rule.statusCode.trim()),
      message: rule.message.trim(),
    })),
    null,
    2
  )
}

/** Return row-level validation codes without coupling the helper to i18n. */
export function validateErrorRewriteRules(
  rules: ErrorRewriteRuleDraft[]
): Record<string, ErrorRewriteRuleErrorCode[]> {
  const errors: Record<string, ErrorRewriteRuleErrorCode[]> = {}
  const statusOwners = new Map<number, string[]>()

  for (const rule of rules) {
    const rowErrors: ErrorRewriteRuleErrorCode[] = []
    const statusCode = Number(rule.statusCode.trim())

    if (!Number.isInteger(statusCode) || statusCode < 100 || statusCode > 599) {
      rowErrors.push('invalid-status-code')
    } else {
      const owners = statusOwners.get(statusCode) ?? []
      owners.push(rule.id)
      statusOwners.set(statusCode, owners)
    }

    if (!rule.message.trim()) rowErrors.push('empty-message')
    if (rowErrors.length > 0) errors[rule.id] = rowErrors
  }

  for (const owners of statusOwners.values()) {
    if (owners.length < 2) continue
    for (const owner of owners) {
      errors[owner] = [...(errors[owner] ?? []), 'duplicate-status-code']
    }
  }

  return errors
}

export function canonicalErrorRewriteRules(value: string) {
  return serializeErrorRewriteRules(parseErrorRewriteRules(value))
}
