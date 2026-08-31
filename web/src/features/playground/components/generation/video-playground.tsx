/* Copyright (C) 2023-2026 QuantumNous */
import { Download, Loader2, Upload, Video, WandSparkles, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Textarea } from '@/components/ui/textarea'

import { generateVideo, getVideoTask } from '../../api'
import type { GroupOption, ModelOption, VideoGenerationResponse } from '../../types'
import { GenerationControls } from './generation-controls'
import { getGenerationErrorMessage, readFileAsDataURL } from './generation-utils'
import { useGenerationModel } from './use-generation-model'

type VideoPlaygroundProps = {
  models: ModelOption[]
  groups: GroupOption[]
  group: string
  onGroupChange: (value: string) => void
  groupModels: Record<string, string[]>
}

export function VideoPlayground(props: VideoPlaygroundProps) {
  const { t } = useTranslation()
  const { model, setModel, group, setGroup } = useGenerationModel({ ...props })
  const [prompt, setPrompt] = useState('')
  const [images, setImages] = useState<string[]>([])
  const [mode, setMode] = useState('text')
  const [size, setSize] = useState('720P')
  const [duration, setDuration] = useState(5)
  const [task, setTask] = useState<VideoGenerationResponse | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const promptRef = useRef<HTMLTextAreaElement>(null)
  const pollRef = useRef<AbortController | null>(null)

  useEffect(() => () => pollRef.current?.abort(), [])

  const addImages = async (files: FileList | null) => {
    if (!files) return
    try {
      const next = await Promise.all([...files].slice(0, 5).map(readFileAsDataURL))
      setImages((current) => [...current, ...next].slice(0, 5))
    } catch (error) {
      toast.error(await getGenerationErrorMessage(error, t('File read failed')))
    }
  }

  const insertImageReference = (index: number) => {
    const token = `<Picture ${index + 1}>`
    const textarea = promptRef.current
    const start = textarea?.selectionStart ?? prompt.length
    const end = textarea?.selectionEnd ?? start
    const nextPrompt = `${prompt.slice(0, start)}${token}${prompt.slice(end)}`
    setPrompt(nextPrompt)
    requestAnimationFrame(() => {
      textarea?.focus()
      const caret = start + token.length
      textarea?.setSelectionRange(caret, caret)
    })
  }

  const pollTask = async (id: string) => {
    pollRef.current?.abort()
    const controller = new AbortController()
    pollRef.current = controller
    for (let attempt = 0; attempt < 300 && !controller.signal.aborted; attempt++) {
      if (attempt > 0) await new Promise((resolve) => window.setTimeout(resolve, 2000))
      try {
        const next = await getVideoTask(id, controller.signal)
        setTask(next)
        if (next.status === 'completed' || next.status === 'failed') break
      } catch (error) {
        if (controller.signal.aborted) break
        if (attempt > 4) throw error
      }
    }
    if (pollRef.current === controller) pollRef.current = null
  }

  const handleGenerate = async () => {
    if (!model || !prompt.trim()) return
    setIsSubmitting(true)
    try {
      const next = await generateVideo({
        model,
        group,
        prompt: prompt.trim(),
        images: images.length ? images : undefined,
        mode: model.includes('2.5') ? mode : undefined,
        size: model.includes('2.5') ? size : undefined,
        duration,
      })
      setTask(next)
      if (next.status !== 'failed') await pollTask(next.id)
    } catch (error) {
      toast.error(await getGenerationErrorMessage(error, t('Video generation failed')))
    } finally {
      setIsSubmitting(false)
    }
  }

  const isAgnes25 = model.includes('2.5')
  return (
    <div className='grid size-full min-h-0 grid-rows-[max-content_minmax(30rem,1fr)] overflow-y-auto lg:grid-cols-[minmax(19rem,24rem)_1fr] lg:grid-rows-1 lg:overflow-hidden'>
      <section className='border-b p-4 sm:p-6 lg:min-h-0 lg:overflow-y-auto lg:border-r lg:border-b-0'>
        <div className='space-y-5'>
          <GenerationControls {...props} group={group} onGroupChange={setGroup} model={model} onModelChange={setModel} disabled={isSubmitting} />
          <Field>
            <FieldLabel htmlFor='playground-video-prompt'>{t('Prompt')}</FieldLabel>
            <Textarea ref={promptRef} id='playground-video-prompt' rows={6} value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder={t('Describe the video you want to create')} />
          </Field>
          {isAgnes25 && (
            <Field>
              <FieldLabel htmlFor='playground-video-mode'>{t('Video mode')}</FieldLabel>
              <select id='playground-video-mode' className='border-input bg-background h-10 w-full rounded-md border px-3 text-sm' value={mode} onChange={(event) => setMode(event.target.value)}>
                <option value='text'>{t('Text')}</option>
                <option value='keyframe'>{t('Keyframe')}</option>
                <option value='reference'>{t('Reference')}</option>
              </select>
            </Field>
          )}
          {isAgnes25 && (
            <Field>
              <FieldLabel htmlFor='playground-video-size'>{t('Size')}</FieldLabel>
              <Input id='playground-video-size' value={size} onChange={(event) => setSize(event.target.value)} />
            </Field>
          )}
          <Field>
            <FieldLabel htmlFor='playground-video-duration'>{t('Duration (seconds)')}</FieldLabel>
            <Input id='playground-video-duration' type='number' min={1} max={3600} value={duration} onChange={(event) => setDuration(Math.max(1, Math.min(3600, Number(event.target.value) || 1)))} />
          </Field>
          <Field>
            <FieldLabel>{t('Reference images')}</FieldLabel>
            <input ref={inputRef} type='file' accept='image/*' multiple className='sr-only' onChange={(event) => void addImages(event.target.files)} />
            <div className='flex flex-wrap gap-2'>
              {images.map((image, index) => (
                <div key={image} className='relative size-16 overflow-hidden rounded border'>
                  <img src={image} alt={t('Reference image')} className='size-full object-cover' />
                  <Button type='button' variant='secondary' size='icon-xs' className='absolute top-0 right-0' onClick={() => setImages((current) => current.filter((_, item) => item !== index))} aria-label={t('Remove image')}><X /></Button>
                  <Button type='button' variant='secondary' size='xs' className='absolute bottom-0 left-0 h-5 px-1 text-[10px]' onClick={() => insertImageReference(index)} title={t('Insert reference image {{index}}', { index: index + 1 })} aria-label={t('Insert reference image {{index}}', { index: index + 1 })}>P{index + 1}</Button>
                </div>
              ))}
              {images.length < 5 && <Button type='button' variant='outline' onClick={() => inputRef.current?.click()}><Upload data-icon='inline-start' />{t('Choose image')}</Button>}
            </div>
          </Field>
          <Button className='w-full' disabled={!model || !prompt.trim() || isSubmitting} onClick={handleGenerate}>
            {isSubmitting ? <Loader2 className='animate-spin' data-icon='inline-start' /> : <WandSparkles data-icon='inline-start' />}
            {isSubmitting ? t('Generating') : t('Generate video')}
          </Button>
          {task && task.status !== 'completed' && task.status !== 'failed' && <div className='space-y-2' aria-live='polite'><div className='flex justify-between text-xs'><span>{t('Processing')}</span><span>{task.progress}%</span></div><Progress value={task.progress} /></div>}
        </div>
      </section>
      <section className='relative min-h-[30rem] overflow-hidden'>
        {task?.status === 'completed' && task.data?.url ? <><video className='size-full object-contain' controls src={task.data.url} /><a href={task.data.url} download className='absolute top-4 right-4 z-10'><Button variant='secondary'><Download data-icon='inline-start' />{t('Download video')}</Button></a></> : <Empty className='h-full min-h-[30rem]'><EmptyHeader><EmptyMedia variant='icon'><Video /></EmptyMedia><EmptyTitle>{task?.status === 'failed' ? t('Generation failed') : t('Video workspace')}</EmptyTitle><EmptyDescription>{task?.error?.message ?? t('The generated video will appear here.')}</EmptyDescription></EmptyHeader></Empty>}
      </section>
    </div>
  )
}
