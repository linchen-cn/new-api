import { useCallback, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  SquarePenIcon,
  Trash2Icon,
  PanelLeftIcon,
  MessageSquareIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import { getPlaygroundModels, getUserGroups } from '../api'
import { PlaygroundChat } from '../components/playground-chat'
import { PlaygroundInput } from '../components/playground-input'
import { usePlaygroundState, useChatHandler } from '../hooks'
import { createUserMessage, createLoadingAssistantMessage } from '../lib'
import type { Message as MessageType, EndpointType } from '../types'

const TEXT_ENDPOINT_TYPES: EndpointType[] = [
  'openai',
  'anthropic',
  'gemini',
  'openai-response',
  'openai-response-compact',
]

function isTextModel(types: EndpointType[]): boolean {
  if (types.length === 0) return true
  const hasText = types.some((t) => TEXT_ENDPOINT_TYPES.includes(t))
  const hasImage = types.includes('image-generation')
  const hasVideo = types.includes('openai-video')
  const hasAudio = types.includes('audio-speech')
  return hasText && !hasImage && !hasVideo && !hasAudio
}

export function TextChatTab() {
  const { t } = useTranslation()
  const {
    config,
    parameterEnabled,
    messages,
    models,
    groups,
    conversations,
    activeConversationId,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    createNewConversation,
    switchToConversation,
    deleteConversation,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const [editingMessageKey, setEditingMessageKey] = useState<string | null>(
    null
  )
  const [sidebarOpen, setSidebarOpen] = useState(true)

  const { data: modelsData, isLoading: isLoadingModels } = useQuery({
    queryKey: ['playground-models'],
    queryFn: async () => {
      try {
        return await getPlaygroundModels()
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to load playground models')
        )
        return []
      }
    },
  })

  const { data: groupsData } = useQuery({
    queryKey: ['playground-groups'],
    queryFn: async () => {
      try {
        return await getUserGroups()
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to load playground groups')
        )
        return []
      }
    },
  })

  useEffect(() => {
    if (!modelsData) return

    const filtered = modelsData
      .filter((m) => isTextModel(m.supported_endpoint_types))
      .map((m) => ({ label: m.name, value: m.name }))

    setModels(filtered)

    const isCurrentModelValid = filtered.some((m) => m.value === config.model)
    if (filtered.length > 0 && !isCurrentModelValid) {
      updateConfig('model', filtered[0].value)
    }
  }, [modelsData, config.model, setModels, updateConfig])

  useEffect(() => {
    if (!groupsData) return

    setGroups(groupsData)

    const hasCurrentGroup = groupsData.some((g) => g.value === config.group)
    if (!hasCurrentGroup && groupsData.length > 0) {
      const fallback =
        groupsData.find((g) => g.value === 'default')?.value ??
        groupsData[0].value
      updateConfig('group', fallback)
    }
  }, [groupsData, setGroups, config.group, updateConfig])

  const handleSendMessage = (text: string) => {
    const userMessage = createUserMessage(text)
    const assistantMessage = createLoadingAssistantMessage()

    const newMessages = [...messages, userMessage, assistantMessage]
    updateMessages(newMessages)

    sendChat(newMessages)
  }

  const handleRegenerateMessage = (message: MessageType) => {
    const messageIndex = messages.findIndex((m) => m.key === message.key)
    if (messageIndex === -1) return

    const messagesUpToHere = messages.slice(0, messageIndex)
    const loadingMessage = createLoadingAssistantMessage()
    const newMessages = [...messagesUpToHere, loadingMessage]

    updateMessages(newMessages)
    sendChat(newMessages)
  }

  const handleEditMessage = useCallback((message: MessageType) => {
    setEditingMessageKey(message.key)
  }, [])

  const handleEditOpenChange = useCallback((open: boolean) => {
    if (!open) setEditingMessageKey(null)
  }, [])

  const applyEdit = useCallback(
    (newContent: string, submit: boolean) => {
      if (!editingMessageKey) return
      const index = messages.findIndex((m) => m.key === editingMessageKey)
      if (index === -1) return

      const updated = messages.map((m) =>
        m.key === editingMessageKey
          ? { ...m, versions: [{ ...m.versions[0], content: newContent }] }
          : m
      )

      setEditingMessageKey(null)

      if (!submit || updated[index].from !== 'user') {
        updateMessages(updated)
        return
      }

      const toSubmit = [
        ...updated.slice(0, index + 1),
        createLoadingAssistantMessage(),
      ]
      updateMessages(toSubmit)
      sendChat(toSubmit)
    },
    [editingMessageKey, messages, updateMessages, sendChat]
  )

  const handleDeleteMessage = (message: MessageType) => {
    const newMessages = messages.filter((m) => m.key !== message.key)
    updateMessages(newMessages)
  }

  const handleDeleteConversation = (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (conversations.length <= 1) {
      toast.warning(t('Cannot delete the last conversation'))
      return
    }
    deleteConversation(id)
  }

  return (
    <div className='relative flex size-full flex-row overflow-hidden'>
      {/* Conversation sidebar */}
      {sidebarOpen && (
        <div className='flex w-60 shrink-0 flex-col border-r'>
          <div className='p-2'>
            <Button
              variant='outline'
              size='sm'
              className='w-full justify-start gap-2'
              onClick={createNewConversation}
              disabled={isGenerating}
            >
              <SquarePenIcon size={14} />
              {t('New Chat')}
            </Button>
          </div>
          <ScrollArea className='flex-1'>
            <div className='flex flex-col gap-0.5 px-2 pb-2'>
              {conversations.map((conv) => (
                <div
                  key={conv.id}
                  onClick={() => {
                    if (isGenerating) return
                    switchToConversation(conv.id)
                  }}
                  className={cn(
                    'group flex items-center gap-2 rounded-lg px-2.5 py-2 text-sm transition-colors',
                    isGenerating
                      ? 'cursor-not-allowed opacity-50'
                      : 'cursor-pointer hover:bg-accent',
                    conv.id === activeConversationId && 'bg-accent'
                  )}
                >
                  <MessageSquareIcon
                    size={14}
                    className='shrink-0 text-muted-foreground'
                  />
                  <span className='flex-1 truncate'>
                    {conv.title || t('New Chat')}
                  </span>
                  <button
                    onClick={(e) => handleDeleteConversation(conv.id, e)}
                    className='shrink-0 opacity-0 transition-opacity group-hover:opacity-100'
                    aria-label={t('Delete')}
                  >
                    <Trash2Icon
                      size={14}
                      className='text-muted-foreground hover:text-destructive'
                    />
                  </button>
                </div>
              ))}
            </div>
          </ScrollArea>
        </div>
      )}

      {/* Main chat area */}
      <div className='relative flex flex-1 flex-col overflow-hidden'>
        <div className='flex flex-1 flex-col overflow-hidden'>
          <div className='mx-auto flex w-full max-w-4xl items-center px-4 pt-2'>
            <Button
              variant='ghost'
              size='icon'
              className='size-7'
              onClick={() => setSidebarOpen((v) => !v)}
            >
              <PanelLeftIcon size={16} />
            </Button>
          </div>
          <PlaygroundChat
            messages={messages}
            onCopyMessage={() => {}}
            onRegenerateMessage={handleRegenerateMessage}
            onEditMessage={handleEditMessage}
            onDeleteMessage={handleDeleteMessage}
            isGenerating={isGenerating}
            editingKey={editingMessageKey}
            onCancelEdit={handleEditOpenChange}
            onSaveEdit={(newContent) => applyEdit(newContent, false)}
            onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
          />
        </div>

        <div className='mx-auto w-full max-w-4xl'>
          <PlaygroundInput
            disabled={isGenerating}
            groups={groups}
            groupValue={config.group}
            isGenerating={isGenerating}
            isModelLoading={isLoadingModels}
            modelValue={config.model}
            models={models}
            onGroupChange={(value) => updateConfig('group', value)}
            onModelChange={(value) => updateConfig('model', value)}
            onStop={stopGeneration}
            onSubmit={handleSendMessage}
          />
        </div>
      </div>
    </div>
  )
}
