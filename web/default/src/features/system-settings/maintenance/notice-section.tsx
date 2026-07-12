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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const noticeSchema = z.object({
  Notice: z.string().optional(),
  NoticePopupEnabled: z.boolean(),
  NoticePopupOnDashboardEnabled: z.boolean(),
})

type NoticeFormValues = z.infer<typeof noticeSchema>

type NoticeSectionProps = {
  defaultValues: NoticeFormValues
}

export function NoticeSection({ defaultValues }: NoticeSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaultNotice = defaultValues.Notice ?? ''
  const defaultPopupEnabled = defaultValues.NoticePopupEnabled
  const defaultDashboardPopupEnabled =
    defaultValues.NoticePopupOnDashboardEnabled
  const form = useForm<NoticeFormValues>({
    resolver: zodResolver(noticeSchema),
    defaultValues: {
      Notice: defaultNotice,
      NoticePopupEnabled: defaultPopupEnabled,
      NoticePopupOnDashboardEnabled: defaultDashboardPopupEnabled,
    },
  })

  useEffect(() => {
    form.reset({
      Notice: defaultNotice,
      NoticePopupEnabled: defaultPopupEnabled,
      NoticePopupOnDashboardEnabled: defaultDashboardPopupEnabled,
    })
  }, [defaultDashboardPopupEnabled, defaultNotice, defaultPopupEnabled, form])

  const onSubmit = async (values: NoticeFormValues) => {
    const normalizedValues: NoticeFormValues = {
      ...values,
      Notice: values.Notice ?? '',
    }
    const initialValues: NoticeFormValues = {
      Notice: defaultNotice,
      NoticePopupEnabled: defaultPopupEnabled,
      NoticePopupOnDashboardEnabled: defaultDashboardPopupEnabled,
    }
    const updates = Object.entries(normalizedValues).filter(
      ([key, value]) => value !== initialValues[key as keyof NoticeFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  const popupEnabled = form.watch('NoticePopupEnabled')

  return (
    <SettingsSection title={t('System Notice')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save notice'
          />
          <FormField
            control={form.control}
            name='Notice'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Announcement content')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={8}
                    placeholder={t(
                      'Planned maintenance on Friday at 22:00 UTC...'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='NoticePopupEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Show notice as a popup')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Show the system notice whenever users open the home page'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='NoticePopupOnDashboardEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Also show on the overview dashboard')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Show the notice whenever users open the backend overview page'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    disabled={!popupEnabled}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
