import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react'
import {
  X,
  Plus,
  Loader2,
  CalendarRange
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { Status, Priority, TrackerSprint, MacroMeta } from '../types'
import { LookupField } from './LookupField'
import { PrioritySelect } from './PrioritySelect'
import { MarkdownEditor } from './Markdown'
import { sprintLookup } from '../lib/lookups'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { initialLabelsForView } from '../lib/boardViews'
import { initialQuickAddMacro, quickAddMacroOptions, QUICK_ADD_FOLLOW_UPS, type QuickAddFollowUp } from '../lib/quickAdd'

export const QuickAddModal: React.FC = () => {
  const {
    isQuickAddOpen,
    setIsQuickAddOpen,
    quickAddInitialStatus,
    createTask,
    projects,
    tasks,
    selectedProjectId,
    currentBoardView,
    macroFilter,
    fetchProjectMacros,
    setSelectedTask,
    runSkill,
    t,
  } = useApp()

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [status, setStatus] = useState<Status>(quickAddInitialStatus)
  const [priority, setPriority] = useState<Priority>('medium')
  const fallbackProjectId = selectedProjectId !== 'all' ? selectedProjectId : (projects[0]?.id || 'default')
  const [taskProjectId, setTaskProjectId] = useState<string>(fallbackProjectId)
  const [macroKey, setMacroKey] = useState('')
  const [macroOptions, setMacroOptions] = useState<MacroMeta[]>([])
  const [macrosLoading, setMacrosLoading] = useState(false)
  const [followUp, setFollowUp] = useState<QuickAddFollowUp>('none')
  const [issueType, setIssueType] = useState<string>('')
  const [sprint, setSprint] = useState('')
  const [labels, setLabels] = useState<string[]>([])
  const [labelInput, setLabelInput] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const inputRef = useRef<HTMLInputElement>(null)
  // The last macro request: a response for a project no longer selected is dropped.
  const macroRequest = useRef(0)
  // The project whose macro list may pre-select the board's macro filter: the
  // one the dialog opens on, or the first one picked from a saved view. Tied to
  // a project because the list of the previous opening's project can still
  // arrive first.
  const preselectMacroFor = useRef<string | null>(null)

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
    if (isQuickAddOpen) {
      setTitle('')
      setDescription('')
      setStatus(quickAddInitialStatus || 'to_clarify')
      setPriority('medium')
      setIssueType('')
      // From a saved view, the source project is the user's choice among the
      // view's projects, never a silent default (#387).
      const initialProjId = currentBoardView
        ? ''
        : selectedProjectId !== 'all' ? selectedProjectId : (projects[0]?.id || 'default')
      setTaskProjectId(initialProjId)
      setMacroKey('')
      setMacroOptions([])
      preselectMacroFor.current = initialProjId || null
      // Launching an agent stays an explicit gesture: never remembered.
      setFollowUp('none')
      setSprint('')
      setLabels(['#new', ...initialLabelsForView(currentBoardView)])
      setLabelInput('')
      setTimeout(() => {
        inputRef.current?.focus()
      }, 50)
    }
  }, [isQuickAddOpen, quickAddInitialStatus, selectedProjectId, projects, currentBoardView])

  // The macros are per project: reload them whenever the project changes.
  useEffect(() => {
    if (!isQuickAddOpen || !taskProjectId) {
      // The field is hidden without a project; drop what is in flight.
      macroRequest.current++
      return
    }
    const request = ++macroRequest.current
    const projectId = taskProjectId
    setMacrosLoading(true)
    fetchProjectMacros(projectId).then(macros => {
      if (request !== macroRequest.current) return
      const options = quickAddMacroOptions(macros)
      setMacroOptions(options)
      if (preselectMacroFor.current === projectId) {
        preselectMacroFor.current = null
        setMacroKey(initialQuickAddMacro(macroFilter, options))
      }
      setMacrosLoading(false)
    })
    // fetchProjectMacros is recreated on every render of the provider, and the
    // board filter only matters on opening: neither may trigger a reload.
  }, [isQuickAddOpen, taskProjectId])

  const handleClose = useCallback(() => setIsQuickAddOpen(false), [setIsQuickAddOpen])
  const backdrop = useBackdropDismiss(handleClose)

  useEffect(() => {
    if (!isQuickAddOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isQuickAddOpen, handleClose])

  const activeProject = projects.find(p => p.id === taskProjectId) || (currentBoardView ? undefined : projects[0])
  const viewProjects = currentBoardView
    ? projects.filter(p => currentBoardView.projectIds.includes(p.id))
    : null

  if (!isQuickAddOpen) return null

  const handleProjectChange = (projId: string) => {
    // A macro belongs to one project: the choice starts over with the list.
    preselectMacroFor.current = taskProjectId ? null : projId
    setMacroKey('')
    setTaskProjectId(projId)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || isSubmitting) return
    if (!taskProjectId) return

    setIsSubmitting(true)
    const created = await createTask({
      title: title.trim(),
      description: description.trim(),
      status,
      priority,
      issueType: issueType.trim() || undefined,
      labels,
      sprint: sprint.trim() || undefined,
      projectId: taskProjectId,
      macroKey: macroKey || undefined,
    })
    setIsSubmitting(false)
    if (!created) return

    setIsQuickAddOpen(false)
    // The rewrite is a proposal: it is reviewed and applied in the ticket's
    // detail modal. The clarification runs in the background.
    if (followUp === 'rewrite') {
      setSelectedTask(created)
      await runSkill(created.id, 'rewrite_story', '')
    } else if (followUp === 'clarify') {
      await runSkill(created.id, 'clarify')
    }
  }

  const handleAddLabel = () => {
    const clean = labelInput.replace(/^#+/, '').trim()
    if (clean && !labels.some(l => l.replace(/^#+/, '').toLowerCase() === clean.toLowerCase())) {
      setLabels([...labels, clean])
      setLabelInput('')
    }
  }

  const removeLabel = (tag: string) => {
    const cleanTarget = tag.replace(/^#+/, '').toLowerCase()
    setLabels(labels.filter(l => l.replace(/^#+/, '').toLowerCase() !== cleanTarget))
  }

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150" {...backdrop}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="quick-add-title"
        className="relative w-full max-w-4xl max-h-full flex flex-col rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden"
      >
        {/* Header */}
        <div className="shrink-0 flex items-center justify-between px-5 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <div className="flex items-center gap-2 font-semibold text-xs text-[var(--text-primary)]">
            <div className="w-5 h-5 rounded-md accent-bg text-white flex items-center justify-center">
              <Plus size={13} />
            </div>
            <span id="quick-add-title">{t.quickAdd.title}</span>
          </div>
          <button
            type="button"
            onClick={() => setIsQuickAddOpen(false)}
            className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="flex flex-col min-h-0 flex-1">
          <div className="flex-1 min-h-0 overflow-y-auto p-5 grid grid-cols-1 md:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] gap-5 items-start">
            {/* Left: what the ticket says */}
            <div data-quick-add-column="main" className="min-w-0 space-y-4">
              <input
                ref={inputRef}
                type="text"
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder={t.quickAdd.placeholder}
                className="w-full px-3.5 py-2 text-xs font-semibold rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)] focus:ring-1 focus:ring-[var(--accent-color)]"
              />

              <MarkdownEditor
                value={description}
                onChange={setDescription}
                placeholder={t.taskModal.descPlaceholder}
                minHeight={260}
                maxHeight={Math.round(window.innerHeight * 0.5)}
              />

              {/* What happens once the ticket exists: one choice, none by default */}
              <fieldset>
                <legend className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                  {t.quickAdd.followUp}
                </legend>
                <div role="radiogroup" aria-label={t.quickAdd.followUp} className="grid grid-cols-1 sm:grid-cols-3 gap-1.5">
                  {QUICK_ADD_FOLLOW_UPS.map(option => {
                    const label = option === 'rewrite'
                      ? t.quickAdd.followUpRewrite
                      : option === 'clarify' ? t.quickAdd.followUpClarify : t.quickAdd.followUpNone
                    return (
                      <label
                        key={option}
                        className={`flex items-center gap-2 px-2.5 py-1.5 rounded-xl border text-[11px] font-semibold cursor-pointer transition-all ${
                          followUp === option
                            ? 'border-[var(--accent-color)] bg-[var(--accent-light)] accent-text'
                            : 'border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:border-[var(--accent-color)]/50'
                        }`}
                      >
                        <input
                          type="radio"
                          name="quick-add-follow-up"
                          value={option}
                          checked={followUp === option}
                          onChange={() => setFollowUp(option)}
                          className="accent-[var(--accent-color)]"
                        />
                        <span>{label}</span>
                      </label>
                    )
                  })}
                </div>
              </fieldset>
            </div>

            {/* Right: where it goes and how it is filed */}
            <div data-quick-add-column="details" className="min-w-0 space-y-4">
              {/* Project Target */}
              {projects.length > 0 && (
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                      Projet *
                    </label>
                    {activeProject && (
                      <span className="text-[10px] text-[var(--text-muted)] flex items-center gap-1 font-mono">
                        <span>Tracker :</span>
                        <span className="font-semibold text-[var(--accent-color)]">
                          {activeProject.issueTracker === 'github'
                            ? `GitHub (${activeProject.githubRepo ? activeProject.githubRepo.split('/')[1] || activeProject.githubRepo : 'repo'})`
                            : activeProject.issueTracker === 'jira'
                            ? `Jira (${activeProject.jiraProject || 'projet non configuré'})`
                            : 'Local'}
                        </span>
                      </span>
                    )}
                  </div>
                  <select
                    value={taskProjectId}
                    onChange={e => handleProjectChange(e.target.value)}
                    required
                    aria-label={t.boardViews.projectForNewTicket}
                    className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                  >
                    {viewProjects ? (
                      <>
                        <option value="" disabled>{t.boardViews.chooseProject}</option>
                        {viewProjects.map(p => (
                          <option key={p.id} value={p.id}>
                            {p.name}
                          </option>
                        ))}
                      </>
                    ) : (() => {
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
                </div>
              )}

              {/* Macro: the chosen project's open macros */}
              {taskProjectId && (
                <div>
                  <label htmlFor="quick-add-macro" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                    {t.quickAdd.macro}
                  </label>
                  <select
                    id="quick-add-macro"
                    value={macroKey}
                    onChange={e => setMacroKey(e.target.value)}
                    disabled={macrosLoading}
                    className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] disabled:opacity-60"
                  >
                    <option value="">{macrosLoading ? t.quickAdd.macroLoading : t.quickAdd.noMacro}</option>
                    {macroOptions.map(m => (
                      <option key={m.key} value={m.key}>
                        {m.title ? `${m.key} · ${m.title}` : m.key}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {/* Issue Type Selector */}
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                  Type de ticket
                </label>
                <div className="grid grid-cols-4 gap-1.5">
                  {[
                    { id: '', label: 'Défaut', icon: '⚪' },
                    { id: 'Story', label: 'Story', icon: '📘' },
                    { id: 'Bug', label: 'Bug', icon: '🐛' },
                    { id: 'Task', label: 'Tâche', icon: '📝' },
                  ].map(opt => (
                    <button
                      key={opt.id}
                      type="button"
                      onClick={() => setIssueType(opt.id)}
                      className={`flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-xl border text-[11px] font-semibold transition-all cursor-pointer ${
                        issueType === opt.id
                          ? 'border-[var(--accent-color)] bg-[var(--accent-light)] accent-text shadow-xs'
                          : 'border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:border-[var(--accent-color)]/50'
                      }`}
                    >
                      <span>{opt.icon}</span>
                      <span>{opt.label}</span>
                    </button>
                  ))}
                </div>
              </div>

              {/* Status & Priority Selectors */}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                    {t.quickAdd.status}
                  </label>
                  <select
                    value={status}
                    onChange={e => setStatus(e.target.value as Status)}
                    className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                  >
                    <option value="to_clarify">{t.status.to_clarify} (#new)</option>
                    <option value="clarified">{t.status.clarified} (#clarified)</option>
                    <option value="to_implement">{t.status.to_implement} (#specified)</option>
                    <option value="to_test">{t.status.to_test} (#implemented)</option>
                    <option value="to_close">{t.status.to_close} (#reviewed)</option>
                    <option value="finished">{t.status.finished} (#finished)</option>
                  </select>
                </div>

                <div>
                  <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                    {t.quickAdd.priority}
                  </label>
                  <PrioritySelect
                    value={priority}
                    onChange={setPriority}
                    className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                  />
                </div>
              </div>

              {/* Sprint */}
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                  Sprint
                </label>
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

              {/* Labels */}
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                  {t.taskModal.labels}
                </label>
                <div className="flex flex-wrap items-center gap-1.5 p-2 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
                  {labels.map(l => {
                    const lower = l.toLowerCase().replace(/^#+/, '')
                    let badgeStyle = 'bg-[var(--accent-light)] accent-text'
                    if (lower === 'new') badgeStyle = 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 font-bold'
                    else if (lower === 'clarified') badgeStyle = 'bg-amber-500/20 text-amber-300 border border-amber-500/40 font-bold'
                    else if (lower === 'specified') badgeStyle = 'bg-blue-500/20 text-blue-300 border border-blue-500/40 font-bold'
                    else if (lower === 'implemented') badgeStyle = 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 font-bold'
                    else if (lower === 'reviewed') badgeStyle = 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 font-bold'

                    return (
                      <span
                        key={l}
                        className={`inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-md ${badgeStyle}`}
                      >
                        #{l.replace(/^#+/, '')}
                        <button
                          type="button"
                          onClick={() => removeLabel(l)}
                          className="hover:opacity-75 ml-0.5"
                        >
                          <X size={11} />
                        </button>
                      </span>
                    )
                  })}
                  <div className="flex items-center gap-1 min-w-[100px] flex-1">
                    <input
                      type="text"
                      value={labelInput}
                      onChange={e => setLabelInput(e.target.value)}
                      onKeyDown={e => {
                        if (e.key === 'Enter' || e.key === ',') {
                          e.preventDefault()
                          handleAddLabel()
                        }
                      }}
                      placeholder={t.taskModal.addLabel}
                      className="w-full text-xs bg-transparent border-none text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none px-1"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Footer */}
          <div className="shrink-0 flex items-center justify-between px-5 py-3 border-t border-[var(--border-color)]">
            <span className="text-[11px] text-[var(--text-muted)]">
              {t.quickAdd.hint}
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setIsQuickAddOpen(false)}
                className="px-3 py-1.5 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] transition-colors"
              >
                {t.taskModal.cancel}
              </button>
              <button
                type="submit"
                disabled={!title.trim() || !taskProjectId || isSubmitting}
                className="px-4 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow hover:opacity-90 active:scale-95 disabled:opacity-50 flex items-center gap-1.5 transition-all"
              >
                {isSubmitting ? (
                  <>
                    <Loader2 size={13} className="animate-spin" />
                    <span>Création CLI...</span>
                  </>
                ) : (
                  <>
                    <Plus size={13} />
                    <span>{t.taskModal.create}</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
