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
import { api } from '@/lib/api'
import type {
  ApiResponse,
  BillingStatement,
  PostpaidUser,
  StatementDetailData,
  StatementPageData,
  StatementPreview,
} from './types'

// ============================================================================
// Postpaid user picker
// ============================================================================

export async function getPostpaidBillingUsers(): Promise<
  ApiResponse<PostpaidUser[]>
> {
  const res = await api.get('/api/billing/postpaid-users')
  return res.data
}

// ============================================================================
// Statement generation
// ============================================================================

export interface StatementPreviewParams {
  user_id: number
  start_timestamp: number
  end_timestamp: number
}

export async function previewBillingStatement(
  params: StatementPreviewParams
): Promise<ApiResponse<StatementPreview>> {
  const res = await api.get('/api/billing/statement/preview', {
    params,
    disableDuplicate: true,
  })
  return res.data
}

export interface CreateStatementPayload {
  user_id: number
  period_start: number
  period_end: number
  remark?: string
}

export async function createBillingStatement(
  data: CreateStatementPayload
): Promise<ApiResponse<BillingStatement>> {
  const res = await api.post('/api/billing/statement/', data)
  return res.data
}

// ============================================================================
// Statement list / detail
// ============================================================================

export interface GetStatementsParams {
  p?: number
  page_size?: number
  user_id?: number
  status?: string
  start_timestamp?: number
  end_timestamp?: number
}

export async function getBillingStatements(
  params: GetStatementsParams
): Promise<ApiResponse<StatementPageData>> {
  const res = await api.get('/api/billing/statement/', {
    params,
    disableDuplicate: true,
  })
  return res.data
}

export async function getBillingStatementDetail(
  id: number
): Promise<ApiResponse<StatementDetailData>> {
  const res = await api.get(`/api/billing/statement/${id}`)
  return res.data
}

// ============================================================================
// Status transitions & adjustments
// ============================================================================

export interface UpdateStatementStatusPayload {
  status: 'paid' | 'voided'
  settle?: boolean
  note?: string
}

export async function updateBillingStatementStatus(
  id: number,
  data: UpdateStatementStatusPayload
): Promise<ApiResponse<BillingStatement>> {
  const res = await api.put(`/api/billing/statement/${id}/status`, data)
  return res.data
}

export async function addBillingStatementAdjustment(
  id: number,
  data: { amount: number; reason: string }
): Promise<ApiResponse<BillingStatement>> {
  const res = await api.post(`/api/billing/statement/${id}/adjustment`, data)
  return res.data
}

export async function deleteBillingStatementAdjustment(
  id: number,
  adjId: number
): Promise<ApiResponse<BillingStatement>> {
  const res = await api.delete(
    `/api/billing/statement/${id}/adjustment/${adjId}`
  )
  return res.data
}

// ============================================================================
// CSV export (auth requires the New-Api-User header, so download via
// axios blob instead of a plain link)
// ============================================================================

export async function downloadStatementCsv(statement: {
  id: number
  statement_no: string
}): Promise<void> {
  const res = await api.get(`/api/billing/statement/${statement.id}/export`, {
    responseType: 'blob',
    disableDuplicate: true,
  })
  const blob = res.data as Blob
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `statement-${statement.statement_no}.csv`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
