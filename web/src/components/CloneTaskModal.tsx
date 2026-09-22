import React, { useState, useEffect, useRef, useMemo } from 'react'
import {
  X,
  Copy,
  CopyPlus,
  Loader2,
  CalendarRange,
  FolderGit2,
  Layers,
  User,
  Tag,
  AlignLeft,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { Status, Priority, TaskSource, TrackerSprint, CloneTaskRequest } from '../types'
import { LookupField } from './LookupField'
import { sprintLookup } from '../lib/lookups'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'

export const CloneTaskModal: React.FC = () => {
  const {
    isCloneModalOpen,
    setIsCloneModalOpen,
    cloneSourceTask,
    setCloneSourceTask,
    cloneTask,
    projects,
    tasks,
    t,
  } = useApp()

  const [title, setTitle] = useState('')
  const [taskProjectId, setTaskProjectId] = useState<string>('default')
  const [status, setStatus] = useState<Status>('to_clarify')
  const [priority, setPriority] = useState<Priority>('medium')
  const [sprint, setSprint] = useState('')
  const [assignee, setAssignee] = useState('')
  const [source, setSource] = useState<TaskSource>('local')

  const [includeDescription, setIncludeDescription] = useState(true)
  const [includeLabels, setIncludeLabels] = useState(true)
  const [includeParent, setIncludeParent] = useState(true)
  const [includeAssignee, setIncludeAssignee] = useState(true)
  const [includeSprint, setIncludeSprint] = useState(true)

  const [isSubmitting, setIsSubmitting] = useState(false)
  const titleInputRef = useRef<HTMLInputElement>(null)

  const availableSprints = useMemo(() => {
    const proj = projects.find(p => p.id === taskProjectId)
    const projSprints = (proj?.sprints || []).filter(sp => sp.name && sp.state !== 'closed')
    const distinctTaskSprints = Array.from(
      new Set(
        tasks
          .filter(t => t.projectId === taskProjectId)
          .map(t => (t.sprint || '').trim())
          .filter(Boolean)
      )
    )
    const combined: TrackerSprint[] = [...projSprints]
    for (const name of distinctTaskSprints) {
      if (!combined.some(s => s.name.toLowerCase() === name.toLowerCase() || (s.id && s.id.toLowerCase() === name.toLowerCase()))) {
        combined.push({ id: name, name, state: 'future' })
      }
    }
    return combined
  }, [projects, taskProjectId, tasks])

  const searchSprint = useMemo(() => sprintLookup(availableSprints), [availableSprints])

  useEffect(() => {
    if (isCloneModalOpen && cloneSourceTask) {
      setTitle(`${cloneSourceTask.title} (Copie)`)
      setTaskProjectId(cloneSourceTask.projectId || 'default')
      setStatus('to_clarify')
      setPriority(cloneSourceTask.priority || 'medium')
      setSprint(cloneSourceTask.sprint || '')
      setAssignee(cloneSourceTask.assignee || '')
      setSource((cloneSourceTask.source as TaskSource) || 'local')

      setIncludeDescription(Boolean(cloneSourceTask.description?.trim()))
      setIncludeLabels(Boolean(cloneSourceTask.labels && cloneSourceTask.labels.length > 0))
      setIncludeParent(Boolean(cloneSourceTask.parentKey || cloneSourceTask.parentTitle))
      setIncludeAssignee(Boolean(cloneSourceTask.assignee?.trim()))
      setIncludeSprint(Boolean(cloneSourceTask.sprint?.trim()))

      setTimeout(() => {
        titleInputRef.current?.focus()
        titleInputRef.current?.select()
      }, 50)
    }
  }, [isCloneModalOpen, cloneSourceTask])

  const handleClose = () => {
    setIsCloneModalOpen(false)
    setCloneSourceTask(null)
  }

  const backdrop = useBackdropDismiss(handleClose)

  useEffect(() => {
    if (!isCloneModalOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isCloneModalOpen])

  if (!isCloneModalOpen || !cloneSourceTask) return null

  const handleProjectChange = (projId: string) => {
    setTaskProjectId(projId)
    const proj = projects.find(p => p.id === projId)
    if (proj?.issueTracker) {
      setSource(proj.issueTracker as TaskSource)
    }
  }

  const handleClone = async (openAfter: boolean) => {
    if (!title.trim() || isSubmitting) return

    setIsSubmitting(true)
    try {
      const payload: CloneTaskRequest = {
        title: title.trim(),
        projectId: taskProjectId,
        status,
        priority,
        sprint: includeSprint ? sprint : '',
        assignee: includeAssignee ? assignee : '',
        source,
        includeDescription,
        includeLabels,
        includeParent,
        includeSprint,
        includeAssignee,
      }

      const cloned = await cloneTask(cloneSourceTask.id, payload, openAfter)
      if (cloned) {
        handleClose()
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150" {...backdrop}>
      <div className="w-full max-w-xl rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150 flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/50">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="w-8 h-8 rounded-xl bg-[var(--accent-light)] accent-text flex items-center justify-center shrink-0 border border-[var(--accent-color)]/30">
              <CopyPlus size={18} />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-bold text-[var(--text-primary)] truncate">
                  Cloner la story
                </h2>
                <span className="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)]">
                  {cloneSourceTask.key}
                </span>
              </div>
              <p className="text-[11px] text-[var(--text-muted)] truncate">
                Duplique la story en conservant son contenu et en lui attribuant un nouvel identifiant.
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <X size={16} />
          </button>
        </div>

        {/* Content Form */}
        <form
          onSubmit={e => {
            e.preventDefault()
            handleClone(true)
          }}
          className="p-5 overflow-y-auto space-y-4 text-xs"
        >
          {/* New Title */}
          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Titre de la nouvelle story <span className="text-rose-400">*</span>
            </label>
            <input
              ref={titleInputRef}
              type="text"
              required
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder="Titre de la story..."
              className="w-full px-3.5 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
            />
          </div>

          {/* Project & Destination */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {/* Target Project */}
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                Projet de destination
              </label>
              <div className="relative">
                <select
                  value={taskProjectId}
                  onChange={e => handleProjectChange(e.target.value)}
                  className="w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                >
                  {(() => {
                    const bookmarked = projects.filter(p => p.bookmarked)
                    const others = projects.filter(p => !p.bookmarked)
                    if (bookmarked.length > 0 && others.length > 0) {
                      return (
                        <>
                          <optgroup label="Favoris">
                            {bookmarked.map(p => (
                              <option key={p.id} value={p.id}>
                                {p.name}
                              </option>
                            ))}
                          </optgroup>
                          <optgroup label="Autres projets">
                            {others.map(p => (
                              <option key={p.id} value={p.id}>
                                {p.name}
                              </option>
                            ))}
                          </optgroup>
                        </>
                      )
                    }
                    return projects.map(p => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))
                  })()}
                </select>
                <FolderGit2 size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
              </div>
            </div>

            {/* Tracker Destination */}
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                Type de Tracker
              </label>
              <div className="grid grid-cols-3 gap-1">
                {[
                  { id: 'github' as TaskSource, label: 'GitHub', icon: '🐙' },
                  { id: 'jira' as TaskSource, label: 'Jira', icon: '🔷' },
                  { id: 'local' as TaskSource, label: 'Local', icon: '📁' },
                ].map(opt => (
                  <button
                    key={opt.id}
                    type="button"
                    onClick={() => setSource(opt.id)}
                    className={`flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-xl border text-[11px] font-semibold transition-all cursor-pointer ${
                      source === opt.id
                        ? 'border-[var(--accent-color)] bg-[var(--accent-light)] accent-text'
                        : 'border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:border-[var(--accent-color)]/50'
                    }`}
                  >
                    <span>{opt.icon}</span>
                    <span>{opt.label}</span>
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Status & Priority */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                Étape initiale (Statut)
              </label>
              <select
                value={status}
                onChange={e => setStatus(e.target.value as Status)}
                className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
              >
                <option value="to_clarify">{t.status.to_clarify} (#new - Recommandé)</option>
                <option value="clarified">{t.status.clarified} (#clarified)</option>
                <option value="to_implement">{t.status.to_implement} (#specified)</option>
                <option value="to_test">{t.status.to_test} (#implemented)</option>
                <option value="to_close">{t.status.to_close} (#reviewed)</option>
                <option value="finished">{t.status.finished} (#finished)</option>
              </select>
            </div>

            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                Priorité
              </label>
              <select
                value={priority}
                onChange={e => setPriority(e.target.value as Priority)}
                className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
              >
                <option value="urgent">{t.priority.urgent}</option>
                <option value="high">{t.priority.high}</option>
                <option value="medium">{t.priority.medium}</option>
                <option value="low">{t.priority.low}</option>
              </select>
            </div>
          </div>

          {/* Sprint */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                Sprint de destination
              </label>
              {cloneSourceTask.sprint && (
                <span className="text-[10px] text-[var(--text-muted)] font-mono">
                  Source : {cloneSourceTask.sprint}
                </span>
              )}
            </div>
            <LookupField
              value={sprint}
              icon={<CalendarRange size={12} />}
              placeholder="Affecter un sprint (optionnel)…"
              clearLabel="Backlog (aucun sprint)"
              emptyHint="Aucun sprint trouvé. Tapez un nom pour créer."
              onSearch={async (query: string) => {
                const res = await searchSprint(query)
                if (query.trim() && !res.some(o => o.label.toLowerCase() === query.trim().toLowerCase())) {
                  res.unshift({ id: query.trim(), label: query.trim(), sublabel: 'Nouveau sprint' })
                }
                return res
              }}
              onPick={option => setSprint(option?.label || '')}
            />
          </div>

          {/* Options de duplication */}
          <div className="p-3.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] space-y-2.5">
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
              Éléments à cloner
            </label>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
              {/* Description */}
              <label className="flex items-center gap-2 p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] cursor-pointer hover:border-[var(--accent-color)]/40 transition-colors">
                <input
                  type="checkbox"
                  checked={includeDescription}
                  onChange={e => setIncludeDescription(e.target.checked)}
                  className="rounded text-[var(--accent-color)] focus:ring-[var(--accent-color)] cursor-pointer"
                />
                <AlignLeft size={13} className="text-blue-400 shrink-0" />
                <span className="text-[11px] text-[var(--text-primary)] font-medium truncate">
                  Description détaillée
                </span>
              </label>

              {/* Labels */}
              <label className="flex items-center gap-2 p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] cursor-pointer hover:border-[var(--accent-color)]/40 transition-colors">
                <input
                  type="checkbox"
                  checked={includeLabels}
                  onChange={e => setIncludeLabels(e.target.checked)}
                  className="rounded text-[var(--accent-color)] focus:ring-[var(--accent-color)] cursor-pointer"
                />
                <Tag size={13} className="text-amber-400 shrink-0" />
                <span className="text-[11px] text-[var(--text-primary)] font-medium truncate">
                  Tags et Labels ({cloneSourceTask.labels?.length || 0})
                </span>
              </label>

              {/* Parent / Macro */}
              {(cloneSourceTask.parentKey || cloneSourceTask.parentTitle) && (
                <label className="flex items-center gap-2 p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] cursor-pointer hover:border-[var(--accent-color)]/40 transition-colors">
                  <input
                    type="checkbox"
                    checked={includeParent}
                    onChange={e => setIncludeParent(e.target.checked)}
                    className="rounded text-[var(--accent-color)] focus:ring-[var(--accent-color)] cursor-pointer"
                  />
                  <Layers size={13} className="text-purple-400 shrink-0" />
                  <span className="text-[11px] text-[var(--text-primary)] font-medium truncate" title={cloneSourceTask.parentTitle || cloneSourceTask.parentKey}>
                    Rattacher à {cloneSourceTask.parentTitle || cloneSourceTask.parentKey}
                  </span>
                </label>
              )}

              {/* Assignee */}
              {cloneSourceTask.assignee && (
                <label className="flex items-center gap-2 p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] cursor-pointer hover:border-[var(--accent-color)]/40 transition-colors">
                  <input
                    type="checkbox"
                    checked={includeAssignee}
                    onChange={e => setIncludeAssignee(e.target.checked)}
                    className="rounded text-[var(--accent-color)] focus:ring-[var(--accent-color)] cursor-pointer"
                  />
                  <User size={13} className="text-emerald-400 shrink-0" />
                  <span className="text-[11px] text-[var(--text-primary)] font-medium truncate">
                    Assigné ({cloneSourceTask.assignee})
                  </span>
                </label>
              )}
            </div>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-2 pt-2 border-t border-[var(--border-color)]">
            <button
              type="button"
              onClick={handleClose}
              className="px-3.5 py-2 rounded-xl text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] border border-[var(--border-color)] transition-colors cursor-pointer"
            >
              Annuler
            </button>

            <button
              type="button"
              disabled={!title.trim() || isSubmitting}
              onClick={() => handleClone(false)}
              className="px-3.5 py-2 rounded-xl text-xs font-semibold bg-[var(--bg-tertiary)] text-[var(--text-primary)] hover:bg-[var(--bg-primary)] border border-[var(--border-color)] transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5"
            >
              {isSubmitting ? <Loader2 size={13} className="animate-spin" /> : <Copy size={13} />}
              <span>Cloner</span>
            </button>

            <button
              type="button"
              disabled={!title.trim() || isSubmitting}
              onClick={() => handleClone(true)}
              className="px-4 py-2 rounded-xl text-xs font-bold bg-[var(--accent-color)] text-white hover:brightness-110 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5 shadow-sm"
            >
              {isSubmitting ? <Loader2 size={13} className="animate-spin" /> : <CopyPlus size={13} />}
              <span>Cloner et ouvrir</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
