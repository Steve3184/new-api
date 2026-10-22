/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getEnabledModels } from '@/features/channels/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import { SettingsSwitchField } from '../components/settings-form-layout'
import {
  DEGRADATION_EFFORTS,
  DEGRADATION_INTERVAL_MAX,
  DEGRADATION_INTERVAL_MIN,
  DEGRADATION_MAX_MODELS,
  type DegradationGroupConfig,
} from './status-check-degradation-options'

// Select items cannot use an empty value, so the model default uses a sentinel.
const DEFAULT_EFFORT_VALUE = 'default'

export function StatusCheckDegradationConfig(props: {
  group: string
  value: DegradationGroupConfig
  onChange: (update: Partial<DegradationGroupConfig>) => void
}) {
  const { t } = useTranslation()
  const modelsQuery = useQuery({
    queryKey: ['status-check', 'group-models', props.group],
    queryFn: async () =>
      requireServerSuccess(await getEnabledModels(props.group)),
    staleTime: 60 * 1000,
  })
  const selectedModels = props.value.models.map((entry) => entry.model)
  const modelOptions = useMemo(() => {
    // Keep saved models selectable even if the group no longer serves them.
    const names = new Set([
      ...(modelsQuery.data?.data ?? []),
      ...props.value.models.map((entry) => entry.model),
    ])
    return [...names].sort().map((name) => ({ label: name, value: name }))
  }, [modelsQuery.data, props.value.models])
  const effortItems = DEGRADATION_EFFORTS.map((effort) => ({
    value: effort || DEFAULT_EFFORT_VALUE,
    label: effort || t('Model default'),
  }))
  const controlId = `status-check-degradation-${props.group}`

  return (
    <div className='space-y-4 border-t pt-4'>
      <SettingsSwitchField
        controlId={controlId}
        checked={props.value.enabled}
        onCheckedChange={(enabled) => props.onChange({ enabled })}
        label={t('Model degradation detection')}
        description={t(
          'Periodically asks each selected model a reasoning question whose correct answer is 21. Degraded models usually answer 29 or another number.'
        )}
      />
      <div className='grid gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor={`${controlId}-interval`}>
            {t('Check interval (minutes)')}
          </Label>
          <Input
            id={`${controlId}-interval`}
            type='number'
            min={DEGRADATION_INTERVAL_MIN}
            max={DEGRADATION_INTERVAL_MAX}
            step={1}
            disabled={!props.value.enabled}
            value={props.value.interval_minutes}
            onChange={(event) => {
              if (Number.isFinite(event.target.valueAsNumber)) {
                props.onChange({
                  interval_minutes: event.target.valueAsNumber,
                })
              }
            }}
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Each check spends reasoning tokens. Longer intervals reduce cost.'
            )}
          </p>
        </div>
        <div className='space-y-2'>
          <Label htmlFor={`${controlId}-models`}>{t('Models to check')}</Label>
          <MultiSelect
            id={`${controlId}-models`}
            options={modelOptions}
            selected={selectedModels}
            onChange={(models) => {
              const next = models.slice(0, DEGRADATION_MAX_MODELS).map(
                (model) =>
                  props.value.models.find((entry) => entry.model === model) ?? {
                    model,
                    effort: '',
                  }
              )
              props.onChange({ models: next })
            }}
            placeholder={t('Select models in this group')}
            emptyText={t('No matching models')}
            disabled={!props.value.enabled}
            maxVisibleChips={6}
          />
        </div>
      </div>
      {props.value.models.length > 0 && (
        <div className='space-y-2'>
          <Label>{t('Reasoning effort')}</Label>
          <ul className='divide-y rounded-md border'>
            {props.value.models.map((entry) => (
              <li
                key={entry.model}
                className='flex items-center justify-between gap-3 px-3 py-2'
              >
                <span className='min-w-0 truncate font-mono text-sm'>
                  {entry.model}
                </span>
                <Select
                  items={effortItems}
                  value={entry.effort || DEFAULT_EFFORT_VALUE}
                  disabled={!props.value.enabled}
                  onValueChange={(value) => {
                    const effort =
                      value === DEFAULT_EFFORT_VALUE ? '' : String(value)
                    props.onChange({
                      models: props.value.models.map((candidate) =>
                        candidate.model === entry.model
                          ? { ...candidate, effort }
                          : candidate
                      ),
                    })
                  }}
                >
                  <SelectTrigger
                    size='sm'
                    className='w-36 shrink-0'
                    aria-label={t('Reasoning effort for {{model}}', {
                      model: entry.model,
                    })}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {effortItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
