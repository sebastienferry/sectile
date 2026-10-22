import React from 'react'
import { Filter, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { clearAll, type ActiveFilter } from '../lib/activeFilters'

/**
 * The active filters of a view that its toolbar does not already show pressed
 * (#866): the values picked in its "Filters" panel, and those set elsewhere,
 * such as the header search, that still narrow it. One chip each; pressing a
 * chip clears that filter alone, "Clear all" clears every chip of the row.
 */
export const ActiveFilterChips: React.FC<{ filters: ActiveFilter[] }> = ({ filters }) => {
  const { t } = useApp()
  const F = t.shell.filters
  if (filters.length === 0) return null
  return (
    <div className="flex items-center gap-1.5 flex-wrap" role="group" aria-label={F.activeFilters}>
      {filters.map(filter => (
        <button
          key={filter.id}
          type="button"
          onClick={filter.clear}
          className="flex items-center gap-1 px-2 py-1 rounded-md text-[10.5px] font-semibold cursor-pointer border"
          style={{
            background: 'rgb(var(--status-warn-rgb) / 0.14)',
            borderColor: 'rgb(var(--status-warn-rgb) / 0.34)',
            color: 'var(--status-warn)',
          }}
          title={format(F.activeFilterTitle, { label: filter.label })}
        >
          <Filter size={10} />
          {filter.label}
          <X size={10} />
        </button>
      ))}
      {filters.length > 1 && (
        <button
          type="button"
          onClick={() => clearAll(filters)}
          title={F.clearAllTitle}
          className="px-1.5 py-1 text-[10.5px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
        >
          {F.clearAll}
        </button>
      )}
    </div>
  )
}
