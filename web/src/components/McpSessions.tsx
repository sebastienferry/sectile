import { useEffect, useRef, useState } from 'react'
import { Plug, PlugZap } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useMcpSessions } from '../hooks/useMcpSessions'
import { connectedFor, sessionLabel, summarizeSessions } from '../lib/mcpSessions'

/**
 * Live MCP clients, in the status bar.
 *
 * Board work can be driven from outside the board, and until now nothing said
 * so. What this shows is a connection, never an intention: a client appears
 * because it is attached, and the runs listed under it are the ones the server
 * would close if it disappeared.
 */
export function McpSessions() {
  const { t } = useApp()
  const { sessions, error } = useMcpSessions()
  const [open, setOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)
  const summary = summarizeSessions(sessions)

  useEffect(() => {
    if (!open) return
    const onClickOutside = (event: MouseEvent) => {
      if (container.current && !container.current.contains(event.target as Node)) setOpen(false)
    }
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onClickOutside)
    document.addEventListener('keydown', onEscape)
    return () => {
      document.removeEventListener('mousedown', onClickOutside)
      document.removeEventListener('keydown', onEscape)
    }
  }, [open])

  const label = summary.clients === 0
    ? t.mcp.noClient
    : `${summary.clients} ${summary.clients > 1 ? t.mcp.clients : t.mcp.client}`

  return (
    <div className="relative" ref={container}>
      <button type="button" aria-expanded={open} aria-haspopup="dialog"
        title={error ? `${t.mcp.unavailable} (${error})` : t.mcp.panelTitle}
        onClick={() => setOpen(previous => !previous)}
        className="inline-flex items-center gap-1.5 rounded px-1.5 py-0.5 hover:bg-[var(--bg-primary)]">
        {summary.clients > 0
          ? <PlugZap size={12} className="text-emerald-400" aria-hidden="true" />
          : <Plug size={12} className={error ? 'text-amber-400' : ''} aria-hidden="true" />}
        <span>{label}</span>
        {summary.runs > 0 && (
          <span className="rounded border border-cyan-500/30 bg-cyan-500/10 px-1 text-[10px] font-semibold text-cyan-400">
            {summary.runs}
          </span>
        )}
      </button>

      {open && (
        <div role="dialog" aria-label={t.mcp.panelTitle}
          className="absolute bottom-full left-1/2 z-50 mb-2 w-80 -translate-x-1/2 rounded-lg border border-[var(--border-color)] bg-[var(--bg-secondary)] p-3 shadow-lg">
          <p className="mb-2 font-semibold text-[var(--text-primary)]">{t.mcp.panelTitle}</p>
          {error && <p className="mb-2 text-amber-400">{t.mcp.unavailable}</p>}
          {sessions.length === 0 && <p className="text-[var(--text-muted)]">{t.mcp.noClient}</p>}
          <ul className="max-h-64 space-y-2 overflow-auto">
            {sessions.map(session => (
              <li key={session.id} className="rounded border border-[var(--border-color)] bg-[var(--bg-primary)] p-2">
                <p className="truncate font-medium text-[var(--text-primary)]" title={sessionLabel(session)}>
                  {sessionLabel(session)}
                </p>
                <p className="text-[var(--text-muted)]">
                  {t.mcp.connected} {connectedFor(session.connectedAt)}
                  {' · '}
                  {session.runs.length === 0
                    ? t.mcp.noRuns
                    : `${session.runs.length} ${session.runs.length > 1 ? t.mcp.runs : t.mcp.run}`}
                </p>
              </li>
            ))}
          </ul>
          <p className="mt-2 text-[10px] text-[var(--text-muted)]">{t.mcp.ownership}</p>
        </div>
      )}
    </div>
  )
}
