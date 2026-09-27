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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Dialog } from '@/components/dialog'
import { StaticDataTable } from '@/components/data-table'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import {
  createBillingStatement,
  getBillingUsers,
  previewBillingStatement,
} from '../api'
import type { StatementPreview, StatementUser } from '../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

// Default billing period: last natural month.
function lastMonthRange(): { start: Date; end: Date } {
  const lastMonth = dayjs().subtract(1, 'month')
  return {
    start: lastMonth.startOf('month').toDate(),
    end: lastMonth.endOf('month').toDate(),
  }
}

function toSeconds(date?: Date): number | undefined {
  return date ? Math.floor(date.getTime() / 1000) : undefined
}

export function StatementCreateDialog(props: Props) {
  const { t } = useTranslation()
  const [users, setUsers] = useState<StatementUser[]>([])
  const [loadingUsers, setLoadingUsers] = useState(false)
  const [userId, setUserId] = useState('')
  const [range, setRange] = useState<{ start?: Date; end?: Date }>(() =>
    lastMonthRange()
  )
  const [preview, setPreview] = useState<StatementPreview | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [remark, setRemark] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const loadUsers = useCallback(async () => {
    setLoadingUsers(true)
    try {
      const res = await getBillingUsers()
      if (res.success) setUsers(res.data || [])
    } catch {
      /* interceptor already surfaced the error toast */
    } finally {
      setLoadingUsers(false)
    }
  }, [])

  useEffect(() => {
    if (!props.open) return
    setUserId('')
    setRange(lastMonthRange())
    setPreview(null)
    setRemark('')
    loadUsers()
  }, [props.open, loadUsers])

  const startSeconds = toSeconds(range.start)
  const endSeconds = toSeconds(range.end)

  // Re-run the preview whenever user or period changes.
  useEffect(() => {
    if (
      !props.open ||
      !userId ||
      !startSeconds ||
      !endSeconds ||
      startSeconds >= endSeconds
    ) {
      setPreview(null)
      return
    }
    const request = {
      user_id: Number(userId),
      start_timestamp: startSeconds,
      end_timestamp: endSeconds,
    }
    let cancelled = false
    const load = async () => {
      setPreviewing(true)
      try {
        const res = await previewBillingStatement(request)
        if (!cancelled && res.success) setPreview(res.data || null)
      } catch {
        /* interceptor already surfaced the error toast */
      } finally {
        if (!cancelled) setPreviewing(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [props.open, userId, startSeconds, endSeconds])

  const selectedUser = users.find((u) => String(u.id) === userId)

  const handleConfirm = async () => {
    if (
      !userId ||
      !startSeconds ||
      !endSeconds ||
      startSeconds >= endSeconds ||
      !preview
    ) {
      return
    }
    setSubmitting(true)
    try {
      const res = await createBillingStatement({
        user_id: Number(userId),
        period_start: startSeconds,
        period_end: endSeconds,
        remark: remark.trim() || undefined,
      })
      if (res.success) {
        toast.success(t('Statement generated'))
        props.onOpenChange(false)
        props.onSuccess()
      }
      // Business failures (e.g. overlapping period) are toasted by the
      // interceptor; keep the dialog open so the admin can adjust.
    } catch {
      /* interceptor already surfaced the error toast */
    } finally {
      setSubmitting(false)
    }
  }

  const userItems = users.map((u) => ({
    value: String(u.id),
    label: `${u.display_name || u.username} · ${t(u.billing_type === 'postpaid' ? 'Postpaid' : 'Prepaid')} · ${t('Balance')}: ${formatQuota(u.quota)}`,
  }))

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Generate Statement')}
      description={t(
        'Aggregate a user’s usage within a billing period into a statement.'
      )}
      bodyClassName='space-y-5'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={submitting || previewing || !preview}
          >
            {submitting ? t('Processing...') : t('Confirm Generation')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label>{t('User')}</Label>
        <Select
          items={userItems}
          value={userId}
          onValueChange={(v) => v !== null && setUserId(v)}
        >
          <SelectTrigger>
            <SelectValue
              placeholder={
                loadingUsers ? t('Loading...') : t('Select a user')
              }
            />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {users.map((u) => (
                <SelectItem key={u.id} value={String(u.id)}>
                  <span className='font-medium'>
                    {u.display_name || u.username}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    #{u.id} · {t(u.billing_type === 'postpaid' ? 'Postpaid' : 'Prepaid')} · {t('Balance')}: {formatQuota(u.quota)}
                  </span>
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>

      <div className='space-y-2'>
        <Label>{t('Billing Period')}</Label>
        <CompactDateTimeRangePicker
          start={range.start}
          end={range.end}
          onChange={(next) => setRange(next)}
        />
      </div>

      {previewing ? (
        <div className='text-muted-foreground flex items-center gap-2 text-sm'>
          <span className='h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent' />
          {t('Loading preview...')}
        </div>
      ) : null}

      {!previewing && preview ? (
        <div className='space-y-4'>
          <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('Amount Due')}
              </div>
              <div className='text-base font-semibold'>
                {formatQuota(preview.total_quota)}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('Consumption')}
              </div>
              <div className='text-sm font-medium'>
                {formatQuota(preview.aggregation.consume_quota)}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>{t('Refund')}</div>
              <div className='text-sm font-medium'>
                {formatQuota(preview.aggregation.refund_quota)}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('Top-up (reference)')}
              </div>
              <div className='text-sm font-medium'>
                {formatQuota(preview.aggregation.topup_quota)}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('Requests')}
              </div>
              <div className='text-sm font-medium'>
                {preview.aggregation.request_count}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('Prompt Tokens')}
              </div>
              <div className='text-sm font-medium'>
                {preview.aggregation.prompt_tokens}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('Completion Tokens')}
              </div>
              <div className='text-sm font-medium'>
                {preview.aggregation.completion_tokens}
              </div>
            </div>
            <div className='bg-muted/50 rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs'>
                {t('User Balance')}
              </div>
              <div className='text-sm font-medium'>
                {formatQuota(preview.quota)}
              </div>
            </div>
          </div>

          <StaticDataTable
            data={preview.aggregation.model_rows ?? []}
            getRowKey={(row) => row.model}
            emptyContent={t('No usage records in this period')}
            columns={[
              {
                id: 'model',
                header: t('Model'),
                cell: (row) => (
                  <span className='font-mono text-sm'>{row.model}</span>
                ),
              },
              {
                id: 'requests',
                header: t('Requests'),
                cell: (row) => row.requests,
              },
              {
                id: 'tokens',
                header: t('Tokens'),
                cell: (row) => row.prompt_tokens + row.completion_tokens,
              },
              {
                id: 'quota',
                header: t('Amount'),
                className: 'text-right',
                cellClassName: 'text-right',
                cell: (row) => formatQuota(row.quota),
              },
            ]}
          />
        </div>
      ) : null}

      {!previewing && !preview && selectedUser ? (
        <div className='text-muted-foreground text-sm'>
          {t('Select a valid billing period to preview the aggregation.')}
        </div>
      ) : null}

      <div className='space-y-2'>
        <Label>{t('Remark')}</Label>
        <Textarea
          placeholder={t('Optional note for this statement')}
          value={remark}
          onChange={(e) => setRemark(e.target.value)}
          rows={2}
        />
      </div>
    </Dialog>
  )
}
