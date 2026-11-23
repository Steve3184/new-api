/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import {
  DEGRADATION_EFFORTS,
  DEGRADATION_INTERVAL_MAX,
  DEGRADATION_INTERVAL_MIN,
  DEGRADATION_MAX_MODELS,
  type DegradationGroupConfig,
  type DegradationModelConfig,
  defaultDegradationGroup,
} from './status-check-degradation-options'
import {
  FLEXIBLE_CONTINUOUS_PROBES,
  FLEXIBLE_IDLE_MAX,
  FLEXIBLE_IDLE_MIN,
  FLEXIBLE_PROBES_MAX,
  type FlexibleGroupConfig,
  defaultFlexibleGroup,
  parseFlexibleMode,
} from './status-check-flexible-options'
import {
  StatusCheckGroupSheet,
  type StatusCheckGroupSheetApply,
} from './status-check-group-sheet'
import {
  type StatusCheckGroupRow,
  StatusCheckGroupTable,
} from './status-check-group-table'

const statusCheckSchema = z.object({
  groups: z.array(z.string()),
  cacheExcludedModels: z.array(z.string()),
  announcement: z.string().max(50000),
  flexibleGroups: z.array(
    z.object({
      group: z.string().min(1).max(64),
      enabled: z.boolean(),
      idle_minutes: z
        .number()
        .int()
        .min(FLEXIBLE_IDLE_MIN)
        .max(FLEXIBLE_IDLE_MAX),
      // 0 is continuous monitoring; any other value is a probe limit.
      max_consecutive_probes: z
        .number()
        .int()
        .min(FLEXIBLE_CONTINUOUS_PROBES)
        .max(FLEXIBLE_PROBES_MAX),
      model: z.string().max(255),
    })
  ),
  degradationGroups: z.array(
    z.object({
      group: z.string().min(1).max(64),
      enabled: z.boolean(),
      interval_minutes: z
        .number()
        .int()
        .min(DEGRADATION_INTERVAL_MIN)
        .max(DEGRADATION_INTERVAL_MAX),
      models: z
        .array(
          z.object({
            model: z.string().min(1).max(255),
            effort: z
              .string()
              .refine((effort) =>
                (DEGRADATION_EFFORTS as readonly string[]).includes(effort)
              ),
          })
        )
        .max(DEGRADATION_MAX_MODELS),
    })
  ),
})
type StatusCheckValues = z.infer<typeof statusCheckSchema>

type FlexibleGroupFormValue = FlexibleGroupConfig & {
  group: string
}

type DegradationModeConfig = {
  groups: Record<string, DegradationGroupConfig>
}

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

function parseDegradationMode(value: string): DegradationModeConfig {
  try {
    const parsed = JSON.parse(value) as { groups?: unknown }
    if (
      !parsed.groups ||
      typeof parsed.groups !== 'object' ||
      Array.isArray(parsed.groups)
    ) {
      return { groups: {} }
    }
    const groups: Record<string, DegradationGroupConfig> = {}
    for (const [group, config] of Object.entries(parsed.groups)) {
      if (!config || typeof config !== 'object' || Array.isArray(config)) {
        continue
      }
      const candidate = config as Partial<DegradationGroupConfig>
      const models = Array.isArray(candidate.models)
        ? candidate.models.filter(
            (entry): entry is DegradationModelConfig =>
              typeof entry?.model === 'string' &&
              (DEGRADATION_EFFORTS as readonly string[]).includes(
                entry.effort ?? ''
              )
          )
        : []
      groups[group] = {
        enabled: candidate.enabled === true,
        interval_minutes:
          typeof candidate.interval_minutes === 'number' &&
          Number.isInteger(candidate.interval_minutes)
            ? candidate.interval_minutes
            : defaultDegradationGroup.interval_minutes,
        models: models.map((entry) => ({
          model: entry.model,
          effort: entry.effort ?? '',
        })),
      }
    }
    return { groups }
  } catch {
    return { groups: {} }
  }
}

