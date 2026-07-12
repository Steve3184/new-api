/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Download, FileText } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Loader } from '@/components/ai-elements/loader'
import { MessageContent } from '@/components/ai-elements/message'
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from '@/components/ai-elements/reasoning'
import { Response } from '@/components/ai-elements/response'
import { Shimmer } from '@/components/ai-elements/shimmer'
import {
  Source,
  Sources,
  SourcesContent,
  SourcesTrigger,
} from '@/components/ai-elements/sources'
import { cn } from '@/lib/utils'

import { MESSAGE_STATUS } from '../../constants'
import {
  getMessageAlignmentClass,
  getMessageContentState,
  isErrorMessage,
  type MessageAlignment,
} from '../../lib'
import { getMessageContentStyles } from '../../lib/message/message-styles'
import type { Message, PlaygroundAttachment } from '../../types'
import { MessageError } from './message-error'
import { MessageMetadata } from './message-metadata'

type PlaygroundMessageContentProps = {
  actions: ReactNode
  alignment: MessageAlignment
  errorActions?: ReactNode
  isSourceVisible?: boolean
  message: Message
  versionContent: string
}

function PlaygroundMessageAttachments(props: {
  attachments: PlaygroundAttachment[]
}) {
  const { t } = useTranslation()

  if (props.attachments.length === 0) return null

  return (
    <div className='flex max-w-full flex-wrap gap-2'>
      {props.attachments.map((attachment, index) => {
        const filename = attachment.filename || t('File')
        const key = `${filename}-${index}`

        if (attachment.mediaType.startsWith('image/')) {
          return (
            <a
              key={key}
              href={attachment.dataUrl}
              download={filename}
              aria-label={`${t('Download')} ${filename}`}
              className='border-border/70 bg-background/60 block overflow-hidden rounded-md border'
            >
              <img
                src={attachment.dataUrl}
                alt={filename}
                className='max-h-48 max-w-64 object-contain'
              />
            </a>
          )
        }

        return (
          <a
            key={key}
            href={attachment.dataUrl}
            download={filename}
            className='border-border/70 bg-background/60 text-foreground hover:bg-muted flex max-w-64 items-center gap-2 rounded-md border px-2.5 py-2 text-xs transition-colors'
          >
            <FileText className='text-muted-foreground size-4 shrink-0' />
            <span className='min-w-0 flex-1 truncate'>{filename}</span>
            <Download className='text-muted-foreground size-3.5 shrink-0' />
          </a>
        )
      })}
    </div>
  )
}

export function PlaygroundMessageContent({
  actions,
  alignment,
  errorActions,
  isSourceVisible = false,
  message,
  versionContent,
}: PlaygroundMessageContentProps) {
  const { t } = useTranslation()
  const {
    displayContent,
    hasReasoning,
    hasSources,
    reasoningContent,
    showLoader,
    showMessageContent,
    sources,
  } = getMessageContentState(message, versionContent)
  const isError = isErrorMessage(message)
  const attachments = message.attachments ?? []
  const isMessageFinal =
    message.status !== MESSAGE_STATUS.LOADING &&
    message.status !== MESSAGE_STATUS.STREAMING

  return (
    <div
      className={cn(
        'flex w-full min-w-0 flex-col',
        getMessageAlignmentClass(alignment)
      )}
    >
      {hasSources && (
        <Sources>
          <SourcesTrigger count={sources.length} />
          <SourcesContent>
            {sources.map((source) => (
              <Source
                href={source.href}
                key={`${source.href}-${source.title}`}
                title={source.title}
              />
            ))}
          </SourcesContent>
        </Sources>
      )}

      {hasReasoning && (
        <Reasoning
          defaultOpen
          duration={message.reasoning?.duration}
          isStreaming={message.isReasoningStreaming}
        >
          <ReasoningTrigger />
          <ReasoningContent>{reasoningContent}</ReasoningContent>
        </Reasoning>
      )}

      {showLoader && (
        <div className='flex items-center gap-2 py-2'>
          <Loader />
          <Shimmer className='text-sm' duration={1}>
            {t('Responding...')}
          </Shimmer>
        </div>
      )}

      {isError && (
        <>
          <MessageError message={message} className='mb-2' />
          <MessageMetadata alignment={alignment} message={message} />
          {errorActions}
        </>
      )}

      {!isError && showMessageContent && (
        <>
          {isSourceVisible ? (
            <div className='flex w-full flex-col gap-2'>
              <PlaygroundMessageAttachments attachments={attachments} />
              <CodeBlock
                code={versionContent}
                className='my-0 group-[.is-assistant]:w-full group-[.is-assistant]:max-w-[78ch]'
                collapsedLines={24}
                defaultCollapsed={false}
                language='markdown'
                maxExpandedLines={48}
                showLineNumbers
                showToolbar
                title={t('Raw response')}
              >
                <CodeBlockCopyButton />
              </CodeBlock>
            </div>
          ) : (
            <MessageContent
              variant='flat'
              className={cn(getMessageContentStyles())}
            >
              <PlaygroundMessageAttachments attachments={attachments} />
              {displayContent && (
                <Response final={isMessageFinal}>{displayContent}</Response>
              )}
            </MessageContent>
          )}
          <MessageMetadata alignment={alignment} message={message} />
          {actions}
        </>
      )}
    </div>
  )
}
