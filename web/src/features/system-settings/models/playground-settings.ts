/* Copyright (C) 2023-2026 QuantumNous */
import * as z from 'zod'

export const playgroundFeatureSchema = z.enum([
  'chat',
  'image',
  'speech',
  'three_d',
  'video',
])
export const chatPresetSchema = z.object({
  icon: z
    .string()
    .trim()
    .max(80)
    .refine((value) => !value || /^[A-Z][A-Za-z0-9]*$/.test(value)),
  title: z.string().trim().min(1).max(80),
  content: z
    .string()
    .min(1)
    .max(8000)
    .refine((value) => value.trim().length > 0),
})

export const playgroundSettingsSchema = z.object({
  chat_presets: z.array(chatPresetSchema).max(12).nullable(),
  enabled_features: z.array(playgroundFeatureSchema).min(1),
  models: z.object({
    chat: z.array(z.string()),
    image: z.array(z.string()),
    speech: z.array(z.string()),
    three_d: z.array(z.string()),
    video: z.array(z.string()),
  }),
  speech_model_types: z.record(
    z.string(),
    z.enum(['openai', 'azure', 'unrealspeech'])
  ),
})

export type PlaygroundSettingsValue = z.infer<typeof playgroundSettingsSchema>
export type PlaygroundFeature = z.infer<typeof playgroundFeatureSchema>

export const DEFAULT_PLAYGROUND_SETTINGS: PlaygroundSettingsValue = {
  chat_presets: null,
  enabled_features: ['chat'],
  models: { chat: [], image: [], speech: [], three_d: [], video: [] },
  speech_model_types: {},
}

export function parsePlaygroundSettings(
  value: string
): PlaygroundSettingsValue {
  try {
    const parsed = JSON.parse(value) as {
      models?: Record<string, unknown>
      chat_presets?: unknown
    }
    parsed.chat_presets ??= null
    if (parsed.models && !Array.isArray(parsed.models.video)) {
      parsed.models.video = []
    }
    return playgroundSettingsSchema.parse(parsed)
  } catch {
    return DEFAULT_PLAYGROUND_SETTINGS
  }
}
