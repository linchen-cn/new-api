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
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { StatementsDialogs } from './components/statements-dialogs'
import { StatementsPrimaryButtons } from './components/statements-primary-buttons'
import { StatementsProvider } from './components/statements-provider'
import { StatementsTable } from './components/statements-table'

function StatementsContent() {
  const { t } = useTranslation()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Billing Statements')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <StatementsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='min-h-0 flex-1'>
            <StatementsTable />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <StatementsDialogs />
    </>
  )
}

export function BillingStatements() {
  return (
    <StatementsProvider>
      <StatementsContent />
    </StatementsProvider>
  )
}
