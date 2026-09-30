import React, { useEffect, useRef, useState } from 'react'
import { Check, FolderKanban } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import type { OfferedOrigin } from '../lib/roadmapOrigins'

interface EpicOriginFilterProps {
  offered: OfferedOrigin[]
  selected: string[]
  onChange: (next: string[]) => void
}

/**
 * Which Jira projects the roadmap shows the epics of (#632): a toolbar button
 * opening the project's own key and its roadmap projects, each with how many
 * epics it holds under the other filters.
 *
 * Unticking every project shows the own key's epics: the caller's selection
 * falls back to it rather than showing nothing.
 */
export const EpicOriginFilter: React.FC<EpicOriginFilterProps> = ({ offered, selected, onChange }) => {
  const { t } = useApp()
  const strings = t.planning.roadmap.origins
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

  if (offered.length === 0) return null

  const own = offered.find(o => o.own)?.key || ''
  const toggle = (key: string) => onChange(selected.includes(key) ? selected.filter(k => k !== key) : [...selected, key])
  const active = !(selected.length === 1 && selected[0] === own)

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
        <FolderKanban size={12} />
        {active ? format(strings.filterCount, { count: selected.length }) : strings.filter}
      </button>
      {open && (
        <div
          role="menu"
          aria-label={strings.filterTitle}
          className="absolute z-30 mt-1 min-w-[200px] max-h-[320px] overflow-y-auto rounded-lg border shadow-lg p-1"
          style={{ background: 'var(--bg-secondary)', borderColor: 'var(--border-color)' }}
        >
          {offered.map(entry => {
            const on = selected.includes(entry.key)
            return (
              <button
                key={entry.key}
                type="button"
                role="menuitemcheckbox"
                aria-checked={on}
                onClick={() => toggle(entry.key)}
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
                <span className="flex-1 min-w-0 truncate font-mono">{entry.key}</span>
                {entry.own && <span className="shrink-0 text-[10px] text-[var(--text-muted)]">{strings.own}</span>}
                <span className="shrink-0 text-[10px] font-mono text-[var(--text-muted)]">{entry.count}</span>
              </button>
            )
          })}
          {active && (
            <button
              type="button"
              onClick={() => onChange([own])}
              className="w-full mt-1 px-2 py-1 rounded text-left text-[11px] text-[var(--text-muted)] cursor-pointer border-t hover:bg-[var(--bg-tertiary)]"
              style={{ borderColor: 'var(--border-color)' }}
            >
              {strings.filterReset}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
