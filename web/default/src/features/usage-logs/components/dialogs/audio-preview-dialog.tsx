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
import { useState, useRef, useEffect } from 'react'
import { ExternalLink, Copy, Music, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'

export interface AudioClip {
  clip_id?: string
  id?: string
  title?: string
  tags?: string
  duration?: number
  audio_url?: string
  image_url?: string
  image_large_url?: string
  metadata?: {
    tags?: string
    duration?: number
  }
}

interface AudioPreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  clips: AudioClip[]
}

function formatDuration(seconds?: number): string {
  if (!seconds || seconds <= 0) return '--:--'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

function AudioClipCard({ clip }: { clip: AudioClip }) {
  const { t } = useTranslation()
  const [hasError, setHasError] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [blobUrl, setBlobUrl] = useState<string | null>(null)
  const audioRef = useRef<HTMLAudioElement>(null)

  const audioUrl = clip.audio_url || ''

  useEffect(() => {
    setHasError(false)
    setBlobUrl(null)
    // For proxy URLs (e.g. /v1/videos/.../content), fetch with auth
    // because <audio> elements don't send session cookies/headers
    if (audioUrl && audioUrl.includes('/v1/videos/')) {
      setIsLoading(true)
      api.get(audioUrl, { responseType: 'blob' })
        .then((res) => {
          const url = URL.createObjectURL(res.data as Blob)
          setBlobUrl(url)
        })
        .catch(() => setHasError(true))
        .finally(() => setIsLoading(false))
    }
  }, [audioUrl])

  // Cleanup blob URL on unmount
  useEffect(() => {
    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl)
    }
  }, [blobUrl])

  const title = clip.title || t('Untitled')
  const tags = clip.tags || clip.metadata?.tags || ''
  const duration = clip.duration || clip.metadata?.duration
  const imageUrl = clip.image_url || clip.image_large_url
  const playbackUrl = blobUrl || audioUrl

  if (!audioUrl) return null

  return (
    <div className='bg-card flex gap-4 rounded-lg border p-4'>
      {imageUrl && (
        <img
          src={imageUrl}
          alt={title}
          className='h-20 w-20 shrink-0 rounded-lg object-cover'
          onError={(e) => {
            ;(e.target as HTMLElement).style.display = 'none'
          }}
        />
      )}
      <div className='min-w-0 flex-1'>
        <div className='mb-1 flex items-center gap-2'>
          <span className='truncate text-sm font-medium'>{title}</span>
          {duration != null && duration > 0 && (
            <StatusBadge
              label={formatDuration(duration)}
              variant='neutral'
              className='shrink-0'
              copyable={false}
            />
          )}
        </div>

        {tags && (
          <p className='text-muted-foreground mb-2 truncate text-xs'>{tags}</p>
        )}

        {hasError ? (
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-destructive text-xs'>
              {t('Audio playback failed')}
            </span>
            <Button
              variant='outline'
              size='sm'
              className='h-7 gap-1 text-xs'
              onClick={() => window.open(audioUrl, '_blank')}
            >
              <ExternalLink className='h-3 w-3' />
              {t('Open in new tab')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              className='h-7 gap-1 text-xs'
              onClick={() => {
                navigator.clipboard.writeText(audioUrl)
                toast.success(t('Copied'))
              }}
            >
              <Copy className='h-3 w-3' />
              {t('Copy Link')}
            </Button>
          </div>
        ) : isLoading ? (
          <div className='flex items-center gap-2 py-1'>
            <Loader2 className='h-4 w-4 animate-spin text-muted-foreground' />
            <span className='text-muted-foreground text-xs'>{t('Loading...')}</span>
          </div>
        ) : (
          <audio
            ref={audioRef}
            src={playbackUrl}
            controls
            preload='none'
            onError={() => setHasError(true)}
            className='h-9 w-full'
          />
        )}
      </div>
    </div>
  )
}

export function AudioPreviewDialog(props: AudioPreviewDialogProps) {
  const { t } = useTranslation()
  const clips = Array.isArray(props.clips) ? props.clips : []

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          <Music className='h-5 w-5' />
          {t('Audio Preview')}
        </>
      }
      contentClassName='sm:max-w-lg'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      {clips.length === 0 ? (
        <p className='text-muted-foreground py-4 text-center text-sm'>
          {t('None')}
        </p>
      ) : (
        <ScrollArea className='max-h-[60vh]'>
          <div className='space-y-3 pr-2'>
            {clips.map((clip, idx) => (
              <AudioClipCard key={clip.clip_id || clip.id || idx} clip={clip} />
            ))}
          </div>
        </ScrollArea>
      )}
    </Dialog>
  )
}
