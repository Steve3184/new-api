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
import { SettingsPage } from '../components/settings-page'
import type { SiteSettings } from '../types'
import {
  SITE_DEFAULT_SECTION,
  getSiteSectionContent,
  getSiteSectionMeta,
} from './section-registry.tsx'

const defaultSiteSettings: SiteSettings = {
  Notice: '',
  NoticePopupEnabled: false,
  NoticePopupMode: 'home',
  NoticePopupOnDashboardEnabled: false,
  NoticeHeaderButtonMode: 'popover',
  SystemName: 'New API',
  Logo: '',
  Footer: '',
  About: '',
  HomePageContent: '',
  ServerAddress: '',
  TaskPublicAddress: '',
  'legal.user_agreement': '',
  'legal.privacy_policy': '',
  HeaderNavModules: '',
  SidebarModulesAdmin: '',
  'console_setting.background_image': '',
  'console_setting.background_blur_opacity': 40,
  'console_setting.default_theme': 'system',
  'console_setting.default_theme_preset': 'default',
  'console_setting.default_theme_font': 'default',
  'console_setting.default_theme_radius': 'default',
  'console_setting.default_theme_scale': 'default',
  'console_setting.default_sidebar_variant': 'inset',
  'console_setting.default_sidebar_layout': 'expanded',
  'console_setting.default_content_layout': 'full',
  'console_setting.default_direction': 'ltr',
  'console_setting.model_square_default_view': 'card',
  'console_setting.model_square_card_page_size': 18,
  'console_setting.model_square_table_page_size': 20,
  'console_setting.spa_meta_description':
    'Unified AI API gateway and admin dashboard.',
  'console_setting.spa_meta_og_type': 'website',
  'console_setting.spa_meta_og_description':
    'Unified AI API gateway and admin dashboard.',
}

export function SiteSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/site/$section'
      defaultSettings={defaultSiteSettings}
      defaultSection={SITE_DEFAULT_SECTION}
      getSectionContent={getSiteSectionContent}
      getSectionMeta={getSiteSectionMeta}
    />
  )
}
