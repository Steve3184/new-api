/* Copyright (C) 2023-2026 QuantumNous */
import { useEffect, useMemo } from 'react'

import { ModelGroupSelector } from '@/components/model-group-selector'

import type { GroupOption, ModelOption } from '../../types'
import { filterGenerationGroups } from './generation-utils'

type GenerationControlsProps = {
  groups: GroupOption[]
  group: string
  onGroupChange: (value: string) => void
  models: ModelOption[]
  model: string
  onModelChange: (value: string) => void
  groupModels: Record<string, string[]>
  disabled?: boolean
}

export function GenerationControls({
  groups,
  group,
  onGroupChange,
  models,
  model,
  onModelChange,
  groupModels,
  disabled,
}: GenerationControlsProps) {
  const eligibleGroups = useMemo(
    () => filterGenerationGroups(groups, groupModels, models, model),
    [groupModels, groups, model, models]
  )

  useEffect(() => {
    if (
      eligibleGroups.length === 0 ||
      eligibleGroups.some((option) => option.value === group)
    ) {
      return
    }
    onGroupChange(eligibleGroups[0].value)
  }, [eligibleGroups, group, onGroupChange])

  return (
    <ModelGroupSelector
      selectedModel={model}
      models={models}
      onModelChange={onModelChange}
      selectedGroup={group}
      groups={eligibleGroups}
      onGroupChange={onGroupChange}
      className='h-10 w-full max-w-none justify-start font-mono'
      disabled={disabled || models.length === 0}
    />
  )
}
