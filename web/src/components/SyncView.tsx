import React, { useCallback, useEffect, useState } from 'react'
import {
  RefreshCw,
  FolderGit2,
  CheckCircle2,
  Clock,
  ExternalLink,
  Zap,
  Terminal,
  Activity as ActivityIcon,
  ChevronRight,
  Sliders,
  AlertCircle,
  Folder,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { AutoSyncState, IssueTracker, TaskActivity, TrackerAutoSyncState } from '../types'
import { TRACKER_PROVIDER_NAMES, syncTracker } from '../lib/trackers'
import { format, formatDateTime, isLocale, plural } from '../lib/i18n'
import { localizeActivityText } from '../lib/activityText'

export const SyncView: React.FC = () => {
  const {
    currentProject,
    setIsProjectModalOpen,
    setEditingProject,
    tasks,
    activities,
    setSelectedActivity,
    setActiveView,
    settings,
    addToast,
    syncCurrentProject,
    isSyncing,
    activeJobCount,
    t,
  } = useApp()
  const locale = isLocale(settings.language) ? settings.language : 'fr'
  const op = t.operations.sync

  // Active project issue tracker
  const activeTracker: IssueTracker = currentProject?.issueTracker || 'local'

  // The pacing of the project's trackers (#741), read for this project: the
  // loop runs per tracker, whatever projects select it.
  const [trackerStates, setTrackerStates] = useState<TrackerAutoSyncState[]>([])
  const [syncingTrackers, setSyncingTrackers] = useState<string[]>([])
  const projectId = currentProject?.id || ''

  const loadTrackerStates = useCallback(async () => {
    if (!projectId) {
      setTrackerStates([])
      return
    }
    try {
      const res = await fetch(`/api/sync/auto?projectId=${encodeURIComponent(projectId)}`)
      if (!res.ok) return
      const state: AutoSyncState = await res.json()
      setTrackerStates(state.trackers || [])
    } catch {
      // Unreachable server: the list keeps what it showed.
    }
  }, [projectId])

  useEffect(() => {
    void loadTrackerStates()
  }, [loadTrackerStates])

  const remoteTrackers = trackerStates.filter(state => state.provider !== 'local')

  const handleSyncTracker = async (trackerId: string, name: string) => {
    setSyncingTrackers(list => [...list, trackerId])
    try {
      await syncTracker(trackerId)
      addToast({ type: 'success', title: format(op.trackerSyncQueued, { name }) })
      await loadTrackerStates()
    } catch (err) {
      addToast({ type: 'error', title: format(op.trackerSyncFailed, { name }), description: err instanceof Error ? err.message : String(err) })
    } finally {
      setSyncingTrackers(list => list.filter(id => id !== trackerId))
    }
  }

  // Filter activities that are sync related for this project
  const syncActivities = activities.filter(
    a => (a.skillId === 'sync_github' || a.skillId === 'sync_jira' || a.skillId === 'sync_all' || a.skillId.startsWith('sync')) &&
         (!currentProject || (!a.projectId && !a.trackerId) || a.projectId === currentProject.id ||
          (a.trackerId !== undefined && (currentProject.trackers || []).some(ref => ref.trackerId === a.trackerId)))
  )

  const githubCount = tasks.filter(t => t.source === 'github').length
  const jiraCount = tasks.filter(t => t.source === 'jira').length
  const gitlabCount = tasks.filter(t => t.source === 'gitlab').length
  const localCount = tasks.filter(t => !t.source || t.source === 'local').length

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'running':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
            <RefreshCw size={11} className="animate-spin" /> {op.status.running}
          </span>
        )
      case 'queued':
      case 'pending':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30">
            <Clock size={11} /> {op.status.queued}
          </span>
        )
      case 'completed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
            <CheckCircle2 size={11} /> {op.status.completed}
          </span>
        )
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30">
            <AlertCircle size={11} /> {op.status.failed}
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-[var(--bg-tertiary)] text-[var(--text-muted)]">
            {status}
          </span>
        )
    }
  }

  const handleInspectActivity = (act: TaskActivity) => {
    setSelectedActivity(act)
    setActiveView('activities')
  }

  return (
    <div className="flex-1 flex flex-col h-full bg-[var(--bg-primary)] overflow-y-auto">
      {/* Header */}
      <div className="border-b border-[var(--border-color)] bg-[var(--bg-secondary)] px-6 py-5 shrink-0">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-[var(--accent-light)] text-[var(--accent-color)] border border-[var(--accent-color)]/30">
                <RefreshCw size={20} className={isSyncing ? 'animate-spin' : ''} />
              </div>
              <div>
                <h1 className="text-xl font-bold text-[var(--text-primary)] flex items-center gap-2">
                  {t.syncView.title}
                  <span className="text-xs px-2 py-0.5 rounded-full font-mono font-medium bg-[var(--accent-light)] accent-text border border-[var(--accent-color)]/30">
                    {currentProject?.name || op.activeProject}
                  </span>
                </h1>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {format(op.subtitle, { tracker: activeTracker.toUpperCase() })}
                </p>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {activeJobCount > 0 && (
              <button
                onClick={() => setActiveView('activities')}
                className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30 hover:bg-blue-500/25 transition-all animate-pulse"
              >
                <ActivityIcon size={14} className="animate-spin" />
                <span>{plural(locale, activeJobCount, op.activeJobs)}</span>
                <ChevronRight size={13} />
              </button>
            )}

            <button
              onClick={syncCurrentProject}
              disabled={isSyncing}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-bold text-white shadow-md accent-bg hover:opacity-90 transition-all disabled:opacity-50 cursor-pointer"
              title={format(op.syncProject, { name: currentProject?.name || '' })}
            >
              <Zap size={14} className={isSyncing ? 'animate-spin' : ''} />
              <span>
                {activeTracker === 'github'
                  ? op.syncGithub
                  : activeTracker === 'jira'
                  ? op.syncJira
                  : activeTracker === 'gitlab'
                  ? op.syncGitlab
                  : op.reloadTasks}
              </span>
            </button>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 max-w-5xl w-full mx-auto p-6 space-y-6">
        {/* Active Project Scope Banner */}
        {currentProject && (
          <div className="rounded-xl border border-[var(--sidebar-border)] bg-[var(--accent-light)]/20 p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shadow-xs">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-[var(--accent-light)] border border-[var(--accent-color)]/30 flex items-center justify-center accent-text font-bold text-base shadow-[0_0_12px_var(--accent-glow)] shrink-0">
                ⚡
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold uppercase tracking-wider text-[var(--accent-color)]">{op.activeProject}</span>
                  <span className="text-[10px] px-2 py-0.2 rounded-full font-mono bg-[var(--bg-tertiary)] text-[var(--text-secondary)]">
                    {plural(locale, tasks.length, op.taskCount)}
                  </span>
                  <span className={`text-[10px] px-2 py-0.2 rounded-full font-bold uppercase ${
                    activeTracker === 'github'
                      ? 'bg-purple-500/20 text-purple-300 border border-purple-500/30'
                      : activeTracker === 'jira'
                      ? 'bg-blue-500/20 text-blue-300 border border-blue-500/30'
                      : activeTracker === 'gitlab'
                      ? 'bg-orange-500/20 text-orange-300 border border-orange-500/30'
                      : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                  }`}>
                    {activeTracker === 'github' ? 'GitHub Issues' : activeTracker === 'jira' ? 'Jira' : activeTracker === 'gitlab' ? 'GitLab' : 'Local SQLite'}
                  </span>
                </div>
                <h3 className="text-sm font-bold text-[var(--text-primary)]">
                  {currentProject.name}
                </h3>
                <p className="text-xs text-[var(--text-muted)] font-mono truncate max-w-lg">
                  {[
                    currentProject.githubRepo ? `GitHub: ${currentProject.githubRepo}` : '',
                    currentProject.jiraProject ? `Jira: ${currentProject.jiraProject}` : '',
                  ].filter(Boolean).join(' · ')}
                </p>
              </div>
            </div>

            <button
              type="button"
              onClick={() => {
                setEditingProject(currentProject)
                setIsProjectModalOpen(true)
              }}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-[var(--bg-secondary)] hover:bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border-color)] transition-all cursor-pointer shrink-0"
            >
              {op.editProject}
            </button>
          </div>
        )}

        {/* Current Project Synchronization Card ONLY */}
        {activeTracker === 'github' && (
          <div className="rounded-xl border border-purple-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-purple-500/15 border border-purple-500/30 flex items-center justify-center text-purple-400">
                  <FolderGit2 size={20} />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    {op.githubTitle}
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    {format(op.githubConnected, { repo: currentProject?.githubRepo || op.notConfigured })}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-purple-500/15 text-purple-300 font-bold border border-purple-500/30">
                {plural(locale, githubCount, op.githubCount)}
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              {op.githubDescription}
            </p>
          </div>
        )}

        {activeTracker === 'jira' && (
          <div className="rounded-xl border border-blue-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-blue-500/15 border border-blue-500/30 flex items-center justify-center text-blue-400 font-black font-sans text-lg">
                  J
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    {op.jiraTitle}
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    {format(op.jiraConnected, { key: currentProject?.jiraProject || op.notDefined })}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-blue-500/15 text-blue-300 font-bold border border-blue-500/30">
                {plural(locale, jiraCount, op.jiraCount)}
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              {op.jiraDescription}
            </p>
          </div>
        )}

        {activeTracker === 'gitlab' && (
          <div className="rounded-xl border border-orange-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-orange-500/15 border border-orange-500/30 flex items-center justify-center text-orange-400">
                  <FolderGit2 size={20} />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    {op.gitlabTitle}
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    {format(op.gitlabConnected, { path: currentProject?.gitlabProject || settings.gitlabProject || op.notConfigured })}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-orange-500/15 text-orange-300 font-bold border border-orange-500/30">
                {plural(locale, gitlabCount, op.gitlabCount)}
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              {op.gitlabDescription}
            </p>
          </div>
        )}

        {activeTracker === 'local' && (
          <div className="rounded-xl border border-emerald-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
                  <Folder size={20} />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    {op.localTitle}
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
                    {op.localActive}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-emerald-500/15 text-emerald-300 font-bold border border-emerald-500/30">
                {plural(locale, localCount, op.localCount)}
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              {op.localDescription}
            </p>
          </div>
        )}

        {/* The project's trackers (#741): each is synchronised in full, on
            its own pace, whatever projects select it. */}
        <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-6 shadow-xs" data-sync-trackers>
          <div className="flex items-center justify-between pb-4 border-b border-[var(--border-color)] mb-4">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-[var(--bg-tertiary)] text-[var(--text-secondary)]">
                <Sliders size={18} />
              </div>
              <div>
                <h3 className="text-sm font-bold text-[var(--text-primary)]">
                  {op.trackersTitle}
                </h3>
                <p className="text-xs text-[var(--text-muted)]">
                  {op.trackersSubtitle}
                </p>
              </div>
            </div>
          </div>

          {remoteTrackers.length === 0 ? (
            <p className="text-xs text-[var(--text-muted)]">{op.trackersEmpty}</p>
          ) : (
            <ul className="divide-y divide-[var(--border-color)]">
              {remoteTrackers.map(state => {
                const when = (iso?: string) => (iso ? formatDateTime(locale, iso, { hour: '2-digit', minute: '2-digit', day: '2-digit', month: 'short' }) : op.never)
                return (
                  <li key={state.trackerId} className="py-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3" data-sync-tracker={state.trackerId}>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)]">
                          {TRACKER_PROVIDER_NAMES[state.provider as keyof typeof TRACKER_PROVIDER_NAMES] || state.provider}
                        </span>
                        <span className="text-xs font-bold text-[var(--text-primary)] truncate">{state.name}</span>
                        <span className="text-[10px] text-[var(--text-muted)]">
                          {state.enabled ? format(op.autoSyncEvery, { minutes: state.intervalMin }) : op.autoSyncOff}
                        </span>
                      </div>
                      <p className="text-[11px] text-[var(--text-muted)] mt-0.5">
                        {format(op.lastPass, { when: when(state.lastPassAt) })}
                        {' · '}
                        {format(op.lastFullSync, { when: when(state.lastFullSyncAt) })}
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => void handleSyncTracker(state.trackerId, state.name)}
                      disabled={syncingTrackers.includes(state.trackerId)}
                      title={format(op.syncTrackerTitle, { name: state.name })}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-[var(--bg-tertiary)] hover:bg-[var(--bg-primary)] text-[var(--text-primary)] border border-[var(--border-color)] transition-all cursor-pointer disabled:opacity-50 shrink-0"
                    >
                      <Zap size={13} className={syncingTrackers.includes(state.trackerId) ? 'animate-spin' : ''} />
                      <span>{op.syncTracker}</span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        {/* Recent Sync Activities History for the active project */}
        <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-6 shadow-xs">
          <div className="flex items-center justify-between pb-4 border-b border-[var(--border-color)] mb-4">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-indigo-500/15 text-indigo-400">
                <Terminal size={18} />
              </div>
              <div>
                <h3 className="text-sm font-bold text-[var(--text-primary)]">
                  {op.historyTitle}
                </h3>
                <p className="text-xs text-[var(--text-muted)]">
                  {op.historySubtitle}
                </p>
              </div>
            </div>

            <button
              onClick={() => setActiveView('activities')}
              className="text-xs text-[var(--accent-color)] hover:underline flex items-center gap-1 font-medium cursor-pointer"
            >
              <span>{op.openActivities}</span>
              <ChevronRight size={13} />
            </button>
          </div>

          {syncActivities.length === 0 ? (
            <div className="text-center py-8 text-[var(--text-muted)]">
              <RefreshCw size={24} className="mx-auto mb-2 opacity-40" />
              <p className="text-xs">{op.historyEmpty}</p>
            </div>
          ) : (
            <div className="divide-y divide-[var(--border-color)]">
              {syncActivities.slice(0, 8).map(act => (
                <div
                  key={act.id}
                  onClick={() => handleInspectActivity(act)}
                  className="py-3.5 px-2 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:bg-[var(--bg-tertiary)]/50 rounded-lg cursor-pointer transition-colors"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-[var(--bg-tertiary)] border border-[var(--border-color)] flex items-center justify-center shrink-0">
                      {act.skillId === 'sync_github' ? (
                        <FolderGit2 size={14} className="text-purple-400" />
                      ) : act.skillId === 'sync_jira' ? (
                        <span className="text-blue-400 font-bold text-xs">J</span>
                      ) : (
                        <RefreshCw size={14} className="text-cyan-400" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-[var(--text-primary)] truncate">
                          {localizeActivityText(act.skillName, locale)}
                        </span>
                        {getStatusBadge(act.status)}
                      </div>
                      <p className="text-xs text-[var(--text-secondary)] truncate mt-0.5">
                        {localizeActivityText(act.summary || act.action, locale)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 shrink-0 text-xs text-[var(--text-muted)] sm:self-center">
                    {act.duration && (
                      <span className="font-mono text-[11px] bg-[var(--bg-tertiary)] px-2 py-0.5 rounded">
                        {act.duration}
                      </span>
                    )}
                    <span className="text-[11px]">
                      {formatDateTime(locale, act.createdAt, {
                        hour: '2-digit',
                        minute: '2-digit',
                        day: '2-digit',
                        month: 'short',
                      })}
                    </span>
                    <button
                      onClick={e => {
                        e.stopPropagation()
                        handleInspectActivity(act)
                      }}
                      className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)]"
                      title={op.inspect}
                    >
                      <ExternalLink size={13} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
