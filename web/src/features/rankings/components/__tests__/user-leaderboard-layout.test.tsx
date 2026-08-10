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

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const { UserLeaderboard } = await import('../user-leaderboard')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type RenderedLeaderboard = {
  container: HTMLDivElement
  root: ReturnType<typeof createRoot>
}

async function renderLeaderboard(): Promise<RenderedLeaderboard> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UserLeaderboard byQuota={[]} byTokens={[]} period='week' />
      </I18nextProvider>
    )
  })

  return { container, root }
}

async function unmountLeaderboard(rendered: RenderedLeaderboard) {
  await act(async () => rendered.root.unmount())
  rendered.container.remove()
}

describe('user leaderboard layout', () => {
  after(() => {
    domWindow.close()
  })

  test('keeps the wide-layout divider on the midpoint without a horizontal grid gap', async () => {
    const rendered = await renderLeaderboard()
    const columns = rendered.container.querySelector('section > div.grid')

    assert.ok(columns)
    assert.equal(columns.classList.contains('md:grid-cols-2'), true)
    assert.equal(
      [...columns.classList].some((className) =>
        className.startsWith('gap-x-')
      ),
      false
    )

    const [firstColumn] = [...columns.children]
    assert.ok(firstColumn)
    assert.equal(firstColumn.classList.contains('md:first:border-r'), true)

    await unmountLeaderboard(rendered)
  })
})
