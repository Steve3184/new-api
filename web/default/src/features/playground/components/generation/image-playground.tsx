/* Copyright (C) 2023-2026 QuantumNous */
import {
  Download,
  ImageIcon,
  Loader2,
  Pencil,
  Upload,
  WandSparkles,
  X,
} from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
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
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import { editImage, generateImage } from '../../api'
import type {
  GroupOption,
  ImageGenerationResponse,
  ModelOption,
} from '../../types'
import { GenerationControls } from './generation-controls'
import {
  getGenerationErrorMessage,
  imageResponseSource,
  workspaceImageToFile,
} from './generation-utils'
import { useGenerationModel } from './use-generation-model'

const SIZE_OPTIONS = [
  { value: '1024x1024', label: '1K · 1:1' },
  { value: '1536x1024', label: '1.5K · 3:2' },
  { value: '1024x1536', label: '1.5K · 2:3' },
  { value: '2048x2048', label: '2K · 1:1' },
  { value: '4096x4096', label: '4K · 1:1' },
]
const QUALITY_OPTIONS = [
  { value: 'default', labelKey: 'Auto' },
  { value: 'standard', labelKey: 'Standard' },
  { value: 'high', labelKey: 'High' },
]

type ImagePlaygroundProps = {
  models: ModelOption[]
  groups: GroupOption[]
  group: string
  onGroupChange: (value: string) => void
  groupModels: Record<string, string[]>
}

