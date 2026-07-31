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
import { Input } from '@/components/ui/input'
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

const statusCheckSchema = z.object({
  groups: z.array(z.string()),
  cacheExcludedModels: z.array(z.string()),
  announcement: z.string().max(50000),
  flexibleGroups: z.array(
    z.object({
      group: z.string().min(1).max(64),
      enabled: z.boolean(),
      idle_minutes: z.number().int().min(1).max(1440),
      max_consecutive_probes: z.number().int().min(1).max(1000),
    })
  ),
})
type StatusCheckValues = z.infer<typeof statusCheckSchema>

type FlexibleGroupConfig = {
  enabled: boolean
  idle_minutes: number
  max_consecutive_probes: number
}

type FlexibleGroupFormValue = FlexibleGroupConfig & {
  group: string
}

type FlexibleModeConfig = {
  groups: Record<string, FlexibleGroupConfig>
}

const defaultFlexibleGroup: FlexibleGroupConfig = {
  enabled: false,
  idle_minutes: 15,
  max_consecutive_probes: 40,
}

const defaultFlexibleMode: FlexibleModeConfig = { groups: {} }

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

function parseFlexibleMode(value: string): FlexibleModeConfig {
  try {
    const parsed = JSON.parse(value) as { groups?: unknown }
    if (!parsed.groups || typeof parsed.groups !== 'object' || Array.isArray(parsed.groups)) {
      return defaultFlexibleMode
    }
    const groups: Record<string, FlexibleGroupConfig> = {}
    for (const [group, config] of Object.entries(parsed.groups)) {
      if (!config || typeof config !== 'object' || Array.isArray(config)) {
        continue
      }
      const candidate = config as Partial<FlexibleGroupConfig>
      if (
        typeof candidate.enabled !== 'boolean' ||
        typeof candidate.idle_minutes !== 'number' ||
        !Number.isInteger(candidate.idle_minutes) ||
        candidate.idle_minutes < 1 ||
        candidate.idle_minutes > 1440 ||
        typeof candidate.max_consecutive_probes !== 'number' ||
        !Number.isInteger(candidate.max_consecutive_probes) ||
        candidate.max_consecutive_probes < 1 ||
        candidate.max_consecutive_probes > 1000
      ) {
        continue
      }
      groups[group] = {
        enabled: candidate.enabled,
        idle_minutes: candidate.idle_minutes,
        max_consecutive_probes: candidate.max_consecutive_probes,
      }
    }
    return { groups }
  } catch {
    return defaultFlexibleMode
  }
}

