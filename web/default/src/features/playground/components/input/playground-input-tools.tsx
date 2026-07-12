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
import { GlobeIcon, PaperclipIcon, Trash2Icon } from 'lucide-react'
import { type ChangeEvent, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInputButton,
  PromptInputTools,
  usePromptInputAttachments,
} from '@/components/ai-elements/prompt-input'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { ATTACHMENT_ACTIONS, getSearchActionNotice } from '../../lib'
import type { ParameterEnabled, PlaygroundConfig } from '../../types'
import { PlaygroundParameterPanel } from './playground-parameter-panel'

type PlaygroundInputToolsProps = {
  config: PlaygroundConfig
  disabled?: boolean
  hasMessages?: boolean
  onClearMessages?: () => void
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

async function captureScreenshotFile(): Promise<File> {
  const getDisplayMedia = navigator.mediaDevices?.getDisplayMedia
  if (!getDisplayMedia) {
    throw new Error('screen-capture-unsupported')
  }

  const stream = await getDisplayMedia.call(navigator.mediaDevices, {
    audio: false,
    video: true,
  })

  try {
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.srcObject = stream

    await new Promise<void>((resolve, reject) => {
      video.addEventListener('loadeddata', () => resolve(), { once: true })
      video.addEventListener(
        'error',
        () => reject(new Error('screen-capture-load-failed')),
        { once: true }
      )
    })
    await video.play()

    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    const context = canvas.getContext('2d')
    if (!context) {
      throw new Error('screen-capture-context-failed')
    }
    context.drawImage(video, 0, 0, canvas.width, canvas.height)

    const blob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob((value) => {
        if (value) {
          resolve(value)
        } else {
          reject(new Error('screen-capture-encode-failed'))
        }
      }, 'image/png')
    })
    const timestamp = new Date().toISOString().replaceAll(/[:.]/g, '-')
    return new File([blob], `screenshot-${timestamp}.png`, {
      type: 'image/png',
    })
  } finally {
    for (const track of stream.getTracks()) {
      track.stop()
    }
  }
}

export function PlaygroundInputTools({
  config,
  disabled,
  hasMessages = false,
  onClearMessages,
  onConfigChange,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputToolsProps) {
  const { t } = useTranslation()
  const attachments = usePromptInputAttachments()
  const [clearConfirmOpen, setClearConfirmOpen] = useState(false)
  const photoInputRef = useRef<HTMLInputElement>(null)
  const cameraInputRef = useRef<HTMLInputElement>(null)

  const handleFileAction = async (action: string) => {
    if (action === 'upload-file') {
      attachments.openFileDialog()
      return
    }
    if (action === 'upload-photo') {
      photoInputRef.current?.click()
      return
    }
    if (action === 'take-photo') {
      cameraInputRef.current?.click()
      return
    }
    if (action !== 'take-screenshot') return

    try {
      attachments.add([await captureScreenshotFile()])
    } catch (error) {
      if (
        error instanceof DOMException &&
        (error.name === 'AbortError' || error.name === 'NotAllowedError')
      ) {
        return
      }
      if (
        error instanceof Error &&
        error.message === 'screen-capture-unsupported'
      ) {
        toast.error(t('Screen capture is not supported in this browser'))
        return
      }
      toast.error(t('Unable to capture screenshot'))
    }
  }

  const handleImageInput = (event: ChangeEvent<HTMLInputElement>) => {
    if (event.currentTarget.files?.length) {
      attachments.add(event.currentTarget.files)
    }
    event.currentTarget.value = ''
  }

  const handleSearchAction = () => {
    const notice = getSearchActionNotice()
    toast.info(t(notice.title))
  }

  const handleClearMessages = () => {
    onClearMessages?.()
    setClearConfirmOpen(false)
    toast.success(t('Conversation cleared'))
  }

  return (
    <>
      <PromptInputTools className='bg-background/70 border-border/60 rounded-lg border p-1 shadow-xs'>
        <Tooltip>
          <DropdownMenu>
            <TooltipTrigger
              render={
                <DropdownMenuTrigger
                  render={
                    <PromptInputButton
                      aria-label={t('Attach')}
                      className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
                      disabled={disabled}
                      variant='ghost'
                    />
                  }
                >
                  <PaperclipIcon size={16} />
                </DropdownMenuTrigger>
              }
            />
            <TooltipContent>
              <p>{t('Attach')}</p>
            </TooltipContent>
            <DropdownMenuContent align='start'>
              {ATTACHMENT_ACTIONS.map(({ action, icon: Icon, label }) => (
                <DropdownMenuItem
                  key={action}
                  onClick={() => void handleFileAction(action)}
                >
                  <Icon className='mr-2' size={16} />
                  {t(label)}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <PromptInputButton
                aria-label={t('Search')}
                className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
                disabled={disabled}
                onClick={handleSearchAction}
                variant='ghost'
              >
                <GlobeIcon size={16} />
              </PromptInputButton>
            }
          />
          <TooltipContent>
            <p>{t('Search')}</p>
          </TooltipContent>
        </Tooltip>

        <PlaygroundParameterPanel
          config={config}
          disabled={disabled}
          onConfigChange={onConfigChange}
          onParameterEnabledChange={onParameterEnabledChange}
          parameterEnabled={parameterEnabled}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <PromptInputButton
                aria-label={t('Clear chat history')}
                className='text-muted-foreground hover:text-destructive hover:bg-destructive/10 font-medium'
                disabled={disabled || !hasMessages || !onClearMessages}
                onClick={() => setClearConfirmOpen(true)}
                variant='ghost'
              >
                <Trash2Icon size={16} />
              </PromptInputButton>
            }
          />
          <TooltipContent>
            <p>{t('Clear chat history')}</p>
          </TooltipContent>
        </Tooltip>
      </PromptInputTools>

      <input
        ref={photoInputRef}
        type='file'
        accept='image/*'
        multiple
        className='hidden'
        aria-label={t('Upload photo')}
        onChange={handleImageInput}
      />
      <input
        ref={cameraInputRef}
        type='file'
        accept='image/*'
        capture='environment'
        className='hidden'
        aria-label={t('Take photo')}
        onChange={handleImageInput}
      />

      <ConfirmDialog
        destructive
        desc={t(
          'All playground messages saved in this browser will be removed. This cannot be undone.'
        )}
        confirmText={t('Clear')}
        handleConfirm={handleClearMessages}
        open={clearConfirmOpen}
        onOpenChange={setClearConfirmOpen}
        title={t('Clear chat history?')}
      />
    </>
  )
}
