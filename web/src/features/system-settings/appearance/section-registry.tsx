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
import type { AppearanceSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AuthPagesSection } from './auth-pages-section'
import { HomepageSection } from './homepage-section'
import { MiscAppearanceSection } from './misc-section'
import { ModelSquareSection } from './model-square-section'
import { ThemeSection } from './theme-section'

const APPEARANCE_SECTIONS = [
  {
    id: 'homepage',
    titleKey: 'Homepage',
    build: (settings: AppearanceSettings) => (
      <HomepageSection
        defaultValues={{
          HomePageContent: settings.HomePageContent,
          HomepageStyle: settings['console_setting.homepage_style'],
          HomepagePresetTitleMode:
            settings['console_setting.homepage_preset_title_mode'],
          HomepagePresetSLAEnabled:
            settings['console_setting.homepage_preset_sla_enabled'],
          HomepagePresetSLAText:
            settings['console_setting.homepage_preset_sla_text'],
        }}
      />
    ),
  },
  {
    id: 'model-square',
    titleKey: 'Model Square',
    build: (settings: AppearanceSettings) => (
      <ModelSquareSection
        defaultValues={{
          modelSquareDefaultView:
            settings['console_setting.model_square_default_view'],
          modelSquareCardPageSize:
            settings['console_setting.model_square_card_page_size'],
          modelSquareTablePageSize:
            settings['console_setting.model_square_table_page_size'],
        }}
      />
    ),
  },
  {
    id: 'auth-pages',
    titleKey: 'Login & Register',
    build: (settings: AppearanceSettings) => (
      <AuthPagesSection
        defaultValues={{
          authBackgroundEnabled:
            settings['console_setting.auth_background_enabled'] ?? true,
          authPageStyle:
            settings['console_setting.auth_page_style'] === 'split'
              ? 'split'
              : 'centered',
          authSplitImage: settings['console_setting.auth_split_image'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'theme',
    titleKey: 'Theme',
    build: (settings: AppearanceSettings) => (
      <ThemeSection
        defaultValues={{
          defaultTheme: settings['console_setting.default_theme'],
          defaultThemeOverride:
            settings['console_setting.default_theme_override'],
          defaultThemePreset: settings['console_setting.default_theme_preset'],
          defaultThemeFont: settings['console_setting.default_theme_font'],
          defaultThemeRadius: settings['console_setting.default_theme_radius'],
          defaultThemeScale: settings['console_setting.default_theme_scale'],
          defaultSidebarVariant:
            settings['console_setting.default_sidebar_variant'],
          defaultSidebarLayout:
            settings['console_setting.default_sidebar_layout'],
          defaultContentLayout:
            settings['console_setting.default_content_layout'],
          defaultDirection: settings['console_setting.default_direction'],
        }}
      />
    ),
  },
  {
    id: 'misc',
    titleKey: 'Miscellaneous',
    build: (settings: AppearanceSettings) => (
      <MiscAppearanceSection
        defaultValues={{
          backgroundImage: settings['console_setting.background_image'],
          backgroundBlurOpacity:
            settings['console_setting.background_blur_opacity'],
          legalBackgroundEnabled:
            settings['console_setting.legal_background_enabled'] ?? true,
        }}
      />
    ),
  },
] as const

export type AppearanceSectionId = (typeof APPEARANCE_SECTIONS)[number]['id']

const appearanceRegistry = createSectionRegistry<
  AppearanceSectionId,
  AppearanceSettings
>({
  sections: APPEARANCE_SECTIONS,
  defaultSection: 'homepage',
  basePath: '/system-settings/appearance',
  urlStyle: 'path',
})

export const APPEARANCE_SECTION_IDS = appearanceRegistry.sectionIds
export const APPEARANCE_DEFAULT_SECTION = appearanceRegistry.defaultSection
export const getAppearanceSectionNavItems =
  appearanceRegistry.getSectionNavItems
export const getAppearanceSectionContent = appearanceRegistry.getSectionContent
export const getAppearanceSectionMeta = appearanceRegistry.getSectionMeta
