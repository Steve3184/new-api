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
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { FormItem } from '@/components/ui/form'

import { SettingsForm, SettingsFormGrid } from '../settings-form-layout'

// jsdom does not compute Tailwind styles, so this pins the class contract that
// keeps a short field's label and control at the top of a taller grid row.
const TOP_ALIGNED = '[&>[data-slot=form-item]]:content-start'

describe('settings form layout', () => {
  it('top-aligns the rows of every form item inside a form grid', () => {
    const { container } = render(
      <SettingsFormGrid>
        <FormItem>short field</FormItem>
        <FormItem>tall field with a description</FormItem>
      </SettingsFormGrid>
    )

    expect(container.querySelectorAll('[data-slot=form-item]')).toHaveLength(2)
    expect(container.firstElementChild).toHaveClass(TOP_ALIGNED)
  })

  it('top-aligns form items placed directly in a settings form', () => {
    const { container } = render(
      <SettingsForm>
        <FormItem>short field</FormItem>
      </SettingsForm>
    )

    expect(container.querySelector('form')).toHaveClass(TOP_ALIGNED)
  })
})
