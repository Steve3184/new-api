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
import { MessageSquarePlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { ReactIconByName } from '@/components/react-icon-by-name'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

import { DEFAULT_CHAT_PRESETS } from '../../constants'
import type { PlaygroundPublicSettings } from '../../types'

type PlaygroundEmptyStateProps = {
  onSelectPrompt: (prompt: string) => void
}

export function PlaygroundEmptyState({
  onSelectPrompt,
}: PlaygroundEmptyStateProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const configured = (
    status?.playground as PlaygroundPublicSettings | undefined
  )?.chat_presets
  const presets = configured ?? DEFAULT_CHAT_PRESETS

  return (
    <div className='flex min-h-[min(520px,calc(100svh-18rem))] items-center justify-center px-1 py-8 md:py-12'>
      <div className='grid w-full max-w-2xl gap-5 text-center'>
        <div className='bg-muted/50 text-muted-foreground mx-auto flex size-11 items-center justify-center rounded-xl border'>
          <MessageSquarePlusIcon className='size-5' aria-hidden='true' />
        </div>

        <div className='grid gap-2'>
          <h2 className='text-xl font-semibold tracking-tight text-balance md:text-2xl'>
            {t('Start a playground chat')}
          </h2>
          <p className='text-muted-foreground mx-auto max-w-lg text-sm leading-6 text-balance'>
            {t(
              'Test a model with a starter prompt, or write your own request below.'
            )}
          </p>
        </div>

        <div className='grid gap-2 sm:grid-cols-2'>
          {presets.map((preset) => (
            <Button
              className='h-auto min-h-11 justify-start gap-2 px-3 py-2.5 text-left whitespace-normal'
              key={preset.title + preset.content + preset.icon}
              onClick={() => onSelectPrompt(preset.content)}
              variant='outline'
            >
              <ReactIconByName
                name={preset.icon}
                aria-hidden='true'
                className='text-muted-foreground size-4 shrink-0'
              />
              <span className='min-w-0 break-words'>
                {configured == null ? t(preset.title) : preset.title}
              </span>
            </Button>
          ))}
        </div>
      </div>
    </div>
  )
}
