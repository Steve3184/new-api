import { describe, expect, test } from 'vitest'

import {
  getInputControlState,
  getSubmittableInputText,
} from './input-control-utils'

describe('playground input submission', () => {
  test('allows attachment-only messages', () => {
    expect(getSubmittableInputText({ text: '', files: [{}] })).toBe('')
    expect(
      getInputControlState({
        attachmentCount: 1,
        groups: [],
        hasStopHandler: false,
        models: [{ label: 'gpt-test', value: 'gpt-test' }],
        text: '',
      }).canSubmit
    ).toBe(true)
  })

  test('keeps empty messages disabled without attachments', () => {
    expect(getSubmittableInputText({ text: '', files: [] })).toBeNull()
  })
})
