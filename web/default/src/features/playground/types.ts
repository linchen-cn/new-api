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
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export interface MessageVersion {
  id: string
  content: string
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Conversation type for chat history
export interface Conversation {
  id: string
  title: string
  messages: Message[]
  createdAt: number
  updatedAt: number
}

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
    }
    finish_reason: string | null
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Configuration types
export interface PlaygroundConfig {
  model: string
  group: string
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}

// Playground model with endpoint types (from /pg/models)
export type EndpointType =
  | 'openai'
  | 'openai-response'
  | 'openai-response-compact'
  | 'anthropic'
  | 'gemini'
  | 'jina-rerank'
  | 'image-generation'
  | 'embeddings'
  | 'openai-video'
  | 'audio-speech'

export interface PlaygroundModel {
  name: string
  supported_endpoint_types: EndpointType[]
}

// Creation Center tab type
export type CreationTab = 'text' | 'image' | 'video' | 'audio'

// Uploaded file type
export interface UploadedFile {
  id: string
  name: string
  url: string
  type: string
  uploading?: boolean
}

// Image generation types
export interface ImageGenerationRequest {
  model: string
  group?: string
  prompt: string
  n?: number
  size?: string
  quality?: string
  style?: string
  image?: string
  images?: string[]
}

export interface ImageGenerationResponse {
  created: number
  data: Array<{
    url?: string
    b64_json?: string
    revised_prompt?: string
  }>
}

// Video generation types
export interface VideoGenerationRequest {
  model: string
  group?: string
  prompt: string
  image?: string
  images?: string[]
  duration?: number
  size?: string
}

// Audio generation types (sync TTS)
export interface AudioGenerationRequest {
  model: string
  group?: string
  input: string
  voice?: string
  response_format?: string
  speed?: number
  instructions?: string
}

// Audio task types (async music generation via VolcMusic)
export interface AudioTaskRequest {
  model: string
  group?: string
  prompt: string
  metadata?: {
    type?: 'song' | 'bgm'
    duration?: number
    version?: string
    lyrics?: string
    genre?: string
    mood?: string
    gender?: string
    timbre?: string
    [key: string]: unknown
  }
}

export interface VideoTaskSubmitResponse {
  id: string
  task_id?: string
  object: string
  model: string
  status: string
  progress: number
  created_at: number
}

export interface VideoTaskStatusResponse {
  id: string
  task_id?: string
  object: string
  model: string
  status: string
  progress: number
  created_at: number
  completed_at?: number
  error?: {
    message: string
    code: string
  }
  metadata?: Record<string, unknown>
}

// Video task fetch response (TaskDto format from /pg/video/generations/:task_id)
export interface VideoTaskFetchResponse {
  code: string
  data: {
    id: number
    task_id: string
    platform: string
    status: string
    result_url?: string
    progress: number
    fail_reason?: string
    created_at: number
    updated_at: number
  }
}

// Video task state for UI tracking
export interface VideoTaskState {
  taskId: string
  model: string
  prompt: string
  status: string
  progress: number
  videoUrl?: string
  error?: string
  createdAt: number
}
