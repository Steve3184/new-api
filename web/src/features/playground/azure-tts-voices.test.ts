/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, test } from 'vitest'

import { AZURE_TTS_VOICE_OPTIONS, AZURE_TTS_VOICES } from './azure-tts-voices'

describe('Azure TTS voice options', () => {
  test('exposes every voice as a distinct combobox option', () => {
    expect(AZURE_TTS_VOICES).toHaveLength(322)
    expect(AZURE_TTS_VOICE_OPTIONS).toHaveLength(322)
    expect(new Set(AZURE_TTS_VOICES).size).toBe(322)
    expect(AZURE_TTS_VOICES[0]).toBe('af-ZA-AdriNeural')
    expect(AZURE_TTS_VOICES).toContain('zh-CN-XiaoxiaoNeural')
    expect(AZURE_TTS_VOICES.every((voice) => !voice.includes('\n'))).toBe(true)
  })
})
