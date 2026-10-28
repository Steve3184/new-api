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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { ChatSettingsVisualEditor } from './chat-settings-visual-editor'
import { formatJsonForEditor, normalizeJsonString } from './utils'

const createChatSchema = (t: (key: string) => string) =>
  z.object({
    Chats: z.string().superRefine((value, ctx) => {
      try {
        const parsed = JSON.parse(value || '[]')
        if (!Array.isArray(parsed)) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: t('Expected a JSON array.'),
          })
          return
        }
        for (const item of parsed) {
          if (
            item === null ||
            typeof item !== 'object' ||
            Array.isArray(item)
          ) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: t(
                'Each item must be an object with a single key-value pair.'
              ),
            })
            return
          }
          const entries = Object.entries(item)
          if (entries.length !== 1) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: t('Each item must have exactly one key-value pair.'),
            })
            return
          }
        }
      } catch {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t('Invalid JSON string.'),
        })
      }
    }),
    ChatPresetSettings: z.string(),
  })

type ChatSettingsFormValues = z.infer<ReturnType<typeof createChatSchema>>

type ChatSettingsSectionProps = {
  defaultValue: string
  defaultPresetSettings: string
}

export function ChatSettingsSection({
  defaultValue,
  defaultPresetSettings,
}: ChatSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')

  const chatSchema = createChatSchema(t)
  const formatted = formatJsonForEditor(defaultValue, '[]')
  const form = useForm<ChatSettingsFormValues>({
    resolver: zodResolver(chatSchema),
    mode: 'onChange', // Enable real-time validation
    defaultValues: {
      Chats: formatted,
      ChatPresetSettings: normalizeJsonString(defaultPresetSettings, '{}'),
    },
  })

  const initialNormalizedRef = useRef(normalizeJsonString(defaultValue, '[]'))
  const initialPresetSettingsRef = useRef(
    normalizeJsonString(defaultPresetSettings, '{}')
  )

  useEffect(() => {
    const presetSettings = normalizeJsonString(defaultPresetSettings, '{}')
    form.reset({
      Chats: formatJsonForEditor(defaultValue, '[]'),
      ChatPresetSettings: presetSettings,
    })
    initialNormalizedRef.current = normalizeJsonString(defaultValue, '[]')
    initialPresetSettingsRef.current = presetSettings
  }, [defaultValue, defaultPresetSettings, form])

  const onSubmit = async (values: ChatSettingsFormValues) => {
    const normalized = normalizeJsonString(values.Chats, '[]')
    const presetSettings = normalizeJsonString(values.ChatPresetSettings, '{}')

    if (normalized !== initialNormalizedRef.current) {
      await updateOption.mutateAsync({
        key: 'Chats',
        value: normalized,
      })
      initialNormalizedRef.current = normalized
    }
    if (presetSettings !== initialPresetSettingsRef.current) {
      await updateOption.mutateAsync({
        key: 'ChatPresetSettings',
        value: presetSettings,
      })
      initialPresetSettingsRef.current = presetSettings
    }
  }

  return (
    <SettingsSection title={t('Chat Presets')}>
      <Form {...form}>
        {/* eslint-disable-next-line react-hooks/refs */}
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save chat settings'
          />
          <Tabs
            value={editMode}
            onValueChange={(value) => setEditMode(value as 'visual' | 'json')}
          >
            <TabsList className='grid w-full grid-cols-2'>
              <TabsTrigger value='visual'>{t('Visual')}</TabsTrigger>
              <TabsTrigger value='json'>{t('JSON')}</TabsTrigger>
            </TabsList>

            <TabsContent value='visual' className='mt-6'>
              <FormField
                control={form.control}
                name='Chats'
                render={({ field }) => (
                  <FormItem>
                    <FormControl>
                      <ChatSettingsVisualEditor
                        value={field.value}
                        onChange={field.onChange}
                        presetSettings={form.watch('ChatPresetSettings')}
                        onPresetSettingsChange={(value) =>
                          form.setValue('ChatPresetSettings', value, {
                            shouldDirty: true,
                          })
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </TabsContent>

            <TabsContent value='json' className='mt-6'>
              <FormField
                control={form.control}
                name='Chats'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Chat configuration JSON')}</FormLabel>
                    <FormControl>
                      <JsonCodeEditor
                        value={field.value}
                        onChange={field.onChange}
                        name={field.name}
                        onBlur={field.onBlur}
                        textareaRef={field.ref}
                        placeholder={t(
                          '[{"ChatGPT":"https://chat.openai.com"},{"Lobe Chat":"https://chat-preview.lobehub.com/?settings={...}"}]'
                        )}
                        heightClassName='h-72 min-h-72 max-h-72'
                        aria-invalid={Boolean(form.formState.errors.Chats)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Array of chat client presets. Each item is an object with one key-value pair: client name and its URL.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </TabsContent>
          </Tabs>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
