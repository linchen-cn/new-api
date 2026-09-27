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
import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import type {
  BillingStatement,
  StatementsDialogType,
} from '../types'

type StatementsContextType = {
  open: StatementsDialogType | null
  setOpen: (str: StatementsDialogType | null) => void
  currentRow: BillingStatement | null
  setCurrentRow: React.Dispatch<React.SetStateAction<BillingStatement | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const StatementsContext =
  React.createContext<StatementsContextType | null>(null)

export function StatementsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<StatementsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<BillingStatement | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <StatementsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </StatementsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useStatements = () => {
  const ctx = React.useContext(StatementsContext)
  if (!ctx) {
    throw new Error(
      'useStatements has to be used within <StatementsProvider>'
    )
  }
  return ctx
}
