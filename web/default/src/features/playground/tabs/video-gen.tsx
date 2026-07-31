import { useEffect, useRef, useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  VideoIcon,
  UploadIcon,
  XIcon,
  Loader2Icon,
  SparklesIcon,
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon,
  LinkIcon,
  PlusIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Progress } from '@/components/ui/progress'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { submitVideoTask, fetchVideoTask, getPlaygroundModels, getUserGroups, uploadFile } from '../api'
import { DEFAULT_VIDEO_CONFIG, VIDEO_POLL_INTERVAL } from '../constants'
import type {
  EndpointType,
  ModelOption,
  GroupOption,
  UploadedFile,
  VideoTaskState,
} from '../types'

const TEXT_ENDPOINT_TYPES: EndpointType[] = [
  'openai',
  'anthropic',
  'gemini',
  'openai-response',
  'openai-response-compact',
]

function isVideoModel(types: EndpointType[]): boolean {
  if (!types.includes('openai-video')) return false
  // Exclude chat models that happen to support video input (vision capability)
  const hasText = types.some((t) => TEXT_ENDPOINT_TYPES.includes(t))
  return !hasText && !types.includes('audio-speech')
}

// Models that do not support 1080p resolution
const NO_1080P_MODELS = ['Doubao-Seedance-2.0-fast', 'Doubao-Seedance-2.0-mini']

function supports1080p(modelName: string): boolean {
  const lower = modelName.toLowerCase()
  return !NO_1080P_MODELS.some((m) => lower.includes(m))
}

const PENDING_STATUSES = ['queued', 'in_progress', 'pending', 'running']

