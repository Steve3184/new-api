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
import { SystemInfoSection } from '../general/system-info-section'
import {
  parseHeaderNavModules,
  parseSidebarModulesAdmin,
  serializeHeaderNavModules,
  serializeSidebarModulesAdmin,
} from '../maintenance/config'
import { HeaderNavigationSection } from '../maintenance/header-navigation-section'
import { NoticeSection } from '../maintenance/notice-section'
import { SidebarModulesSection } from '../maintenance/sidebar-modules-section'
import type { SiteSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AppearanceSection } from './appearance-section'
import { HomepageSection } from './homepage-section'
import { SPAMetaSection } from './spa-meta-section'

const SITE_SECTIONS = [
  {
    id: 'system-info',
    titleKey: 'System Information',
    build: (settings: SiteSettings) => (
      <SystemInfoSection
        defaultValues={{
          SystemName: settings.SystemName,
          Logo: settings.Logo,
          Footer: settings.Footer,
          About: settings.About,
          ServerAddress: settings.ServerAddress,
          TaskPublicAddress: settings.TaskPublicAddress,
          legal: {
            user_agreement: settings['legal.user_agreement'],
            privacy_policy: settings['legal.privacy_policy'],
          },
        }}
      />
    ),
  },
  {
    id: 'homepage',
    titleKey: 'Homepage',
    build: (settings: SiteSettings) => (
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
    id: 'appearance',
    titleKey: 'Appearance & Model Square',
    build: (settings: SiteSettings) => (
      <AppearanceSection
        defaultValues={{
          backgroundImage: settings['console_setting.background_image'],
          backgroundBlurOpacity:
            settings['console_setting.background_blur_opacity'],
          defaultTheme: settings['console_setting.default_theme'],
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
    id: 'spa-metadata',
    titleKey: 'SPA Metadata',
    build: (settings: SiteSettings) => (
      <SPAMetaSection
        defaultValues={{
          description: settings['console_setting.spa_meta_description'],
          ogType: settings['console_setting.spa_meta_og_type'],
          ogDescription: settings['console_setting.spa_meta_og_description'],
        }}
      />
    ),
  },
  {
    id: 'notice',
    titleKey: 'System Notice',
    build: (settings: SiteSettings) => (
      <NoticeSection
        defaultValues={{
          Notice: settings.Notice ?? '',
          NoticePopupEnabled: settings.NoticePopupEnabled,
          NoticePopupMode:
            settings.NoticePopupMode ||
            (settings.NoticePopupOnDashboardEnabled ? 'both' : 'home'),
          NoticeHeaderButtonMode: settings.NoticeHeaderButtonMode || 'popover',
        }}
      />
    ),
  },
  {
    id: 'header-navigation',
    titleKey: 'Header navigation',
    build: (settings: SiteSettings) => {
      const headerNavConfig = parseHeaderNavModules(settings.HeaderNavModules)
      const headerNavSerialized = serializeHeaderNavModules(headerNavConfig)
      return (
        <HeaderNavigationSection
          config={headerNavConfig}
          initialSerialized={headerNavSerialized}
        />
      )
    },
  },
  {
    id: 'sidebar-modules',
    titleKey: 'Sidebar modules',
    build: (settings: SiteSettings) => {
      const sidebarConfig = parseSidebarModulesAdmin(
        settings.SidebarModulesAdmin
      )
      const sidebarSerialized = serializeSidebarModulesAdmin(sidebarConfig)
      return (
        <SidebarModulesSection
          config={sidebarConfig}
          initialSerialized={sidebarSerialized}
        />
      )
    },
  },
] as const

export type SiteSectionId = (typeof SITE_SECTIONS)[number]['id']

const siteRegistry = createSectionRegistry<SiteSectionId, SiteSettings>({
  sections: SITE_SECTIONS,
  defaultSection: 'system-info',
  basePath: '/system-settings/site',
  urlStyle: 'path',
})

export const SITE_SECTION_IDS = siteRegistry.sectionIds
export const SITE_DEFAULT_SECTION = siteRegistry.defaultSection
export const getSiteSectionNavItems = siteRegistry.getSectionNavItems
export const getSiteSectionContent = siteRegistry.getSectionContent
export const getSiteSectionMeta = siteRegistry.getSectionMeta
