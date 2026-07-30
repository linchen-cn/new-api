import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Loader2Icon,
  SparklesIcon,
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon,
  DownloadIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ModelGroupSelector } from '@/components/model-group-selector'
import {
  getPlaygroundModels,
  getUserGroups,
  generateAudio,
  submitAudioTask,
  fetchVideoTask,
} from '../api'
import { DEFAULT_AUDIO_CONFIG, VIDEO_POLL_INTERVAL } from '../constants'
import type {
  EndpointType,
  ModelOption,
  GroupOption,
} from '../types'

// Voice map: friendly name -> actual voice ID sent to the API
const VOICE_MAP: Record<string, string> = {
  'Vivi': 'zh_female_vv_uranus_bigtts',
  '小何': 'zh_female_xiaohe_uranus_bigtts',
  '云舟': 'zh_male_m191_uranus_bigtts',
  '小天': 'zh_male_taocheng_uranus_bigtts',
  '刘飞': 'zh_male_liufei_uranus_bigtts',
  'Tina老师': 'zh_female_yingyujiaoxue_uranus_bigtts',
}
const VOICE_SUGGESTIONS = Object.keys(VOICE_MAP)
const AUDIO_FORMATS = ['mp3', 'opus', 'aac', 'flac', 'wav', 'pcm']

const PENDING_STATUSES = ['queued', 'in_progress', 'pending', 'running']

function isAudioModel(types: EndpointType[]): boolean {
  return types.includes('audio-speech')
}

function isAsyncModel(types: EndpointType[]): boolean {
  return types.includes('openai-video')
}

interface AudioResult {
  url: string
  label: string
}

interface AsyncTaskState {
  taskId: string
  status: string
  progress: number
  resultUrl?: string
  error?: string
}

