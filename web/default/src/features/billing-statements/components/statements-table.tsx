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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { getBillingStatements } from '../api'
import { getStatementStatusOptions } from '../constants'
import { useStatementsColumns } from './statements-columns'
import { useStatements } from './statements-provider'

const route = getRouteApi('/_authenticated/billing-statements/')

export function StatementsTable() {
  const { t } = useTranslation()
  const { refreshTrigger } = useStatements()
  const columns = useStatementsColumns()
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    // No keyword search on the backend list API; the toolbar search slot is
    // replaced by the period date-range picker below.
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []

  const startDate = search.start_timestamp
    ? new Date(search.start_timestamp * 1000)
    : undefined
  const endDate = search.end_timestamp
    ? new Date(search.end_timestamp * 1000)
    : undefined

  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'billing-statements',
      pagination.pageIndex + 1,
      pagination.pageSize,
      statusFilter,
      search.start_timestamp,
      search.end_timestamp,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getBillingStatements({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        status: statusFilter[0] ?? '',
        start_timestamp: search.start_timestamp || undefined,
        end_timestamp: search.end_timestamp || undefined,
      })
      // Business failures are toasted by the axios interceptor; just
      // degrade to an empty page here.
      if (!result.success) return { items: [], total: 0 }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const statements = data?.items || []

  const { table } = useDataTable({
    data: statements,
    columns,
    columnFilters,
    pagination,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  const handleDateRangeChange = ({
    start,
    end,
  }: {
    start?: Date
    end?: Date
  }) => {
    navigate({
      search: (prev) => ({
        ...prev,
        page: undefined,
        start_timestamp: start ? Math.floor(start.getTime() / 1000) : undefined,
        end_timestamp: end ? Math.floor(end.getTime() / 1000) : undefined,
      }),
    })
  }

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Statements Found')}
      emptyDescription={t(
        'No billing statements yet. Generate one from postpaid usage.'
      )}
      skeletonKeyPrefix='billing-statements-skeleton'
      applyHeaderSize
      toolbarProps={{
        customSearch: (
          <CompactDateTimeRangePicker
            start={startDate}
            end={endDate}
            onChange={handleDateRangeChange}
          />
        ),
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: getStatementStatusOptions(t),
            singleSelect: true,
          },
        ],
        hasAdditionalFilters: Boolean(
          search.start_timestamp || search.end_timestamp
        ),
        onReset: () => {
          navigate({
            search: (prev) => ({
              ...prev,
              page: undefined,
              start_timestamp: undefined,
              end_timestamp: undefined,
            }),
          })
        },
      }}
    />
  )
}
