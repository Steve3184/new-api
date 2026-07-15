/* Copyright (C) 2023-2026 QuantumNous */
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { AZURE_TTS_VOICE_OPTIONS, AZURE_TTS_VOICES } from './azure-tts-voices'

describe('Azure TTS voice options', () => {
  test('exposes every voice as a distinct combobox option', () => {
    assert.equal(AZURE_TTS_VOICES.length, 322)
    assert.equal(AZURE_TTS_VOICE_OPTIONS.length, 322)
    assert.equal(new Set(AZURE_TTS_VOICES).size, 322)
    assert.equal(AZURE_TTS_VOICES[0], 'af-ZA-AdriNeural')
    assert.ok(AZURE_TTS_VOICES.includes('zh-CN-XiaoxiaoNeural'))
    assert.ok(AZURE_TTS_VOICES.every((voice) => !voice.includes('\n')))
  })
})
