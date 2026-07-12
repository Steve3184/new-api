import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { createUserMessage, formatMessageForAPI } from './message-utils'

describe('playground attachment payloads', () => {
  test('maps images and files to supported chat content parts', () => {
    const message = createUserMessage('Inspect these attachments', [
      {
        dataUrl: 'data:image/png;base64,aW1hZ2U=',
        filename: 'screen.png',
        mediaType: 'image/png',
      },
      {
        dataUrl: 'data:text/plain;base64,aGVsbG8=',
        filename: 'notes.txt',
        mediaType: 'text/plain',
      },
    ])

    assert.deepEqual(formatMessageForAPI(message), {
      role: 'user',
      content: [
        { type: 'text', text: 'Inspect these attachments' },
        {
          type: 'image_url',
          image_url: { url: 'data:image/png;base64,aW1hZ2U=' },
        },
        {
          type: 'file',
          file: {
            filename: 'notes.txt',
            file_data: 'data:text/plain;base64,aGVsbG8=',
          },
        },
      ],
    })
  })
})
