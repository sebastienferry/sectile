import React, { useCallback, useEffect, useState } from 'react'
import { Database, Plus, RefreshCw, Save, Settings2, Trash2, X, Zap } from 'lucide-react'
import { useApp } from '../../context/AppContext'
import { format } from '../../lib/i18n'
import {
  EMPTY_TRACKER_DRAFT,
  TRACKER_PROVIDERS,
  TRACKER_PROVIDER_NAMES,
  createTracker,
  deleteTracker,
  fetchAdminTracker,
  fetchAdminTrackers,
  fetchTrackerIssueTypes,
  syncTracker,
  trackerDisplayName,
  trackerDraftOf,
  trackerDraftProblem,
  trackerPayload,
  trackerProjects,
  trackerSourceLocked,
  updateTracker,
  type TrackerDraft,
} from '../../lib/trackers'
import type { Tracker, TrackerColumn, TrackerProvider } from '../../types'
import { BoardColumnsEditor } from '../BoardColumnsEditor'

const fieldClass =
  'w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]'
const buttonClass =
  'flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed'
const labelClass = 'block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1'

const errorText = (err: unknown) => (err instanceof Error ? err.message : String(err))

interface SourceFieldsProps {
  draft: TrackerDraft
  onChange: (draft: TrackerDraft) => void
  /** A recorded tracker keeps its provider: its tickets were imported from it. */
  providerLocked?: boolean
  /** A tracker holding tickets also keeps its site and scope: they were read from them. */
  sourceLocked?: boolean
  idPrefix: string
}

