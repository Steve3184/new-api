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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, Trash2 } from 'lucide-react'
import { useState } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
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
import { Switch } from '@/components/ui/switch'

import { clearSupportData } from '../api'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const supportSettingsSchema = z.object({
  SupportEnabled: z.boolean(),
  SupportMessageLimit: z.coerce.number().int().min(1).max(1000),
})

type SupportSettingsValues = z.infer<typeof supportSettingsSchema>

type SupportSettingsSectionProps = {
  defaultValues: SupportSettingsValues
}

export function SupportSettingsSection(props: SupportSettingsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<SupportSettingsValues>({
      resolver: zodResolver(supportSettingsSchema) as Resolver<
        SupportSettingsValues,
        unknown,
        SupportSettingsValues
      >,
      defaultValues: props.defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })
  const clearMutation = useMutation({
    mutationFn: async () => {
      const response = await clearSupportData()
      if (!response.success) {
        throw new Error(response.message || t('Failed to clear support data'))
      }
      return response.data ?? { conversations: 0, messages: 0 }
    },
    onSuccess: (data) => {
      setConfirmOpen(false)
      void queryClient.invalidateQueries({
        predicate: (query) =>
          String(query.queryKey[0] ?? '').startsWith('support'),
      })
      toast.success(
        t(
          'All support data cleared: {{conversations}} conversations and {{messages}} messages deleted.',
          data
        )
      )
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to clear support data')
      )
    },
  })

  return (
    <SettingsSection title={t('Support Settings')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <FormField
            control={form.control}
            name='SupportEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable support inbox')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users and administrators to use the in-site support inbox.'
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
            name='SupportMessageLimit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Support message limit')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={1000}
                    step={1}
                    value={field.value}
                    onChange={(event) => field.onChange(event.target.value)}
                    onBlur={field.onBlur}
                    name={field.name}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Maximum messages retained per support conversation')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>

      <div className='border-border space-y-3 border-t pt-5'>
        <div className='space-y-1'>
          <h4 className='text-sm font-medium'>{t('Data management')}</h4>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Delete all support conversations and messages. Quota grants, subscriptions, and completed orders are not reverted.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='destructive'
          onClick={() => setConfirmOpen(true)}
          disabled={clearMutation.isPending}
        >
          {clearMutation.isPending ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <Trash2 className='size-4' />
          )}
          {t('Clear all support data')}
        </Button>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Clear support data?')}
        desc={t(
          'This permanently deletes every support conversation and message. This action cannot be undone.'
        )}
        confirmText={t('Clear data')}
        destructive
        isLoading={clearMutation.isPending}
        handleConfirm={() => clearMutation.mutate()}
      />
    </SettingsSection>
  )
}
