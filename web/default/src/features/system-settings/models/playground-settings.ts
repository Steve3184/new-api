/* Copyright (C) 2023-2026 QuantumNous */
import * as z from 'zod'

export const playgroundFeatureSchema = z.enum([
  'chat',
  'image',
  'speech',
  'three_d',
])
export const playgroundSettingsSchema = z.object({
  enabled_features: z.array(playgroundFeatureSchema).min(1),
  models: z.object({
    chat: z.array(z.string()),
    image: z.array(z.string()),
    speech: z.array(z.string()),
    three_d: z.array(z.string()),
  }),
  speech_model_types: z.record(z.string(), z.enum(['openai', 'azure'])),
})

export type PlaygroundSettingsValue = z.infer<typeof playgroundSettingsSchema>
export type PlaygroundFeature = z.infer<typeof playgroundFeatureSchema>

export const DEFAULT_PLAYGROUND_SETTINGS: PlaygroundSettingsValue = {
  enabled_features: ['chat'],
  models: { chat: [], image: [], speech: [], three_d: [] },
  speech_model_types: {},
}

export function parsePlaygroundSettings(
  value: string
): PlaygroundSettingsValue {
  try {
    return playgroundSettingsSchema.parse(JSON.parse(value))
  } catch {
    return DEFAULT_PLAYGROUND_SETTINGS
  }
}