export function AudioGenTab() {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [model, setModel] = useState(DEFAULT_AUDIO_CONFIG.model)
  const [group, setGroup] = useState(DEFAULT_AUDIO_CONFIG.group)
  const [voice, setVoice] = useState(DEFAULT_AUDIO_CONFIG.voice)
  const [speed, setSpeed] = useState(DEFAULT_AUDIO_CONFIG.speed)
  const [responseFormat, setResponseFormat] = useState(
    DEFAULT_AUDIO_CONFIG.responseFormat
  )
  const [musicDuration, setMusicDuration] = useState(30)
  const [isGenerating, setIsGenerating] = useState(false)
  const [results, setResults] = useState<AudioResult[]>([])
  const [tasks, setTasks] = useState<AsyncTaskState[]>([])
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)

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
    .filter((m) => isAudioModel(m.supported_endpoint_types))
    .map((m) => ({ label: m.name, value: m.name }))

  const groups: GroupOption[] = groupsData ?? []

  // Auto-select first model
  if (models.length > 0 && !model) {
    setModel(models[0].value)
  }
  if (groups.length > 0 && !group) {
    const fallback =
      groups.find((g) => g.value === 'default')?.value ?? groups[0].value
    setGroup(fallback)
  }

  // Determine if the selected model is async (task-based) or sync (TTS)
  const selectedModel = (modelsData ?? []).find((m) => m.name === model)
  const isAsync = selectedModel
    ? isAsyncModel(selectedModel.supported_endpoint_types)
    : false
  // seed-audio models don't use voice parameter
  const needsVoice = !isAsync && !model.toLowerCase().includes('seed-audio')

  // Polling for async tasks
  const pollTask = async (taskId: string) => {
    try {
      const res = await fetchVideoTask(taskId)
      const taskData = res.data
      const status = taskData.status?.toLowerCase() || 'unknown'
      const progress = taskData.progress ?? 0

      if (status === 'succeeded' || status === 'success') {
        const url = taskData.result_url || ''
        setTasks((prev) =>
          prev.map((t) =>
            t.taskId === taskId
              ? { ...t, status: 'success', progress: 100, resultUrl: url }
              : t
          )
        )
      } else if (status === 'failed' || status === 'failure') {
        const errMsg = taskData.fail_reason || 'Task failed'
        setTasks((prev) =>
          prev.map((t) =>
            t.taskId === taskId
              ? { ...t, status: 'failed', error: errMsg }
              : t
          )
        )
      } else {
        setTasks((prev) =>
          prev.map((t) =>
            t.taskId === taskId
              ? { ...t, status, progress }
              : t
          )
        )
      }
    } catch {
      // ignore polling errors
    }
  }

  useEffect(() => {
    const hasPending = tasks.some((t) =>
      PENDING_STATUSES.includes(t.status)
    )
    if (hasPending && !pollingRef.current) {
      pollingRef.current = setInterval(() => {
        tasks.forEach((t) => {
          if (PENDING_STATUSES.includes(t.status)) {
            pollTask(t.taskId)
          }
        })
      }, VIDEO_POLL_INTERVAL)
    } else if (!hasPending && pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }

    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
        pollingRef.current = null
      }
    }
  }, [tasks, pollTask])

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

    try {
      if (isAsync) {
        // Async music generation (volc_song, volc_bgm, etc.)
        // Determine music type from model name
        const modelLower = model.toLowerCase()
        const musicType = modelLower.includes('bgm') ? 'bgm' : 'song'

        const response = await submitAudioTask({
          model,
          group,
          prompt: prompt.trim(),
          metadata: {
            type: musicType,
            duration: musicDuration,
          },
        })

        const taskId = response.id || response.task_id || ''
        if (!taskId) {
          toast.error(t('Failed to submit task'))
          return
        }

        setTasks((prev) => [
          {
            taskId,
            status: 'queued',
            progress: 0,
          },
          ...prev,
        ])
        toast.success(t('Task submitted'))
      } else {
        // Sync TTS
        const blob = await generateAudio({
          model,
          group,
          input: prompt.trim(),
          voice: needsVoice
            ? (VOICE_MAP[voice] || voice || VOICE_MAP[VOICE_SUGGESTIONS[0]])
            : undefined,
          response_format: responseFormat,
          speed,
        })

        const url = URL.createObjectURL(blob)
        setResults((prev) => [
          { url, label: `${model} - ${new Date().toLocaleTimeString()}` },
          ...prev,
        ])
        toast.success(t('Audio generated'))
      }
    } catch (error: unknown) {
      const err = error as {
        response?: { data?: { message?: string; error?: { message?: string } } }
        message?: string
      }
      const msg =
        err?.response?.data?.error?.message ||
        err?.response?.data?.message ||
        err?.message ||
        String(error)
      toast.error(msg)
    } finally {
      setIsGenerating(false)
    }
  }

  return (
    <div className='flex h-full flex-col gap-4 p-4'>
      {/* Model & Group Selectors */}
      <div className='flex flex-wrap items-center gap-2'>
        <ModelGroupSelector
          models={models}
          groups={groups}
          selectedModel={model}
          selectedGroup={group}
          onModelChange={setModel}
          onGroupChange={setGroup}
        />
      </div>

      {/* TTS-specific controls (only for sync TTS models, not seed-audio) */}
      {!isAsync && (
        <div className='flex flex-wrap items-center gap-4'>
          {needsVoice && (
            <div className='flex items-center gap-2'>
              <label className='text-sm text-muted-foreground'>{t('Voice')}</label>
              <input
                type='text'
                list='voice-suggestions'
                value={voice}
                onChange={(e) => setVoice(e.target.value)}
                disabled={isGenerating}
                placeholder={t('Voice ID or leave empty for default')}
                className='h-9 w-48 rounded-md border bg-background px-3 text-sm'
              />
              <datalist id='voice-suggestions'>
                {VOICE_SUGGESTIONS.map((v) => (
                  <option key={v} value={v} />
                ))}
              </datalist>
            </div>
          )}
          <div className='flex items-center gap-2'>
            <label className='text-sm text-muted-foreground'>{t('Speed')}</label>
            <input
              type='number'
              min={0.25}
              max={4}
              step={0.25}
              value={speed}
              onChange={(e) => setSpeed(Number(e.target.value))}
              disabled={isGenerating}
              className='h-9 w-20 rounded-md border bg-background px-3 text-sm'
            />
          </div>
          <div className='flex items-center gap-2'>
            <label className='text-sm text-muted-foreground'>{t('Format')}</label>
            <select
              value={responseFormat}
              onChange={(e) => setResponseFormat(e.target.value)}
              disabled={isGenerating}
              className='h-9 rounded-md border bg-background px-3 text-sm'
            >
              {AUDIO_FORMATS.map((f) => (
                <option key={f} value={f}>
                  {f}
                </option>
              ))}
            </select>
          </div>
        </div>
      )}

      {/* Async music controls (only for task-based models) */}
      {isAsync && (
        <div className='flex flex-wrap items-center gap-4'>
          <div className='flex items-center gap-2'>
            <label className='text-sm text-muted-foreground'>{t('Duration (s)')}</label>
            <input
              type='number'
              min={10}
              max={300}
              value={musicDuration}
              onChange={(e) => setMusicDuration(Number(e.target.value))}
              disabled={isGenerating}
              className='h-9 w-20 rounded-md border bg-background px-3 text-sm'
            />
          </div>
        </div>
      )}

      {/* Prompt Input */}
      <Textarea
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        placeholder={
          isAsync
            ? t('Describe the music you want to generate...')
            : t('Enter the text you want to convert to speech...')
        }
        className='min-h-[100px] resize-y'
        disabled={isGenerating}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault()
            handleGenerate()
          }
        }}
      />

      {/* Generate Button */}
      <div className='flex items-center gap-2'>
        <Button
          onClick={handleGenerate}
          disabled={isGenerating || !prompt.trim() || !model}
        >
          {isGenerating ? (
            <>
              <Loader2Icon size={16} className='animate-spin' />
              {t('Generating...')}
            </>
          ) : (
            <>
              <SparklesIcon size={16} />
              {isAsync ? t('Generate Audio') : t('Generate Speech')}
            </>
          )}
        </Button>
      </div>

      {/* Sync Results */}
      {results.length > 0 && (
        <div className='space-y-3'>
          <h3 className='text-sm font-medium'>{t('Results')}</h3>
          {results.map((result, idx) => (
            <div
              key={idx}
              className='flex items-center gap-3 rounded-lg border p-3'
            >
              <audio
                src={result.url}
                controls
                preload='none'
                className='h-9 flex-1'
              />
              <Button
                variant='outline'
                size='sm'
                className='h-8 shrink-0 gap-1'
                onClick={() => {
                  const a = document.createElement('a')
                  a.href = result.url
                  a.download = `audio-${idx + 1}.${responseFormat}`
                  a.click()
                }}
              >
                <DownloadIcon size={14} />
                {t('Download')}
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* Async Task Results */}
      {tasks.length > 0 && (
        <div className='space-y-3'>
          <h3 className='text-sm font-medium'>{t('Tasks')}</h3>
          {tasks.map((task) => (
            <div
              key={task.taskId}
              className='rounded-lg border p-3'
            >
              <div className='mb-2 flex items-center gap-2'>
                {task.status === 'success' && (
                  <CheckCircleIcon size={16} className='text-green-500' />
                )}
                {task.status === 'failed' && (
                  <XCircleIcon size={16} className='text-red-500' />
                )}
                {PENDING_STATUSES.includes(task.status) && (
                  <ClockIcon size={16} className='text-blue-500' />
                )}
                <span className='text-sm font-medium'>
                  {t('Task')}: {task.taskId.slice(0, 12)}...
                </span>
                <span className='text-muted-foreground text-xs'>
                  {task.status} {task.progress > 0 && `${task.progress}%`}
                </span>
              </div>

              {task.status === 'success' && task.resultUrl && (
                <div className='flex items-center gap-3'>
                  <audio
                    src={task.resultUrl}
                    controls
                    preload='none'
                    className='h-9 flex-1'
                  />
                  <a
                    href={task.resultUrl}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='inline-flex h-8 shrink-0 items-center gap-1 rounded-md border px-3 text-xs'
                  >
                    <DownloadIcon size={14} />
                    {t('Open')}
                  </a>
                </div>
              )}

              {task.status === 'failed' && (
                <p className='text-red-500 text-xs'>{task.error}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
