/* Copyright (C) 2023-2026 QuantumNous */
import { useMemo } from 'react'

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

export function GenerationControls(props: GenerationControlsProps) {
  const eligibleGroups = useMemo(
    () =>
      filterGenerationGroups(
        props.groups,
        props.groupModels,
        props.models,
        props.model
      ),
    [props.groupModels, props.groups, props.model, props.models]
  )

  return (
    <ModelGroupSelector
      selectedModel={props.model}
      models={props.models}
      onModelChange={props.onModelChange}
      selectedGroup={props.group}
      groups={eligibleGroups}
      onGroupChange={props.onGroupChange}
      className='h-10 w-full max-w-none justify-start font-mono'
      disabled={props.disabled || props.models.length === 0}
    />
  )
}
