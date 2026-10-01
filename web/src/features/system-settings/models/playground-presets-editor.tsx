/* Copyright (C) 2023-2026 QuantumNous */
import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useFieldArray, useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { ReactIconByName } from '@/components/react-icon-by-name'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { DEFAULT_CHAT_PRESETS } from '@/features/playground/constants'

import type { PlaygroundSettingsValue } from './playground-settings'

export function PlaygroundPresetsEditor() {
  const { t } = useTranslation()
  const form = useFormContext<PlaygroundSettingsValue>()
  const configured = form.watch('chat_presets')
  const presets = configured ?? DEFAULT_CHAT_PRESETS
  const setPresets = (value: PlaygroundSettingsValue['chat_presets']) =>
    form.setValue('chat_presets', value, {
      shouldDirty: true,
      shouldValidate: true,
    })

  return (
    <section className='grid min-w-0 gap-4 rounded-xl border p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h3 className='font-medium'>{t('Chat starter presets')}</h3>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          disabled={configured === null}
          onClick={() => setPresets(null)}
        >
          {t('Reset to default')}
        </Button>
      </div>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Configure up to 12 starter prompts. An empty list hides the presets.'
        )}
      </p>
      {configured === null ? (
        <div className='grid gap-3'>
          <div className='flex flex-wrap gap-2'>
            {presets.map((preset) => (
              <span
                key={preset.title}
                className='bg-muted rounded-lg px-3 py-2 text-sm'
              >
                {t(preset.title)}
              </span>
            ))}
          </div>
          <Button
            type='button'
            variant='outline'
            className='w-fit'
            onClick={() =>
              setPresets(
                DEFAULT_CHAT_PRESETS.map((preset) => ({
                  ...preset,
                  title: t(preset.title),
                }))
              )
            }
          >
            {t('Customize presets')}
          </Button>
        </div>
      ) : (
        <CustomPresets />
      )}
    </section>
  )
}

function CustomPresets() {
  const { t } = useTranslation()
  const form = useFormContext<PlaygroundSettingsValue>()
  const { fields, move, remove, append } = useFieldArray({
    control: form.control,
    name: 'chat_presets',
  })
  const presets = form.watch('chat_presets') ?? []
  return (
    <>
      {presets.map((preset, index) => (
        <div
          key={fields[index]?.id}
          className='grid min-w-0 gap-3 rounded-lg border p-3'
        >
          <div className='flex items-center justify-between gap-2'>
            <div className='flex min-w-0 items-center gap-2'>
              <ReactIconByName
                name={preset.icon}
                className='size-5 shrink-0'
                aria-hidden='true'
              />
              <span className='truncate text-sm font-medium'>
                {preset.title || t('Title')}
              </span>
            </div>
            <div className='flex shrink-0 gap-1'>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                aria-label={t('Move up')}
                disabled={index === 0}
                onClick={() => {
                  move(index, index - 1)
                }}
              >
                <ArrowUpIcon className='size-4' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                aria-label={t('Move down')}
                disabled={index === presets.length - 1}
                onClick={() => {
                  move(index, index + 1)
                }}
              >
                <ArrowDownIcon className='size-4' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                aria-label={t('Delete')}
                onClick={() => remove(index)}
              >
                <Trash2Icon className='size-4' />
              </Button>
            </div>
          </div>
          <div className='grid gap-3 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name={`chat_presets.${index}.title`}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Title')}</FormLabel>
                  <FormControl>
                    <Input {...field} maxLength={80} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name={`chat_presets.${index}.icon`}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Icon')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='LuSparkles' maxLength={80} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
          <FormField
            control={form.control}
            name={`chat_presets.${index}.content`}
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Content to send')}</FormLabel>
                <FormControl>
                  <Textarea {...field} rows={3} maxLength={8000} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      ))}
      <p className='text-muted-foreground text-xs'>
        {t('Enter a react-icons component name. Invalid names show no icon.')}
      </p>
      <Button
        type='button'
        variant='outline'
        className='w-fit'
        disabled={presets.length >= 12}
        onClick={() => append({ icon: 'LuSparkles', title: '', content: '' })}
      >
        <PlusIcon className='size-4' />
        {t('Add preset')}
      </Button>
    </>
  )
}