/** The provider, name, site and scope of a tracker. */
function SourceFields({ draft, onChange, providerLocked, sourceLocked, idPrefix }: SourceFieldsProps) {
  const { t } = useApp()
  const labels = t.admin.trackers
  const scopePlaceholder = draft.provider ? labels.scopePlaceholders[draft.provider] : ''
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div>
        <label htmlFor={`${idPrefix}-provider`} className={labelClass}>{labels.provider}</label>
        <select
          id={`${idPrefix}-provider`}
          value={draft.provider}
          disabled={providerLocked || sourceLocked}
          onChange={e => onChange({ ...draft, provider: e.target.value as TrackerProvider | '' })}
          className={`${fieldClass} cursor-pointer`}
        >
          <option value="">{labels.chooseProvider}</option>
          {TRACKER_PROVIDERS.map(provider => (
            <option key={provider} value={provider}>{TRACKER_PROVIDER_NAMES[provider]}</option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor={`${idPrefix}-scope`} className={labelClass}>{labels.scope}</label>
        <input
          id={`${idPrefix}-scope`}
          type="text"
          value={draft.scope}
          disabled={sourceLocked}
          onChange={e => onChange({ ...draft, scope: e.target.value })}
          placeholder={scopePlaceholder}
          className={`${fieldClass} font-mono`}
        />
      </div>
      <div>
        <label htmlFor={`${idPrefix}-name`} className={labelClass}>{labels.name}</label>
        <input
          id={`${idPrefix}-name`}
          type="text"
          value={draft.name}
          onChange={e => onChange({ ...draft, name: e.target.value })}
          placeholder={labels.namePlaceholder}
          className={fieldClass}
        />
      </div>
      <div>
        <label htmlFor={`${idPrefix}-site`} className={labelClass}>{labels.site}</label>
        <input
          id={`${idPrefix}-site`}
          type="text"
          value={draft.site}
          disabled={sourceLocked}
          onChange={e => onChange({ ...draft, site: e.target.value })}
          placeholder={labels.sitePlaceholder}
          className={`${fieldClass} font-mono`}
        />
      </div>
      {sourceLocked && (
        <span className="text-[9px] text-[var(--text-muted)] sm:col-span-2">{labels.sourceLocked}</span>
      )}
    </div>
  )
}

interface EditorProps {
  tracker: Tracker
  onSaved: (tracker: Tracker) => void
  onClose: () => void
}

/**
 * One tracker's configuration: its source, the issue types it imports, its
 * background synchronisation and its board columns with their mapping onto the
 * workflow stages. These moved here from the project settings (#741): every
 * project selecting the tracker reads them.
 */
function TrackerEditor({ tracker, onSaved, onClose }: EditorProps) {
  const { t, addToast, fetchProjects, fetchTrackers } = useApp()
  const labels = t.admin.trackers
  const ps = t.projectSettings.tracker
  const [current, setCurrent] = useState<Tracker>(tracker)
  const [draft, setDraft] = useState<TrackerDraft>(trackerDraftOf(tracker))
  const [columns, setColumns] = useState<TrackerColumn[]>(tracker.trackerColumns || [])
  const [stageColumns, setStageColumns] = useState<Record<string, string[]>>(tracker.stageColumns || {})
  const [issueTypes, setIssueTypes] = useState<string[]>(tracker.issueTypes || [])
  const [availableIssueTypes, setAvailableIssueTypes] = useState<string[]>([])
  const [loadingIssueTypes, setLoadingIssueTypes] = useState(false)
  const [autoSyncEnabled, setAutoSyncEnabled] = useState(Boolean(tracker.autoSyncEnabled))
  const [autoSyncIntervalMin, setAutoSyncIntervalMin] = useState(tracker.autoSyncIntervalMin || 5)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  // The work item types the Jira space exposes: without them, the choice would
  // be made blind, which is where a space that imports nothing hides.
  useEffect(() => {
    if (tracker.provider !== 'jira') return
    let cancelled = false
    setLoadingIssueTypes(true)
    fetchTrackerIssueTypes(tracker.id)
      .then(list => { if (!cancelled) setAvailableIssueTypes(list) })
      .catch(() => { if (!cancelled) setAvailableIssueTypes([]) })
      .finally(() => { if (!cancelled) setLoadingIssueTypes(false) })
    return () => { cancelled = true }
  }, [tracker.id, tracker.provider])

  const problem = trackerDraftProblem(draft)

  const save = async () => {
    if (problem) {
      setError(labels.problems[problem])
      return
    }
    setBusy(true)
    setError('')
    try {
      // A rewrite replaces the whole tracker: the sprints and the board a sync
      // refreshed since the editor opened are read again rather than sent
      // back as they were.
      const latest = await fetchAdminTracker(tracker.id).catch(() => current)
      const saved = await updateTracker(tracker.id, {
        ...trackerPayload(draft, latest),
        trackerColumns: columns,
        stageColumns,
        issueTypes,
        autoSyncEnabled,
        autoSyncIntervalMin,
      })
      setCurrent(saved)
      onSaved(saved)
      // The projects read their board mirror through from their tracker.
      void fetchProjects()
      void fetchTrackers()
      addToast({ type: 'success', title: format(labels.saved, { name: saved.name }) })
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const toggleIssueType = (type: string) => {
    // The first click freezes the current selection: otherwise unticking
    // "Task" on a tracker left at the defaults would change nothing.
    const selected = issueTypes.length === 0
      ? availableIssueTypes.filter(item => item === 'Task' || item === 'Story')
      : issueTypes
    setIssueTypes(selected.includes(type) ? selected.filter(item => item !== type) : [...selected, type])
  }

  return (
    <div className="space-y-4 border-t border-[var(--border-color)] pt-3" data-tracker-editor={tracker.id}>
      <SourceFields
        draft={draft}
        onChange={setDraft}
        providerLocked
        sourceLocked={trackerSourceLocked(current)}
        idPrefix={`tracker-${tracker.id}`}
      />

      {tracker.provider === 'jira' && (
        <div>
          <span className={labelClass}>{ps.issueTypesLabel}</span>
          {loadingIssueTypes ? (
            <span className="text-[10px] text-[var(--text-muted)]">{ps.issueTypesLoading}</span>
          ) : availableIssueTypes.length === 0 ? (
            <span className="text-[10px] text-[var(--text-muted)]">{labels.issueTypesUnavailable}</span>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {availableIssueTypes.map(type => {
                const isDefault = type === 'Task' || type === 'Story'
                const isActive = issueTypes.length === 0 ? isDefault : issueTypes.includes(type)
                return (
                  <button
                    key={type}
                    type="button"
                    aria-pressed={isActive}
                    onClick={() => toggleIssueType(type)}
                    className="px-2 py-1 rounded-lg text-[10.5px] font-semibold border cursor-pointer transition-colors"
                    style={{
                      color: isActive ? 'var(--accent-color)' : 'var(--text-secondary)',
                      background: isActive ? 'var(--accent-light)' : 'var(--bg-tertiary)',
                      borderColor: isActive ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
                    }}
                  >
                    {type}
                  </button>
                )
              })}
            </div>
          )}
          <span className="text-[9px] text-[var(--text-muted)] mt-1 block">{ps.issueTypesHelp}</span>
        </div>
      )}

      <div className="p-3 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] space-y-2">
        <div className="flex items-center justify-between gap-3">
          <div>
            <span className="text-xs font-bold text-[var(--text-primary)] block">{ps.syncTitle}</span>
            <span className="text-[10px] text-[var(--text-secondary)] block mt-0.5">{labels.syncHelp}</span>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={autoSyncEnabled}
            aria-label={ps.syncTitle}
            onClick={() => setAutoSyncEnabled(value => !value)}
            className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out ${
              autoSyncEnabled ? 'bg-[var(--accent-color)]' : 'bg-[var(--border-color)]'
            }`}
          >
            <span
              className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out ${
                autoSyncEnabled ? 'translate-x-4' : 'translate-x-0'
              }`}
            />
          </button>
        </div>
        {autoSyncEnabled && (
          <div className="pt-2 border-t border-[var(--border-color)] space-y-1.5">
            <div className="flex items-center justify-between text-xs">
              <span className="text-[var(--text-secondary)] font-medium">{ps.syncPeriod}</span>
              <span className="font-mono font-bold text-[var(--accent-color)]">{format(ps.minutes, { count: autoSyncIntervalMin })}</span>
            </div>
            <input
              type="range"
              min={1}
              max={30}
              value={autoSyncIntervalMin}
              onChange={e => setAutoSyncIntervalMin(parseInt(e.target.value, 10) || 5)}
              className="w-full h-1.5 bg-[var(--bg-primary)] rounded-lg appearance-none cursor-pointer accent-[var(--accent-color)]"
            />
          </div>
        )}
      </div>

      <div>
        <span className={labelClass}>{labels.board}</span>
        <BoardColumnsEditor
          tracker={current}
          columns={columns}
          onColumnsChange={setColumns}
          stageColumns={stageColumns}
          onStageColumnsChange={setStageColumns}
          onTrackerChange={updated => { setCurrent(updated); onSaved(updated) }}
        />
      </div>

      {error && <p role="alert" className="text-[11px] text-red-400">{error}</p>}

      <div className="flex items-center justify-end gap-2">
        <button type="button" onClick={onClose} disabled={busy} className={buttonClass}>
          <X size={12} />
          <span>{labels.close}</span>
        </button>
        <button type="button" onClick={() => void save()} disabled={busy} className={`${buttonClass} accent-bg text-white border-transparent`}>
          <Save size={12} />
          <span>{labels.save}</span>
        </button>
      </div>
    </div>
  )
}

/**
 * The Administration page's Trackers section (#741, D11): the trackers the
 * deployment synchronises, each recorded once and selected by any number of
 * projects. Members pick trackers for their projects; only an admin records
 * them and configures their board, so this section lives on the admin page.
 */
export const TrackersPanel: React.FC = () => {
  const { t, addToast, projects, fetchProjects, fetchTrackers } = useApp()
  const labels = t.admin.trackers
  const [trackers, setTrackers] = useState<Tracker[]>([])
  const [loadError, setLoadError] = useState('')
  const [editing, setEditing] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [draft, setDraft] = useState<TrackerDraft>(EMPTY_TRACKER_DRAFT)
  const [createError, setCreateError] = useState('')
  const [rowErrors, setRowErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      setTrackers(await fetchAdminTrackers())
      setLoadError('')
    } catch (err) {
      setLoadError(errorText(err))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const replace = (next: Tracker) => {
    setTrackers(list => list.map(item => (item.id === next.id ? next : item)))
  }

  const setRowError = (id: string, text: string) => setRowErrors(errors => ({ ...errors, [id]: text }))

  const create = async () => {
    const problem = trackerDraftProblem(draft)
    if (problem) {
      setCreateError(labels.problems[problem])
      return
    }
    setBusy(true)
    setCreateError('')
    try {
      const created = await createTracker(trackerPayload(draft))
      setTrackers(list => [...list, created])
      setDraft(EMPTY_TRACKER_DRAFT)
      setCreating(false)
      setEditing(created.id)
      void fetchTrackers()
      addToast({ type: 'success', title: format(labels.created, { name: created.name }) })
    } catch (err) {
      setCreateError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const remove = async (tracker: Tracker) => {
    if (!window.confirm(format(labels.confirmDelete, { name: tracker.name }))) return
    setRowError(tracker.id, '')
    try {
      await deleteTracker(tracker.id)
      setTrackers(list => list.filter(item => item.id !== tracker.id))
      if (editing === tracker.id) setEditing(null)
      void fetchProjects()
      void fetchTrackers()
      addToast({ type: 'success', title: format(labels.deleted, { name: tracker.name }) })
    } catch (err) {
      // A tracker a project selects, or one tickets belong to, is refused:
      // the reason reads beside the tracker rather than in a passing toast.
      setRowError(tracker.id, format(labels.deleteRefused, { error: errorText(err) }))
    }
  }

  const sync = async (tracker: Tracker) => {
    setRowError(tracker.id, '')
    try {
      await syncTracker(tracker.id)
      addToast({ type: 'success', title: format(labels.syncQueued, { name: tracker.name }) })
    } catch (err) {
      setRowError(tracker.id, errorText(err))
    }
  }

  return (
    <section
      className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4"
      aria-labelledby="admin-trackers-title"
      data-admin-trackers
    >
      <div className="flex items-center gap-2">
        <h3 id="admin-trackers-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
          <Database size={14} /> {labels.title}
        </h3>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => { setCreating(open => !open); setCreateError('') }}
            className={`${buttonClass} text-[11px]`}
          >
            <Plus size={12} />
            <span>{labels.newTracker}</span>
          </button>
          <button type="button" onClick={() => void load()} className={`${buttonClass} text-[11px]`}>
            <RefreshCw size={12} />
            <span>{t.admin.refresh}</span>
          </button>
        </div>
      </div>
      <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">{labels.intro}</p>
      {loadError && <p className="text-[11px] text-amber-400">{labels.loadFailed} ({loadError})</p>}

      {creating && (
        <div className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-primary)] p-3" data-tracker-create>
          <SourceFields draft={draft} onChange={setDraft} idPrefix="tracker-new" />
          {createError && <p role="alert" className="text-[11px] text-red-400">{createError}</p>}
          <div className="flex items-center justify-end gap-2">
            <button type="button" onClick={() => { setCreating(false); setDraft(EMPTY_TRACKER_DRAFT); setCreateError('') }} className={buttonClass}>
              <X size={12} />
              <span>{labels.cancel}</span>
            </button>
            <button type="button" onClick={() => void create()} disabled={busy} className={`${buttonClass} accent-bg text-white border-transparent`}>
              <Plus size={12} />
              <span>{labels.create}</span>
            </button>
          </div>
        </div>
      )}

      {trackers.length === 0 && !loadError && <p className="text-[11px] text-[var(--text-muted)]">{labels.empty}</p>}

      <ul className="space-y-2">
        {trackers.map(tracker => {
          const linked = trackerProjects(tracker.id, projects)
          const open = editing === tracker.id
          return (
            <li
              key={tracker.id}
              className="space-y-2 rounded-xl border border-[var(--border-color)] bg-[var(--bg-primary)] p-3"
              data-tracker={tracker.id}
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)]">
                  {TRACKER_PROVIDER_NAMES[tracker.provider]}
                </span>
                <span className="font-bold text-[var(--text-primary)]">{trackerDisplayName(tracker)}</span>
                {tracker.site && <span className="text-[10px] font-mono text-[var(--text-muted)]">{tracker.site}</span>}
                <div className="ml-auto flex items-center gap-1.5">
                  <button type="button" onClick={() => void sync(tracker)} className={buttonClass} title={labels.sync}>
                    <Zap size={12} />
                    <span>{labels.sync}</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setEditing(open ? null : tracker.id)}
                    aria-expanded={open}
                    className={buttonClass}
                  >
                    <Settings2 size={12} />
                    <span>{labels.edit}</span>
                  </button>
                  <button type="button" onClick={() => void remove(tracker)} className={`${buttonClass} text-red-400`}>
                    <Trash2 size={12} />
                    <span>{labels.delete}</span>
                  </button>
                </div>
              </div>
              <p className="text-[11px] text-[var(--text-muted)]" data-tracker-projects>
                {linked.length > 0
                  ? format(labels.usedBy, { projects: linked.map(project => project.name).join(', ') })
                  : labels.unused}
              </p>
              {rowErrors[tracker.id] && <p role="alert" className="text-[11px] text-red-400">{rowErrors[tracker.id]}</p>}
              {open && (
                <TrackerEditor
                  key={tracker.id}
                  tracker={tracker}
                  onSaved={replace}
                  onClose={() => setEditing(null)}
                />
              )}
            </li>
          )
        })}
      </ul>
    </section>
  )
}
