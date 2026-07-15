/* Copyright (C) 2023-2026 QuantumNous */
import { Download, Loader2, Volume2, WandSparkles } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { ComboboxInput } from '@/components/ui/combobox-input'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { generateSpeech } from '../../api'
import { AZURE_TTS_VOICE_OPTIONS } from '../../azure-tts-voices'
import { OPENAI_SPEECH_VOICES } from '../../constants'
import type { GroupOption, ModelOption, SpeechModelType } from '../../types'
import { GenerationControls } from './generation-controls'
import { getGenerationErrorMessage } from './generation-utils'
import { useGenerationModel } from './use-generation-model'

const OPENAI_VOICE_OPTIONS = OPENAI_SPEECH_VOICES.map((voice) => ({
  label: voice,
  value: voice,
}))
const FORMAT_OPTIONS = ['mp3', 'wav', 'opus', 'aac', 'flac', 'pcm'].map(
  (value) => ({ label: value.toUpperCase(), value })
)

type SpeechPlaygroundProps = {
  models: ModelOption[]
  groups: GroupOption[]
  group: string
  onGroupChange: (value: string) => void
  modelTypes: Record<string, SpeechModelType>
  groupModels: Record<string, string[]>
}

export function SpeechPlayground(props: SpeechPlaygroundProps) {
  const { t } = useTranslation()
  const { model, setModel, group } = useGenerationModel({
    models: props.models,
    groups: props.groups,
    group: props.group,
    groupModels: props.groupModels,
    onGroupChange: props.onGroupChange,
  })
  const modelType = props.modelTypes[model] ?? 'openai'
  const [input, setInput] = useState('')
  const [openAIVoice, setOpenAIVoice] = useState('alloy')
  const [azureVoice, setAzureVoice] = useState('zh-CN-XiaoxiaoNeural')
  const [format, setFormat] = useState('mp3')
  const [speed, setSpeed] = useState(1)
  const [volume, setVolume] = useState(1)
  const [pitch, setPitch] = useState(0)
  const [volumeEnabled, setVolumeEnabled] = useState(false)
  const [pitchEnabled, setPitchEnabled] = useState(false)
  const [audioURL, setAudioURL] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)

  useEffect(
    () => () => {
      if (audioURL) URL.revokeObjectURL(audioURL)
    },
    [audioURL]
  )

  const handleGenerate = async () => {
    if (!model || !input.trim()) return
    setIsGenerating(true)
    try {
      const blob = await generateSpeech({
        model,
        group,
        input: input.trim(),
        voice,
        response_format: format,
        speed,
        ...(modelType === 'azure' && volumeEnabled ? { volume } : {}),
        ...(modelType === 'azure' && pitchEnabled ? { pitch } : {}),
      })
      if (audioURL) URL.revokeObjectURL(audioURL)
      setAudioURL(URL.createObjectURL(blob))
    } catch (error) {
      toast.error(
        await getGenerationErrorMessage(error, t('Speech generation failed'))
      )
    } finally {
      setIsGenerating(false)
    }
  }

  const voiceOptions =
    modelType === 'azure' ? AZURE_TTS_VOICE_OPTIONS : OPENAI_VOICE_OPTIONS
  const voice = modelType === 'azure' ? azureVoice : openAIVoice
  const setVoice = modelType === 'azure' ? setAzureVoice : setOpenAIVoice

  return (
    <div className='flex size-full min-h-0 flex-col overflow-hidden'>
      <section className='min-h-0 flex-1 overflow-x-hidden overflow-y-auto border-b p-4 sm:p-6'>
        <div className='mx-auto flex h-full w-full max-w-6xl flex-col gap-5'>
          <GenerationControls
            groups={props.groups}
            group={group}
            onGroupChange={props.onGroupChange}
            models={props.models}
            model={model}
            onModelChange={setModel}
            disabled={isGenerating}
            groupModels={props.groupModels}
          />

          <div className='grid min-h-0 flex-1 gap-5 lg:grid-cols-[minmax(0,1fr)_18rem]'>
            <Field className='flex min-h-64 flex-col lg:min-h-0'>
              <FieldLabel htmlFor='playground-speech-input'>
                {t('Text')}
              </FieldLabel>
              <Textarea
                id='playground-speech-input'
                rows={14}
                className='max-h-none min-h-64 flex-1 resize-none lg:min-h-0'
                value={input}
                onChange={(event) => setInput(event.target.value)}
                placeholder={t('Enter text to synthesize')}
                disabled={isGenerating}
              />
            </Field>

            <div className='flex flex-col gap-5 lg:min-h-0'>
              <Field>
                <FieldLabel htmlFor='playground-speech-voice'>
                  {t('Voice')}
                </FieldLabel>
                <ComboboxInput
                  id='playground-speech-voice'
                  options={voiceOptions}
                  value={voice}
                  onValueChange={setVoice}
                  allowCustomValue={modelType !== 'azure'}
                  placeholder={t('Select voice')}
                  emptyText='No voices found'
                  disabled={isGenerating}
                />
              </Field>

              <FieldGroup className='grid grid-cols-2 gap-4'>
                <Field>
                  <FieldLabel>{t('Format')}</FieldLabel>
                  <Select
                    items={FORMAT_OPTIONS}
                    value={format}
                    onValueChange={(value) => value && setFormat(value)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {FORMAT_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Field>
                <Field>
                  <FieldLabel htmlFor='playground-speech-speed'>
                    {t('Speed')}
                  </FieldLabel>
                  <Input
                    id='playground-speech-speed'
                    type='number'
                    min={0.25}
                    max={4}
                    step={0.05}
                    value={speed}
                    onChange={(event) =>
                      setSpeed(Number(event.target.value) || 1)
                    }
                  />
                </Field>
              </FieldGroup>

              {modelType === 'azure' && (
                <FieldGroup className='grid grid-cols-2 gap-4'>
                  <Field>
                    <div className='flex min-h-7 items-center justify-between gap-3'>
                      <FieldLabel htmlFor='playground-speech-volume'>
                        {t('Volume')}
                      </FieldLabel>
                      <Switch
                        checked={volumeEnabled}
                        onCheckedChange={setVolumeEnabled}
                        aria-label={t('Enable volume')}
                      />
                    </div>
                    <Input
                      id='playground-speech-volume'
                      type='number'
                      min={0}
                      step={0.05}
                      value={volume}
                      disabled={!volumeEnabled || isGenerating}
                      onChange={(event) =>
                        setVolume(Number(event.target.value))
                      }
                    />
                  </Field>
                  <Field>
                    <div className='flex min-h-7 items-center justify-between gap-3'>
                      <FieldLabel htmlFor='playground-speech-pitch'>
                        {t('Pitch (Hz)')}
                      </FieldLabel>
                      <Switch
                        checked={pitchEnabled}
                        onCheckedChange={setPitchEnabled}
                        aria-label={t('Enable pitch')}
                      />
                    </div>
                    <Input
                      id='playground-speech-pitch'
                      type='number'
                      step={1}
                      value={pitch}
                      disabled={!pitchEnabled || isGenerating}
                      onChange={(event) =>
                        setPitch(Number.parseInt(event.target.value, 10) || 0)
                      }
                    />
                  </Field>
                </FieldGroup>
              )}

              <Button
                className='w-full'
                disabled={!model || !input.trim() || !voice || isGenerating}
                onClick={handleGenerate}
              >
                {isGenerating ? (
                  <Loader2 className='animate-spin' data-icon='inline-start' />
                ) : (
                  <WandSparkles data-icon='inline-start' />
                )}
                {isGenerating ? t('Generating') : t('Generate speech')}
              </Button>
            </div>
          </div>
        </div>
      </section>

      <section className='h-40 shrink-0 p-4 sm:h-48 sm:p-6'>
        {!audioURL ? (
          <Empty className='h-full min-h-0'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <Volume2 />
              </EmptyMedia>
              <EmptyTitle>{t('Speech workspace')}</EmptyTitle>
              <EmptyDescription>
                {t('Generated audio will appear here.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className='mx-auto flex h-full w-full max-w-4xl flex-col justify-center gap-3 sm:flex-row sm:items-center'>
            <div className='bg-muted/20 min-w-0 flex-1 rounded-md border p-3'>
              <audio src={audioURL} controls autoPlay className='w-full' />
            </div>
            <div className='flex shrink-0 justify-end'>
              <a href={audioURL} download={`playground-speech.${format}`}>
                <Button variant='outline'>
                  <Download data-icon='inline-start' />
                  {t('Download')}
                </Button>
              </a>
            </div>
          </div>
        )}
      </section>
    </div>
  )
}
