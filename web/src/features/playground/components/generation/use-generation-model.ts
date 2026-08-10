/* Copyright (C) 2023-2026 QuantumNous */
import { useState } from 'react'

import type { GroupOption, ModelOption } from '../../types'
import {
  filterGenerationModelsForGroup,
  resolveGenerationGroup,
} from './generation-utils'

type GenerationModelOptions = {
  models: ModelOption[]
  groups: GroupOption[]
  group: string
  groupModels: Record<string, string[]>
  onGroupChange: (value: string) => void
}

export function useGenerationModel(options: GenerationModelOptions) {
  const [selectedModel, setSelectedModel] = useState('')
  const group = resolveGenerationGroup(
    options.groups,
    options.groupModels,
    options.models,
    options.group
  )
  const visibleModels = filterGenerationModelsForGroup(
    options.models,
    options.groupModels,
    group
  )
  const model = visibleModels.some((option) => option.value === selectedModel)
    ? selectedModel
    : (visibleModels[0]?.value ?? '')

  const setModel = (value: string) => {
    if (!visibleModels.some((option) => option.value === value)) return
    setSelectedModel(value)
  }

  const setGroup = (value: string) => {
    const nextModels = filterGenerationModelsForGroup(
      options.models,
      options.groupModels,
      value
    )
    if (nextModels.length === 0) return
    if (!nextModels.some((option) => option.value === model)) {
      setSelectedModel(nextModels[0].value)
    }
    options.onGroupChange(value)
  }

  return { model, setModel, group, setGroup }
}
