import React, { useEffect, useRef, useState } from 'react'
import { Check, Tags } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import type { EpicLabelCount } from '../lib/roadmap'

interface EpicLabelFilterProps {
  inventory: EpicLabelCount[]
  selected: string[]
  onChange: (next: string[]) => void
}

/**
 * The roadmap's filter on epic labels (#626): a toolbar button opening the list
 * of the free labels the visible epics carry, each with how many carry it.
 *
 * It is local to the roadmap on purpose. The board's label filter is sent to the
 * server to filter tickets; reusing it here would make epics vanish from the
 * roadmap for a filter set on the board.
 */
export const EpicLabelFilter: React.FC<EpicLabelFilterProps> = ({ inventory, selected, onChange }) => {
  const { t } = useApp()
  const strings = t.planning.roadmap.epicLabels
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false)
    }
    const escape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', escape)
    }
  }, [open])

  if (inventory.length === 0) return null

  const isSelected = (label: string) => selected.some(s => s.toLowerCase() === label.toLowerCase())
  const toggle = (label: string) =>
    onChange(isSelected(label) ? selected.filter(s => s.toLowerCase() !== label.toLowerCase()) : [...selected, label])
  const active = selected.length > 0

  return (
    <div ref={root} className="relative">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        aria-haspopup="true"
        aria-expanded={open}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold cursor-pointer border"
        style={{
          background: active ? 'var(--accent-light)' : 'var(--bg-tertiary)',
          borderColor: active ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
          color: active ? 'var(--accent-color)' : 'var(--text-secondary)',
        }}
        title={strings.filterTitle}
      >
        <Tags size={12} />
        {active ? format(strings.filterCount, { count: selected.length }) : strings.filter}
      </button>
      {open && (
        <div
          className="absolute z-30 mt-1 min-w-[200px] max-h-[320px] overflow-y-auto rounded-lg border shadow-lg p-1"
          style={{ background: 'var(--bg-secondary)', borderColor: 'var(--border-color)' }}
        >
          {inventory.map(entry => {
            const on = isSelected(entry.label)
            return (
              <button
                key={entry.label}
                type="button"
                role="menuitemcheckbox"
                aria-checked={on}
                onClick={() => toggle(entry.label)}
                className="w-full flex items-center gap-2 px-2 py-1 rounded text-left text-[11.5px] cursor-pointer hover:bg-[var(--bg-tertiary)]"
              >
                <span
                  className="shrink-0 w-3.5 h-3.5 rounded border flex items-center justify-center"
                  style={{
                    borderColor: on ? 'var(--accent-color)' : 'var(--border-color)',
                    background: on ? 'var(--accent-color)' : 'transparent',
                  }}
                >
                  {on && <Check size={10} color="#fff" />}
                </span>
                <span className="flex-1 min-w-0 truncate font-mono">{entry.label}</span>
                <span className="shrink-0 text-[10px] font-mono text-[var(--text-muted)]">{entry.count}</span>
              </button>
            )
          })}
          {active && (
            <button
              type="button"
              onClick={() => onChange([])}
              className="w-full mt-1 px-2 py-1 rounded text-left text-[11px] text-[var(--text-muted)] cursor-pointer border-t hover:bg-[var(--bg-tertiary)]"
              style={{ borderColor: 'var(--border-color)' }}
            >
              {strings.filterClear}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
