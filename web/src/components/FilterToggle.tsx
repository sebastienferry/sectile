import React from 'react'

/**
 * An on/off filter of a view's filter bar, such as "pinned only" (#866). Every
 * view draws its toggles with this one look, so a pressed filter reads the same
 * wherever it sits.
 */
export const FilterToggle: React.FC<{
  pressed: boolean
  onClick: () => void
  icon: React.ReactNode
  label: string
  count?: number
  title?: string
  disabled?: boolean
}> = ({ pressed, onClick, icon, label, count, title, disabled }) => (
  <button
    type="button"
    onClick={onClick}
    disabled={disabled}
    aria-pressed={pressed}
    title={title}
    className={`flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-medium border transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed ${
      pressed
        ? 'accent-text bg-[var(--accent-light)] border-[var(--accent-color)]/50 font-bold'
        : 'bg-[var(--bg-secondary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
    }`}
  >
    {icon}
    <span>{label}</span>
    {count !== undefined && count > 0 && <span className="font-mono text-[10px] opacity-70">{count}</span>}
  </button>
)
