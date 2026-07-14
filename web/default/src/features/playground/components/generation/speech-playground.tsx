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
import { OPENAI_SPEECH_VOICES } from '../../constants'
import { AZURE_TTS_VOICE_OPTIONS } from '../../data/azure-tts-voices'
import type { GroupOption, ModelOption, SpeechModelType } from '../../types'
import { GenerationControls } from './generation-controls'
import { getGenerationErrorMessage } from './generation-utils'
import { useGenerationModel } from './use-generation-model'

const OPENAI_VOICE_OPTIONS = OPENAI_SPEECH_VOICES.map((voice) => ({
  label: voice,
  value: voice,
}))
const FORMAT_OPTIONS = ['mp3', 'wav', 'opus', 'aac', 'flac', 'pcm']

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
  const { model, setModel } = useGenerationModel(props.models)
  const modelType = props.modelTypes[model] ?? 'openai'
  const [input, setInput] = useState('')
  const [voice, setVoice] = useState('alloy')
  const [format, setFormat] = useState('mp3')
  const [speed, setSpeed] = useState(1)
  const [volume, setVolume] = useState(1)
  const [pitch, setPitch] = useState(0)
  const [volumeEnabled, setVolumeEnabled] = useState(false)
  const [pitchEnabled, setPitchEnabled] = useState(false)
  const [audioURL, setAudioURL] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)

  useEffect(() => {
    setVoice(modelType === 'azure' ? 'zh-CN-XiaoxiaoNeural' : 'alloy')
  }, [modelType])

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
        group: props.group,
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

  return (
    <div className='mx-auto grid size-full min-h-0 max-w-7xl grid-rows-[max-content_minmax(12rem,auto)] overflow-y-auto lg:grid-cols-[minmax(30rem,3fr)_minmax(18rem,2fr)] lg:grid-rows-1 lg:overflow-hidden'>
      <section className='border-b p-4 sm:p-6 lg:min-h-0 lg:overflow-y-auto lg:border-r lg:border-b-0'>
        <div className='space-y-5'>
          <GenerationControls
            groups={props.groups}
            group={props.group}
            onGroupChange={props.onGroupChange}
            models={props.models}
            model={model}
            onModelChange={setModel}
            disabled={isGenerating}
            groupModels={props.groupModels}
          />

          <Field>
            <FieldLabel htmlFor='playground-speech-input'>
              {t('Text')}
            </FieldLabel>
            <Textarea
              id='playground-speech-input'
              rows={14}
              className='min-h-64 lg:min-h-[22rem]'
              value={input}
              onChange={(event) => setInput(event.target.value)}
              placeholder={t('Enter text to synthesize')}
              disabled={isGenerating}
            />
          </Field>

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
                value={format}
                onValueChange={(value) => value && setFormat(value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {FORMAT_OPTIONS.map((value) => (
                      <SelectItem key={value} value={value}>
                        {value.toUpperCase()}
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
                onChange={(event) => setSpeed(Number(event.target.value) || 1)}
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
                  onChange={(event) => setVolume(Number(event.target.value))}
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
      </section>

      <section className='min-h-48 p-4 sm:p-6'>
        {!audioURL ? (
          <Empty className='h-full min-h-40'>
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
          <div className='mx-auto flex h-full max-w-2xl flex-col justify-center gap-5'>
            <div className='bg-muted/20 rounded-md border p-5'>
              <audio src={audioURL} controls autoPlay className='w-full' />
            </div>
            <div className='flex justify-end'>
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
