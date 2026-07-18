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

export function resolveGenerationGroup(
  groups: GroupOption[],
  groupModels: Record<string, string[]>,
  models: ModelOption[],
  selectedModel: string,
  selectedGroup: string
): string {
  const eligibleGroups = filterGenerationGroups(
    groups,
    groupModels,
    models,
    selectedModel
  )
  if (eligibleGroups.some((group) => group.value === selectedGroup)) {
    return selectedGroup
  }
  return eligibleGroups[0]?.value ?? selectedGroup
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

export function normalizeImageAspectRatio(value: string): string | null {
  const match = value.match(/^\s*(\d+)\s*:\s*(\d+)\s*$/)
  if (!match) return null

  let width = Number(match[1])
  let height = Number(match[2])
  if (
    !Number.isSafeInteger(width) ||
    !Number.isSafeInteger(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return null
  }

  let left = width
  let right = height
  while (right !== 0) {
    const remainder = left % right
    left = right
    right = remainder
  }
  width /= left
  height /= left
  return `${width}:${height}`
}

export function imageSizeFromResolution(
  resolution: number,
  aspectRatio: string
): string | null {
  const normalizedRatio = normalizeImageAspectRatio(aspectRatio)
  if (
    !normalizedRatio ||
    !Number.isSafeInteger(resolution) ||
    resolution <= 0
  ) {
    return null
  }

  const [widthUnit, heightUnit] = normalizedRatio.split(':').map(Number)
  const maxUnit = Math.max(widthUnit, heightUnit)
  const rawScale = Math.floor(resolution / maxUnit)
  const alignedScale = Math.floor(rawScale / 8) * 8
  const scale = alignedScale || rawScale

  if (scale > 0) {
    return `${widthUnit * scale}x${heightUnit * scale}`
  }

  if (widthUnit > heightUnit) {
    return `${resolution}x${Math.max(1, Math.round((resolution * heightUnit) / widthUnit))}`
  }
  return `${Math.max(1, Math.round((resolution * widthUnit) / heightUnit))}x${resolution}`
}

export async function workspaceImageToFile(
  source: string,
  baseName: string
): Promise<File> {
  const response = await fetch(source)
  if (!response.ok) throw new Error('File read failed')
  const blob = await response.blob()
  const mimeType = blob.type || 'image/png'
  const extension = mimeType.split('/')[1]?.split('+')[0] || 'png'
  return new File([blob], `${baseName}.${extension}`, { type: mimeType })
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
