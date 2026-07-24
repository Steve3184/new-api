import { useQuery } from '@tanstack/react-query'
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
import { Check, ChevronsUpDown, Terminal } from 'lucide-react'
import { memo, useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getUserModels } from '@/lib/api'
import { cn } from '@/lib/utils'

import { safeJsonParse } from '../utils/json-parser'

type GroupCodingModelEditorProps = {
  groups: string[]
  defaultModels: string
  onChange: (field: 'GroupDefaultModel', value: string) => void
}

function GroupModelRow(props: {
  group: string
  value: string
  onValueChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const requestedGroup = props.group === 'auto' ? undefined : props.group
  const { data, isLoading } = useQuery({
    queryKey: ['user-models', requestedGroup ?? 'all'],
    queryFn: () => getUserModels(requestedGroup),
    staleTime: 5 * 60 * 1000,
  })
  const options = useMemo(
    () =>
      [...(data?.data ?? [])]
        .sort()
        .map((model) => ({ label: model, value: model })),
    [data?.data]
  )
  const filteredOptions = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return options
    return options.filter((option) =>
      option.value.toLowerCase().includes(query)
    )
  }, [options, search])
  const customValue = search.trim()
  const showCustomValue =
    customValue.length > 0 &&
    !options.some((option) => option.value === customValue)

  const selectValue = (value: string) => {
    props.onValueChange(value)
    setSearch('')
    setOpen(false)
  }

  return (
    <TableRow>
      <TableCell className='font-mono text-xs'>{props.group}</TableCell>
      <TableCell>
        <Popover
          open={open}
          onOpenChange={(nextOpen) => {
            setOpen(nextOpen)
            if (!nextOpen) setSearch('')
          }}
        >
          <PopoverTrigger
            render={
              <Button
                type='button'
                variant='outline'
                role='combobox'
                aria-expanded={open}
                className='h-9 w-full min-w-64 justify-between font-normal'
              />
            }
          >
            <span className='truncate font-mono text-xs'>
              {props.value ||
                (isLoading
                  ? t('Loading models...')
                  : t('Select or enter model name'))}
            </span>
            <ChevronsUpDown className='size-4 shrink-0 opacity-50' />
          </PopoverTrigger>
          <PopoverContent className='w-[var(--anchor-width)] p-0' align='start'>
            <Command shouldFilter={false}>
              <CommandInput
                value={search}
                onValueChange={setSearch}
                placeholder={t('Search models...')}
              />
              <CommandList className='max-h-72'>
                <CommandEmpty>{t('No model found.')}</CommandEmpty>
                <CommandGroup>
                  {filteredOptions.map((option) => (
                    <CommandItem
                      key={option.value}
                      value={option.value}
                      onSelect={() => selectValue(option.value)}
                    >
                      <Check
                        className={cn(
                          'size-4',
                          props.value === option.value
                            ? 'opacity-100'
                            : 'opacity-0'
                        )}
                      />
                      <span className='truncate font-mono text-xs'>
                        {option.label}
                      </span>
                    </CommandItem>
                  ))}
                  {showCustomValue ? (
                    <CommandItem
                      value={customValue}
                      onSelect={() => selectValue(customValue)}
                    >
                      <Terminal className='size-4' />
                      {t('Press Enter to use "{{value}}"', {
                        value: customValue,
                      })}
                    </CommandItem>
                  ) : null}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      </TableCell>
    </TableRow>
  )
}

export const GroupCodingModelEditor = memo(function GroupCodingModelEditor({
  groups,
  defaultModels,
  onChange,
}: GroupCodingModelEditorProps) {
  const { t } = useTranslation()
  const defaultModelMap = useMemo(
    () =>
      safeJsonParse<Record<string, string>>(defaultModels, {
        fallback: {},
        silent: true,
      }),
    [defaultModels]
  )
  const updateModel = useCallback(
    (group: string, value: string) => {
      const next = { ...defaultModelMap }
      const normalized = value.trim()
      if (normalized) {
        next[group] = normalized
      } else {
        delete next[group]
      }
      onChange('GroupDefaultModel', JSON.stringify(next, null, 2))
    },
    [defaultModelMap, onChange]
  )

  if (groups.length === 0) return null

  return (
    <section className='space-y-3 border-t pt-5'>
      <div className='space-y-1'>
        <div className='flex items-center gap-2 text-sm font-medium'>
          <Terminal className='size-4' />
          {t('Coding tool models')}
        </div>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Set the default model shown in coding tool setup instructions for each group.'
          )}
        </p>
      </div>

      <div className='overflow-x-auto rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-40'>{t('Group')}</TableHead>
              <TableHead className='min-w-72'>{t('Default model')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {groups.map((group) => (
              <GroupModelRow
                key={group}
                group={group}
                value={defaultModelMap[group] ?? ''}
                onValueChange={(value) => updateModel(group, value)}
              />
            ))}
          </TableBody>
        </Table>
      </div>
    </section>
  )
})
