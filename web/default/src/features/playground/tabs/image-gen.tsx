import { useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  ImageIcon,
  UploadIcon,
  XIcon,
  Loader2Icon,
  DownloadIcon,
  SparklesIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { getPlaygroundModels, getUserGroups, generateImage, uploadFile } from '../api'
import { DEFAULT_IMAGE_CONFIG } from '../constants'
import type {
  EndpointType,
  ModelOption,
  GroupOption,
  UploadedFile,
} from '../types'

const IMAGE_SIZES = [
  '1024x1024',
  '1024x1792',
  '1792x1024',
  '768x768',
  '512x512',
]

// Seedream model-specific size configurations
const SEEDREAM_SIZE_CONFIG = [
  { pattern: '5.0-pro', sizes: ['1K', '1.5K', '2K'], defaultSize: '2K' },
  { pattern: '5.0-lite', sizes: ['2K', '3K', '4K'], defaultSize: '2K' },
  { pattern: '4.5', sizes: ['2K', '4K'], defaultSize: '2K' },
  { pattern: '4.0', sizes: ['1K', '2K', '4K'], defaultSize: '2K' },
] as const

function getModelSizes(modelName: string): { sizes: string[]; defaultSize: string } {
  const name = modelName.toLowerCase()
  if (name.includes('seedream')) {
    for (const config of SEEDREAM_SIZE_CONFIG) {
      if (name.includes(config.pattern)) {
        return { sizes: [...config.sizes], defaultSize: config.defaultSize }
      }
    }
  }
  return { sizes: IMAGE_SIZES, defaultSize: DEFAULT_IMAGE_CONFIG.size }
}

function isImageModel(types: EndpointType[]): boolean {
  return types.includes('image-generation')
}

interface ImageResult {
  url: string
  revisedPrompt?: string
}

export function ImageGenTab() {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [model, setModel] = useState(DEFAULT_IMAGE_CONFIG.model)
  const [group, setGroup] = useState(DEFAULT_IMAGE_CONFIG.group)
  const [size, setSize] = useState(DEFAULT_IMAGE_CONFIG.size)
  const [n, setN] = useState(DEFAULT_IMAGE_CONFIG.n)
  const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([])
  const [isGenerating, setIsGenerating] = useState(false)
  const [results, setResults] = useState<ImageResult[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: modelsData } = useQuery({
    queryKey: ['playground-models'],
    queryFn: async () => {
      try {
        return await getPlaygroundModels()
      } catch {
        return []
      }
    },
  })

  const { data: groupsData } = useQuery({
    queryKey: ['playground-groups'],
    queryFn: async () => {
      try {
        return await getUserGroups()
      } catch {
        return []
      }
    },
  })

  const models: ModelOption[] = (modelsData ?? [])
    .filter((m) => isImageModel(m.supported_endpoint_types))
    .map((m) => ({ label: m.name, value: m.name }))

  const groups: GroupOption[] = groupsData ?? []

  // Auto-select first model
  if (models.length > 0 && !model) {
    setModel(models[0].value)
  }
  if (groups.length > 0 && !group) {
    const fallback = groups.find((g) => g.value === 'default')?.value ?? groups[0].value
    setGroup(fallback)
  }

  const { sizes: availableSizes } = getModelSizes(model)

  const handleModelChange = (newModel: string) => {
    setModel(newModel)
    const { sizes, defaultSize } = getModelSizes(newModel)
    if (!sizes.includes(size)) {
      setSize(defaultSize)
    }
  }

  const handleFileUpload = async (files: FileList | null) => {
    if (!files) return
    for (const file of files) {
      if (!file.type.startsWith('image/')) {
        toast.error(t('Only image files are supported'))
        continue
      }
      const fileId = `${Date.now()}-${file.name}`
      // Add placeholder with uploading state
      setUploadedFiles((prev) => [
        ...prev,
        { id: fileId, name: file.name, url: '', type: file.type, uploading: true },
      ])
      try {
        const result = await uploadFile(file)
        setUploadedFiles((prev) =>
          prev.map((f) =>
            f.id === fileId
              ? { ...f, url: result.url, uploading: false }
              : f
          )
        )
      } catch (err: unknown) {
        setUploadedFiles((prev) => prev.filter((f) => f.id !== fileId))
        const msg = err instanceof Error ? err.message : String(err)
        toast.error(`${t('Failed to upload file')}: ${msg}`)
      }
    }
  }

  const removeFile = (id: string) => {
    setUploadedFiles((prev) => prev.filter((f) => f.id !== id))
  }

  const handleGenerate = async () => {
    if (!prompt.trim()) {
      toast.error(t('Please enter a prompt'))
      return
    }
    if (!model) {
      toast.error(t('Please select a model'))
      return
    }

    setIsGenerating(true)
    setResults([])

    try {
      const imageUrls = uploadedFiles.filter((f) => !f.uploading).map((f) => f.url)
      const basePayload = {
        model,
        group,
        prompt: prompt.trim(),
        size: size || undefined,
        image: imageUrls.length > 0 ? imageUrls[0] : undefined,
        images: imageUrls.length > 0 ? imageUrls : undefined,
      }

      // Send n parallel requests (not all providers support the n parameter)
      const requests = Array.from({ length: n }, () =>
        generateImage(basePayload).catch(() => null)
      )
      const responses = await Promise.all(requests)

      const newResults: ImageResult[] = []
      for (const response of responses) {
        if (!response) continue
        for (const item of response.data || []) {
          newResults.push({
            url: item.b64_json
              ? `data:image/png;base64,${item.b64_json}`
              : item.url || '',
            revisedPrompt: item.revised_prompt,
          })
        }
      }

      setResults(newResults)
      if (newResults.length === 0) {
        toast.error(t('No images generated'))
      }
    } catch (error: unknown) {
      const err = error as {
        response?: { data?: { message?: string; error?: { message?: string } } }
        message?: string
      }
      toast.error(
        err?.response?.data?.error?.message ||
          err?.response?.data?.message ||
          err?.message ||
          t('Failed to generate image')
      )
    } finally {
      setIsGenerating(false)
    }
  }

  return (
    <div className='mx-auto w-full max-w-4xl space-y-4 p-4'>
      {/* Model & Group Selector */}
      <ModelGroupSelector
        selectedModel={model}
        models={models}
        onModelChange={handleModelChange}
        selectedGroup={group}
        groups={groups}
        onGroupChange={setGroup}
        disabled={isGenerating}
      />

      {/* Prompt */}
      <Textarea
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        placeholder={t('Describe the image you want to generate...')}
        className='min-h-[100px] resize-y'
        disabled={isGenerating}
      />

      {/* Image Upload */}
      <div className='space-y-2'>
        <input
          ref={fileInputRef}
          type='file'
          accept='image/*'
          multiple
          className='hidden'
          onChange={(e) => {
            handleFileUpload(e.target.files)
            e.target.value = ''
          }}
        />
        <div className='flex flex-wrap gap-2'>
          {uploadedFiles.map((file) => (
            <div
              key={file.id}
              className='group relative size-20 overflow-hidden rounded-lg border'
            >
              {file.uploading ? (
                <div className='flex size-full items-center justify-center bg-muted'>
                  <Loader2Icon size={16} className='animate-spin text-muted-foreground' />
                </div>
              ) : (
                <img
                  src={file.url}
                  alt={file.name}
                  className='size-full object-cover'
                />
              )}
              <button
                type='button'
                onClick={() => removeFile(file.id)}
                className='absolute right-0 top-0 rounded-bl-lg bg-black/60 p-1 text-white opacity-0 transition-opacity group-hover:opacity-100'
              >
                <XIcon size={12} />
              </button>
            </div>
          ))}
          <button
            type='button'
            onClick={() => fileInputRef.current?.click()}
            disabled={isGenerating}
            className='flex size-20 flex-col items-center justify-center gap-1 rounded-lg border border-dashed text-muted-foreground transition-colors hover:bg-muted'
          >
            <UploadIcon size={20} />
            <span className='text-xs'>{t('Upload')}</span>
          </button>
        </div>
        {uploadedFiles.length > 0 && (
          <p className='text-xs text-muted-foreground'>
            {t('Uploaded images will be used as reference for editing')}
          </p>
        )}
      </div>

      {/* Parameters */}
      <div className='flex flex-wrap items-center gap-3'>
        <div className='flex items-center gap-2'>
          <label className='text-sm text-muted-foreground'>{t('Size')}</label>
          <select
            value={size}
            onChange={(e) => setSize(e.target.value)}
            disabled={isGenerating}
            className='h-9 rounded-md border bg-background px-3 text-sm'
          >
            {availableSizes.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div className='flex items-center gap-2'>
          <label className='text-sm text-muted-foreground'>{t('Count')}</label>
          <select
            value={n}
            onChange={(e) => setN(Number(e.target.value))}
            disabled={isGenerating}
            className='h-9 rounded-md border bg-background px-3 text-sm'
          >
            {[1, 2, 3, 4].map((num) => (
              <option key={num} value={num}>
                {num}
              </option>
            ))}
          </select>
        </div>
        <Button
          onClick={handleGenerate}
          disabled={isGenerating || !prompt.trim() || !model}
          className='ml-auto'
        >
          {isGenerating ? (
            <>
              <Loader2Icon size={16} className='animate-spin' />
              {t('Generating...')}
            </>
          ) : (
            <>
              <SparklesIcon size={16} />
              {t('Generate')}
            </>
          )}
        </Button>
      </div>

      {/* Results */}
      {results.length > 0 && (
        <div className='space-y-2'>
          <h3 className='text-sm font-medium text-muted-foreground'>
            {t('Results')}
          </h3>
          <div className='grid grid-cols-2 gap-4 md:grid-cols-3'>
            {results.map((result, idx) => (
              <div
                key={result.url}
                className='group relative overflow-hidden rounded-lg border'
              >
                <img
                  src={result.url}
                  alt={result.revisedPrompt || `Result ${idx + 1}`}
                  className='aspect-square w-full object-cover'
                />
                {result.revisedPrompt && (
                  <div className='absolute inset-x-0 bottom-0 bg-black/60 p-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100'>
                    {result.revisedPrompt}
                  </div>
                )}
                <a
                  href={result.url}
                  download={`image-${idx + 1}.png`}
                  target='_blank'
                  rel='noopener noreferrer'
                  className='absolute right-2 top-2 rounded-lg bg-black/60 p-1.5 text-white opacity-0 transition-opacity group-hover:opacity-100'
                >
                  <DownloadIcon size={14} />
                </a>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Empty State */}
      {!isGenerating && results.length === 0 && (
        <div className='flex flex-col items-center justify-center py-16 text-muted-foreground'>
          <ImageIcon size={48} className='mb-4 opacity-30' />
          <p className='text-sm'>{t('Generated images will appear here')}</p>
        </div>
      )}
    </div>
  )
}
