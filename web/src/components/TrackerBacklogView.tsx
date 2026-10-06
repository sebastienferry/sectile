import React, { useCallback, useEffect, useState } from 'react'
import { Inbox, RefreshCw } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { addBacklogTaskToProject, fetchTrackerBacklog, labellingProjects, trackerDisplayName } from '../lib/trackers'
import type { Task } from '../types'

/**
 * A tracker's tickets that no project shows (#741): those carrying none of
 * the labels of the projects selecting the tracker. "Ajouter au projet…" gives
 * a ticket a project's label, locally at once and on the tracker through the
 * person's own credential, and the ticket joins the project.
 */
export const TrackerBacklogView: React.FC = () => {
  const { backlogTrackerId, trackers, projects, addToast, t } = useApp()
  const strings = t.trackerBacklog
  const tracker = trackers.find(item => item.id === backlogTrackerId)
  const trackerName = tracker ? trackerDisplayName(tracker) : strings.unknownTracker
  const targets = backlogTrackerId ? labellingProjects(backlogTrackerId, projects) : []
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!backlogTrackerId) return
    setLoading(true)
    try {
      setTasks(await fetchTrackerBacklog(backlogTrackerId))
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [backlogTrackerId])

  useEffect(() => { void load() }, [load])

  const addToProject = async (task: Task, projectId: string) => {
    if (!backlogTrackerId || !projectId) return
    const project = projects.find(item => item.id === projectId)
    setBusy(task.id)
    try {
      await addBacklogTaskToProject(backlogTrackerId, task.id, projectId)
      // The ticket carries the project's label now: it left the backlog.
      setTasks(list => list.filter(item => item.id !== task.id))
      addToast({ type: 'success', title: format(strings.added, { key: task.key, project: project?.name || projectId }) })
    } catch (err) {
      addToast({
        type: 'error',
        title: format(strings.addFailed, { key: task.key }),
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="flex flex-col h-full overflow-hidden" data-tracker-backlog={backlogTrackerId || ''}>
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5 border-b border-[var(--border-color)] shrink-0">
        <div className="w-7 h-7 rounded-xl bg-[var(--accent-light)] accent-text flex items-center justify-center">
          <Inbox size={14} />
        </div>
        <div className="min-w-0">
          <h2 className="text-sm font-bold text-[var(--text-primary)]">{format(strings.title, { tracker: trackerName })}</h2>
          <p className="text-[11px] text-[var(--text-muted)]">{strings.subtitle}</p>
        </div>
        <button
          type="button"
          onClick={() => void load()}
          className="ml-auto flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
        >
          <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
          <span>{strings.refresh}</span>
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-3 text-xs">
        {targets.length === 0 && <p className="text-[11px] text-amber-400">{strings.noLabelledProject}</p>}
        {error && <p className="text-[11px] text-amber-400">{strings.loadFailed} ({error})</p>}
        {loading && tasks.length === 0 && <p className="text-[var(--text-muted)]">{strings.loading}</p>}
        {!loading && !error && tasks.length === 0 && <p className="text-[var(--text-muted)]">{strings.empty}</p>}
        <ul className="divide-y divide-[var(--border-color)] rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)]">
          {tasks.map(task => (
            <li key={task.id} className="flex flex-wrap items-center gap-3 px-3 py-2" data-backlog-task={task.id}>
              <span className="font-mono text-[11px] text-[var(--text-muted)] shrink-0">{task.key}</span>
              <span className="flex-1 min-w-0 truncate font-medium text-[var(--text-primary)]">{task.title}</span>
              {task.labels.length > 0 && (
                <span className="flex flex-wrap gap-1">
                  {task.labels.map(label => (
                    <span key={label} className="px-1.5 py-px rounded text-[9px] font-mono bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)]">
                      {label}
                    </span>
                  ))}
                </span>
              )}
              {targets.length > 0 && (
                <select
                  value=""
                  disabled={busy === task.id}
                  aria-label={`${strings.addToProject} ${task.key}`}
                  onChange={e => void addToProject(task, e.target.value)}
                  className="px-2 py-1 text-[11px] rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
                >
                  <option value="">{strings.addToProject}</option>
                  {targets.map(project => (
                    <option key={project.id} value={project.id}>
                      {project.name} ({project.label})
                    </option>
                  ))}
                </select>
              )}
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