export function VideoGenTab() {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [model, setModel] = useState(DEFAULT_VIDEO_CONFIG.model)
  const [group, setGroup] = useState(DEFAULT_VIDEO_CONFIG.group)
  const [duration, setDuration] = useState(DEFAULT_VIDEO_CONFIG.duration)
  const [resolution, setResolution] = useState('480p')
  const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([])
  const [urlInput, setUrlInput] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [tasks, setTasks] = useState<VideoTaskState[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const { data: modelsData, isLoading: isLoadingModels } = useQuery({
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
    .filter((m) => isVideoModel(m.supported_endpoint_types))
    .map((m) => ({ label: m.name, value: m.name }))

  const groups: GroupOption[] = groupsData ?? []

  if (models.length > 0 && !model) {
    setModel(models[0].value)
  }
  if (groups.length > 0 && !group) {
    const fallback = groups.find((g) => g.value === 'default')?.value ?? groups[0].value
    setGroup(fallback)
  }

  // Reset resolution to 720p when switching to a model that doesn't support 1080p
  useEffect(() => {
    if (model && !supports1080p(model) && resolution === '1080p') {
      setResolution('720p')
    }
  }, [model, resolution])

  const updateTask = useCallback((taskId: string, updates: Partial<VideoTaskState>) => {
    setTasks((prev) =>
      prev.map((task) =>
        task.taskId === taskId ? { ...task, ...updates } : task
      )
    )
  }, [])

  const pollTask = useCallback(
    async (taskId: string) => {
      try {
        const resp = await fetchVideoTask(taskId)
        const data = resp.data
        if (!data) return

        const isDone =
          data.status === 'completed' || data.status === 'failed' || data.status === 'succeeded'

        updateTask(taskId, {
          status: data.status,
          progress: data.progress,
          videoUrl: data.result_url,
          error: data.fail_reason || undefined,
        })

        if (isDone && data.status !== 'completed' && data.status !== 'succeeded') {
          toast.error(data.fail_reason || t('Video generation failed'))
        }
      } catch (error: unknown) {
        const err = error as { response?: { status?: number } }
        if (err?.response?.status === 404) {
          updateTask(taskId, {
            status: 'failed',
            error: t('Task not found'),
          })
        }
      }
    },
    [updateTask, t]
  )

  // Polling effect
  useEffect(() => {
    const hasPending = tasks.some((task) => PENDING_STATUSES.includes(task.status))
    if (!hasPending) {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
        pollingRef.current = null
      }
      return
    }

    if (!pollingRef.current) {
      pollingRef.current = setInterval(() => {
        tasks.forEach((task) => {
          if (PENDING_STATUSES.includes(task.status)) {
            pollTask(task.taskId)
          }
        })
      }, VIDEO_POLL_INTERVAL)
    }

    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
        pollingRef.current = null
      }
    }
  }, [tasks, pollTask])

  const handleFileUpload = async (files: FileList | null) => {
    if (!files) return
    for (const file of Array.from(files)) {
      if (!file.type.startsWith('image/')) {
        toast.error(t('Only image files are supported'))
        continue
      }
      const fileId = `${Date.now()}-${file.name}`
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
        toast.error(t('Failed to upload file') + ': ' + msg)
      }
    }
  }

  const removeFile = (id: string) => {
    setUploadedFiles((prev) => prev.filter((f) => f.id !== id))
  }

  const addUrl = () => {
    const url = urlInput.trim()
    if (!url) return
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      toast.error(t('Please enter a valid URL'))
      return
    }
    setUploadedFiles((prev) => [
      ...prev,
      { id: `url-${Date.now()}`, name: url, url, type: 'url' },
    ])
    setUrlInput('')
  }

  const handleSubmit = async () => {
    if (!prompt.trim()) {
      toast.error(t('Please enter a prompt'))
      return
    }
    if (!model) {
      toast.error(t('Please select a model'))
      return
    }

    setIsSubmitting(true)

    try {
      const imageUrls = uploadedFiles.filter((f) => !f.uploading).map((f) => f.url)
      const response = await submitVideoTask({
        model,
        group,
        prompt: prompt.trim(),
        image: imageUrls.length > 0 ? imageUrls[0] : undefined,
        images: imageUrls.length > 0 ? imageUrls : undefined,
        duration,
        size: resolution,
      })

      const taskId = response.id || response.task_id || ''
      if (!taskId) {
        toast.error(t('Failed to submit video task'))
        return
      }

      const newTask: VideoTaskState = {
        taskId,
        model,
        prompt: prompt.trim(),
        status: response.status || 'queued',
        progress: response.progress || 0,
        createdAt: Date.now(),
      }

      setTasks((prev) => [newTask, ...prev])
      toast.success(t('Video task submitted') + ': ' + taskId)

      // Start polling immediately
      setTimeout(() => pollTask(taskId), 1000)
    } catch (error: unknown) {
      const err = error as {
        response?: { data?: { message?: string; error?: { message?: string } } }
        message?: string
      }
      toast.error(
        err?.response?.data?.error?.message ||
          err?.response?.data?.message ||
          err?.message ||
          t('Failed to submit video task')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const renderTaskStatus = (status: string) => {
    switch (status) {
      case 'completed':
      case 'succeeded':
        return (
          <span className='flex items-center gap-1 text-green-600'>
            <CheckCircleIcon size={14} />
            {t('Completed')}
          </span>
        )
      case 'failed':
        return (
          <span className='flex items-center gap-1 text-red-600'>
            <XCircleIcon size={14} />
            {t('Failed')}
          </span>
        )
      default:
        return (
          <span className='flex items-center gap-1 text-blue-600'>
            <ClockIcon size={14} />
            {t('Processing')}
          </span>
        )
    }
  }

  return (
    <div className='mx-auto w-full max-w-4xl space-y-4 p-4'>
      {/* Model & Group Selector */}
      <ModelGroupSelector
        selectedModel={model}
        models={models}
        onModelChange={setModel}
        selectedGroup={group}
        groups={groups}
        onGroupChange={setGroup}
        disabled={isSubmitting}
      />

      {/* Prompt */}
      <Textarea
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        placeholder={t('Describe the video you want to generate...')}
        className='min-h-[100px] resize-y'
        disabled={isSubmitting}
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
                onClick={() => removeFile(file.id)}
                className='absolute right-0 top-0 rounded-bl-lg bg-black/60 p-1 text-white opacity-0 transition-opacity group-hover:opacity-100'
              >
                <XIcon size={12} />
              </button>
            </div>
          ))}
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={isSubmitting}
            className='flex size-20 flex-col items-center justify-center gap-1 rounded-lg border border-dashed text-muted-foreground transition-colors hover:bg-muted'
          >
            <UploadIcon size={20} />
            <span className='text-xs'>{t('Upload')}</span>
          </button>
        </div>
        {uploadedFiles.length > 0 && (
          <p className='text-xs text-muted-foreground'>
            {t('Reference images will be used as input for video generation')}
          </p>
        )}

        {/* URL Input */}
        <div className='flex items-center gap-2'>
          <LinkIcon size={16} className='shrink-0 text-muted-foreground' />
          <input
            type='url'
            value={urlInput}
            onChange={(e) => setUrlInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                addUrl()
              }
            }}
            placeholder={t('Paste image URL...')}
            disabled={isSubmitting}
            className='h-9 flex-1 rounded-md border bg-background px-3 text-sm'
          />
          <Button
            variant='outline'
            size='sm'
            onClick={addUrl}
            disabled={isSubmitting || !urlInput.trim()}
          >
            <PlusIcon size={14} />
            {t('Add')}
          </Button>
        </div>
      </div>

      {/* Parameters & Submit */}
      <div className='flex flex-wrap items-center gap-3'>
        <div className='flex items-center gap-2'>
          <label className='text-sm text-muted-foreground'>{t('Duration (s)')}</label>
          <input
            type='number'
            min={1}
            max={60}
            value={duration}
            onChange={(e) => setDuration(Number(e.target.value))}
            disabled={isSubmitting}
            className='h-9 w-20 rounded-md border bg-background px-3 text-sm'
          />
        </div>
        <div className='flex items-center gap-2'>
          <label className='text-sm text-muted-foreground'>{t('Resolution')}</label>
          <select
            value={resolution}
            onChange={(e) => setResolution(e.target.value)}
            disabled={isSubmitting}
            className='h-9 rounded-md border bg-background px-3 text-sm'
          >
            <option value='480p'>480p</option>
            <option value='720p'>720p</option>
            {model && supports1080p(model) && (
              <option value='1080p'>1080p</option>
            )}
          </select>
        </div>
        <Button
          onClick={handleSubmit}
          disabled={isSubmitting || !prompt.trim() || !model}
          className='ml-auto'
        >
          {isSubmitting ? (
            <>
              <Loader2Icon size={16} className='animate-spin' />
              {t('Submitting...')}
            </>
          ) : (
            <>
              <SparklesIcon size={16} />
              {t('Generate Video')}
            </>
          )}
        </Button>
      </div>

      {/* Async notice */}
      <div className='rounded-lg bg-blue-50 p-3 text-sm text-blue-700 dark:bg-blue-950/50 dark:text-blue-300'>
        {t('Video generation is asynchronous. After submitting, you will receive a task ID. The system will automatically poll for the result.')}
      </div>

      {/* Task List */}
      {tasks.length > 0 && (
        <div className='space-y-3'>
          <h3 className='text-sm font-medium text-muted-foreground'>
            {t('Video Tasks')}
          </h3>
          {tasks.map((task) => (
            <div
              key={task.taskId}
              className='rounded-lg border p-4 space-y-3'
            >
              <div className='flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <VideoIcon size={16} className='text-muted-foreground' />
                  <span className='text-sm font-medium'>{task.model}</span>
                </div>
                {renderTaskStatus(task.status)}
              </div>

              <p className='text-sm text-muted-foreground line-clamp-2'>
                {task.prompt}
              </p>

              <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                <span className='font-mono'>{t('Task ID')}: {task.taskId}</span>
              </div>

              {/* Progress */}
              {PENDING_STATUSES.includes(task.status) && (
                <div className='space-y-1'>
                  <Progress value={task.progress} className='h-2' />
                  <p className='text-xs text-muted-foreground'>
                    {task.progress}%
                  </p>
                </div>
              )}

              {/* Error */}
              {task.status === 'failed' && task.error && (
                <p className='text-sm text-red-600'>{task.error}</p>
              )}

              {/* Video Result */}
              {(task.status === 'completed' || task.status === 'succeeded') && task.videoUrl && (
                <div className='mt-2'>
                  <video
                    src={task.videoUrl}
                    controls
                    className='w-full rounded-lg border'
                    preload='metadata'
                  />
                  <a
                    href={task.videoUrl}
                    download
                    target='_blank'
                    rel='noopener noreferrer'
                    className='mt-2 inline-block text-sm text-blue-600 hover:underline'
                  >
                    {t('Download video')}
                  </a>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Empty State */}
      {tasks.length === 0 && !isSubmitting && (
        <div className='flex flex-col items-center justify-center py-16 text-muted-foreground'>
          <VideoIcon size={48} className='mb-4 opacity-30' />
          <p className='text-sm'>{t('Video tasks will appear here')}</p>
        </div>
      )}
    </div>
  )
}
