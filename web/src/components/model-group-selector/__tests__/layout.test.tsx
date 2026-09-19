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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { ModelGroupSelector } from '../../model-group-selector'
import {
  modelGroupSelectorLayoutClasses,
  scrollSelectedOptionIntoView,
} from '../layout'

const desktopWidth = window.innerWidth

afterEach(() => {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    value: desktopWidth,
  })
})

describe('model group selector layout', () => {
  test('keeps the mobile drawer height stable and scrolls the model list internally', () => {
    const drawerClasses =
      modelGroupSelectorLayoutClasses.mobileDrawer.split(' ')
    const contentClasses =
      modelGroupSelectorLayoutClasses.mobileContent.split(' ')
    const modelColumnClasses =
      modelGroupSelectorLayoutClasses.mobileModelColumn.split(' ')

    expect(drawerClasses).toContain('h-[min(80svh,40rem)]')
    expect(drawerClasses).toContain('min-h-0')
    expect(contentClasses).toContain('h-full')
    expect(contentClasses).toContain('min-h-0')
    expect(modelColumnClasses).toContain('h-full')
    expect(modelColumnClasses).toContain('min-h-0')
    expect(modelGroupSelectorLayoutClasses.modelList.split(' ')).toContain(
      'overflow-y-auto'
    )
  })

  test('keeps group options at a fixed height and aligned to the top', () => {
    const groupScrollClasses =
      modelGroupSelectorLayoutClasses.groupScroll.split(' ')

    expect(groupScrollClasses.includes('auto-rows-[2rem]')).toBeTruthy()
    expect(groupScrollClasses.includes('content-start')).toBeTruthy()
  })

  test('centers the selected group inside its own scroll container', () => {
    const scrollCalls: ScrollToOptions[] = []
    const selectedOption = {
      offsetHeight: 32,
      offsetTop: 160,
      scrollIntoView() {},
    }
    const scrollContainer = {
      clientHeight: 200,
      scrollTop: 0,
      scrollTo(options: ScrollToOptions) {
        scrollCalls.push(options)
      },
    }

    scrollSelectedOptionIntoView(selectedOption, scrollContainer)

    expect(scrollCalls).toEqual([{ top: 76, behavior: 'auto' }])
  })

  test('falls back to scrollIntoView when no group container is provided', () => {
    const scrollCalls: ScrollIntoViewOptions[] = []
    const selectedOption = {
      scrollIntoView(options?: ScrollIntoViewOptions) {
        scrollCalls.push(options ?? {})
      },
    }

    scrollSelectedOptionIntoView(selectedOption)

    expect(scrollCalls).toEqual([{ block: 'center', inline: 'nearest' }])
  })

  test('opening on mobile scrolls the selected model list without moving drawer ancestors', async () => {
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      value: 375,
    })
    const ancestorScroll = vi.spyOn(HTMLElement.prototype, 'scrollIntoView')

    render(
      <ModelGroupSelector
        selectedModel='model-8'
        models={Array.from({ length: 10 }, (_, index) => ({
          label: `Model ${index}`,
          value: `model-${index}`,
        }))}
        onModelChange={() => undefined}
        selectedGroup='default'
        groups={[{ label: 'Default', value: 'default' }]}
        onGroupChange={() => undefined}
      />
    )

    fireEvent.click(screen.getByRole('combobox'))

    const modelList = document.querySelector<HTMLElement>(
      '[data-slot="command-list"]'
    )
    if (!modelList) throw new Error('Model list was not rendered')
    const selectedModel = within(modelList)
      .getByText('Model 8')
      .closest<HTMLElement>('[cmdk-item]')
    if (!selectedModel) throw new Error('Selected model was not rendered')

    const listScroll = vi.fn()
    Object.defineProperties(modelList, {
      clientHeight: { configurable: true, value: 200 },
      scrollTo: { configurable: true, value: listScroll },
    })
    Object.defineProperties(selectedModel, {
      offsetHeight: { configurable: true, value: 32 },
      offsetTop: { configurable: true, value: 320 },
    })

    await waitFor(() => {
      expect(listScroll).toHaveBeenCalledWith({
        top: 236,
        behavior: 'auto',
      })
    })
    expect(ancestorScroll).not.toHaveBeenCalledWith({
      block: 'center',
      inline: 'nearest',
    })
  })
})