export function StatusCheckSection(props: {
  defaultValue: string
  cacheExcludedModels: string
  announcement: string
  flexibleMode: string
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
  const defaultFlexibleConfig = useMemo(
    () => parseFlexibleMode(props.flexibleMode),
    [props.flexibleMode]
  )
  const defaultFlexibleGroups = useMemo(
    () =>
      Object.entries(defaultFlexibleConfig.groups).map(([group, config]) => ({
        group,
        ...config,
      })),
    [defaultFlexibleConfig]
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
      flexibleGroups: defaultFlexibleGroups,
    },
  })
  const selectedGroups = form.watch('groups')
  const flexibleGroups = form.watch('flexibleGroups')
  const selectableGroupNames = useMemo(
    () => new Set(options.map((option) => option.value)),
    [options]
  )
  const visibleProbeGroups = selectedGroups.filter(
    (group) => group !== 'auto' && selectableGroupNames.has(group)
  )

  useEffect(() => {
    form.reset({
      groups: defaultGroups,
      cacheExcludedModels: defaultCacheExcludedModels,
      announcement: props.announcement,
      flexibleGroups: defaultFlexibleGroups,
    })
  }, [
    defaultCacheExcludedModels,
    defaultFlexibleGroups,
    defaultGroups,
    form,
    props.announcement,
  ])

  const getFlexibleGroupConfig = (group: string): FlexibleGroupConfig => {
    const config = flexibleGroups.find((candidate) => candidate.group === group)
    return config ?? defaultFlexibleGroup
  }

  const updateFlexibleGroupConfig = (
    group: string,
    update: Partial<FlexibleGroupConfig>
  ) => {
    const current = getFlexibleGroupConfig(group)
    const nextConfig: FlexibleGroupFormValue = {
      group,
      ...current,
      ...update,
    }
    const nextGroups = flexibleGroups.some(
      (candidate) => candidate.group === group
    )
      ? flexibleGroups.map((candidate) =>
          candidate.group === group ? nextConfig : candidate
        )
      : [...flexibleGroups, nextConfig]
    form.setValue('flexibleGroups', nextGroups, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const onSubmit = async (values: StatusCheckValues) => {
    const flexibleGroupSettings = values.groups.reduce<
      Record<string, FlexibleGroupConfig>
    >((settings, group) => {
      if (group === 'auto' || !selectableGroupNames.has(group)) {
        return settings
      }
      const config = values.flexibleGroups.find(
        (candidate) => candidate.group === group
      )
      settings[group] = config
        ? {
            enabled: config.enabled,
            idle_minutes: config.idle_minutes,
            max_consecutive_probes: config.max_consecutive_probes,
          }
        : defaultFlexibleGroup
      return settings
    }, {})
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
      {
        key: 'StatusCheckFlexibleMode',
        value: JSON.stringify({ groups: flexibleGroupSettings }),
        initial: JSON.stringify(defaultFlexibleConfig),
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
          <div className='space-y-4'>
            <div className='space-y-1'>
              <FormLabel>{t('Flexible active probes')}</FormLabel>
              <FormDescription>
                {t(
                  'Configure each visible group independently. A probe tests one enabled channel only after that group has been idle for its configured period.'
                )}
              </FormDescription>
            </div>
            {visibleProbeGroups.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {t('Select one or more visible groups to configure probes.')}
              </p>
            ) : (
              visibleProbeGroups.map((group) => {
                const config = getFlexibleGroupConfig(group)
                return (
                  <div
                    key={group}
                    className='border-border space-y-4 rounded-lg border p-4'
                  >
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{group}</FormLabel>
                        <FormDescription>
                          {t(
                            'Probe this group only when it has no real relay requests.'
                          )}
                        </FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={config.enabled}
                          onCheckedChange={(enabled) =>
                            updateFlexibleGroupConfig(group, { enabled })
                          }
                          aria-label={`${t('Enable')} ${group}`}
                        />
                      </FormControl>
                    </SettingsSwitchItem>
                    <div className='grid gap-4 sm:grid-cols-2'>
                      <FormItem>
                        <FormLabel>{t('Idle period (minutes)')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            max={1440}
                            step={1}
                            disabled={!config.enabled}
                            value={config.idle_minutes}
                            onChange={(event) => {
                              if (Number.isFinite(event.target.valueAsNumber)) {
                                updateFlexibleGroupConfig(group, {
                                  idle_minutes: event.target.valueAsNumber,
                                })
                              }
                            }}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Minimum time without a relay request before probing.'
                          )}
                        </FormDescription>
                      </FormItem>
                      <FormItem>
                        <FormLabel>
                          {t('Maximum consecutive probes')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            max={1000}
                            step={1}
                            disabled={!config.enabled}
                            value={config.max_consecutive_probes}
                            onChange={(event) => {
                              if (Number.isFinite(event.target.valueAsNumber)) {
                                updateFlexibleGroupConfig(group, {
                                  max_consecutive_probes:
                                    event.target.valueAsNumber,
                                })
                              }
                            }}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'After this many automatic probes without a real relay request, probing pauses until traffic resumes.'
                          )}
                        </FormDescription>
                      </FormItem>
                    </div>
                  </div>
                )
              })
            )}
            <p className='text-muted-foreground text-sm'>
              {t(
                'Flexible probes never affect cache hit rate and do not change channel state.'
              )}
            </p>
          </div>
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