export function StatusCheckSection(props: {
  defaultValue: string
  cacheExcludedModels: string
  announcement: string
  flexibleMode: string
  degradationMode: string
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
  const defaultDegradationConfig = useMemo(
    () => parseDegradationMode(props.degradationMode),
    [props.degradationMode]
  )
  const defaultDegradationGroups = useMemo(
    () =>
      Object.entries(defaultDegradationConfig.groups).map(
        ([group, config]) => ({ group, ...config })
      ),
    [defaultDegradationConfig]
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
      degradationGroups: defaultDegradationGroups,
    },
  })
  const selectedGroups = form.watch('groups')
  const flexibleGroups = form.watch('flexibleGroups')
  const degradationGroups = form.watch('degradationGroups')
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
      degradationGroups: defaultDegradationGroups,
    })
  }, [
    defaultCacheExcludedModels,
    defaultDegradationGroups,
    defaultFlexibleGroups,
    defaultGroups,
    form,
    props.announcement,
  ])

  const getFlexibleGroupConfig = (group: string): FlexibleGroupConfig =>
    flexibleGroups.find((candidate) => candidate.group === group) ??
    defaultFlexibleGroup

  const getDegradationGroupConfig = (group: string): DegradationGroupConfig =>
    degradationGroups.find((candidate) => candidate.group === group) ??
    defaultDegradationGroup

  // Batch edits update several groups at once, so each write reads the
  // latest form value instead of the array captured by this render.
  const updateFlexibleGroups = (
    groups: string[],
    update: Partial<FlexibleGroupConfig>
  ) => {
    const current = form.getValues('flexibleGroups')
    const next: FlexibleGroupFormValue[] = current.map((candidate) =>
      groups.includes(candidate.group) ? { ...candidate, ...update } : candidate
    )
    for (const group of groups) {
      if (!current.some((candidate) => candidate.group === group)) {
        next.push({ ...defaultFlexibleGroup, ...update, group })
      }
    }
    form.setValue('flexibleGroups', next, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const updateDegradationGroup = (
    group: string,
    config: DegradationGroupConfig
  ) => {
    const current = form.getValues('degradationGroups')
    const nextConfig = { ...config, group }
    const next = current.some((candidate) => candidate.group === group)
      ? current.map((candidate) =>
          candidate.group === group ? nextConfig : candidate
        )
      : [...current, nextConfig]
    form.setValue('degradationGroups', next, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const [selectedRows, setSelectedRows] = useState<string[]>([])
  // The sheet stays mounted while it closes; a new session remounts it so its
  // draft starts from the current values of the groups being edited.
  const [editor, setEditor] = useState<{
    groups: string[]
    session: number
    open: boolean
  } | null>(null)
  const openEditor = (groups: string[]) =>
    setEditor((current) => ({
      groups,
      session: (current?.session ?? 0) + 1,
      open: true,
    }))
  const applyEditor = (update: StatusCheckGroupSheetApply) => {
    if (!editor) return
    updateFlexibleGroups(editor.groups, update.flexible)
    if (update.degradation) {
      updateDegradationGroup(editor.groups[0], update.degradation)
    }
  }

  const visibleSelectedRows = selectedRows.filter((group) =>
    visibleProbeGroups.includes(group)
  )
  const groupRows: StatusCheckGroupRow[] = visibleProbeGroups.map((group) => ({
    group,
    flexible: getFlexibleGroupConfig(group),
    degradation: getDegradationGroupConfig(group),
    invalid:
      !!form.formState.errors.flexibleGroups?.[
        flexibleGroups.findIndex((candidate) => candidate.group === group)
      ] ||
      !!form.formState.errors.degradationGroups?.[
        degradationGroups.findIndex((candidate) => candidate.group === group)
      ],
  }))

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
            model: config.model.trim(),
          }
        : defaultFlexibleGroup
      return settings
    }, {})
    const degradationGroupSettings: Record<string, DegradationGroupConfig> = {}
    for (const config of values.degradationGroups) {
      if (
        config.group === 'auto' ||
        !selectableGroupNames.has(config.group) ||
        !values.groups.includes(config.group)
      ) {
        continue
      }
      degradationGroupSettings[config.group] = {
        enabled: config.enabled,
        interval_minutes: config.interval_minutes,
        models: config.models,
      }
    }
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
      {
        key: 'StatusCheckDegradationMode',
        value: JSON.stringify({ groups: degradationGroupSettings }),
        initial: JSON.stringify(defaultDegradationConfig),
      },
    ].filter((item) => item.value !== item.initial)

    for (const update of updates) {
      await updateOption.mutateAsync({ key: update.key, value: update.value })
    }
  }

  // The per-group controls are not registered fields, so a failed
  // validation must be reported here or the save silently does nothing.
  const submit = form.handleSubmit(onSubmit, () => {
    toast.error(t('Please fix the highlighted fields before saving'))
  })
  return (
    <SettingsSection title={t('Status Check')}>
      <Form {...form}>
        <SettingsForm onSubmit={submit}>
          <SettingsPageFormActions
            onSave={submit}
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
                  'Configure each visible group independently. A probe runs only after that group has been idle for its configured period, falling back through channel priorities like a real request; only the final attempt counts.'
                )}
              </FormDescription>
            </div>
            <StatusCheckGroupTable
              rows={groupRows}
              selected={visibleSelectedRows}
              onSelectedChange={setSelectedRows}
              onProbeToggle={(group, enabled) =>
                updateFlexibleGroups([group], { enabled })
              }
              onEdit={openEditor}
            />
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
      {editor && (
        <StatusCheckGroupSheet
          key={editor.session}
          open={editor.open}
          onOpenChange={(open) =>
            setEditor((current) => current && { ...current, open })
          }
          groups={editor.groups}
          flexible={getFlexibleGroupConfig(editor.groups[0])}
          degradation={getDegradationGroupConfig(editor.groups[0])}
          onApply={applyEditor}
        />
      )}
    </SettingsSection>
  )
}
