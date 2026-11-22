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
import { zodResolver } from '@hookform/resolvers/zod'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { AuthPageStylePicker } from './auth-page-style-picker'

const authPagesSchema = z.object({
  authPageStyle: z.enum(['centered', 'split']),
  authBackgroundEnabled: z.boolean(),
  authSplitImage: z.string(),
})

export type AuthPagesFormValues = z.infer<typeof authPagesSchema>

const OPTION_KEYS: Record<keyof AuthPagesFormValues, string> = {
  authPageStyle: 'console_setting.auth_page_style',
  authBackgroundEnabled: 'console_setting.auth_background_enabled',
  authSplitImage: 'console_setting.auth_split_image',
}

export function AuthPagesSection(props: {
  defaultValues: AuthPagesFormValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const schema = authPagesSchema.extend({
    authSplitImage: z
      .string()
      .refine(
        (value) =>
          !value ||
          (value.startsWith('/') && !value.startsWith('//')) ||
          /^https?:\/\//i.test(value),
        t('Use an HTTP(S) URL or a path starting with /')
      ),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<AuthPagesFormValues>({
      resolver: zodResolver(schema) as Resolver<AuthPagesFormValues>,
      defaultValues: props.defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [name, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key: OPTION_KEYS[name as keyof AuthPagesFormValues],
            value: String(value),
          })
        }
      },
    })

  const pageStyle = form.watch('authPageStyle')

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Login & Register')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='authPageStyle'
                render={({ field }) => (
                  <FormItem data-settings-form-span='full'>
                    <FormLabel>{t('Login page style')}</FormLabel>
                    <AuthPageStylePicker
                      value={field.value}
                      onChange={field.onChange}
                    />
                    <FormDescription>
                      {t(
                        'The split layout shows a branded panel beside the form on wide screens.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              {pageStyle === 'split' && (
                <FormField
                  control={form.control}
                  name='authSplitImage'
                  render={({ field }) => (
                    <FormItem data-settings-form-span='full'>
                      <FormLabel>{t('Split layout illustration')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder='/_custom/img/login-illustration.webp'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Leave empty to use the animated particle background.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
              {pageStyle === 'centered' && (
                <FormField
                  control={form.control}
                  name='authBackgroundEnabled'
                  render={({ field }) => (
                    <SettingsSwitchField
                      controlId='auth-background-enabled'
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      label={t(
                        'Show background image on login and register pages'
                      )}
                      description={t(
                        'Apply the configured background image to the sign-in and sign-up pages.'
                      )}
                    />
                  )}
                />
              )}
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
