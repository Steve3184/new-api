/* Copyright (C) 2023-2026 QuantumNous */
import { Box, Download, Loader2, Upload, WandSparkles, X } from 'lucide-react'
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldLabel } from '@/components/ui/field'
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
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'

import { generateThreeD, getThreeDTask } from '../../api'
import type {
  GroupOption,
  ModelOption,
  ThreeDGenerationResponse,
} from '../../types'
import { GenerationControls } from './generation-controls'
import {
  getGenerationErrorMessage,
  readFileAsDataURL,
} from './generation-utils'
import { useGenerationModel } from './use-generation-model'

const ThreeDViewer = lazy(() =>
  import('./three-d-viewer').then((module) => ({
    default: module.ThreeDViewer,
  }))
)
const ART_STYLES = ['realistic', 'cartoon', 'sculpture', 'pbr']

type ThreeDPlaygroundProps = {
  models: ModelOption[]
  groups: GroupOption[]
  group: string
  onGroupChange: (value: string) => void
  groupModels: Record<string, string[]>
}

export function ThreeDPlayground(props: ThreeDPlaygroundProps) {
  const { t } = useTranslation()
  const { model, setModel, group, setGroup } = useGenerationModel({
    models: props.models,
    groups: props.groups,
    group: props.group,
    groupModels: props.groupModels,
    onGroupChange: props.onGroupChange,
  })
  const [prompt, setPrompt] = useState('')
  const [sourceTaskId, setSourceTaskId] = useState('')
  const [inputReference, setInputReference] = useState('')
  const [inputPreview, setInputPreview] = useState('')
  const [artStyle, setArtStyle] = useState('realistic')
  const [task, setTask] = useState<ThreeDGenerationResponse | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const pollAbortRef = useRef<AbortController | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const isTextureModel = model.endsWith('-texture')
  const artStyleOptions = useMemo(
    () =>
      ART_STYLES.map((style) => ({
        value: style,
        label: t(style[0].toUpperCase() + style.slice(1)),
      })),
    [t]
  )

  useEffect(
    () => () => {
      pollAbortRef.current?.abort()
    },
    []
  )

  const pollTask = async (taskId: string) => {
    pollAbortRef.current?.abort()
    const controller = new AbortController()
    pollAbortRef.current = controller
    let consecutiveErrors = 0
    for (
      let attempt = 0;
      attempt < 300 && !controller.signal.aborted;
      attempt++
    ) {
      if (attempt > 0) {
        await new Promise((resolve) => window.setTimeout(resolve, 2000))
      }
      let next: ThreeDGenerationResponse
      try {
        next = await getThreeDTask(taskId, controller.signal)
        consecutiveErrors = 0
      } catch (error) {
        if (controller.signal.aborted) break
        consecutiveErrors++
        if (consecutiveErrors >= 5) throw error
        continue
      }
      setTask(next)
      if (next.status === 'completed' || next.status === 'failed') break
    }
    if (pollAbortRef.current === controller) pollAbortRef.current = null
  }

  const handleImage = async (file: File | null) => {
    if (!file) {
      setInputReference('')
      setInputPreview('')
      return
    }
    try {
      const dataURL = await readFileAsDataURL(file)
      setInputReference(dataURL)
      setInputPreview(dataURL)
    } catch (error) {
      toast.error(await getGenerationErrorMessage(error, t('File read failed')))
    }
  }

  const handleGenerate = async () => {
    if (!model) return
    setIsSubmitting(true)
    try {
      const next = await generateThreeD({
        model,
        group,
        ...(isTextureModel
          ? { source_task_id: sourceTaskId.trim() }
          : {
              prompt: prompt.trim() || undefined,
              input_reference: inputReference || undefined,
              metadata: { art_style: artStyle },
            }),
      })
      setTask(next)
      if (next.status !== 'failed') {
        await pollTask(next.id)
      }
    } catch (error) {
      toast.error(
        await getGenerationErrorMessage(error, t('3D generation failed'))
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const canSubmit = isTextureModel
    ? Boolean(sourceTaskId.trim())
    : Boolean(prompt.trim() || inputReference)

  return (
    <div className='grid size-full min-h-0 grid-rows-[max-content_minmax(30rem,1fr)] overflow-y-auto lg:grid-cols-[minmax(19rem,24rem)_1fr] lg:grid-rows-1 lg:overflow-hidden'>
      <section className='border-b p-4 sm:p-6 lg:min-h-0 lg:overflow-y-auto lg:border-r lg:border-b-0'>
        <div className='space-y-5'>
          <GenerationControls
            groups={props.groups}
            group={group}
            onGroupChange={setGroup}
            models={props.models}
            model={model}
            onModelChange={setModel}
            disabled={isSubmitting}
            groupModels={props.groupModels}
          />

          {isTextureModel ? (
            <Field>
              <FieldLabel htmlFor='playground-3d-source-task'>
                {t('Source task ID')}
              </FieldLabel>
              <Input
                id='playground-3d-source-task'
                value={sourceTaskId}
                onChange={(event) => setSourceTaskId(event.target.value)}
                placeholder='task_...'
              />
            </Field>
          ) : (
            <>
              <Field>
                <FieldLabel htmlFor='playground-3d-prompt'>
                  {t('Prompt')}
                </FieldLabel>
                <Textarea
                  id='playground-3d-prompt'
                  rows={6}
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  placeholder={t('Describe the 3D object')}
                />
              </Field>
              <Field>
                <FieldLabel>{t('Reference image')}</FieldLabel>
                <input
                  ref={fileInputRef}
                  type='file'
                  accept='image/*'
                  className='sr-only'
                  onChange={(event) =>
                    void handleImage(event.target.files?.[0] ?? null)
                  }
                />
                {inputPreview ? (
                  <div className='bg-muted relative aspect-video overflow-hidden rounded-md border'>
                    <img
                      src={inputPreview}
                      alt={t('Reference image')}
                      className='size-full object-contain'
                    />
                    <Button
                      type='button'
                      variant='secondary'
                      size='icon'
                      className='absolute top-2 right-2'
                      onClick={() => void handleImage(null)}
                      aria-label={t('Remove image')}
                    >
                      <X />
                    </Button>
                  </div>
                ) : (
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <Upload data-icon='inline-start' />
                    {t('Choose image')}
                  </Button>
                )}
              </Field>
              <Field>
                <FieldLabel>{t('Art style')}</FieldLabel>
                <Select
                  items={artStyleOptions}
                  value={artStyle}
                  onValueChange={(value) => value && setArtStyle(value)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {artStyleOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            </>
          )}

          <Button
            className='w-full'
            disabled={!model || !canSubmit || isSubmitting}
            onClick={handleGenerate}
          >
            {isSubmitting ? (
              <Loader2 className='animate-spin' data-icon='inline-start' />
            ) : (
              <WandSparkles data-icon='inline-start' />
            )}
            {isSubmitting ? t('Generating') : t('Generate 3D model')}
          </Button>

          {task && task.status !== 'completed' && task.status !== 'failed' && (
            <div className='space-y-2' aria-live='polite'>
              <div className='flex justify-between text-xs'>
                <span>{t('Processing')}</span>
                <span>{task.progress}%</span>
              </div>
              <Progress value={task.progress} />
            </div>
          )}
        </div>
      </section>

      <section className='relative min-h-[30rem] overflow-hidden'>
        {task?.status === 'completed' && task.data?.url ? (
          <>
            <Suspense
              fallback={
                <Skeleton className='h-[clamp(28rem,68vh,52rem)] w-full rounded-none' />
              }
            >
              <ThreeDViewer key={task.data.url} url={task.data.url} />
            </Suspense>
            <a
              href={task.data.url}
              download
              className='absolute top-4 right-4 z-10'
            >
              <Button variant='secondary'>
                <Download data-icon='inline-start' />
                {t('Download GLB')}
              </Button>
            </a>
          </>
        ) : (
          <Empty className='h-full min-h-[30rem]'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <Box />
              </EmptyMedia>
              <EmptyTitle>
                {task?.status === 'failed'
                  ? t('Generation failed')
                  : t('3D workspace')}
              </EmptyTitle>
              <EmptyDescription>
                {task?.error?.message ??
                  t('The generated model will appear here.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
      </section>
    </div>
  )
}
