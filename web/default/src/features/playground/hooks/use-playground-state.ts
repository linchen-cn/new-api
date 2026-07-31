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
import { useState, useCallback, useEffect, useRef, useMemo } from 'react'
import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../constants'
import {
  loadConfig,
  saveConfig,
  loadParameterEnabled,
  saveParameterEnabled,
  loadConversations,
  saveConversations,
  loadActiveConversationId,
  saveActiveConversationId,
} from '../lib'
import type {
  Message,
  PlaygroundConfig,
  ParameterEnabled,
  ModelOption,
  GroupOption,
  Conversation,
} from '../types'

function createEmptyConversation(): Conversation {
  return {
    id: `conv-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    title: '',
    messages: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
  }
}

/**
 * Main state management hook for playground
 */
export function usePlaygroundState() {
  // Load initial state from localStorage
  const [config, setConfig] = useState<PlaygroundConfig>(() => {
    const savedConfig = loadConfig()
    return { ...DEFAULT_CONFIG, ...savedConfig }
  })

  const [parameterEnabled, setParameterEnabled] = useState<ParameterEnabled>(
    () => {
      const saved = loadParameterEnabled()
      return { ...DEFAULT_PARAMETER_ENABLED, ...saved }
    }
  )

  const [conversations, setConversations] = useState<Conversation[]>(() => {
    return loadConversations()
  })

  const [activeConversationId, setActiveConversationId] = useState<
    string | null
  >(() => loadActiveConversationId())

  // Ref to keep activeConversationId accessible in stable callbacks
  const activeConversationIdRef = useRef(activeConversationId)
  useEffect(() => {
    activeConversationIdRef.current = activeConversationId
  }, [activeConversationId])

  // Ensure there is always an active conversation
  useEffect(() => {
    if (conversations.length === 0) {
      const newConv = createEmptyConversation()
      setConversations([newConv])
      setActiveConversationId(newConv.id)
      saveConversations([newConv])
      saveActiveConversationId(newConv.id)
    } else if (
      !activeConversationId ||
      !conversations.some((c) => c.id === activeConversationId)
    ) {
      const firstId = conversations[0].id
      setActiveConversationId(firstId)
      saveActiveConversationId(firstId)
    }
  }, [conversations, activeConversationId])

  // Derive messages from the active conversation
  const messages = useMemo(() => {
    return (
      conversations.find((c) => c.id === activeConversationId)?.messages ?? []
    )
  }, [conversations, activeConversationId])

  const [models, setModels] = useState<ModelOption[]>([])
  const [groups, setGroups] = useState<GroupOption[]>([])

  // Update config with automatic save
  const updateConfig = useCallback(
    <K extends keyof PlaygroundConfig>(key: K, value: PlaygroundConfig[K]) => {
      setConfig((prev) => {
        const updated = { ...prev, [key]: value }
        saveConfig(updated)
        return updated
      })
    },
    []
  )

  // Update parameter enabled with automatic save
  const updateParameterEnabled = useCallback(
    (key: keyof ParameterEnabled, value: boolean) => {
      setParameterEnabled((prev) => {
        const updated = { ...prev, [key]: value }
        saveParameterEnabled(updated)
        return updated
      })
    },
    []
  )

  // Update messages in the active conversation (stable callback via ref)
  const updateMessages = useCallback(
    (updater: Message[] | ((prev: Message[]) => Message[])) => {
      setConversations((prev) => {
        const activeId = activeConversationIdRef.current
        const newConversations = prev.map((c) => {
          if (c.id !== activeId) return c
          const newMessages =
            typeof updater === 'function' ? updater(c.messages) : updater
          // Auto-title from first user message
          let title = c.title
          if (!title && newMessages.length > 0) {
            const firstUserMsg = newMessages.find((m) => m.from === 'user')
            if (firstUserMsg?.versions?.[0]?.content) {
              title = firstUserMsg.versions[0].content.slice(0, 40).trim()
            }
          }
          return {
            ...c,
            messages: newMessages,
            title,
            updatedAt: Date.now(),
          }
        })
        saveConversations(newConversations)
        return newConversations
      })
    },
    []
  )

  // Create a new empty conversation and switch to it
  const createNewConversation = useCallback(() => {
    const newConv = createEmptyConversation()
    setConversations((prev) => {
      const updated = [newConv, ...prev]
      saveConversations(updated)
      return updated
    })
    setActiveConversationId(newConv.id)
    saveActiveConversationId(newConv.id)
  }, [])

  // Switch to an existing conversation
  const switchToConversation = useCallback((id: string) => {
    setActiveConversationId(id)
    saveActiveConversationId(id)
  }, [])

  // Delete a conversation (the effect will pick a new active one)
  const deleteConversation = useCallback((id: string) => {
    setConversations((prev) => {
      const filtered = prev.filter((c) => c.id !== id)
      saveConversations(filtered)
      return filtered
    })
    if (activeConversationIdRef.current === id) {
      setActiveConversationId(null)
      saveActiveConversationId(null)
    }
  }, [])

  // Reset config to defaults
  const resetConfig = useCallback(() => {
    setConfig(DEFAULT_CONFIG)
    setParameterEnabled(DEFAULT_PARAMETER_ENABLED)
    saveConfig(DEFAULT_CONFIG)
    saveParameterEnabled(DEFAULT_PARAMETER_ENABLED)
  }, [])

  return {
    // State
    config,
    parameterEnabled,
    messages,
    models,
    groups,
    conversations,
    activeConversationId,

    // Setters
    setModels,
    setGroups,

    // Actions
    updateConfig,
    updateParameterEnabled,
    updateMessages,
    createNewConversation,
    switchToConversation,
    deleteConversation,
    resetConfig,
  }
}
