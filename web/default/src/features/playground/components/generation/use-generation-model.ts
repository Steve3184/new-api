/* Copyright (C) 2023-2026 QuantumNous */
import { useEffect, useState } from 'react'

import type { ModelOption } from '../../types'

export function useGenerationModel(models: ModelOption[]) {
  const [model, setModel] = useState('')

  useEffect(() => {
    if (models.some((option) => option.value === model)) return
    setModel(models[0]?.value ?? '')
  }, [model, models])

  return { model, setModel }
}
