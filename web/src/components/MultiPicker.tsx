import React from 'react'
import { Check } from 'lucide-react'

export interface MultiPickerOption {
  value: string
  /** Shown instead of the value when given. */
  label?: string
  count?: number
  /** A muted word after the label, such as "hidden" or "this project". */
  hint?: string
}

/**
 * A filter choosing several values, each with how many items carry it (#866):
 * tracker statuses and issue types on the board, epic labels and origins on the
 * roadmap. The list stays open between picks. What "nothing ticked" means is the
 * caller's: every value on the board, the project's own key on the roadmap.
 */
export const MultiPicker: React.FC<{
  title: string
  options: MultiPickerOption[]
  isSelected: (value: string) => boolean
  onToggle: (value: string) => void
  /** The link clearing the selection, shown while `canClear`. */
  clearLabel: string
  canClear: boolean
  onClear: () => void
  mono?: boolean
  footer?: React.ReactNode
}> = ({ title, options, isSelected, onToggle, clearLabel, canClear, onClear, mono, footer }) => (
  <div className="border-t border-[var(--border-color)] pt-2" role="group" aria-label={title}>
    <div className="flex items-center justify-between px-1.5 pb-1">
      <span className="text-[9.5px] uppercase tracking-wider font-bold text-[var(--text-muted)]">{title}</span>
      {canClear && (
        <button
          type="button"
          onClick={onClear}
          className="text-[10px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
        >
          {clearLabel}
        </button>
      )}
    </div>
    <div className="max-h-[180px] overflow-auto">
      {options.map(option => {
        const isActive = isSelected(option.value)
        return (
          <button
            key={option.value}
            type="button"
            role="menuitemcheckbox"
            aria-checked={isActive}
            onClick={() => onToggle(option.value)}
            className="w-full flex items-center gap-2 px-1.5 py-1 rounded-lg hover:bg-[var(--bg-tertiary)] cursor-pointer"
          >
            <span
              className="w-3 h-3 rounded flex items-center justify-center shrink-0"
              style={{
                background: isActive ? 'var(--accent-color)' : 'transparent',
                border: `1px solid ${isActive ? 'var(--accent-color)' : 'var(--border-color)'}`,
              }}
            >
              {isActive && <Check size={8} className="text-white" />}
            </span>
            <span className={`text-[11px] text-[var(--text-primary)] truncate flex-1 text-left ${mono ? 'font-mono' : ''}`}>
              {option.label ?? option.value}
              {option.hint && <span className="ml-1 text-[9px] font-sans text-[var(--text-muted)]">{option.hint}</span>}
            </span>
            {option.count !== undefined && (
              <span className="text-[9.5px] font-mono text-[var(--text-muted)]">{option.count}</span>
            )}
          </button>
        )
      })}
    </div>
    {footer}
  </div>
)
