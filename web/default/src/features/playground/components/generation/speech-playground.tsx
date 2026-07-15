/* Copyright (C) 2023-2026 QuantumNous */
import { Download, Loader2, Volume2, WandSparkles } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
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
import { Progress } from '@/components/ui/progress'
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

import {
  generateSpeech,
  generateSpeechTask,
  getSpeechTask,
  getSpeechTaskContent,
} from '../../api'
import { AZURE_TTS_VOICE_OPTIONS } from '../../azure-tts-voices'
import { OPENAI_SPEECH_VOICES, UNREAL_SPEECH_VOICES } from '../../constants'
import type {
  GroupOption,
  ModelOption,
  SpeechGenerationRequest,
  SpeechModelType,
} from '../../types'
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
const UNREAL_STREAM_FORMAT_OPTIONS = ['mp3', 'pcm'].map((value) => ({
  label: value.toUpperCase(),
  value,
}))
const UNREAL_SPEECH_FORMAT_OPTIONS = [{ label: 'MP3', value: 'mp3' }]
const UNREAL_BITRATE_OPTIONS = [
  '16k',
  '32k',
  '48k',
  '64k',
  '128k',
  '192k',
  '256k',
  '320k',
].map((value) => ({ label: value, value }))

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
  const [unrealVoice, setUnrealVoice] = useState('Sierra')
  const [unrealMode, setUnrealMode] = useState<'speech' | 'stream' | 'async'>(
    'speech'
  )
  const [unrealBitrate, setUnrealBitrate] = useState('192k')
  const [unrealPitch, setUnrealPitch] = useState(1)
  const [format, setFormat] = useState('mp3')
  const [speed, setSpeed] = useState(1)
  const [unrealSpeed, setUnrealSpeed] = useState(0)
  const [volume, setVolume] = useState(1)
  const [pitch, setPitch] = useState(0)
  const [volumeEnabled, setVolumeEnabled] = useState(false)
  const [pitchEnabled, setPitchEnabled] = useState(false)
  const [audioURL, setAudioURL] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)
  const [taskProgress, setTaskProgress] = useState<number | null>(null)
  const pollAbortRef = useRef<AbortController | null>(null)

  const isUnrealSpeech = modelType === 'unrealspeech'
  const inputCharacters = useMemo(() => [...input].length, [input])
  const streamDisabled = isUnrealSpeech && inputCharacters > 1000
  const syncDisabled = isUnrealSpeech && inputCharacters > 5000
  let selectedUnrealMode = unrealMode
  if (syncDisabled) {
    selectedUnrealMode = 'async'
  } else if (streamDisabled && unrealMode === 'stream') {
    selectedUnrealMode = 'speech'
  }
  const unrealVoiceOptions = useMemo(
    () =>
      UNREAL_SPEECH_VOICES.map((item) => ({
        label: `${item.value} (${t(item.language)})`,
        value: item.value,
      })),
    [t]
  )
  let voiceOptions: Array<{ label: string; value: string }> =
    OPENAI_VOICE_OPTIONS
  let voice = openAIVoice
  let setVoice = setOpenAIVoice
  if (modelType === 'azure') {
    voiceOptions = AZURE_TTS_VOICE_OPTIONS
    voice = azureVoice
    setVoice = setAzureVoice
  } else if (isUnrealSpeech) {
    voiceOptions = unrealVoiceOptions
    voice = unrealVoice
    setVoice = setUnrealVoice
  }
  let formatOptions: Array<{ label: string; value: string }> = FORMAT_OPTIONS
  if (isUnrealSpeech) {
    formatOptions =
      selectedUnrealMode === 'stream'
        ? UNREAL_STREAM_FORMAT_OPTIONS
        : UNREAL_SPEECH_FORMAT_OPTIONS
  }
  const selectedFormat = formatOptions.some((option) => option.value === format)
    ? format
    : 'mp3'
  const selectedSpeed = isUnrealSpeech ? unrealSpeed : speed

  useEffect(
    () => () => {
      if (audioURL) URL.revokeObjectURL(audioURL)
    },
    [audioURL]
  )

  useEffect(
    () => () => {
      pollAbortRef.current?.abort()
    },
    []
  )

  const pollSpeechTask = async (taskId: string) => {
    pollAbortRef.current?.abort()
    const controller = new AbortController()
    pollAbortRef.current = controller
    try {
      for (
        let attempt = 0;
        attempt < 300 && !controller.signal.aborted;
        attempt++
      ) {
        if (attempt > 0) {
          await new Promise((resolve) => window.setTimeout(resolve, 2000))
        }
        const task = await getSpeechTask(taskId, controller.signal)
        setTaskProgress(task.progress)
        if (task.status === 'failed') {
          throw new Error(task.error?.message || t('Speech generation failed'))
        }
        if (task.status === 'completed') {
          return getSpeechTaskContent(taskId, controller.signal)
        }
      }
      throw new Error(t('Speech generation failed'))
    } finally {
      if (pollAbortRef.current === controller) pollAbortRef.current = null
    }
  }

  const handleInputChange = (value: string) => {
    setInput(value)
    if (!isUnrealSpeech) return
    const characters = [...value].length
    if (characters > 5000) {
      setUnrealMode('async')
    } else if (characters > 1000 && unrealMode === 'stream') {
      setUnrealMode('speech')
    }
  }

  const handleGenerate = async () => {
    if (!model || !input.trim()) return
    setIsGenerating(true)
    setTaskProgress(isUnrealSpeech && selectedUnrealMode === 'async' ? 0 : null)
    try {
      const payload: SpeechGenerationRequest = {
        model,
        group,
        input: input.trim(),
        voice,
        response_format: selectedFormat,
        speed: selectedSpeed,
        ...(modelType === 'azure' && volumeEnabled ? { volume } : {}),
        ...(modelType === 'azure' && pitchEnabled ? { pitch } : {}),
        ...(isUnrealSpeech
          ? {
              stream: selectedUnrealMode === 'stream',
              speech: selectedUnrealMode === 'speech',
              bitrate: unrealBitrate,
              pitch: unrealPitch,
            }
          : {}),
      }
      let blob: Blob
      if (isUnrealSpeech && selectedUnrealMode === 'async') {
        const task = await generateSpeechTask(payload)
        setTaskProgress(task.progress)
        if (task.status === 'failed') {
          throw new Error(task.error?.message || t('Speech generation failed'))
        }
        blob = await pollSpeechTask(task.id)
      } else {
        blob = await generateSpeech(payload)
      }
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
                onChange={(event) => handleInputChange(event.target.value)}
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

              {isUnrealSpeech && (
                <FieldGroup className='grid grid-cols-2 gap-4'>
                  <Field>
                    <FieldLabel>{t('Mode')}</FieldLabel>
                    <Select
                      items={[
                        { value: 'speech', label: t('Speech') },
                        { value: 'stream', label: t('Stream') },
                        { value: 'async', label: t('Async') },
                      ]}
                      value={selectedUnrealMode}
                      onValueChange={(value) => {
                        if (
                          value === 'async' ||
                          (value === 'speech' && !syncDisabled) ||
                          (value === 'stream' && !streamDisabled)
                        ) {
                          setUnrealMode(value)
                        }
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          <SelectItem value='speech' disabled={syncDisabled}>
                            {t('Speech')}
                          </SelectItem>
                          <SelectItem value='stream' disabled={streamDisabled}>
                            {t('Stream')}
                          </SelectItem>
                          <SelectItem value='async'>{t('Async')}</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field>
                    <FieldLabel>{t('Bitrate')}</FieldLabel>
                    <Select
                      items={UNREAL_BITRATE_OPTIONS}
                      value={unrealBitrate}
                      onValueChange={(value) =>
                        value && setUnrealBitrate(value)
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {UNREAL_BITRATE_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                </FieldGroup>
              )}

              <FieldGroup className='grid grid-cols-2 gap-4'>
                <Field>
                  <FieldLabel>{t('Format')}</FieldLabel>
                  <Select
                    items={formatOptions}
                    value={selectedFormat}
                    onValueChange={(value) => value && setFormat(value)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {formatOptions.map((option) => (
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
                    min={isUnrealSpeech ? -1 : 0.25}
                    max={isUnrealSpeech ? 1 : 4}
                    step={0.05}
                    value={selectedSpeed}
                    onChange={(event) => {
                      const value = Number(event.target.value)
                      if (isUnrealSpeech) {
                        setUnrealSpeed(Number.isFinite(value) ? value : 0)
                      } else {
                        setSpeed(value || 1)
                      }
                    }}
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

              {isUnrealSpeech && (
                <Field>
                  <FieldLabel htmlFor='playground-speech-unreal-pitch'>
                    {t('Pitch')}
                  </FieldLabel>
                  <Input
                    id='playground-speech-unreal-pitch'
                    type='number'
                    min={0.5}
                    max={1.5}
                    step={0.01}
                    value={unrealPitch}
                    disabled={isGenerating}
                    onChange={(event) =>
                      setUnrealPitch(Number(event.target.value) || 1)
                    }
                  />
                </Field>
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

              {isGenerating && taskProgress !== null && (
                <div className='space-y-2' aria-live='polite'>
                  <div className='flex justify-between text-xs'>
                    <span>{t('Processing')}</span>
                    <span>{taskProgress}%</span>
                  </div>
                  <Progress value={taskProgress} />
                </div>
              )}
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
              <a
                href={audioURL}
                download={`playground-speech.${selectedFormat}`}
              >
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
