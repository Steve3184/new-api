/* Copyright (C) 2023-2026 QuantumNous */
import axios from 'axios'

import type { GroupOption, ModelOption } from '../../types'

export function filterGenerationGroups(
  groups: GroupOption[],
  groupModels: Record<string, string[]>,
  models: ModelOption[],
  selectedModel: string
): GroupOption[] {
  const allowedModels = new Set(models.map((model) => model.value))
  return groups.filter((group) => {
    const availableModels = groupModels[group.value] ?? []
    if (selectedModel) return availableModels.includes(selectedModel)
    return availableModels.some((model) => allowedModels.has(model))
  })
}

export function filterGenerationModels(
  models: ModelOption[],
  allowedModels: string[]
): ModelOption[] {
  if (allowedModels.length === 0) return models
  const allowed = new Set(allowedModels)
  return models.filter((model) => allowed.has(model.value))
}

export function imageResponseSource(image: {
  url?: string
  b64_json?: string
}): string {
  if (image.url) return image.url
  return image.b64_json ? `data:image/png;base64,${image.b64_json}` : ''
}

export async function getGenerationErrorMessage(
  error: unknown,
  fallback: string
): Promise<string> {
  if (!axios.isAxiosError(error)) {
    return error instanceof Error ? error.message : fallback
  }
  const data = error.response?.data
  if (data instanceof Blob) {
    try {
      const parsed = JSON.parse(await data.text()) as {
        error?: { message?: string }
        message?: string
      }
      return parsed.error?.message ?? parsed.message ?? fallback
    } catch {
      return fallback
    }
  }
  if (data && typeof data === 'object') {
    const body = data as { error?: { message?: string }; message?: string }
    return body.error?.message ?? body.message ?? fallback
  }
  return error.message || fallback
}

export function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener(
      'load',
      () => resolve(String(reader.result ?? '')),
      {
        once: true,
      }
    )
    reader.addEventListener(
      'error',
      () => reject(reader.error ?? new Error('File read failed')),
      { once: true }
    )
    reader.readAsDataURL(file)
  })
}
