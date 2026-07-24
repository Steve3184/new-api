/* Copyright (C) 2023-2026 QuantumNous */
import { useState } from 'react'

import type { GroupOption, ModelOption } from '../../types'
import {
  filterGroupsForGenerationModel,
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
  const model = options.models.some((option) => option.value === selectedModel)
    ? selectedModel
    : (options.models[0]?.value ?? '')
  const group = resolveGenerationGroup(
    options.groups,
    options.groupModels,
    options.models,
    model,
    options.group
  )

  const setModel = (value: string) => {
    setSelectedModel(value)
    const nextGroups = filterGroupsForGenerationModel(
      options.groups,
      options.groupModels,
      value
    )
    if (
      nextGroups.length > 0 &&
      !nextGroups.some((option) => option.value === options.group)
    ) {
      options.onGroupChange(nextGroups[0].value)
    }
  }

  const setGroup = (value: string) => {
    const groupModels = new Set(options.groupModels[value] ?? [])
    if (!groupModels.has(model)) {
      const nextModel = options.models.find((option) =>
        groupModels.has(option.value)
      )
      if (!nextModel) return
      setSelectedModel(nextModel.value)
    }
    options.onGroupChange(value)
  }

  return { model, setModel, group, setGroup }
}
