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
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ErrorRewriteTable } = await import('../error-rewrite-table')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Add Row': 'Add Row',
        Actions: 'Actions',
        Delete: 'Delete',
        'Error Message': 'Error Message',
        'Status Code': 'Status Code',
        'Rewrite rules': 'Rewrite rules',
        'Rules match the upstream HTTP status code exactly.':
          'Rules match the upstream HTTP status code exactly.',
        'No global error rewrite rules configured. Click "Add Row" to get started.':
          'No global error rewrite rules configured. Click "Add Row" to get started.',
        'e.g. Model {model} is unavailable':
          'e.g. Model {model} is unavailable',
      },
    },
  },
})

const getErrorText = (code: string) => code

describe('global error rewrite table', () => {
  after(() => {
    domWindow.close()
  })

  test('adds a row and deletes it from the table', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let rows: Array<{ id: string; statusCode: string; message: string }> = []
    let addCount = 0

    const render = () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ErrorRewriteTable
            rules={rows}
            validationErrors={{}}
            disabled={false}
            onAddRow={() => {
              addCount += 1
              rows = [...rows, { id: 'new-row', statusCode: '', message: '' }]
              render()
            }}
            onDeleteRow={(id) => {
              rows = rows.filter((row) => row.id !== id)
              render()
            }}
            onChangeRule={() => undefined}
            getErrorText={getErrorText}
          />
        </I18nextProvider>
      )

    await act(async () => render())
    const addButton = [...container.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === 'Add Row'
    )
    assert.ok(addButton)

    await act(async () => addButton.click())
    assert.equal(addCount, 1)
    assert.ok(container.querySelector('input[aria-label="Status Code 1"]'))

    const deleteButton = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Delete 1"]'
    )
    assert.ok(deleteButton)
    await act(async () => deleteButton.click())
    assert.equal(
      container.querySelector('input[aria-label="Status Code 1"]'),
      null
    )

    await act(async () => root.unmount())
    container.remove()
  })
})
