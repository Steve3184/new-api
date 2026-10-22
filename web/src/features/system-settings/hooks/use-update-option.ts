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
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { updatePasskeyDomains, updateSystemOption } from '../api'
import type { UpdateOptionRequest, UpdatePasskeyDomainsRequest } from '../types'

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
  'SupportEnabled',
  'LogConsumeEnabled',
  'log_setting.user_export_enabled',
  'log_setting.max_range_days',
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
  'checkin_setting.min_used_quota',
  'checkin_setting.daily_user_limit',
  'checkin_setting.daily_quota_limit',
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
  'console_setting.default_theme_override',
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
  'console_setting.homepage_style',
  'console_setting.homepage_preset_title_mode',
  'console_setting.homepage_preset_sla_enabled',
  'console_setting.homepage_preset_sla_text',
  'ServerAddress',
  'passkey.enabled',
  'passkey.rp_id',
  'passkey.legacy_rp_ids',
  'passkey.origins',
  'QuotaRemindEnabled',
  'QuotaRemindThreshold',
])

const STATUS_CHECK_RELATED_KEYS = new Set([
  'StatusCheckGroups',
  'StatusCheckCacheExcludedModels',
  'StatusCheckFlexibleMode',
  'StatusCheckDegradationMode',
])

export function useUpdateOption() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (request: UpdateOptionRequest) =>
      requireServerSuccess(await updateSystemOption(request)),
    onSuccess: (_data, variables) => {
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
    },
    onError: (error: Error) => {
      handleServerError(error, i18next.t('Failed to update setting'))
    },
  })
}

export function useUpdatePasskeyDomains() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (request: UpdatePasskeyDomainsRequest) => {
      const result = await updatePasskeyDomains(request)
      if (
        result.code === 'PASSKEY_RP_ID_REMOVAL_CONFIRMATION_REQUIRED' &&
        result.data
      ) {
        return result
      }
      return requireServerSuccess(result)
    },
    onSuccess: (result, request) => {
      if (request.preview || !result.success) return
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      queryClient.invalidateQueries({ queryKey: ['status'] })
      try {
        window.localStorage.removeItem('status')
      } catch {
        /* Storage may be disabled. */
      }
      toast.success(i18next.t('Setting updated successfully'))
    },
    onError: (error: Error) =>
      handleServerError(error, i18next.t('Failed to update setting')),
  })
}
