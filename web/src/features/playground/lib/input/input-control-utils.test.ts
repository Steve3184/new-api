import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getInputControlState,
  getSubmittableInputText,
} from './input-control-utils'

describe('playground input submission', () => {
  test('allows attachment-only messages', () => {
    assert.equal(getSubmittableInputText({ text: '', files: [{}] }), '')
    assert.equal(
      getInputControlState({
        attachmentCount: 1,
        groups: [],
        hasStopHandler: false,
        models: [{ label: 'gpt-test', value: 'gpt-test' }],
        text: '',
      }).canSubmit,
      true
    )
  })

  test('keeps empty messages disabled without attachments', () => {
    assert.equal(getSubmittableInputText({ text: '', files: [] }), null)
  })
})
