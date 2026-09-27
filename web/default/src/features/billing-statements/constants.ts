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
import { STATEMENT_STATUS, type StatementStatus } from './types'
import type { StatusVariant } from '@/components/status-badge'

type TranslateFn = (key: string) => string

export function getStatementStatusOptions(
  t: TranslateFn
): { label: string; value: StatementStatus }[] {
  return [
    { label: t('Unpaid'), value: STATEMENT_STATUS.UNPAID },
    { label: t('Paid'), value: STATEMENT_STATUS.PAID },
    { label: t('Voided'), value: STATEMENT_STATUS.VOIDED },
  ]
}

export function getStatementStatusBadge(
  status: string,
  t: TranslateFn
): { label: string; variant: StatusVariant } {
  switch (status) {
    case STATEMENT_STATUS.PAID:
      return { label: t('Paid'), variant: 'success' }
    case STATEMENT_STATUS.VOIDED:
      return { label: t('Voided'), variant: 'neutral' }
    default:
      return { label: t('Unpaid'), variant: 'warning' }
  }
}
