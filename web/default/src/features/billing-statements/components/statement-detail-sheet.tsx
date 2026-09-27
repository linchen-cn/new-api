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
import { Ban, CheckCircle2, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota, formatTimestamp, parseQuotaFromDollars } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import {
  addBillingStatementAdjustment,
  deleteBillingStatementAdjustment,
  getBillingStatementDetail,
  updateBillingStatementStatus,
} from '../api'
import { getStatementStatusBadge } from '../constants'
import {
  STATEMENT_STATUS,
  type BillingStatement,
  type StatementDetailData,
  type StatementModelRow,
} from '../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  statement: BillingStatement | null
  onSuccess: () => void
}

function parseModelBreakdown(raw: string): StatementModelRow[] {
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as StatementModelRow[]) : []
  } catch {
    return []
  }
}

export function StatementDetailSheet(props: Props) {
  const { t } = useTranslation()
  const [detail, setDetail] = useState<StatementDetailData | null>(null)
  const [loading, setLoading] = useState(false)
  const [adjAmount, setAdjAmount] = useState('')
  const [adjReason, setAdjReason] = useState('')
  const [adding, setAdding] = useState(false)
  const [showMarkPaid, setShowMarkPaid] = useState(false)
  const [settle, setSettle] = useState(true)
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [showVoid, setShowVoid] = useState(false)

  const loadDetail = useCallback(async () => {
    if (!props.statement?.id) return
    setLoading(true)
    try {
      const res = await getBillingStatementDetail(props.statement.id)
      if (res.success) setDetail(res.data || null)
    } catch {
      /* interceptor already surfaced the error toast */
    } finally {
      setLoading(false)
    }
  }, [props.statement?.id])

  useEffect(() => {
    if (props.open) {
      setAdjAmount('')
      setAdjReason('')
      setShowMarkPaid(false)
      setShowVoid(false)
      setSettle(true)
      setNote('')
      loadDetail()
    }
  }, [props.open, loadDetail])

  const stmt = detail?.statement ?? props.statement
  const isPending = stmt?.status === STATEMENT_STATUS.PENDING

  const handleAddAdjustment = async () => {
    if (!stmt?.id) return
    const amount = parseQuotaFromDollars(Number.parseFloat(adjAmount) || 0)
    if (amount === 0 || !adjReason.trim()) {
      toast.error(t('Enter a non-zero amount and a reason'))
      return
    }
    setAdding(true)
    try {
      const res = await addBillingStatementAdjustment(stmt.id, {
        amount,
        reason: adjReason.trim(),
      })
      if (res.success) {
        toast.success(t('Adjustment added'))
        setAdjAmount('')
        setAdjReason('')
        await loadDetail()
        props.onSuccess()
      }
    } catch {
      /* interceptor already surfaced the error toast */
    } finally {
      setAdding(false)
    }
  }

  const handleDeleteAdjustment = async (adjId: number) => {
    if (!stmt?.id) return
    try {
      const res = await deleteBillingStatementAdjustment(stmt.id, adjId)
      if (res.success) {
        toast.success(t('Adjustment removed'))
        await loadDetail()
        props.onSuccess()
      }
    } catch {
      /* interceptor already surfaced the error toast */
    }
  }

  const handleConfirmStatement = async () => {
    if (!stmt?.id) return
    setSubmitting(true)
    try {
      const res = await updateBillingStatementStatus(stmt.id, {
        status: 'confirmed',
        settle,
        note: note.trim() || undefined,
      })
      if (res.success) {
        toast.success(t('Statement confirmed'))
        setShowMarkPaid(false)
        await loadDetail()
        props.onSuccess()
      }
    } catch {
      /* interceptor already surfaced the error toast */
    } finally {
      setSubmitting(false)
    }
  }

  const handleVoid = async () => {
    if (!stmt?.id) return
    setSubmitting(true)
    try {
      const res = await updateBillingStatementStatus(stmt.id, {
        status: 'voided',
      })
      if (res.success) {
        toast.success(t('Statement voided'))
        setShowVoid(false)
        await loadDetail()
        props.onSuccess()
      }
    } catch {
      /* interceptor already surfaced the error toast */
    } finally {
      setSubmitting(false)
    }
  }

  const statusBadge = stmt ? getStatementStatusBadge(stmt.status, t) : null
  const modelRows = stmt ? parseModelBreakdown(stmt.model_breakdown) : []
  const showLoading = loading && !detail

  return (
    <>
      <Sheet open={props.open} onOpenChange={props.onOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-2'>
              <span className='font-mono'>{stmt?.statement_no || '-'}</span>
              {statusBadge ? (
                <StatusBadge
                  label={statusBadge.label}
                  variant={statusBadge.variant}
                  copyable={false}
                />
              ) : null}
            </SheetTitle>
            <SheetDescription>
              {stmt ? `${stmt.username} (#${stmt.user_id})` : '-'}
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName()}>
            {showLoading ? (
              <div className='text-muted-foreground py-8 text-center text-sm'>
                {t('Loading...')}
              </div>
            ) : null}
            {!showLoading && stmt ? (
              <div className='space-y-6'>
                {/* Summary */}
                <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Amount Due')}
                    </div>
                    <div className='text-base font-semibold'>
                      {formatQuota(stmt.total_quota)}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Consumption')}
                    </div>
                    <div className='text-sm font-medium'>
                      {formatQuota(stmt.consume_quota)}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Refund')}
                    </div>
                    <div className='text-sm font-medium'>
                      {formatQuota(stmt.refund_quota)}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Top-up (reference)')}
                    </div>
                    <div className='text-sm font-medium'>
                      {formatQuota(stmt.topup_quota)}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Adjustments')}
                    </div>
                    <div
                      className={`text-sm font-medium ${stmt.adjust_quota < 0 ? 'text-emerald-600' : ''}`}
                    >
                      {formatQuota(stmt.adjust_quota)}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Requests')}
                    </div>
                    <div className='text-sm font-medium'>
                      {stmt.request_count}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Tokens')}
                    </div>
                    <div className='text-sm font-medium'>
                      {stmt.prompt_tokens + stmt.completion_tokens}
                    </div>
                  </div>
                  <div className='bg-muted/50 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Billing Period')}
                    </div>
                    <div className='text-sm font-medium'>
                      {formatTimestamp(stmt.period_start)}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      ~ {formatTimestamp(stmt.period_end)}
                    </div>
                  </div>
                </div>

                {stmt.paid_at > 0 ? (
                  <div className='bg-muted/30 space-y-1 rounded-lg border p-3 text-sm'>
                    <div>
                      <span className='text-muted-foreground'>
                        {t('Confirmed At')}:{' '}
                      </span>
                      {formatTimestamp(stmt.paid_at)}
                    </div>
                    {stmt.settled_quota > 0 ? (
                      <div>
                        <span className='text-muted-foreground'>
                          {t('Settled to balance')}:{' '}
                        </span>
                        {formatQuota(stmt.settled_quota)}
                      </div>
                    ) : null}
                    {stmt.paid_note ? (
                      <div>
                        <span className='text-muted-foreground'>
                          {t('Note')}:{' '}
                        </span>
                        {stmt.paid_note}
                      </div>
                    ) : null}
                  </div>
                ) : null}

                {stmt.remark ? (
                  <div className='bg-muted/30 rounded-lg border p-3 text-sm'>
                    <span className='text-muted-foreground'>
                      {t('Remark')}:{' '}
                    </span>
                    {stmt.remark}
                  </div>
                ) : null}

                {/* Model breakdown (read-only snapshot) */}
                <div className='space-y-2'>
                  <div className='text-sm font-semibold'>
                    {t('Model Breakdown')}
                  </div>
                  <StaticDataTable
                    data={modelRows}
                    getRowKey={(row) => row.model}
                    emptyContent={t('No usage records')}
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
                        cell: (row) =>
                          row.prompt_tokens + row.completion_tokens,
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

                {/* Adjustments */}
                <div className='space-y-2'>
                  <div className='text-sm font-semibold'>
                    {t('Adjustments')}
                  </div>
                  {isPending ? (
                    <div className='flex flex-wrap items-center gap-2'>
                      <Input
                        type='number'
                        step={0.000001}
                        placeholder={t('Amount (±)')}
                        aria-label={t('Amount (±)')}
                        value={adjAmount}
                        onChange={(e) => setAdjAmount(e.target.value)}
                        className='w-32'
                      />
                      <Input
                        placeholder={t('Reason')}
                        aria-label={t('Reason')}
                        value={adjReason}
                        onChange={(e) => setAdjReason(e.target.value)}
                        className='min-w-40 flex-1'
                      />
                      <Button
                        size='sm'
                        onClick={handleAddAdjustment}
                        disabled={adding}
                      >
                        <Plus className='h-4 w-4' />
                        {t('Add')}
                      </Button>
                      <div className='text-muted-foreground w-full text-xs'>
                        {t(
                          'Positive amount increases the due total; negative decreases it.'
                        )}
                      </div>
                    </div>
                  ) : null}
                  <StaticDataTable
                    data={detail?.adjustments || []}
                    getRowKey={(row) => row.id}
                    emptyContent={t('No adjustments')}
                    columns={[
                      {
                        id: 'amount',
                        header: t('Amount'),
                        cell: (row) => (
                          <span
                            className={row.amount < 0 ? 'text-emerald-600' : ''}
                          >
                            {formatQuota(row.amount)}
                          </span>
                        ),
                      },
                      {
                        id: 'reason',
                        header: t('Reason'),
                        cell: (row) => row.reason,
                      },
                      {
                        id: 'created_by',
                        header: t('Created By'),
                        cell: (row) => row.created_by || '-',
                      },
                      {
                        id: 'created_at',
                        header: t('Created At'),
                        cell: (row) => formatTimestamp(row.created_at),
                      },
                      {
                        id: 'actions',
                        header: t('Actions'),
                        className: 'text-right',
                        cellClassName: 'text-right',
                        cell: (row) =>
                          isPending ? (
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              aria-label={t('Delete')}
                              onClick={() => handleDeleteAdjustment(row.id)}
                            >
                              <Trash2 className='text-destructive' />
                            </Button>
                          ) : null,
                      },
                    ]}
                  />
                </div>

                {/* Bottom actions */}
                {isPending ? (
                  <div className='flex flex-wrap gap-2 border-t pt-4'>
                    <Button onClick={() => setShowMarkPaid(true)}>
                      <CheckCircle2 className='h-4 w-4' />
                      {t('Confirm Statement')}
                    </Button>
                    <Button variant='destructive' onClick={() => setShowVoid(true)}>
                      <Ban className='h-4 w-4' />
                      {t('Void')}
                    </Button>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={showMarkPaid}
        onOpenChange={setShowMarkPaid}
        title={t('Confirm Statement')}
        desc={t(
          'Confirm this statement. This action cannot be undone.'
        )}
        confirmText={t('Confirm')}
        isLoading={submitting}
        handleConfirm={handleConfirmStatement}
      >
        <div className='space-y-3'>
          <div className='flex items-start gap-2'>
            <Checkbox
              id='statement-settle'
              checked={settle}
              onCheckedChange={(checked) => setSettle(checked === true)}
              className='mt-1'
            />
            <Label
              htmlFor='statement-settle'
              className='text-sm leading-5 font-normal'
            >
              {t(
                'Also settle the outstanding balance (credit the statement total back to the user balance)'
              )}
            </Label>
          </div>
          <Input
            placeholder={t('Confirmation note (optional)')}
            aria-label={t('Confirmation note (optional)')}
            value={note}
            onChange={(e) => setNote(e.target.value)}
          />
        </div>
      </ConfirmDialog>

      <ConfirmDialog
        open={showVoid}
        onOpenChange={setShowVoid}
        title={t('Void Statement')}
        desc={t(
          'Voiding permanently invalidates this statement without refunding any balance. Continue?'
        )}
        confirmText={t('Void')}
        destructive
        isLoading={submitting}
        handleConfirm={handleVoid}
      />
    </>
  )
}
