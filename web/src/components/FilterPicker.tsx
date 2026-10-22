import React from 'react'
import { LookupField, type LookupOption } from './LookupField'

/** One labelled line of a filter panel: an icon and a name, then the control. */
export const FilterRow: React.FC<{ icon: React.ReactNode; label: string; children: React.ReactNode }> = ({ icon, label, children }) => (
  <div className="flex items-center gap-2 px-1">
    <span className="flex items-center gap-1.5 w-[76px] shrink-0 text-[11px] font-medium text-[var(--text-secondary)]">
      {icon}
      {label}
    </span>
    <div className="flex-1 min-w-0">{children}</div>
  </div>
)

/**
 * A filter choosing one value, searchable (#866): the same lookup for a sprint,
 * a macro, a person or a skill, in every view. `value` is the label shown while
 * closed; `onPick` gets the chosen id, or null when the filter is cleared.
 */
export const SinglePicker: React.FC<{
  icon: React.ReactNode
  label: string
  value: string
  placeholder: string
  clearLabel?: string
  allowClear?: boolean
  onSearch: (query: string) => Promise<LookupOption[]>
  onPick: (id: string | null) => void
}> = ({ icon, label, value, placeholder, clearLabel, allowClear, onSearch, onPick }) => (
  <FilterRow icon={icon} label={label}>
    <LookupField
      value={value}
      placeholder={placeholder}
      clearLabel={clearLabel ?? placeholder}
      allowClear={allowClear}
      onSearch={onSearch}
      onPick={option => onPick(option?.id || null)}
    />
  </FilterRow>
)
