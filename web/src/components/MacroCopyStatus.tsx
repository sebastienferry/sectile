import React, { useState } from 'react'
import { Check, ExternalLink, RefreshCw } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { todosMirrorState } from '../lib/roadmap'
import { getTrackers, type TrackerKind } from '../lib/trackers'
import type { MacroTodosMirror } from '../types'

interface Props {
  /** The status the server sent; an older server sends none, and nothing shows. */
  mirror?: MacroTodosMirror | null
  macroKey: string
  testId: string
  /** The line of an up-to-date copy, formatted with the key. */
  upToDate: string
  republishTitle: string
  /** Queues a write at once; the line is busy until it answers. */
  onRepublish: (() => Promise<unknown>) | null
}

/**
 * The status line of a one-way copy of a macro part on its tracker: the todos
 * (#663) or the framing (#636). It says whether the copy is the current text,
 * offers to publish it again when it is not, and to add the missing token when
 * the last write lacked one (#645).
 */
export const MacroCopyStatus: React.FC<Props> = ({ mirror, macroKey, testId, upToDate, republishTitle, onRepublish }) => {
  const { t, openTrackerCredentials } = useApp()
  const strings = t.planning.roadmap.framing
  const [busy, setBusy] = useState(false)
  const state = todosMirrorState(mirror)
  if (!mirror || state === 'none') return null
  const tracker = (mirror.credentialMissing || '') as TrackerKind
  const provider = tracker ? getTrackers(t).find(entry => entry.id === tracker)?.label || tracker : ''
  return (
    <div className="flex flex-wrap items-center gap-1.5 mt-1.5 text-[10.5px]" data-testid={testId} data-state={state}>
      {state === 'upToDate' && (
        <span className="inline-flex items-center gap-1" style={{ color: 'var(--status-ok)' }}>
          <Check size={10} />
          {format(upToDate, { key: macroKey })}
          {mirror.url && (
            <a href={mirror.url} target="_blank" rel="noreferrer" className="hover:text-[var(--accent-color)]"
              title={format(t.planning.macro.openOnTracker, { key: macroKey })}>
              <ExternalLink size={10} />
            </a>
          )}
        </span>
      )}
      {state === 'pending' && (
        <span className="text-[var(--text-muted)]">{strings.mirrorPending}</span>
      )}
      {state === 'failed' && (
        <span className="text-rose-400">{format(strings.mirrorFailed, { reason: mirror.error || '' })}</span>
      )}
      {state === 'local' && (
        <span className="text-[var(--text-muted)]">{format(strings.mirrorLocal, { reason: mirror.reason || '' })}</span>
      )}
      {(state === 'pending' || state === 'failed') && (
        <button
          type="button"
          disabled={busy || !onRepublish}
          onClick={async () => {
            if (!onRepublish) return
            setBusy(true)
            try {
              await onRepublish()
            } finally {
              setBusy(false)
            }
          }}
          className="inline-flex items-center gap-1 px-1.5 py-px rounded border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer disabled:opacity-40"
          title={republishTitle}
        >
          <RefreshCw size={9} className={busy ? 'animate-spin' : ''} />
          {strings.mirrorRepublish}
        </button>
      )}
      {state === 'failed' && tracker && (
        <button
          type="button"
          onClick={() => openTrackerCredentials(tracker)}
          className="px-1.5 py-px rounded border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
        >
          {format(strings.mirrorAddToken, { provider })}
        </button>
      )}
    </div>
  )
}