export function ImagePlayground(props: ImagePlaygroundProps) {
  const { t } = useTranslation()
  const { model, setModel, group } = useGenerationModel({
    models: props.models,
    groups: props.groups,
    group: props.group,
    groupModels: props.groupModels,
    onGroupChange: props.onGroupChange,
  })
  const [mode, setMode] = useState<'generate' | 'edit'>('generate')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [quality, setQuality] = useState('default')
  const [count, setCount] = useState(1)
  const [sourceFile, setSourceFile] = useState<File | null>(null)
  const [sourcePreview, setSourcePreview] = useState('')
  const [result, setResult] = useState<ImageGenerationResponse | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const controlsRef = useRef<HTMLElement>(null)
  const qualityOptions = useMemo(
    () =>
      QUALITY_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.labelKey),
      })),
    [t]
  )

  const handleFile = (file: File | null) => {
    if (sourcePreview) URL.revokeObjectURL(sourcePreview)
    setSourceFile(file)
    setSourcePreview(file ? URL.createObjectURL(file) : '')
  }

  const handleGenerate = async () => {
    if (!model || !prompt.trim()) return
    if (mode === 'edit' && !sourceFile) {
      toast.error(t('Select an image to edit'))
      return
    }

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller
    setIsGenerating(true)
    try {
      if (mode === 'edit' && sourceFile) {
        const form = new FormData()
        form.set('model', model)
        form.set('group', group)
        form.set('prompt', prompt.trim())
        form.set('size', size)
        if (quality !== 'default') form.set('quality', quality)
        form.set('n', String(count))
        form.set('response_format', 'b64_json')
        form.set('image', sourceFile)
        setResult(await editImage(form, controller.signal))
      } else {
        setResult(
          await generateImage(
            {
              model,
              group,
              prompt: prompt.trim(),
              size,
              quality: quality === 'default' ? undefined : quality,
              n: count,
              response_format: 'b64_json',
            },
            controller.signal
          )
        )
      }
    } catch (error) {
      if (!controller.signal.aborted) {
        toast.error(
          await getGenerationErrorMessage(error, t('Image generation failed'))
        )
      }
    } finally {
      if (abortRef.current === controller) abortRef.current = null
      setIsGenerating(false)
    }
  }

  const handleWorkspaceEdit = async (source: string, index: number) => {
    try {
      handleFile(
        await workspaceImageToFile(
          source,
          `playground-${result?.created ?? Date.now()}-${index + 1}`
        )
      )
      setMode('edit')
      window.requestAnimationFrame(() => {
        controlsRef.current?.scrollIntoView({ block: 'start' })
      })
    } catch {
      toast.error(t('File read failed'))
    }
  }

  return (
    <div className='grid size-full min-h-0 grid-rows-[max-content_minmax(28rem,1fr)] overflow-y-auto lg:grid-cols-[minmax(19rem,24rem)_1fr] lg:grid-rows-1 lg:overflow-hidden'>
      <section
        ref={controlsRef}
        className='border-b p-4 sm:p-6 lg:min-h-0 lg:overflow-y-auto lg:border-r lg:border-b-0'
      >
        <div className='space-y-5'>
          <ToggleGroup
            value={[mode]}
            onValueChange={(values) => {
              const next = values[0]
              if (next === 'generate' || next === 'edit') setMode(next)
            }}
            className='w-full'
          >
            <ToggleGroupItem value='generate' className='flex-1'>
              {t('Generate')}
            </ToggleGroupItem>
            <ToggleGroupItem value='edit' className='flex-1'>
              {t('Edit')}
            </ToggleGroupItem>
          </ToggleGroup>

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

          {mode === 'edit' && (
            <Field>
              <FieldLabel>{t('Source image')}</FieldLabel>
              <input
                ref={fileInputRef}
                type='file'
                accept='image/*'
                className='sr-only'
                onChange={(event) =>
                  handleFile(event.target.files?.[0] ?? null)
                }
              />
              {sourcePreview ? (
                <div className='bg-muted relative aspect-video overflow-hidden rounded-md border'>
                  <img
                    src={sourcePreview}
                    alt={t('Source image')}
                    className='size-full object-contain'
                  />
                  <Button
                    type='button'
                    variant='secondary'
                    size='icon'
                    className='absolute top-2 right-2'
                    onClick={() => handleFile(null)}
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
          )}

          <Field>
            <FieldLabel htmlFor='playground-image-prompt'>
              {t('Prompt')}
            </FieldLabel>
            <Textarea
              id='playground-image-prompt'
              rows={6}
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={
                mode === 'edit'
                  ? t('Describe the changes')
                  : t('Describe the image')
              }
              disabled={isGenerating}
            />
          </Field>

          <FieldGroup className='grid grid-cols-2 gap-4'>
            <Field>
              <FieldLabel>{t('Image size')}</FieldLabel>
              <Select
                items={SIZE_OPTIONS}
                value={size}
                onValueChange={(value) => value && setSize(value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {SIZE_OPTIONS.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel>{t('Quality')}</FieldLabel>
              <Select
                items={qualityOptions}
                value={quality}
                onValueChange={(value) => value && setQuality(value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {qualityOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          </FieldGroup>

          <Field>
            <FieldLabel htmlFor='playground-image-count'>
              {t('Image count')}
            </FieldLabel>
            <Input
              id='playground-image-count'
              type='number'
              min={1}
              max={4}
              value={count}
              onChange={(event) =>
                setCount(
                  Math.min(4, Math.max(1, Number(event.target.value) || 1))
                )
              }
            />
          </Field>

          <Button
            className='w-full'
            disabled={!model || !prompt.trim() || isGenerating}
            onClick={handleGenerate}
          >
            {isGenerating ? (
              <Loader2 className='animate-spin' data-icon='inline-start' />
            ) : (
              <WandSparkles data-icon='inline-start' />
            )}
            {isGenerating
              ? t('Generating')
              : t(mode === 'edit' ? 'Edit image' : 'Generate image')}
          </Button>
        </div>
      </section>

      <section className='min-h-[28rem] overflow-y-auto p-4 sm:p-6'>
        {!result?.data.length ? (
          <Empty className='h-full min-h-[24rem]'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <ImageIcon />
              </EmptyMedia>
              <EmptyTitle>{t('Image workspace')}</EmptyTitle>
              <EmptyDescription>
                {t('Generated images will appear here.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className='grid gap-4 sm:grid-cols-2'>
            {result.data.map((image, index) => {
              const source = imageResponseSource(image)
              return (
                <figure
                  key={`${result.created}-${source.slice(-96)}`}
                  className='group bg-muted/30 relative overflow-hidden rounded-md border'
                >
                  <img
                    src={source}
                    alt={image.revised_prompt || t('Generated image')}
                    className='aspect-square size-full object-contain'
                    loading='lazy'
                  />
                  <div className='absolute top-2 right-2 flex gap-2'>
                    <Button
                      type='button'
                      variant='secondary'
                      size='icon'
                      aria-label={t('Edit image')}
                      disabled={!source}
                      onClick={() => void handleWorkspaceEdit(source, index)}
                    >
                      <Pencil />
                    </Button>
                    <a
                      href={source}
                      download={`playground-${result.created}-${index + 1}.png`}
                    >
                      <Button
                        type='button'
                        variant='secondary'
                        size='icon'
                        aria-label={t('Download')}
                      >
                        <Download />
                      </Button>
                    </a>
                  </div>
                  {image.revised_prompt && (
                    <figcaption className='text-muted-foreground border-t px-3 py-2 text-xs'>
                      {image.revised_prompt}
                    </figcaption>
                  )}
                </figure>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
