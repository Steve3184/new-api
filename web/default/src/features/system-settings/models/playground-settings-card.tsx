/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { getModels } from '@/features/models/api'

import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  playgroundSettingsSchema,
  type PlaygroundFeature,
  type PlaygroundSettingsValue,
} from './playground-settings'

type ModelFeature = keyof PlaygroundSettingsValue['models']

const FEATURE_ROWS: Array<{
  feature: PlaygroundFeature
  modelKey: ModelFeature
  labelKey: string
  descriptionKey: string
}> = [
  {
    feature: 'chat',
    modelKey: 'chat',
    labelKey: 'Chat',
    descriptionKey: 'Enable conversational chat in Playground.',
  },
  {
    feature: 'image',
    modelKey: 'image',
    labelKey: 'Image generation',
    descriptionKey: 'Enable image generation and editing in Playground.',
  },
  {
    feature: 'speech',
    modelKey: 'speech',
    labelKey: 'Speech',
    descriptionKey: 'Enable text-to-speech generation in Playground.',
  },
  {
    feature: 'three_d',
    modelKey: 'three_d',
    labelKey: '3D generation',
    descriptionKey: 'Enable asynchronous 3D generation in Playground.',
  },
]

type PlaygroundSettingsCardProps = {
  defaultValues: PlaygroundSettingsValue
}

export function PlaygroundSettingsCard(props: PlaygroundSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<PlaygroundSettingsValue>({
    resolver: zodResolver(playgroundSettingsSchema),
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [form, props.defaultValues])

  const modelsQuery = useQuery({
    queryKey: ['playground-settings-models'],
    queryFn: () => getModels({ page_size: 1000, status: '1' }),
    staleTime: 5 * 60 * 1000,
  })
  const modelOptions = useMemo(
    () =>
      (modelsQuery.data?.data?.items ?? [])
        .map((model) => model.model_name)
        .sort()
        .map((modelName) => ({ label: modelName, value: modelName })),
    [modelsQuery.data?.data?.items]
  )
  const enabledFeatures = form.watch('enabled_features')
  const speechModels = form.watch('models.speech')
  const speechModelTypes = form.watch('speech_model_types')

  const toggleFeature = (feature: PlaygroundFeature, enabled: boolean) => {
    const current = form.getValues('enabled_features')
    const next = enabled
      ? [...new Set([...current, feature])]
      : current.filter((item) => item !== feature)
    form.setValue('enabled_features', next, { shouldDirty: true })
  }

  const onSubmit = async (values: PlaygroundSettingsValue) => {
    const speechTypes = Object.fromEntries(
      values.models.speech.map((modelName) => [
        modelName,
        values.speech_model_types[modelName] ?? 'openai',
      ])
    )
    await updateOption.mutateAsync({
      key: 'PlaygroundSettings',
      value: JSON.stringify({ ...values, speech_model_types: speechTypes }),
    })
  }

  return (
    <SettingsSection title={t('Playground generation')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Chat is enabled by default. An empty model list allows every model available to the user.'
            )}
          </p>

          {FEATURE_ROWS.map((row) => {
            const enabled = enabledFeatures.includes(row.feature)
            return (
              <SettingsControlGroup key={row.feature}>
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t(row.labelKey)}</FormLabel>
                    <FormDescription>{t(row.descriptionKey)}</FormDescription>
                  </SettingsSwitchContent>
                  <Switch
                    checked={enabled}
                    disabled={enabled && enabledFeatures.length === 1}
                    onCheckedChange={(checked) =>
                      toggleFeature(row.feature, checked)
                    }
                  />
                </SettingsSwitchItem>
                <SettingsControlChildren>
                  <FormField
                    control={form.control}
                    name={`models.${row.modelKey}`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Allowed models')}</FormLabel>
                        <FormControl>
                          <MultiSelect
                            id={`playground-${row.modelKey}-models`}
                            options={modelOptions}
                            selected={field.value}
                            onChange={field.onChange}
                            allowCreate
                            maxVisibleChips={5}
                            placeholder={t('Select or enter model names')}
                            disabled={!enabled}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />

                  {row.feature === 'speech' && speechModels.length > 0 && (
                    <div className='mt-4 grid gap-3 sm:grid-cols-2'>
                      {speechModels.map((modelName) => {
                        const modelType =
                          speechModelTypes[modelName] ?? 'openai'
                        return (
                          <FormItem key={modelName}>
                            <FormLabel className='truncate' title={modelName}>
                              {modelName}
                            </FormLabel>
                            <Select
                              items={[
                                { value: 'openai', label: 'OpenAI' },
                                { value: 'azure', label: 'Azure' },
                              ]}
                              value={modelType}
                              onValueChange={(value) => {
                                if (value === null) return
                                form.setValue(
                                  'speech_model_types',
                                  {
                                    ...form.getValues('speech_model_types'),
                                    [modelName]: value as 'openai' | 'azure',
                                  },
                                  { shouldDirty: true }
                                )
                              }}
                            >
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectGroup>
                                  <SelectItem value='openai'>OpenAI</SelectItem>
                                  <SelectItem value='azure'>Azure</SelectItem>
                                </SelectGroup>
                              </SelectContent>
                            </Select>
                          </FormItem>
                        )
                      })}
                    </div>
                  )}
                </SettingsControlChildren>
              </SettingsControlGroup>
            )
          })}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
