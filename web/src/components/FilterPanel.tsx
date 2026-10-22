import React from 'react'
import { SlidersHorizontal } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useClickOutside } from '../hooks/useClickOutside'
import { useEscapeKey } from '../hooks/useEscapeKey'

/**
 * The value filters of a view, in one panel behind a single "Filters" button
 * (#866). Laid out side by side they no longer fit a toolbar fed by a Jira
 * project; the button's count says how many are narrowing the view, so a
 * filtered view never looks like a complete one.
 *
 * The panel stays open while a value is picked inside it: the lookups render
 * their list through a portal marked for `useClickOutside`. It closes on a press
 * outside and on Escape.
 *
 * `children` may be a function receiving `close`, for an entry that leaves the
 * panel for somewhere else.
 */
export const FilterPanel: React.FC<{
  activeCount: number
  onReset: () => void
  title?: string
  children: React.ReactNode | ((close: () => void) => React.ReactNode)
}> = ({ activeCount, onReset, title, children }) => {
  const { t } = useApp()
  const F = t.shell.filters
  const [isOpen, setIsOpen] = React.useState(false)
  const panelRef = React.useRef<HTMLDivElement>(null)
  const close = React.useCallback(() => setIsOpen(false), [])
  useClickOutside(panelRef, close, isOpen)
  useEscapeKey(isOpen, close)

  const active = activeCount > 0
  return (
    <div className="relative" ref={panelRef}>
      <button
        type="button"
        onClick={() => setIsOpen(open => !open)}
        title={title}
        aria-expanded={isOpen}
        aria-haspopup="true"
        className="flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-semibold border cursor-pointer transition-colors"
        style={{
          color: active ? 'var(--accent-color)' : 'var(--text-secondary)',
          background: active ? 'var(--accent-light)' : 'var(--bg-secondary)',
          borderColor: active ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
        }}
      >
        <SlidersHorizontal size={11} />
        <span>{F.panel}</span>
        {active && (
          <span className="min-w-4 h-4 px-1 rounded-full bg-[var(--accent-color)] text-white text-[9.5px] font-bold flex items-center justify-center">
            {activeCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div
          role="dialog"
          aria-label={F.panel}
          className="absolute right-0 z-50 mt-1 w-[300px] max-w-[calc(100vw-2rem)] max-h-[70vh] overflow-auto rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-lg p-2 space-y-3"
        >
          <div className="flex items-center justify-between px-1">
            <span className="text-[9.5px] uppercase tracking-wider font-bold text-[var(--text-muted)]">{F.panel}</span>
            {active && (
              <button
                type="button"
                onClick={onReset}
                className="text-[10px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {F.reset}
              </button>
            )}
          </div>
          {typeof children === 'function' ? children(close) : children}
        </div>
      )}
    </div>
  )
}
