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

import { ComboboxInput } from '@/components/ui/combobox-input'
import { Label } from '@/components/ui/label'
import { getEnabledModels } from '@/features/channels/api'
import { requireServerSuccess } from '@/lib/server-error-message'

/**
 * Picks the one model a group's flexible probe requests. An empty value keeps
 * the previous behavior of testing each channel with its own test model.
 */
export function StatusCheckProbeModelField(props: {
  group: string
  value: string
  disabled: boolean
  onChange: (model: string) => void
}) {
  const { t } = useTranslation()
  // Shares the cache with the degradation model picker of the same group.
  const modelsQuery = useQuery({
    queryKey: ['status-check', 'group-models', props.group],
    queryFn: async () =>
      requireServerSuccess(await getEnabledModels(props.group)),
    staleTime: 60 * 1000,
  })
  const options = useMemo(() => {
    // Keep a saved model selectable even if the group no longer serves it.
    const names = new Set(modelsQuery.data?.data ?? [])
    if (props.value) names.add(props.value)
    return [
      { value: '', label: t('Channel test model') },
      ...[...names].sort().map((name) => ({ value: name, label: name })),
    ]
  }, [modelsQuery.data, props.value, t])
  const controlId = `status-check-probe-model-${props.group}`

  return (
    <div className='space-y-2'>
      <Label htmlFor={controlId}>{t('Probe model')}</Label>
      <ComboboxInput
        id={controlId}
        options={options}
        value={props.value}
        onValueChange={props.onChange}
        placeholder={t('Channel test model')}
        emptyText={t('No matching models')}
        disabled={props.disabled}
      />
      <p className='text-muted-foreground text-xs'>
        {t(
          'Probe one model of this group, routed by channel priority like a real request. Leave as the channel test model to test channels directly.'
        )}
      </p>
    </div>
  )
}
