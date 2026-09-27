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
import { Download, Eye, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { downloadStatementCsv } from '../api'
import type { BillingStatement } from '../types'
import { useStatements } from './statements-provider'

export function StatementRowActions({
  statement,
}: {
  statement: BillingStatement
}) {
  const { t } = useTranslation()
  const { setCurrentRow, setOpen } = useStatements()
  const [downloading, setDownloading] = useState(false)

  const handleDetail = () => {
    setCurrentRow(statement)
    setOpen('detail')
  }

  const handleExport = async () => {
    setDownloading(true)
    try {
      await downloadStatementCsv(statement)
    } catch {
      // The axios interceptor already surfaced the error toast.
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div className='-ml-1.5 flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleDetail}
              aria-label={t('Detail')}
            />
          }
        >
          <Eye />
        </TooltipTrigger>
        <TooltipContent>{t('Detail')}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleExport}
              disabled={downloading}
              aria-label={t('Export CSV')}
            />
          }
        >
          {downloading ? <Loader2 className='animate-spin' /> : <Download />}
        </TooltipTrigger>
        <TooltipContent>{t('Export CSV')}</TooltipContent>
      </Tooltip>
    </div>
  )
}
