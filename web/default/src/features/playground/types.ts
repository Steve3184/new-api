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

export type PlaygroundMessageLayoutMode = 'alternating' | 'left'

export interface MessageVersion {
  id: string
  content: string
}

export interface PlaygroundAttachment {
  dataUrl: string
  filename: string
  mediaType: string
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  attachments?: PlaygroundAttachment[]
  createdAt?: number
  startedAt?: number
  completedAt?: number
  durationMs?: number
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
    startedAt?: number
    completedAt?: number
    durationMs?: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
}

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ContentPart {
  type: 'text' | 'image_url' | 'file'
  text?: string
  image_url?: {
    url: string
  }
  file?: {
    filename: string
    file_data: string
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

export type PlaygroundFeature = 'chat' | 'image' | 'speech' | 'three_d'
export type SpeechModelType = 'openai' | 'azure' | 'unrealspeech'

export interface PlaygroundPublicSettings {
  enabled_features: PlaygroundFeature[]
  models: Record<PlaygroundFeature, string[]>
  speech_model_types: Record<string, SpeechModelType>
}

export interface ImageGenerationRequest {
  model: string
  group: string
  prompt: string
  n: number
  size: string
  quality?: string
  response_format: 'b64_json'
}

export interface ImageGenerationResponse {
  created: number
  data: Array<{
    url?: string
    b64_json?: string
    revised_prompt?: string
  }>
}

export interface SpeechGenerationRequest {
  model: string
  group: string
  input: string
  voice: string
  response_format: string
  speed: number
  volume?: number
  pitch?: number
  stream?: boolean
  speech?: boolean
  bitrate?: string
}

export interface SpeechGenerationTaskResponse {
  id: string
  object: 'audio.speech'
  model: string
  status: 'queued' | 'in_progress' | 'completed' | 'failed'
  progress: number
  created_at: number
  content_url?: string
  error?: { message: string; code: string }
}

export interface ThreeDGenerationRequest {
  model: string
  group: string
  prompt?: string
  input_reference?: string
  source_task_id?: string
  metadata?: { art_style?: string }
}

export interface ThreeDGenerationResponse {
  id: string
  object: '3d'
  model: string
  status: 'queued' | 'in_progress' | 'completed' | 'failed'
  progress: number
  created_at: number
  completed_at?: number
  data?: { format: string; url: string }
  error?: { message: string; code: string }
}
