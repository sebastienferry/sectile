import React, { useState, useMemo } from 'react'
import {
  Activity,
  RotateCcw,
  Trash2,
  XCircle,
  CheckCircle2,
  Clock,
  AlertTriangle,
  Loader2,
  Search,
  ExternalLink,
  Copy,
  Check,
  Terminal,
  FileText,
  HelpCircle,
  FileCode,
  Flame,
  ShieldCheck,
  Sparkles,
  RefreshCw,
  ChevronRight,
  Bot,
  Hand,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { MarkdownView } from './Markdown'
import type { ActivityStatus } from '../types'

export const ActivitiesView: React.FC = () => {
  const {
    currentProject,
    activities,
    activityStats,
    selectedActivity,
    setSelectedActivity,
    fetchActivities,
    fetchActivityStats,
    retryActivity,
    cancelActivity,
    deleteActivity,
    clearCompletedActivities,
    setSelectedTask,
    tasks,
    skills,
    t,
  } = useApp()

  // 'waiting' is not an activity status: a blocked run is still running. It is
  // a filter of its own because it answers the question the status cannot.
  const [statusFilter, setStatusFilter] = useState<'all' | ActivityStatus | 'waiting'>('all')
  const [skillFilter, setSkillFilter] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState<string>('')
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [copiedOutput, setCopiedOutput] = useState(false)
  const [outputViewMode, setOutputViewMode] = useState<'rendered' | 'raw'>('rendered')

  const handleRefresh = async () => {
    setIsRefreshing(true)
    await Promise.all([fetchActivities(), fetchActivityStats()])
    setTimeout(() => setIsRefreshing(false), 400)
  }

  const handleCopyOutput = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedOutput(true)
    setTimeout(() => setCopiedOutput(false), 2000)
  }

  const handleOpenTask = (taskId: string) => {
    const found = tasks.find(t => t.id === taskId)
    if (found) {
      setSelectedTask(found)
    }
  }

  const getSkillIcon = (skillId: string, size = 16) => {
    switch (skillId) {
      case 'clarify':
        return <HelpCircle size={size} className="text-amber-400" />
      case 'specify':
        return <FileCode size={size} className="text-blue-400" />
      case 'implement':
        return <Flame size={size} className="text-indigo-400" />
      case 'adjust':
      case 'create_pr':
      case 'review':
        return <ShieldCheck size={size} className="text-purple-400" />
      case 'pick':
        return <Sparkles size={size} className="text-emerald-400" />
      default:
        return <Bot size={size} className="text-slate-400" />
    }
  }

  const getStatusBadge = (status: ActivityStatus) => {
    switch (status) {
      case 'running':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30 animate-pulse">
            <Loader2 size={12} className="animate-spin" />
            <span>{t.activities.stats.running}</span>
          </span>
        )
      case 'queued':
      case 'pending':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
            <Clock size={12} />
            <span>{t.activities.stats.queued}</span>
          </span>
        )
      case 'completed':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
            <CheckCircle2 size={12} />
            <span>{t.activities.stats.completed}</span>
          </span>
        )
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30">
            <AlertTriangle size={12} />
            <span>{t.activities.stats.failed}</span>
          </span>
        )
      case 'canceled':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-slate-500/15 text-slate-400 border border-slate-500/30">
            <XCircle size={12} />
            <span>{t.activities.stats.canceled}</span>
          </span>
        )
      default:
        return null
    }
  }

  const filteredActivities = useMemo(() => {
    return activities.filter(act => {
      if (statusFilter === 'waiting') {
        if (act.status !== 'running' || !act.waitingSince) return false
      } else if (statusFilter !== 'all') {
        if (statusFilter === 'queued' || statusFilter === 'pending') {
          if (act.status !== 'queued' && act.status !== 'pending') return false
        } else if (act.status !== statusFilter) {
          return false
        }
      }

      if (skillFilter !== 'all' && act.skillId !== skillFilter) {
        return false
      }

      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchKey = (act.taskKey || '').toLowerCase().includes(q)
        const matchTitle = (act.taskTitle || '').toLowerCase().includes(q)
        const matchSkill = (act.skillName || '').toLowerCase().includes(q)
        const matchSummary = (act.summary || '').toLowerCase().includes(q)
        const matchAction = (act.action || '').toLowerCase().includes(q)
        const matchOutput = (act.output || '').toLowerCase().includes(q)
        if (!matchKey && !matchTitle && !matchSkill && !matchSummary && !matchAction && !matchOutput) {
          return false
        }
      }

      return true
    })
  }, [activities, statusFilter, skillFilter, searchQuery])

  const formatDate = (isoString?: string) => {
    if (!isoString) return ''
    try {
      const d = new Date(isoString)
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    } catch {
      return isoString
    }
  }

  const formatFullDate = (isoString?: string) => {
    if (!isoString) return ''
    try {
      const d = new Date(isoString)
      return d.toLocaleString([], {
        day: '2-digit',
        month: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    } catch {
      return isoString
    }
  }

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-[var(--bg-primary)]">
      {/* Top Banner / Metrics Header */}
      <div className="p-4 sm:p-6 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg flex items-center justify-center font-bold text-white shadow-md accent-bg shrink-0">
                <Activity size={18} />
              </div>
              <h1 className="text-lg sm:text-xl font-bold tracking-tight text-[var(--text-primary)] flex items-center gap-2">
                <span>{t.activities.title}</span>
                {currentProject && (
                  <span className="text-xs px-2.5 py-0.5 rounded-full font-bold bg-[var(--accent-light)] accent-text border border-[var(--accent-color)]/30">
                    {currentProject.name}
                  </span>
                )}
              </h1>
              {activityStats.running > 0 && (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-500/20 text-blue-400 border border-blue-500/40 animate-pulse font-mono">
                  <span className="w-2 h-2 rounded-full bg-blue-400 animate-ping" />
                  {activityStats.running} en cours
                </span>
              )}
            </div>
            <p className="text-xs text-[var(--text-muted)] mt-1 max-w-2xl">
              {t.activities.subtitle}
            </p>
          </div>

          {/* Action Buttons */}
          <div className="flex items-center gap-2 self-start md:self-auto">
            <button
              onClick={handleRefresh}
              disabled={isRefreshing}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-[var(--bg-tertiary)] hover:bg-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-color)] transition-all disabled:opacity-50"
              title={t.activities.actions.refresh}
            >
              <RefreshCw size={13} className={isRefreshing ? 'animate-spin text-amber-400' : ''} />
              <span>{t.activities.actions.refresh}</span>
            </button>

            {activityStats.completed + activityStats.failed + activityStats.canceled > 0 && (
              <button
                onClick={() => {
                  if (window.confirm(t.activities.actions.clearConfirm)) {
                    clearCompletedActivities()
                  }
                }}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 transition-all"
                title={t.activities.actions.clearCompleted}
              >
                <Trash2 size={13} />
                <span>{t.activities.actions.clearCompleted}</span>
              </button>
            )}
          </div>
        </div>

        {/* Metric Cards Bar */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2.5 mt-4">
          <div
            onClick={() => setStatusFilter('all')}
            className={`cursor-pointer p-3 rounded-xl border transition-all ${
              statusFilter === 'all'
                ? 'bg-[var(--accent-light)] border-[var(--accent-color)] shadow-xs'
                : 'bg-[var(--bg-tertiary)]/40 border-[var(--border-color)] hover:bg-[var(--bg-tertiary)]'
            }`}
          >
            <div className="flex items-center justify-between text-xs text-[var(--text-muted)] font-medium">
              <span>{t.activities.stats.total}</span>
              <Activity size={14} className="text-slate-400" />
            </div>
            <div className="text-xl font-bold text-[var(--text-primary)] mt-1 font-mono">
              {activityStats.total}
            </div>
          </div>

          <div
            onClick={() => setStatusFilter('running')}
            className={`cursor-pointer p-3 rounded-xl border transition-all ${
              statusFilter === 'running'
                ? 'bg-blue-500/20 border-blue-500 shadow-xs'
                : activityStats.running > 0
                ? 'bg-blue-500/10 border-blue-500/30 hover:bg-blue-500/15'
                : 'bg-[var(--bg-tertiary)]/40 border-[var(--border-color)] hover:bg-[var(--bg-tertiary)]'
            }`}
          >
            <div className="flex items-center justify-between text-xs font-medium text-blue-400">
              <span className="flex items-center gap-1.5">
                {activityStats.running > 0 && <span className="w-2 h-2 rounded-full bg-blue-400 animate-ping" />}
                {t.activities.stats.running}
              </span>
              <Loader2 size={14} className={activityStats.running > 0 ? 'animate-spin' : ''} />
            </div>
            <div className="text-xl font-bold text-blue-400 mt-1 font-mono">
              {activityStats.running}
            </div>
          </div>

          <div
            onClick={() => setStatusFilter('queued')}
            className={`cursor-pointer p-3 rounded-xl border transition-all ${
              statusFilter === 'queued'
                ? 'bg-amber-500/20 border-amber-500 shadow-xs'
                : 'bg-[var(--bg-tertiary)]/40 border-[var(--border-color)] hover:bg-[var(--bg-tertiary)]'
            }`}
          >
            <div className="flex items-center justify-between text-xs text-amber-400 font-medium">
              <span>{t.activities.stats.queued}</span>
              <Clock size={14} />
            </div>
            <div className="text-xl font-bold text-amber-400 mt-1 font-mono">
              {activityStats.queued}
            </div>
          </div>

          <div
            onClick={() => setStatusFilter('completed')}
            className={`cursor-pointer p-3 rounded-xl border transition-all ${
              statusFilter === 'completed'
                ? 'bg-emerald-500/20 border-emerald-500 shadow-xs'
                : 'bg-[var(--bg-tertiary)]/40 border-[var(--border-color)] hover:bg-[var(--bg-tertiary)]'
            }`}
          >
            <div className="flex items-center justify-between text-xs text-emerald-400 font-medium">
              <span>{t.activities.stats.completed}</span>
              <CheckCircle2 size={14} />
            </div>
            <div className="text-xl font-bold text-emerald-400 mt-1 font-mono">
              {activityStats.completed}
            </div>
          </div>

          <div
            onClick={() => setStatusFilter('failed')}
            className={`cursor-pointer p-3 rounded-xl border transition-all ${
              statusFilter === 'failed'
                ? 'bg-rose-500/20 border-rose-500 shadow-xs'
                : 'bg-[var(--bg-tertiary)]/40 border-[var(--border-color)] hover:bg-[var(--bg-tertiary)]'
            }`}
          >
            <div className="flex items-center justify-between text-xs text-rose-400 font-medium">
              <span>{t.activities.stats.failed}</span>
              <AlertTriangle size={14} />
            </div>
            <div className="text-xl font-bold text-rose-400 mt-1 font-mono">
              {activityStats.failed}
            </div>
          </div>
        </div>
      </div>

      {/* Control Bar: Filters & Search */}
      <div className="px-4 sm:px-6 py-3 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] flex flex-wrap items-center justify-between gap-3 shrink-0">
        <div className="flex items-center gap-2 flex-wrap">
          {/* Status Tabs */}
          <div className="flex items-center bg-[var(--bg-tertiary)] p-0.5 rounded-lg border border-[var(--border-color)] text-xs">
            <button
              onClick={() => setStatusFilter('all')}
              className={`px-2.5 py-1 rounded-md font-medium transition-all ${
                statusFilter === 'all'
                  ? 'bg-[var(--bg-secondary)] text-[var(--text-primary)] shadow-xs font-semibold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {t.activities.filters.all}
            </button>
            <button
              onClick={() => setStatusFilter('running')}
              className={`px-2.5 py-1 rounded-md font-medium transition-all ${
                statusFilter === 'running'
                  ? 'bg-[var(--bg-secondary)] text-blue-400 shadow-xs font-semibold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {t.activities.filters.running}
            </button>
            <button
              onClick={() => setStatusFilter('waiting')}
              className={`px-2.5 py-1 rounded-md font-medium transition-all ${
                statusFilter === 'waiting'
                  ? 'bg-[var(--bg-secondary)] text-amber-400 shadow-xs font-semibold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {t.activities.filters.waiting}
            </button>
            <button
              onClick={() => setStatusFilter('queued')}
              className={`px-2.5 py-1 rounded-md font-medium transition-all ${
                statusFilter === 'queued'
                  ? 'bg-[var(--bg-secondary)] text-amber-400 shadow-xs font-semibold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {t.activities.filters.queued}
            </button>
            <button
              onClick={() => setStatusFilter('completed')}
              className={`px-2.5 py-1 rounded-md font-medium transition-all ${
                statusFilter === 'completed'
                  ? 'bg-[var(--bg-secondary)] text-emerald-400 shadow-xs font-semibold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {t.activities.filters.completed}
            </button>
            <button
              onClick={() => setStatusFilter('failed')}
              className={`px-2.5 py-1 rounded-md font-medium transition-all ${
                statusFilter === 'failed'
                  ? 'bg-[var(--bg-secondary)] text-rose-400 shadow-xs font-semibold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {t.activities.filters.failed}
            </button>
          </div>

          {/* Skill Filter Dropdown */}
          <select
            value={skillFilter}
            onChange={e => setSkillFilter(e.target.value)}
            className="px-2.5 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] focus:outline-none focus:border-[var(--accent-color)]"
          >
            <option value="all">{t.activities.filters.allSkills}</option>
            {skills.map(sk => (
              <option key={sk.id} value={sk.id}>
                {sk.name} ({sk.command})
              </option>
            ))}
          </select>
        </div>

        {/* Search Filter Input */}
        <div className="relative w-full sm:w-64">
          <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none">
            <Search size={13} />
          </div>
          <input
            type="text"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            placeholder={t.activities.filters.searchPlaceholder}
            className="w-full pl-8 pr-3 py-1.5 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)]"
          />
        </div>
      </div>

      {/* Main Content Area (Split View: Activities List + Detailed Inspector) */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left: Activities List */}
        <div
          className={`flex-1 overflow-y-auto p-4 sm:p-6 space-y-3 ${
            selectedActivity ? 'hidden lg:block lg:w-1/2 lg:border-r lg:border-[var(--border-color)]' : 'w-full'
          }`}
        >
          {filteredActivities.length === 0 ? (
            <div className="h-64 flex flex-col items-center justify-center text-center p-6 rounded-2xl border border-dashed border-[var(--border-color)] bg-[var(--bg-secondary)]/30">
              <div className="w-12 h-12 rounded-2xl flex items-center justify-center bg-[var(--bg-tertiary)] text-[var(--text-muted)] mb-3">
                <Activity size={24} />
              </div>
              <h3 className="text-sm font-bold text-[var(--text-primary)]">
                {activities.length === 0 ? t.activities.empty.title : t.activities.empty.noFilterMatch}
              </h3>
              <p className="text-xs text-[var(--text-muted)] max-w-sm mt-1">
                {activities.length === 0 ? t.activities.empty.desc : 'Modifiez vos filtres de recherche pour afficher les activités.'}
              </p>
            </div>
          ) : (
            filteredActivities.map(act => {
              const isSelected = selectedActivity?.id === act.id
              const isRunning = act.status === 'running'
              const isQueued = act.status === 'queued' || act.status === 'pending'

              return (
                <div
                  key={act.id}
                  onClick={() => setSelectedActivity(act)}
                  className={`group relative p-4 rounded-2xl border transition-all cursor-pointer select-none ${
                    isSelected
                      ? 'bg-[var(--accent-light)] border-[var(--accent-color)] shadow-md ring-1 ring-[var(--accent-color)]'
                      : isRunning
                      ? 'bg-blue-500/5 border-blue-500/40 hover:bg-blue-500/10 shadow-xs'
                      : 'bg-[var(--bg-secondary)] border-[var(--border-color)] hover:bg-[var(--bg-tertiary)]/50 hover:border-[var(--text-muted)]/30'
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    {/* Header: Skill Icon + Name + Status */}
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className="p-2 rounded-xl bg-[var(--bg-tertiary)] shrink-0 border border-[var(--border-color)]">
                        {getSkillIcon(act.skillId, 18)}
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-xs text-[var(--text-primary)] truncate">
                            {act.skillName || act.skillId}
                          </span>
                          {getStatusBadge(act.status)}
                          {act.status === 'running' && act.waitingSince && (
                            <span
                              className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30"
                              title={t.activities.filters.waiting}
                            >
                              <Hand size={12} />
                              <span>{t.activities.stats.waiting}</span>
                            </span>
                          )}
                        </div>
                        {/* Task Key & Title Link */}
                        <div className="flex items-center gap-2 mt-0.5 text-xs text-[var(--text-muted)]">
                          {act.taskKey && (
                            <button
                              onClick={e => {
                                e.stopPropagation()
                                handleOpenTask(act.taskId)
                              }}
                              className="font-mono font-bold text-[11px] text-[var(--accent-color)] hover:underline flex items-center gap-0.5"
                            >
                              <span>{act.taskKey}</span>
                              <ExternalLink size={10} />
                            </button>
                          )}
                          {act.taskTitle && (
                            <span className="truncate max-w-xs text-[var(--text-secondary)] font-medium">
                              {act.taskTitle}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Meta: Duration & Date */}
                    <div className="text-right shrink-0 flex flex-col items-end gap-1">
                      <span className="text-[11px] font-mono text-[var(--text-muted)]">
                        {formatDate(act.createdAt)}
                      </span>
                      {act.duration && (
                        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] font-semibold">
                          ⏱ {act.duration}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Summary / Output Snippet */}
                  {act.summary && (
                    <p className="text-xs text-[var(--text-secondary)] mt-2.5 line-clamp-2 bg-[var(--bg-tertiary)]/40 p-2 rounded-xl border border-[var(--border-color)]/60 font-mono text-[11px]">
                      {act.summary}
                    </p>
                  )}

                  {/* Error if failed */}
                  {act.error && (
                    <div className="mt-2 text-xs text-rose-400 font-mono bg-rose-500/10 p-2 rounded-xl border border-rose-500/20 line-clamp-2">
                      ⚠️ {act.error}
                    </div>
                  )}

                  {/* Actions Bar on Card */}
                  <div className="flex items-center justify-between mt-3 pt-2.5 border-t border-[var(--border-color)]/50 text-xs text-[var(--text-muted)]">
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] uppercase font-bold text-[var(--text-muted)]">
                        Action :
                      </span>
                      <span className="text-[11px] text-[var(--text-secondary)] truncate max-w-xs">
                        {act.action}
                      </span>
                    </div>

                    <div className="flex items-center gap-1.5" onClick={e => e.stopPropagation()}>
                      {/* Retry Button */}
                      {(act.status === 'completed' || act.status === 'failed' || act.status === 'canceled') && (
                        <button
                          onClick={() => retryActivity(act.id)}
                          className="p-1 rounded-lg hover:bg-[var(--bg-tertiary)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                          title={t.activities.detail.retry}
                        >
                          <RotateCcw size={13} />
                        </button>
                      )}

                      {/* La console du run : une étape autonome tourne dans la
                          session PTY de sa tâche, on peut la regarder et y
                          répondre pendant qu'elle travaille. */}


                      {/* Cancel Button */}
                      {(isQueued || isRunning) && (
                        <button
                          onClick={() => cancelActivity(act.id)}
                          className="p-1 rounded-lg hover:bg-rose-500/15 text-rose-400 transition-colors"
                          title={t.activities.detail.cancel}
                        >
                          <XCircle size={13} />
                        </button>
                      )}

                      {/* Delete Button */}
                      <button
                        onClick={() => deleteActivity(act.id)}
                        className="p-1 rounded-lg hover:bg-rose-500/15 text-[var(--text-muted)] hover:text-rose-400 transition-colors"
                        title={t.activities.detail.delete}
                      >
                        <Trash2 size={13} />
                      </button>

                      <ChevronRight size={14} className="text-[var(--text-muted)] ml-1" />
                    </div>
                  </div>
                </div>
              )
            })
          )}
        </div>

        {/* Right: Detailed Activity Inspector */}
        {selectedActivity ? (
          <div className="flex-1 flex flex-col h-full overflow-hidden bg-[var(--bg-secondary)] border-l border-[var(--border-color)]">
            {/* Inspector Header */}
            <div className="p-4 sm:p-5 border-b border-[var(--border-color)] flex items-start justify-between gap-4 bg-[var(--bg-tertiary)]/30 shrink-0">
              <div className="flex items-center gap-3 min-w-0">
                <div className="p-2.5 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-xs shrink-0">
                  {getSkillIcon(selectedActivity.skillId, 22)}
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h2 className="text-base font-bold text-[var(--text-primary)] truncate">
                      {selectedActivity.skillName}
                    </h2>
                    {getStatusBadge(selectedActivity.status)}
                  </div>
                  <p className="text-xs text-[var(--text-muted)] font-mono mt-0.5">
                    ID: {selectedActivity.id.slice(0, 8)}... | {formatFullDate(selectedActivity.createdAt)}
                  </p>
                </div>
              </div>

              {/* Header Right Actions */}
              <div className="flex items-center gap-2 shrink-0">
                {(selectedActivity.status === 'completed' || selectedActivity.status === 'failed' || selectedActivity.status === 'canceled') && (
                  <button
                    onClick={() => retryActivity(selectedActivity.id)}
                    className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold bg-[var(--bg-secondary)] hover:bg-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-color)] transition-colors"
                    title={t.activities.detail.retry}
                  >
                    <RotateCcw size={12} />
                    <span className="hidden sm:inline">{t.activities.detail.retry}</span>
                  </button>
                )}

                {(selectedActivity.status === 'queued' || selectedActivity.status === 'running') && (
                  <button
                    onClick={() => cancelActivity(selectedActivity.id)}
                    className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold bg-rose-500/15 hover:bg-rose-500/25 text-rose-400 border border-rose-500/30 transition-colors"
                    title={t.activities.detail.cancel}
                  >
                    <XCircle size={12} />
                    <span>{t.activities.detail.cancel}</span>
                  </button>
                )}

                <button
                  onClick={() => setSelectedActivity(null)}
                  className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-colors"
                  title="Fermer l'inspecteur"
                >
                  <XCircle size={18} />
                </button>
              </div>
            </div>

            {/* Inspector Body (Scrollable) */}
            <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-6">
              {/* Associated Task Info Box */}
              <div className="p-3.5 rounded-2xl bg-[var(--bg-primary)] border border-[var(--border-color)] flex items-center justify-between gap-3">
                <div className="flex items-center gap-3 min-w-0">
                  <div className="w-8 h-8 rounded-lg flex items-center justify-center bg-[var(--bg-tertiary)] text-[var(--accent-color)] shrink-0 font-mono font-bold text-xs border border-[var(--border-color)]">
                    {selectedActivity.taskKey ? selectedActivity.taskKey.slice(0, 3) : 'TSK'}
                  </div>
                  <div className="min-w-0">
                    <div className="text-xs font-bold text-[var(--text-primary)] truncate">
                      {selectedActivity.taskKey ? `[${selectedActivity.taskKey}] ` : ''}
                      {selectedActivity.taskTitle || 'Tâche liée'}
                    </div>
                    <div className="text-[11px] text-[var(--text-muted)] truncate">
                      {selectedActivity.action}
                    </div>
                  </div>
                </div>

                <button
                  onClick={() => handleOpenTask(selectedActivity.taskId)}
                  className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold bg-[var(--accent-light)] accent-text hover:opacity-80 transition-opacity shrink-0"
                >
                  <span>{t.activities.detail.openTask}</span>
                  <ExternalLink size={12} />
                </button>
              </div>

              {/* Execution Timing Metadata */}
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 text-xs">
                <div className="p-2.5 rounded-xl bg-[var(--bg-tertiary)]/50 border border-[var(--border-color)]">
                  <div className="text-[10px] text-[var(--text-muted)] font-bold uppercase">Créée à</div>
                  <div className="font-mono text-[var(--text-primary)] font-semibold mt-0.5">
                    {formatDate(selectedActivity.createdAt) || 'N/A'}
                  </div>
                </div>
                <div className="p-2.5 rounded-xl bg-[var(--bg-tertiary)]/50 border border-[var(--border-color)]">
                  <div className="text-[10px] text-[var(--text-muted)] font-bold uppercase">Démarrée à</div>
                  <div className="font-mono text-[var(--text-primary)] font-semibold mt-0.5">
                    {selectedActivity.startedAt ? formatDate(selectedActivity.startedAt) : 'En attente...'}
                  </div>
                </div>
                <div className="p-2.5 rounded-xl bg-[var(--bg-tertiary)]/50 border border-[var(--border-color)]">
                  <div className="text-[10px] text-[var(--text-muted)] font-bold uppercase">Durée totale</div>
                  <div className="font-mono text-emerald-400 font-semibold mt-0.5">
                    {selectedActivity.duration || (selectedActivity.status === 'running' ? 'En cours...' : 'N/A')}
                  </div>
                </div>
              </div>

              {/* Custom Prompt Input if provided */}
              {selectedActivity.prompt && (
                <div>
                  <div className="text-xs font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1.5 flex items-center gap-1">
                    <FileText size={13} />
                    <span>{t.activities.detail.prompt}</span>
                  </div>
                  <div className="p-3 rounded-xl bg-[var(--bg-tertiary)]/40 border border-[var(--border-color)] text-xs text-[var(--text-secondary)] whitespace-pre-wrap font-mono">
                    {selectedActivity.prompt}
                  </div>
                </div>
              )}

              {/* Step-by-Step Progress Timeline */}
              {selectedActivity.steps && selectedActivity.steps.length > 0 && (
                <div>
                  <div className="text-xs font-bold text-[var(--text-muted)] uppercase tracking-wider mb-2 flex items-center gap-1.5">
                    <Activity size={13} />
                    <span>{t.activities.detail.steps}</span>
                  </div>
                  <div className="p-3.5 rounded-2xl bg-[var(--bg-tertiary)]/30 border border-[var(--border-color)] space-y-2">
                    {selectedActivity.steps.map((step, idx) => (
                      <div key={idx} className="flex items-start gap-2.5 text-xs text-[var(--text-secondary)]">
                        <div className="w-4 h-4 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center shrink-0 mt-0.5 text-[10px] font-bold">
                          ✓
                        </div>
                        <span className="font-mono text-[11px] leading-relaxed">{step}</span>
                      </div>
                    ))}
                    {selectedActivity.status === 'running' && (
                      <div className="flex items-center gap-2.5 text-xs text-blue-400 animate-pulse pt-1">
                        <Loader2 size={13} className="animate-spin shrink-0" />
                        <span className="font-mono text-[11px]">Exécution en cours avec l'agent IA...</span>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Real AI Output / Terminal Console */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <div className="text-xs font-bold text-[var(--text-muted)] uppercase tracking-wider flex items-center gap-1.5">
                    <Terminal size={13} />
                    <span>{t.activities.detail.output}</span>
                  </div>

                  <div className="flex items-center gap-2">
                    {/* Rendered vs Raw Toggle */}
                    <div className="flex items-center bg-[var(--bg-tertiary)] p-0.5 rounded-lg border border-[var(--border-color)] text-[11px]">
                      <button
                        onClick={() => setOutputViewMode('rendered')}
                        className={`px-2 py-0.5 rounded font-medium transition-colors ${
                          outputViewMode === 'rendered'
                            ? 'bg-[var(--bg-secondary)] text-[var(--text-primary)] shadow-xs'
                            : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                        }`}
                      >
                        {t.activities.detail.renderedMarkdown}
                      </button>
                      <button
                        onClick={() => setOutputViewMode('raw')}
                        className={`px-2 py-0.5 rounded font-medium transition-colors ${
                          outputViewMode === 'raw'
                            ? 'bg-[var(--bg-secondary)] text-[var(--text-primary)] shadow-xs'
                            : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                        }`}
                      >
                        {t.activities.detail.rawLogs}
                      </button>
                    </div>

                    {/* Copy Output Button */}
                    {selectedActivity.output && (
                      <button
                        onClick={() => handleCopyOutput(selectedActivity.output)}
                        className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium bg-[var(--bg-tertiary)] hover:bg-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-color)] transition-colors"
                        title={t.activities.detail.copyOutput}
                      >
                        {copiedOutput ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                        <span className="text-[11px]">{copiedOutput ? t.activities.detail.copied : t.activities.detail.copyOutput}</span>
                      </button>
                    )}
                  </div>
                </div>

                {/* Sortie de l'agent. Le fond console vaut pour les logs bruts ;
                    en Markdown rendu, il vaut mieux le fond du thème, sur lequel
                    titres, tableaux et blocs de code se lisent vraiment. Le
                    bouton « Markdown rendu » existait déjà mais affichait le
                    texte tel quel : ses classes prose supposaient une extension
                    Tailwind absente du projet. */}
                <div
                  className={`p-4 rounded-2xl border shadow-inner overflow-x-auto min-h-[160px] max-h-[450px] ${
                    outputViewMode === 'rendered'
                      ? 'bg-[var(--bg-tertiary)] border-[var(--border-color)]'
                      : 'bg-slate-950 border-slate-800 font-mono text-xs text-slate-200'
                  }`}
                >
                  {selectedActivity.status === 'running' && !selectedActivity.output ? (
                    <div className="flex items-center justify-center py-12 text-slate-400 gap-2">
                      <Loader2 size={16} className="animate-spin text-blue-400" />
                      <span>Exécution du processus IA en cours... Les logs apparaîtront ici.</span>
                    </div>
                  ) : selectedActivity.output ? (
                    outputViewMode === 'rendered' ? (
                      <MarkdownView>{selectedActivity.output}</MarkdownView>
                    ) : (
                      <pre className="text-[11px] leading-relaxed text-emerald-400 whitespace-pre-wrap">
                        {selectedActivity.output}
                      </pre>
                    )
                  ) : (
                    <div className="text-slate-500 italic py-8 text-center">
                      Aucune sortie générée.
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="hidden lg:flex flex-1 flex-col items-center justify-center p-8 text-center text-[var(--text-muted)] bg-[var(--bg-secondary)]/20">
            <div className="w-14 h-14 rounded-2xl flex items-center justify-center bg-[var(--bg-tertiary)]/50 text-[var(--text-muted)] mb-3">
              <Terminal size={28} />
            </div>
            <h3 className="text-sm font-bold text-[var(--text-primary)]">
              Sélectionnez une activité
            </h3>
            <p className="text-xs text-[var(--text-muted)] max-w-xs mt-1">
              Cliquez sur une exécution de skill pour visualiser ses étapes détaillées, son statut et ses logs de sortie en temps réel.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
