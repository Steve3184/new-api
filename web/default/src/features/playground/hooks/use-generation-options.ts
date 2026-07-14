/* Copyright (C) 2023-2026 QuantumNous */
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { getUserModels } from '../api'
import type { GroupOption, ModelOption } from '../types'

export function useGenerationOptions(groups: GroupOption[]) {
  const groupValues = useMemo(
    () => groups.map((group) => group.value).sort(),
    [groups]
  )
  const query = useQuery({
    queryKey: ['playground-generation-models', ...groupValues],
    queryFn: async () =>
      Promise.all(
        groupValues.map(async (group) => ({
          group,
          models: await getUserModels(group),
        }))
      ),
    enabled: groupValues.length > 0,
    staleTime: 5 * 60 * 1000,
  })

  return useMemo(() => {
    const modelsByGroup: Record<string, string[]> = {}
    const uniqueModels = new Map<string, ModelOption>()
    for (const entry of query.data ?? []) {
      modelsByGroup[entry.group] = entry.models.map((model) => model.value)
      for (const model of entry.models) uniqueModels.set(model.value, model)
    }
    return {
      groupModels: modelsByGroup,
      models: [...uniqueModels.values()].sort((a, b) =>
        a.label.localeCompare(b.label)
      ),
      isLoading: query.isLoading,
    }
  }, [query.data, query.isLoading])
}
