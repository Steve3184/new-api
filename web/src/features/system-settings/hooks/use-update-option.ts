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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'

import { mapStatusDataToConfig } from '@/hooks/use-system-config'
import { getStatus } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { updateSystemOption } from '../api'
import type { UpdateOptionRequest } from '../types'

// Configuration keys that require status refresh
const STATUS_RELATED_KEYS = new Set([
  'theme.frontend',
  'HeaderNavModules',
  'SidebarModulesAdmin',
  'Notice',
  'NoticePopupEnabled',
  'NoticePopupMode',
  'NoticePopupOnDashboardEnabled',
  'NoticeHeaderButtonMode',
  'LogConsumeEnabled',
  'QuotaPerUnit',
  'USDExchangeRate',
  'DisplayInCurrencyEnabled',
  'DisplayTokenStatEnabled',
  'general_setting.quota_display_type',
  'general_setting.custom_currency_symbol',
  'general_setting.custom_currency_exchange_rate',
  'CaptchaType',
  'TurnstileCheckEnabled',
  'TurnstileSiteKey',
  'HCaptchaEnabled',
  'HCaptchaSiteKey',
  'CapEnabled',
  'CapServerURL',
  'CapSiteKey',
  'CapCheckinSiteKey',
  'ForceCheckinCaptcha',
  'checkin_setting.min_user_quota',
  'CustomTabs',
  'StatusCheckGroups',
  'StatusCheckCacheExcludedModels',
  'StatusCheckAnnouncement',
  'StatusCheckFlexibleMode',
  'PlaygroundSettings',
  'oidc.display_name',
  'console_setting.background_image',
  'console_setting.background_blur_opacity',
  'console_setting.default_theme',
  'console_setting.default_theme_preset',
  'console_setting.default_theme_font',
  'console_setting.default_theme_radius',
  'console_setting.default_theme_scale',
  'console_setting.default_sidebar_variant',
  'console_setting.default_sidebar_layout',
  'console_setting.default_content_layout',
  'console_setting.default_direction',
  'console_setting.model_square_default_view',
  'console_setting.model_square_card_page_size',
  'console_setting.model_square_table_page_size',
  'console_setting.spa_meta_description',
  'console_setting.spa_meta_og_type',
  'console_setting.spa_meta_og_description',
])

const STATUS_CHECK_RELATED_KEYS = new Set([
  'StatusCheckGroups',
  'StatusCheckCacheExcludedModels',
  'StatusCheckFlexibleMode',
])

export function useUpdateOption() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateOptionRequest) => updateSystemOption(request),
    onSuccess: (data, variables) => {
      if (data.success) {
        // Always refresh system-options
        queryClient.invalidateQueries({ queryKey: ['system-options'] })

        // If updating frontend-display-related config, also refresh status
        if (STATUS_RELATED_KEYS.has(variables.key)) {
          queryClient.invalidateQueries({ queryKey: ['status'] })
          try {
            window.localStorage.removeItem('status')
          } catch {
            /* empty */
          }
          void getStatus()
            .then((status) => {
              useSystemConfigStore
                .getState()
                .setConfig(mapStatusDataToConfig(status))
              window.localStorage.setItem('status', JSON.stringify(status))
            })
            .catch(() => {
              /* The next page load retries the public status request. */
            })
        }

        if (STATUS_CHECK_RELATED_KEYS.has(variables.key)) {
          queryClient.invalidateQueries({ queryKey: ['status-check'] })
        }

        toast.success(i18next.t('Setting updated successfully'))
      } else {
        toast.error(data.message || i18next.t('Failed to update setting'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Failed to update setting'))
    },
  })
}
