import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { MessageSquareIcon, ImageIcon, VideoIcon, MusicIcon } from 'lucide-react'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { STORAGE_KEYS } from './constants'
import { TextChatTab } from './tabs/text-chat'
import { ImageGenTab } from './tabs/image-gen'
import { VideoGenTab } from './tabs/video-gen'
import { AudioGenTab } from './tabs/audio-gen'
import type { CreationTab } from './types'

function loadTab(): CreationTab {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.CREATION_TAB)
    if (saved === 'text' || saved === 'image' || saved === 'video' || saved === 'audio') {
      return saved
    }
  } catch {
    /* ignore */
  }
  return 'text'
}

function saveTab(tab: CreationTab) {
  try {
    localStorage.setItem(STORAGE_KEYS.CREATION_TAB, tab)
  } catch {
    /* ignore */
  }
}

export function Playground() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<CreationTab>(loadTab)

  const handleTabChange = (value: string) => {
    const tab = value as CreationTab
    setActiveTab(tab)
    saveTab(tab)
  }

  return (
    <div className='flex size-full flex-col overflow-hidden'>
      <Tabs
        value={activeTab}
        onValueChange={handleTabChange}
        className='flex h-full flex-col gap-0'
      >
        <div className='border-b px-4 pt-3'>
          <TabsList>
            <TabsTrigger value='text'>
              <MessageSquareIcon size={14} />
              {t('Text Chat')}
            </TabsTrigger>
            <TabsTrigger value='image'>
              <ImageIcon size={14} />
              {t('Image Generation')}
            </TabsTrigger>
            <TabsTrigger value='video'>
              <VideoIcon size={14} />
              {t('Video Generation')}
            </TabsTrigger>
            <TabsTrigger value='audio'>
              <MusicIcon size={14} />
              {t('Audio Generation')}
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value='text' className='flex-1 overflow-hidden'>
          <TextChatTab />
        </TabsContent>
        <TabsContent value='image' className='flex-1 overflow-y-auto'>
          <ImageGenTab />
        </TabsContent>
        <TabsContent value='video' className='flex-1 overflow-y-auto'>
          <VideoGenTab />
        </TabsContent>
        <TabsContent value='audio' className='flex-1 overflow-y-auto'>
          <AudioGenTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
