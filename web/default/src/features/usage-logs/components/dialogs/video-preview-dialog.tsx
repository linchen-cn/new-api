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
import { useState } from 'react'
import { Video, ExternalLink, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'

interface VideoPreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  url: string
  title?: string
}

export function VideoPreviewDialog({
  open,
  onOpenChange,
  url,
  title,
}: VideoPreviewDialogProps) {
  const { t } = useTranslation()
  const [hasError, setHasError] = useState(false)

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <>
          <Video className='h-5 w-5' />
          {title || t('Video Preview')}
        </>
      }
      contentClassName='sm:max-w-2xl'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      {hasError ? (
        <div className='flex flex-col items-center gap-3 py-8'>
          <span className='text-destructive text-sm'>
            {t('Video playback failed')}
          </span>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              size='sm'
              className='h-8 gap-1'
              onClick={() => window.open(url, '_blank')}
            >
              <ExternalLink className='h-3.5 w-3.5' />
              {t('Open in new tab')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              className='h-8 gap-1'
              onClick={() => {
                navigator.clipboard.writeText(url)
                toast.success(t('Copied'))
              }}
            >
              <Copy className='h-3.5 w-3.5' />
              {t('Copy Link')}
            </Button>
          </div>
        </div>
      ) : (
        <video
          src={url}
          controls
          autoPlay
          className='w-full rounded-lg bg-black'
          onError={() => setHasError(true)}
        >
          {t('Your browser does not support video playback')}
        </video>
      )}
      <div className='flex justify-end gap-2'>
        <Button
          variant='outline'
          size='sm'
          className='h-8 gap-1'
          onClick={() => window.open(url, '_blank')}
        >
          <ExternalLink className='h-3.5 w-3.5' />
          {t('Open in new tab')}
        </Button>
        <Button
          variant='outline'
          size='sm'
          className='h-8 gap-1'
          onClick={() => {
            navigator.clipboard.writeText(url)
            toast.success(t('Copied'))
          }}
        >
          <Copy className='h-3.5 w-3.5' />
          {t('Copy Link')}
        </Button>
      </div>
    </Dialog>
  )
}
