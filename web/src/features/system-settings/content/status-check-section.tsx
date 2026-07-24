/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { MultiSelect } from '@/components/multi-select'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const statusCheckSchema = z.object({
  groups: z.array(z.string()),
  cacheExcludedModels: z.array(z.string()),
  announcement: z.string().max(50000),
})
type StatusCheckValues = z.infer<typeof statusCheckSchema>

function parseGroupNames(value: string): string[] {
  try {
    const parsed = JSON.parse(value) as unknown
    return Array.isArray(parsed)
      ? parsed.filter((item): item is string => typeof item === 'string')
      : []
  } catch {
    return []
  }
}

function parseAvailableGroups(value: string): string[] {
  try {
    const parsed = JSON.parse(value) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }
    return [...new Set([...Object.keys(parsed), 'auto'])].sort()
  } catch {
    return ['auto']
  }
}

function parseAvailableModels(value: string): string[] {
  try {
    const parsed = JSON.parse(value) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }
    return Object.keys(parsed).sort()
  } catch {
    return []
  }
}

export function StatusCheckSection(props: {
  defaultValue: string
  cacheExcludedModels: string
  announcement: string
  groupRatio: string
  modelRatio: string
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaultGroups = useMemo(
    () => parseGroupNames(props.defaultValue),
    [props.defaultValue]
  )
  const options = useMemo(
    () =>
      parseAvailableGroups(props.groupRatio).map((group) => ({
        label: group,
        value: group,
      })),
    [props.groupRatio]
  )
  const defaultCacheExcludedModels = useMemo(
    () => parseGroupNames(props.cacheExcludedModels),
    [props.cacheExcludedModels]
  )
  const modelOptions = useMemo(
    () =>
      parseAvailableModels(props.modelRatio).map((model) => ({
        label: model,
        value: model,
      })),
    [props.modelRatio]
  )
  const form = useForm<StatusCheckValues>({
    resolver: zodResolver(statusCheckSchema),
    defaultValues: {
      groups: defaultGroups,
      cacheExcludedModels: defaultCacheExcludedModels,
      announcement: props.announcement,
    },
  })

  useEffect(() => {
    form.reset({
      groups: defaultGroups,
      cacheExcludedModels: defaultCacheExcludedModels,
      announcement: props.announcement,
    })
  }, [defaultCacheExcludedModels, defaultGroups, form, props.announcement])

  const onSubmit = async (values: StatusCheckValues) => {
    const updates = [
      {
        key: 'StatusCheckGroups',
        value: JSON.stringify(values.groups),
        initial: JSON.stringify(defaultGroups),
      },
      {
        key: 'StatusCheckCacheExcludedModels',
        value: JSON.stringify(values.cacheExcludedModels),
        initial: JSON.stringify(defaultCacheExcludedModels),
      },
      {
        key: 'StatusCheckAnnouncement',
        value: values.announcement.trim(),
        initial: props.announcement.trim(),
      },
    ].filter((item) => item.value !== item.initial)

    for (const update of updates) {
      await updateOption.mutateAsync({ key: update.key, value: update.value })
    }
  }

  return (
    <SettingsSection title={t('Status Check')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='announcement'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Status check announcement')}</FormLabel>
                <FormControl>
                  <Textarea rows={4} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Markdown content displayed above the status check groups.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='groups'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Visible groups')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    id='status-check-groups'
                    options={options}
                    selected={field.value}
                    onChange={field.onChange}
                    placeholder={t('All active groups')}
                    maxVisibleChips={8}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Leave empty to show every active group on the status page.'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='cacheExcludedModels'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Models excluded from cache hit rate')}
                </FormLabel>
                <FormControl>
                  <MultiSelect
                    id='status-check-cache-excluded-models'
                    options={modelOptions}
                    selected={field.value}
                    onChange={field.onChange}
                    placeholder={t('No excluded models')}
                    emptyText={t('No matching models')}
                    allowCreate
                    maxVisibleChips={8}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Requests from these models remain in availability and latency metrics but are excluded from cache hit rate.'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
