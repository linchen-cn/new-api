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
import { STORAGE_KEYS } from '../constants'
import type {
  PlaygroundConfig,
  ParameterEnabled,
  Message,
  Conversation,
} from '../types'
import { sanitizeMessagesOnLoad } from './message-utils'

/**
 * Load playground config from localStorage
 */
export function loadConfig(): Partial<PlaygroundConfig> {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.CONFIG)
    if (saved) {
      return JSON.parse(saved)
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load config:', error)
  }
  return {}
}

/**
 * Save playground config to localStorage
 */
export function saveConfig(config: Partial<PlaygroundConfig>): void {
  try {
    localStorage.setItem(STORAGE_KEYS.CONFIG, JSON.stringify(config))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save config:', error)
  }
}

/**
 * Load parameter enabled state from localStorage
 */
export function loadParameterEnabled(): Partial<ParameterEnabled> {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.PARAMETER_ENABLED)
    if (saved) {
      return JSON.parse(saved)
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load parameter enabled:', error)
  }
  return {}
}

/**
 * Save parameter enabled state to localStorage
 */
export function saveParameterEnabled(
  parameterEnabled: Partial<ParameterEnabled>
): void {
  try {
    localStorage.setItem(
      STORAGE_KEYS.PARAMETER_ENABLED,
      JSON.stringify(parameterEnabled)
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save parameter enabled:', error)
  }
}

/**
 * Load messages from localStorage
 */
export function loadMessages(): Message[] | null {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.MESSAGES)
    if (saved) {
      const parsed: unknown = JSON.parse(saved)
      if (!Array.isArray(parsed)) {
        return null
      }
      const sanitized = sanitizeMessagesOnLoad(parsed as Message[])
      // Persist sanitized result to avoid re-sanitizing on subsequent loads
      if (sanitized !== parsed) {
        saveMessages(sanitized)
      }
      return sanitized
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load messages:', error)
  }
  return null
}

/**
 * Save messages to localStorage
 */
export function saveMessages(messages: Message[]): void {
  try {
    localStorage.setItem(STORAGE_KEYS.MESSAGES, JSON.stringify(messages))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save messages:', error)
  }
}

/**
 * Clear all playground data
 */
export function clearPlaygroundData(): void {
  try {
    localStorage.removeItem(STORAGE_KEYS.CONFIG)
    localStorage.removeItem(STORAGE_KEYS.PARAMETER_ENABLED)
    localStorage.removeItem(STORAGE_KEYS.MESSAGES)
    localStorage.removeItem(STORAGE_KEYS.CONVERSATIONS)
    localStorage.removeItem(STORAGE_KEYS.ACTIVE_CONVERSATION)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to clear playground data:', error)
  }
}

/**
 * Load conversations from localStorage, migrating from old single-messages format
 */
export function loadConversations(): Conversation[] {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.CONVERSATIONS)
    if (saved) {
      const parsed: unknown = JSON.parse(saved)
      if (Array.isArray(parsed)) {
        return (parsed as Conversation[]).map((c) => ({
          ...c,
          messages: sanitizeMessagesOnLoad(c.messages || []),
        }))
      }
    }
    // Migrate from old single messages array
    const oldMessages = loadMessages()
    if (oldMessages && oldMessages.length > 0) {
      const firstUserMsg = oldMessages.find((m) => m.from === 'user')
      const title =
        firstUserMsg?.versions?.[0]?.content?.slice(0, 40).trim() ||
        'New Chat'
      const conversation: Conversation = {
        id: `conv-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        title,
        messages: oldMessages,
        createdAt: Date.now(),
        updatedAt: Date.now(),
      }
      const conversations = [conversation]
      saveConversations(conversations)
      saveActiveConversationId(conversation.id)
      return conversations
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load conversations:', error)
  }
  return []
}

/**
 * Save conversations to localStorage
 */
export function saveConversations(conversations: Conversation[]): void {
  try {
    localStorage.setItem(
      STORAGE_KEYS.CONVERSATIONS,
      JSON.stringify(conversations)
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save conversations:', error)
  }
}

/**
 * Load active conversation ID from localStorage
 */
export function loadActiveConversationId(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEYS.ACTIVE_CONVERSATION)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load active conversation ID:', error)
  }
  return null
}

/**
 * Save active conversation ID to localStorage
 */
export function saveActiveConversationId(id: string | null): void {
  try {
    if (id) {
      localStorage.setItem(STORAGE_KEYS.ACTIVE_CONVERSATION, id)
    } else {
      localStorage.removeItem(STORAGE_KEYS.ACTIVE_CONVERSATION)
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save active conversation ID:', error)
  }
}
