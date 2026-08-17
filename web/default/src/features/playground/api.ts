/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import { API_ENDPOINTS } from './constants'
import type {
  ChatCompletionRequest,
  ChatCompletionResponse,
  ModelOption,
  GroupOption,
  PlaygroundModel,
  ImageGenerationRequest,
  ImageGenerationResponse,
  AudioGenerationRequest,
  AudioTaskRequest,
  VideoGenerationRequest,
  VideoTaskSubmitResponse,
  VideoTaskFetchResponse,
} from './types'

/**
 * Send chat completion request (non-streaming)
 */
export async function sendChatCompletion(
  payload: ChatCompletionRequest
): Promise<ChatCompletionResponse> {
  const res = await api.post(API_ENDPOINTS.CHAT_COMPLETIONS, payload, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export interface UploadResult {
  url: string
  fileType: string
  fileName: string
}

/**
 * Upload a file to OSS and return the public URL.
 */
export async function uploadFile(file: File): Promise<UploadResult> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post(API_ENDPOINTS.UPLOAD, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    skipErrorHandler: true,
  } as Record<string, unknown>)
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Upload failed')
  }
  return res.data.data as UploadResult
}

/**
 * Get user available models
 */
export async function getUserModels(): Promise<ModelOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_MODELS)
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data.map((model: string) => ({
    label: model,
    value: model,
  }))
}

/**
 * Get user groups
 */
export async function getUserGroups(): Promise<GroupOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_GROUPS)
  const { data } = res

  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>

  // label is for button display (name only); desc is for dropdown content
  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: info.ratio,
    desc: info.desc,
  }))
}

/**
 * Get playground models with endpoint types
 */
export async function getPlaygroundModels(): Promise<PlaygroundModel[]> {
  const res = await api.get(API_ENDPOINTS.PLAYGROUND_MODELS)
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data as PlaygroundModel[]
}

/**
 * Generate image
 */
export async function generateImage(
  payload: ImageGenerationRequest
): Promise<ImageGenerationResponse> {
  const hasImageInput =
    payload.image || (payload.images && payload.images.length > 0)
  const endpoint = hasImageInput
    ? API_ENDPOINTS.IMAGE_EDITS
    : API_ENDPOINTS.IMAGE_GENERATIONS
  const res = await api.post(endpoint, payload, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Generate audio via TTS (synchronous)
 * Returns a Blob containing the audio data.
 */
export async function generateAudio(
  payload: AudioGenerationRequest
): Promise<Blob> {
  const res = await api.post(API_ENDPOINTS.AUDIO_SPEECH, payload, {
    responseType: 'blob',
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data as Blob
}

/**
 * Submit audio/music generation task (async, VolcMusic format).
 * Sends `metadata` with type/duration/version as expected by the VolcMusic adaptor.
 * Uses the same task endpoint and polling as video tasks.
 */
export async function submitAudioTask(
  payload: AudioTaskRequest
): Promise<VideoTaskSubmitResponse> {
  const res = await api.post(API_ENDPOINTS.VIDEO_GENERATIONS, payload, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Submit video generation task (async)
 * Constructs the `content` array format expected by upstream video APIs
 * (e.g. VolcEngine Seedance) while also sending `prompt`/`images` for
 * TaskSubmitReq validation and billing.
 */
export async function submitVideoTask(
  payload: VideoGenerationRequest
): Promise<VideoTaskSubmitResponse> {
  const body: Record<string, unknown> = {
    model: payload.model,
    group: payload.group,
    prompt: payload.prompt,
  }

  // Always build the Seedance-style content array (upstream requires it);
  // text-only generation sends just the text item, images become
  // reference_image entries.
  const content: Array<Record<string, unknown>> = [
    { type: 'text', text: payload.prompt },
  ]
  if (payload.images && payload.images.length > 0) {
    for (const url of payload.images) {
      content.push({
        type: 'image_url',
        image_url: { url },
        role: 'reference_image',
      })
    }
  } else if (payload.image) {
    content.push({
      type: 'image_url',
      image_url: { url: payload.image },
      role: 'reference_image',
    })
  }
  body.content = content

  // Only include duration when explicitly set
  if (payload.duration != null && payload.duration > 0) {
    body.duration = payload.duration
  }
  if (payload.size) {
    body.resolution = payload.size
  }

  const res = await api.post(API_ENDPOINTS.VIDEO_GENERATIONS, body, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Fetch video task status
 */
export async function fetchVideoTask(
  taskId: string
): Promise<VideoTaskFetchResponse> {
  const res = await api.get(API_ENDPOINTS.VIDEO_FETCH(taskId))
  return res.data
}
