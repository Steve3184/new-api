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
import { describe, test } from 'node:test'

import { mapStatusDataToConfig } from './use-system-config'

describe('mapStatusDataToConfig', () => {
  test('maps public appearance and SPA metadata settings', () => {
    const config = mapStatusDataToConfig({
      site_appearance: {
        background_image: '/background.webp',
        background_blur_opacity: 60,
        default_theme: 'dark',
        default_theme_preset: 'anthropic',
        default_theme_font: 'serif',
        default_theme_radius: 'lg',
        default_theme_scale: 'sm',
        default_sidebar_variant: 'floating',
        default_sidebar_layout: 'icon',
        default_content_layout: 'centered',
        default_direction: 'rtl',
        model_square_default_view: 'table',
        model_square_card_page_size: 24,
        model_square_table_page_size: 50,
      },
      spa_meta: {
        description: 'Description',
        og_type: 'website',
        og_description: 'Open Graph description',
      },
    })

    assert.deepEqual(config.appearance, {
      backgroundImage: '/background.webp',
      backgroundBlurOpacity: 60,
      defaultTheme: 'dark',
      defaultThemePreset: 'anthropic',
      defaultThemeFont: 'serif',
      defaultThemeRadius: 'lg',
      defaultThemeScale: 'sm',
      defaultSidebarVariant: 'floating',
      defaultSidebarLayout: 'icon',
      defaultContentLayout: 'centered',
      defaultDirection: 'rtl',
      modelSquareDefaultView: 'table',
      modelSquareCardPageSize: 24,
      modelSquareTablePageSize: 50,
    })
    assert.deepEqual(config.spaMeta, {
      description: 'Description',
      ogType: 'website',
      ogDescription: 'Open Graph description',
    })
  })

  test('falls back for invalid enum values', () => {
    const config = mapStatusDataToConfig({
      site_appearance: {
        default_theme: 'invalid',
        default_sidebar_layout: 'invalid',
        model_square_default_view: 'invalid',
      },
    })

    assert.equal(config.appearance?.defaultTheme, 'system')
    assert.equal(config.appearance?.defaultSidebarLayout, 'expanded')
    assert.equal(config.appearance?.modelSquareDefaultView, 'card')
  })

  test('clamps the background blur opacity to its supported range', () => {
    const config = mapStatusDataToConfig({
      site_appearance: {
        background_blur_opacity: 125,
      },
    })

    assert.equal(config.appearance?.backgroundBlurOpacity, 100)
  })
})
