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
import { z } from 'zod'

export const STATEMENT_STATUS = {
  PENDING: 'pending',
  CONFIRMED: 'confirmed',
  VOIDED: 'voided',
} as const

export type StatementStatus =
  (typeof STATEMENT_STATUS)[keyof typeof STATEMENT_STATUS]

// ============================================================================
// Billing Statement Schema & Types (mirrors model.BillingStatement)
// ============================================================================

export const statementModelRowSchema = z.object({
  model: z.string(),
  quota: z.number(),
  requests: z.number(),
  prompt_tokens: z.number(),
  completion_tokens: z.number(),
})
export type StatementModelRow = z.infer<typeof statementModelRowSchema>

export const billingStatementSchema = z.object({
  id: z.number(),
  statement_no: z.string(),
  user_id: z.number(),
  username: z.string(),
  period_start: z.number(),
  period_end: z.number(),
  consume_quota: z.number(),
  refund_quota: z.number(),
  topup_quota: z.number(),
  adjust_quota: z.number(),
  total_quota: z.number(),
  request_count: z.number(),
  prompt_tokens: z.number(),
  completion_tokens: z.number(),
  model_breakdown: z.string(),
  status: z.string(),
  settled_quota: z.number(),
  paid_at: z.number(),
  paid_note: z.string(),
  remark: z.string(),
  created_at: z.number(),
  updated_at: z.number(),
})
export type BillingStatement = z.infer<typeof billingStatementSchema>

export const statementAdjustmentSchema = z.object({
  id: z.number(),
  statement_id: z.number(),
  amount: z.number(),
  reason: z.string(),
  created_at: z.number(),
  created_by: z.string(),
})
export type StatementAdjustment = z.infer<typeof statementAdjustmentSchema>

// ============================================================================
// Statement user picker (mirrors model.StatementUserSummary)
// ============================================================================

export const statementUserSchema = z.object({
  id: z.number(),
  username: z.string(),
  display_name: z.string(),
  billing_type: z.string(),
  quota: z.number(),
  credit_limit: z.number(),
  pending_count: z.number(),
  pending_total: z.number(),
})
export type StatementUser = z.infer<typeof statementUserSchema>

// ============================================================================
// Statement preview (mirrors controller.PreviewBillingStatement response)
// ============================================================================

export const statementPreviewSchema = z.object({
  user_id: z.number(),
  username: z.string(),
  quota: z.number(),
  credit_limit: z.number(),
  period_start: z.number(),
  period_end: z.number(),
  total_quota: z.number(),
  aggregation: z.object({
    consume_quota: z.number(),
    refund_quota: z.number(),
    topup_quota: z.number(),
    request_count: z.number(),
    prompt_tokens: z.number(),
    completion_tokens: z.number(),
    model_rows: z.array(statementModelRowSchema),
  }),
})
export type StatementPreview = z.infer<typeof statementPreviewSchema>

// ============================================================================
// Response wrappers
// ============================================================================

export interface StatementPageData {
  items: BillingStatement[]
  total: number
  page: number
  page_size: number
}

export interface StatementDetailData {
  statement: BillingStatement
  adjustments: StatementAdjustment[]
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export type StatementsDialogType = 'create' | 'detail'
