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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

interface PanelWrapperProps {
  title: ReactNode
  description?: ReactNode
  loading?: boolean
  empty?: boolean
  emptyMessage?: string
  height?: string
  className?: string
  contentClassName?: string
  headerActions?: ReactNode
  children?: ReactNode
}

function PanelHeader(props: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
}) {
  const heading = (
    <div className='flex flex-col gap-1'>
      <div className='text-sm font-semibold'>{props.title}</div>
      {props.description != null && (
        <div className='text-muted-foreground text-xs'>{props.description}</div>
      )}
    </div>
  )

  return (
    <div className='border-b px-4 py-3 sm:px-5'>
      {props.actions != null ? (
        <div className='flex items-start justify-between gap-2'>
          {heading}
          {props.actions}
        </div>
      ) : (
        heading
      )}
    </div>
  )
}

export function PanelWrapper(props: PanelWrapperProps) {
  const { t } = useTranslation()
  const resolvedEmptyMessage = props.emptyMessage ?? t('No data available')
  const height = props.height ?? 'h-64'
  // The frame fills its slot, so a panel that ends a dashboard column can
  // stretch to line up with the neighbouring column. Bodies grow from their
  // natural height (flex-auto) and never shrink below it.
  const frameClassName = cn(
    'flex h-full flex-col overflow-hidden rounded-2xl border bg-card shadow-xs',
    props.className
  )

  if (props.loading) {
    return (
      <div className={frameClassName}>
        <PanelHeader title={props.title} description={props.description} />
        <div className={cn('flex-auto p-4 sm:p-5', props.contentClassName)}>
          <Skeleton className={`w-full ${height}`} />
        </div>
      </div>
    )
  }

  if (props.empty) {
    return (
      <div className={frameClassName}>
        <PanelHeader title={props.title} description={props.description} />
        <div
          className={cn(
            'text-muted-foreground flex flex-auto items-center justify-center px-4 text-sm',
            height,
            props.contentClassName
          )}
        >
          {resolvedEmptyMessage}
        </div>
      </div>
    )
  }

  return (
    <div className={frameClassName}>
      <PanelHeader
        title={props.title}
        description={props.description}
        actions={props.headerActions}
      />
      <div
        className={cn(
          'flex flex-auto flex-col p-4 sm:p-5',
          props.contentClassName
        )}
      >
        {props.children}
      </div>
    </div>
  )
}

/**
 * Scrolling body of a panel. It keeps at least `minHeight` and grows with the
 * panel; the content scrolls inside instead of making the panel taller, so a
 * long list never pushes the dashboard columns out of line.
 *
 * The absolute box is a plain wrapper on purpose: the ScrollArea root sets an
 * inline `position: relative`, which would override an `absolute` class on it
 * and put the whole list back into the layout.
 */
export function PanelScrollArea(props: {
  minHeight: 'min-h-48 xl:min-h-0' | 'min-h-72' | 'min-h-80'
  children: ReactNode
}) {
  return (
    <div className={cn('relative flex-1', props.minHeight)}>
      <div className='absolute inset-0'>
        <ScrollArea className='h-full'>{props.children}</ScrollArea>
      </div>
    </div>
  )
}
