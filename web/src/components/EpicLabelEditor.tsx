import React, { useId, useState } from 'react'
import { Plus, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { canEditEpicLabels, freeEpicLabels, isEpicAxisLabel, type EpicRow } from '../lib/roadmap'
import type { Project } from '../types'

interface EpicLabelEditorProps {
  project: Project
  row: EpicRow
  /** The free labels carried by the project's epics, offered as suggestions. */
  suggestions: string[]
}

/**
 * An epic's free labels in the roadmap panel (#626): chips to remove, an input
 * to add one.
 *
 * Nothing changes on screen until the tracker accepted the edit: the write is
 * queued, and the roadmap reloads the macros once it ran. The obvious refusals
 * are caught here so they never leave the browser; the server refuses them too.
 */
export const EpicLabelEditor: React.FC<EpicLabelEditorProps> = ({ project, row, suggestions }) => {
  const { t, editMacroLabels, addToast } = useApp()
  const strings = t.planning.roadmap.epicLabels
  const listId = useId()
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)

  // Only Jira reads epics, so elsewhere there is nothing to show at all.
  if (project.issueTracker !== 'jira') return null

  const labels = freeEpicLabels(row.meta)
  const editable = canEditEpicLabels(project, row)
  if (!editable && labels.length === 0) return null

  const carried = new Set(labels.map(l => l.toLowerCase()))
  const offered = suggestions.filter(s => !carried.has(s.toLowerCase()))

  const refuse = (description: string) => addToast({ type: 'error', title: strings.refused, description })

  const send = async (patch: { add?: string[]; remove?: string[] }) => {
    setBusy(true)
    const ok = await editMacroLabels(project.id, row.key, patch)
    setBusy(false)
    return ok
  }

  const add = async () => {
    const label = draft.trim()
    if (!label) return
    if (/\s/.test(label)) return refuse(format(strings.refusedSpace, { label }))
    if (isEpicAxisLabel(label)) return refuse(format(strings.refusedAxis, { label }))
    if (carried.has(label.toLowerCase())) return refuse(format(strings.alreadyThere, { key: row.key, label }))
    if (await send({ add: [label] })) setDraft('')
  }

  const readOnlyReason = /^M-\d+$/i.test(row.key.trim()) ? strings.readOnlyMilestone : strings.readOnlyForeign

  return (
    <div>
      <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
        {strings.panelTitle}
      </div>
      <div className="flex items-center gap-1.5 flex-wrap">
        {labels.length === 0 && <span className="text-[11px] text-[var(--text-muted)]">{strings.none}</span>}
        {labels.map(label => (
          <span
            key={label}
            className="inline-flex items-center gap-1 text-[10.5px] px-1.5 py-0.5 rounded font-mono bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)]"
          >
            {label}
            {editable && (
              <button
                type="button"
                disabled={busy}
                onClick={() => send({ remove: [label] })}
                className="text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer disabled:opacity-50"
                aria-label={format(strings.remove, { label })}
                title={format(strings.remove, { label })}
              >
                <X size={10} />
              </button>
            )}
          </span>
        ))}
      </div>
      {editable ? (
        <form
          className="flex items-center gap-1.5 mt-2"
          onSubmit={e => {
            e.preventDefault()
            add()
          }}
        >
          <input
            type="text"
            value={draft}
            list={listId}
            disabled={busy}
            onChange={e => setDraft(e.target.value)}
            placeholder={strings.addPlaceholder}
            className="flex-1 min-w-0 px-2 py-1 rounded-md text-[11.5px] font-mono border bg-[var(--bg-primary)] text-[var(--text-primary)] border-[var(--border-color)]"
          />
          <datalist id={listId}>
            {offered.map(label => (
              <option key={label} value={label} />
            ))}
          </datalist>
          <button
            type="submit"
            disabled={busy || !draft.trim()}
            className="flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-semibold border cursor-pointer disabled:opacity-50 bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)]"
          >
            <Plus size={11} />
            {strings.add}
          </button>
        </form>
      ) : (
        <div className="mt-1.5 text-[10.5px] text-[var(--text-muted)]">{readOnlyReason}</div>
      )}
    </div>
  )
}
