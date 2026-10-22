import React from 'react'
import { Search, X } from 'lucide-react'
import { useApp } from '../context/AppContext'

/**
 * The free-text filter of a view (#866): one look everywhere, filtering as the
 * user types, with a clear button once it holds text.
 */
export const FilterSearch: React.FC<{
  value: string
  onChange: (value: string) => void
  placeholder: string
  className?: string
}> = ({ value, onChange, placeholder, className = 'w-56' }) => {
  const { t } = useApp()
  return (
    <div className={`relative ${className}`}>
      <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none" />
      <input
        type="search"
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className="w-full pl-7 pr-7 py-1 text-[11px] rounded-md bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)] [&::-webkit-search-cancel-button]:hidden"
      />
      {value && (
        <button
          type="button"
          onClick={() => onChange('')}
          aria-label={t.shell.filters.clearSearch}
          title={t.shell.filters.clearSearch}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
        >
          <X size={12} />
        </button>
      )}
    </div>
  )
}
