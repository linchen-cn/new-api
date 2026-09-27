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
import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { getStatementStatusBadge } from '../constants'
import type { BillingStatement } from '../types'
import { StatementRowActions } from './data-table-row-actions'

export function useStatementsColumns(): ColumnDef<BillingStatement>[] {
  const { t } = useTranslation()

  return useMemo(
    (): ColumnDef<BillingStatement>[] => [
      {
        accessorFn: (row) => row.id,
        id: 'id',
        header: t('ID'),
        meta: { mobileHidden: true },
        cell: ({ row }) => <TableId value={row.original.id} />,
        size: 60,
      },
      {
        accessorFn: (row) => row.statement_no,
        id: 'statement_no',
        header: t('Statement No.'),
        meta: { mobileTitle: true },
        cell: ({ row }) => (
          <span className='font-mono text-sm font-medium'>
            {row.original.statement_no}
          </span>
        ),
        size: 170,
      },
      {
        accessorFn: (row) => row.username,
        id: 'user',
        header: t('User'),
        cell: ({ row }) => (
          <div className='min-w-0'>
            <div className='truncate font-medium'>
              {row.original.username}
            </div>
            <div className='text-muted-foreground text-xs'>
              #{row.original.user_id}
            </div>
          </div>
        ),
        size: 140,
      },
      {
        id: 'period',
        header: t('Billing Period'),
        cell: ({ row }) => (
          <div className='text-sm tabular-nums'>
            <div>{formatTimestamp(row.original.period_start)}</div>
            <div className='text-muted-foreground'>
              ~ {formatTimestamp(row.original.period_end)}
            </div>
          </div>
        ),
        size: 160,
      },
      {
        accessorFn: (row) => row.consume_quota,
        id: 'consume',
        header: t('Consumption'),
        cell: ({ row }) => formatQuota(row.original.consume_quota),
        size: 110,
      },
      {
        accessorFn: (row) => row.adjust_quota,
        id: 'adjust',
        header: t('Adjustments'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <span
            className={
              row.original.adjust_quota < 0 ? 'text-emerald-600' : undefined
            }
          >
            {formatQuota(row.original.adjust_quota)}
          </span>
        ),
        size: 110,
      },
      {
        accessorFn: (row) => row.total_quota,
        id: 'total',
        header: t('Amount Due'),
        cell: ({ row }) => (
          <span className='font-semibold'>
            {formatQuota(row.original.total_quota)}
          </span>
        ),
        size: 110,
      },
      {
        accessorFn: (row) => row.status,
        id: 'status',
        header: t('Status'),
        meta: { mobileBadge: true },
        cell: ({ row }) => {
          const badge = getStatementStatusBadge(row.original.status, t)
          return (
            <StatusBadge
              label={badge.label}
              variant={badge.variant}
              copyable={false}
              className='-ml-1.5'
            />
          )
        },
        size: 90,
      },
      {
        accessorFn: (row) => row.created_at,
        id: 'created_at',
        header: t('Created At'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <span className='text-muted-foreground tabular-nums'>
            {formatTimestamp(row.original.created_at)}
          </span>
        ),
        size: 150,
      },
      {
        id: 'actions',
        header: () => t('Actions'),
        cell: ({ row }) => (
          <StatementRowActions statement={row.original} />
        ),
        meta: { pinned: 'right' as const },
      },
    ],
    [t]
  )
}
