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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { getCustomOAuthBindRewardKey } from '@/features/auth/lib/oauth'
import { formatQuota } from '@/lib/format'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { useCustomOAuthProviders } from './custom-oauth/hooks/use-custom-oauth-providers'

export const OAUTH_BIND_REWARDS_OPTION_KEY = 'oauth_bind_reward.rewards'

const BUILTIN_PROVIDERS = [
  { key: 'github', label: 'GitHub' },
  { key: 'discord', label: 'Discord' },
  { key: 'oidc', label: 'OIDC' },
  { key: 'linuxdo', label: 'LinuxDO' },
  { key: 'telegram', label: 'Telegram' },
  { key: 'wechat', label: 'WeChat' },
]

type RewardValue = number | ''
type OAuthBindRewardFormValues = { rewards: Record<string, RewardValue> }

type OAuthBindRewardSectionProps = {
  /** JSON map of provider key to quota, as stored in the option */
  defaultValue: string
}

function parseRewards(value: string): Record<string, RewardValue> {
  try {
    const parsed: unknown = JSON.parse(value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }
    const rewards: Record<string, RewardValue> = {}
    for (const [key, quota] of Object.entries(parsed)) {
      if (typeof quota === 'number' && quota > 0) rewards[key] = quota
    }
    return rewards
  } catch {
    return {}
  }
}

export function OAuthBindRewardSection(props: OAuthBindRewardSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const customProviders = useCustomOAuthProviders()
  const providers = useMemo(
    () => [
      ...BUILTIN_PROVIDERS,
      ...(customProviders.data ?? []).map((provider) => ({
        key: getCustomOAuthBindRewardKey(provider.id),
        label: provider.name,
      })),
    ],
    [customProviders.data]
  )
  // Every rendered field needs a default, otherwise an untouched empty field
  // counts as a change and the form reports unsaved edits on load.
  const defaultValues = useMemo(() => {
    const rewards = parseRewards(props.defaultValue)
    for (const provider of providers) {
      rewards[provider.key] ??= ''
    }
    return { rewards }
  }, [props.defaultValue, providers])

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<OAuthBindRewardFormValues>({
      defaultValues,
      onSubmit: async (data) => {
        const rewards: Record<string, number> = {}
        for (const [key, quota] of Object.entries(data.rewards ?? {})) {
          if (typeof quota === 'number' && quota > 0) {
            rewards[key] = Math.floor(quota)
          }
        }
        await updateOption.mutateAsync({
          key: OAUTH_BIND_REWARDS_OPTION_KEY,
          value: JSON.stringify(rewards),
        })
      },
    })

  return (
    <SettingsSection title={t('OAuth Binding Rewards')}>
      <FormNavigationGuard when={isDirty} />
      <Alert>
        <AlertDescription>
          {t(
            'One-time quota paid the first time an external account is linked, including sign-up with that provider. Each external account and each user can receive a provider reward only once. Leave empty or 0 to disable.'
          )}
        </AlertDescription>
      </Alert>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            {providers.map((provider) => (
              <FormField
                key={provider.key}
                control={form.control}
                name={`rewards.${provider.key}`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{provider.label}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={field.value ?? ''}
                        onChange={(event) => {
                          const value = event.currentTarget.valueAsNumber
                          field.onChange(Number.isNaN(value) ? '' : value)
                        }}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Binding reward ({{formattedQuota}})', {
                        formattedQuota: formatQuota(
                          typeof field.value === 'number' ? field.value : 0
                        ),
                      })}
                    </FormDescription>
                  </FormItem>
                )}
              />
            ))}
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
