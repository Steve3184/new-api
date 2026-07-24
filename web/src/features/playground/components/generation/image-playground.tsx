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
import { useEffect, useMemo, useRef, useState } from 'react'
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
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
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
  ImageEditRequest,
  ImageGenerationResponse,
  ModelOption,
} from '../../types'
import { GenerationControls } from './generation-controls'
import {
  getGenerationErrorMessage,
  imageResponseSource,
  imageSizeFromResolution,
  normalizeImageAspectRatio,
  readFileAsDataURL,
} from './generation-utils'
import { useGenerationModel } from './use-generation-model'

const RESOLUTION_OPTIONS = [
  { value: '1024', label: '1K' },
  { value: '1536', label: '1.5K' },
  { value: '2048', label: '2K' },
  { value: '2560', label: '2.5K' },
  { value: '4096', label: '4K' },
]
const ASPECT_RATIO_OPTIONS = ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3']
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

type ImageReference = {
  id: string
  preview: string
  file?: File
  imageURL?: string
}

export function ImagePlayground(props: ImagePlaygroundProps) {
  const { t } = useTranslation()
  const { model, setModel, group, setGroup } = useGenerationModel({
    models: props.models,
    groups: props.groups,
    group: props.group,
    groupModels: props.groupModels,
    onGroupChange: props.onGroupChange,
  })
  const [mode, setMode] = useState<'generate' | 'edit'>('generate')
  const [prompt, setPrompt] = useState('')
  const [resolution, setResolution] = useState('1024')
  const [aspectRatio, setAspectRatio] = useState('1:1')
  const [customAspectRatio, setCustomAspectRatio] = useState('')
  const [quality, setQuality] = useState('default')
  const [count, setCount] = useState(1)
  const [sourceImages, setSourceImages] = useState<ImageReference[]>([])
  const [result, setResult] = useState<ImageGenerationResponse | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const controlsRef = useRef<HTMLElement>(null)
  const sourceImagesRef = useRef<ImageReference[]>([])
  const qualityOptions = useMemo(
    () =>
      QUALITY_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.labelKey),
      })),
    [t]
  )
  const normalizedCustomAspectRatio = customAspectRatio.trim()
    ? normalizeImageAspectRatio(customAspectRatio)
    : null
  const hasInvalidCustomAspectRatio =
    customAspectRatio.trim().length > 0 && !normalizedCustomAspectRatio
  const effectiveAspectRatio = customAspectRatio.trim()
    ? normalizedCustomAspectRatio
    : aspectRatio
  const size = effectiveAspectRatio
    ? imageSizeFromResolution(Number(resolution), effectiveAspectRatio)
    : null

  useEffect(
    () => () => {
      sourceImagesRef.current.forEach((image) => {
        if (image.file) URL.revokeObjectURL(image.preview)
      })
    },
    []
  )

  const addSourceFiles = (files: File[]) => {
    if (files.length === 0) return
    const uploads = files.map((file) => ({
      id: crypto.randomUUID(),
      file,
      preview: URL.createObjectURL(file),
    }))
    setSourceImages((previous) => {
      const next = [...previous, ...uploads]
      sourceImagesRef.current = next
      return next
    })
  }

  const removeSourceImage = (id: string) => {
    setSourceImages((previous) => {
      const removed = previous.find((image) => image.id === id)
      if (removed?.file) URL.revokeObjectURL(removed.preview)
      const next = previous.filter((image) => image.id !== id)
      sourceImagesRef.current = next
      return next
    })
  }

  const clearSourceImages = () => {
    setSourceImages((previous) => {
      previous.forEach((image) => {
        if (image.file) URL.revokeObjectURL(image.preview)
      })
      sourceImagesRef.current = []
      return []
    })
  }

  const handleGenerate = async () => {
    if (!model || !prompt.trim() || !size) return
    if (mode === 'edit' && sourceImages.length === 0) {
      toast.error(t('Select an image to edit'))
      return
    }

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller
    setIsGenerating(true)
    try {
      if (mode === 'edit') {
        const images = await Promise.all(
          sourceImages.map(async (image) => ({
            image_url: image.file
              ? await readFileAsDataURL(image.file)
              : (image.imageURL ?? image.preview),
          }))
        )
        const payload: ImageEditRequest = {
          model,
          group,
          prompt: prompt.trim(),
          size,
          quality: quality === 'default' ? undefined : quality,
          n: count,
          response_format: 'url',
          images,
        }
        setResult(await editImage(payload, controller.signal))
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
              response_format: 'url',
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

  const handleWorkspaceEdit = (source: string, index: number) => {
    clearSourceImages()
    const image = {
      id: `workspace-${result?.created ?? Date.now()}-${index}`,
      imageURL: source,
      preview: source,
    }
    sourceImagesRef.current = [image]
    setSourceImages([image])
    setMode('edit')
    window.requestAnimationFrame(() => {
      controlsRef.current?.scrollIntoView({ block: 'start' })
    })
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
            onGroupChange={setGroup}
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
                multiple
                className='sr-only'
                disabled={isGenerating}
                onChange={(event) => {
                  addSourceFiles([...(event.target.files ?? [])])
                  event.target.value = ''
                }}
              />
              {sourceImages.length > 0 && (
                <div className='grid grid-cols-2 gap-2'>
                  {sourceImages.map((image) => (
                    <div
                      key={image.id}
                      className='bg-muted relative aspect-video overflow-hidden rounded-md border'
                    >
                      <img
                        src={image.preview}
                        alt={t('Source image')}
                        className='size-full object-contain'
                      />
                      <Button
                        type='button'
                        variant='secondary'
                        size='icon'
                        className='absolute top-2 right-2'
                        onClick={() => removeSourceImage(image.id)}
                        aria-label={t('Remove image')}
                      >
                        <X />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
              <Button
                type='button'
                variant='outline'
                disabled={isGenerating}
                onClick={() => fileInputRef.current?.click()}
              >
                <Upload data-icon='inline-start' />
                {t('Choose image')}
              </Button>
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
              <FieldLabel>{t('Resolution')}</FieldLabel>
              <Select
                items={RESOLUTION_OPTIONS}
                value={resolution}
                onValueChange={(value) => value && setResolution(value)}
                disabled={isGenerating}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {RESOLUTION_OPTIONS.map((option) => (
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
                disabled={isGenerating}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
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

          <Field data-invalid={hasInvalidCustomAspectRatio || undefined}>
            <FieldLabel id='playground-image-aspect-ratio-label'>
              {t('Aspect ratio')}
            </FieldLabel>
            <ToggleGroup
              value={customAspectRatio.trim() ? [] : [aspectRatio]}
              onValueChange={(values) => {
                const next = values[0]
                if (!next) return
                setAspectRatio(next)
                setCustomAspectRatio('')
              }}
              variant='outline'
              spacing={2}
              className='grid w-full grid-cols-4'
              aria-labelledby='playground-image-aspect-ratio-label'
              disabled={isGenerating}
            >
              {ASPECT_RATIO_OPTIONS.map((option) => (
                <ToggleGroupItem key={option} value={option} className='w-full'>
                  {option}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <FieldLabel htmlFor='playground-image-custom-aspect-ratio'>
              {t('Custom aspect ratio')}
            </FieldLabel>
            <Input
              id='playground-image-custom-aspect-ratio'
              value={customAspectRatio}
              onChange={(event) => setCustomAspectRatio(event.target.value)}
              placeholder={t('e.g. 5:4')}
              inputMode='numeric'
              aria-invalid={hasInvalidCustomAspectRatio || undefined}
              disabled={isGenerating}
            />
            {hasInvalidCustomAspectRatio && (
              <FieldError>
                {t(
                  'Aspect ratio must use positive whole numbers in width:height format.'
                )}
              </FieldError>
            )}
          </Field>

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
            disabled={
              !model ||
              !prompt.trim() ||
              !size ||
              isGenerating ||
              (mode === 'edit' && sourceImages.length === 0)
            }
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
                  <div className='flex aspect-square items-center justify-center'>
                    <img
                      src={source}
                      alt={image.revised_prompt || t('Generated image')}
                      className='h-auto max-h-full w-auto max-w-full object-contain'
                      loading='lazy'
                    />
                  </div>
                  <div className='absolute top-2 right-2 flex gap-2'>
                    <Button
                      type='button'
                      variant='secondary'
                      size='icon'
                      aria-label={t('Edit image')}
                      disabled={!source}
                      onClick={() => handleWorkspaceEdit(source, index)}
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
