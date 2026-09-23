import React, { useEffect, useMemo, useState } from 'react'
import {
  Check,
  ExternalLink,
  Layers,
  Target,
  User,
  CalendarRange,
  Inbox,
  Eye,
  EyeOff,
  Search,
  CheckCircle2,
  X,
  SlidersHorizontal,
  Sparkles,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { LookupField, type LookupOption } from './LookupField'
import { macroLookup, sprintLookup } from '../lib/lookups'
import type { MacroMeta, Task } from '../types'

type Dimension = 'sprint' | 'macro' | 'team' | 'assignee'

const DIMENSION_LABELS: Record<Dimension, string> = {
  sprint: 'sans sprint',
  macro: 'sans macro',
  team: 'sans équipe',
  assignee: 'sans assigné',
}

export const TriageView: React.FC = () => {
  const {
    tasks,
    currentProject,
    searchTrackerTeams,
    searchAssignableUsers,
    setTaskSprint,
    setTasksSprint,
    setTaskTeam,
    setTasksTeam,
    setTaskMacro,
    createMacro,
    moveTasksToMacro,
    updateTask,
    fetchProjectMacros,
    setSelectedTask,
    activeJobCount,
    hideDone,
    toggleHideDone,
    parentFilter,
    setParentFilter,
    projects,
    migrateTasks,
    startBatchPickup,
    t,
  } = useApp()

  const [macros, setMacros] = useState<MacroMeta[]>([])
  const [dimensions, setDimensions] = useState<Dimension[]>(['sprint', 'macro'])
  const [checked, setChecked] = useState<Record<string, boolean>>({})
  const [batchBusy, setBatchBusy] = useState<string | null>(null)
  const [filterSprint, setFilterSprint] = useState<string>('all')
  const [filterMacro, setFilterMacro] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState<string>('')

  // Batch action states
  const [batchSprint, setBatchSprint] = useState<{ id: string; name: string }>({ id: '', name: '' })
  const [batchTeam, setBatchTeam] = useState<{ id: string; name: string }>({ id: '', name: '' })
  const [batchMacro, setBatchMacro] = useState('')
  const [batchProjectId, setBatchProjectId] = useState('')

  useEffect(() => {
    if (!currentProject?.id) {
      setMacros([])
      return
    }
    fetchProjectMacros(currentProject.id).then(setMacros)
  }, [currentProject?.id, fetchProjectMacros, activeJobCount])

  const sprintOptions = useMemo(
    () =>
      (currentProject?.sprints || [])
        .filter(sp => sp.id && sp.state !== 'closed')
        .map(sp => ({ id: sp.id as string, name: sp.name, state: sp.state })),
    [currentProject?.sprints]
  )

  const macroOptions = useMemo(
    () => macros.filter(e => !e.closed).map(e => ({ key: e.key, title: e.title || e.key })),
    [macros]
  )

  const isMissing = (task: Task, dimension: Dimension): boolean => {
    if (dimension === 'sprint') return !(task.sprint || '').trim()
    if (dimension === 'macro') return !(task.parentKey || task.parentTitle || '').trim()
    if (dimension === 'team') return !(task.team || '').trim()
    return !(task.assignee || '').trim()
  }

  const pool = useMemo(
    () => tasks.filter(t => (hideDone ? t.status !== 'finished' && t.status !== 'done' : true)),
    [tasks, hideDone]
  )

  const counts = useMemo(() => {
    const out: Record<Dimension, number> = { sprint: 0, macro: 0, team: 0, assignee: 0 }
    pool.forEach(task => {
      (Object.keys(out) as Dimension[]).forEach(dimension => {
        if (isMissing(task, dimension)) out[dimension] += 1
      })
    })
    return out
  }, [pool])

  const filteredRows = useMemo(() => {
    let list = pool

    // Dimension filter
    if (dimensions.length > 0) {
      list = list.filter(task => dimensions.some(dimension => isMissing(task, dimension)))
    }

    // Sprint dropdown filter
    if (filterSprint === 'none') {
      list = list.filter(t => !(t.sprint || '').trim())
    } else if (filterSprint !== 'all') {
      list = list.filter(t => (t.sprint || '').trim() === filterSprint)
    }

    // Macro dropdown filter
    const activeMacro = parentFilter === '__no_macro__' ? 'none' : parentFilter || filterMacro
    if (activeMacro === 'none') {
      list = list.filter(t => !(t.parentKey || t.parentTitle || '').trim())
    } else if (activeMacro !== 'all') {
      list = list.filter(t => t.parentKey === activeMacro || t.parentTitle === activeMacro)
    }

    // Search query
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim()
      list = list.filter(
        t =>
          t.key.toLowerCase().includes(q) ||
          t.title.toLowerCase().includes(q) ||
          (t.sprint && t.sprint.toLowerCase().includes(q)) ||
          (t.parentKey && t.parentKey.toLowerCase().includes(q)) ||
          (t.parentTitle && t.parentTitle.toLowerCase().includes(q)) ||
          (t.assignee && t.assignee.toLowerCase().includes(q))
      )
    }

    return list
  }, [pool, dimensions, filterSprint, filterMacro, searchQuery])

  const selectedIds = useMemo(() => filteredRows.filter(t => checked[t.id]).map(t => t.id), [filteredRows, checked])

  const toggleDimension = (dimension: Dimension) => {
    setDimensions(prev =>
      prev.includes(dimension) ? prev.filter(d => d !== dimension) : [...prev, dimension]
    )
  }

  const searchMacro = useMemo(() => {
    const base = macroLookup(macros)
    return async (query: string): Promise<LookupOption[]> => {
      const res = await base(query)
      if (query.trim() && !res.some(o => o.label.toLowerCase() === query.trim().toLowerCase() || o.id.toLowerCase() === query.trim().toLowerCase())) {
        res.unshift({ id: `__create__:${query.trim()}`, label: query.trim(), sublabel: 'Créer ce milestone GitHub' })
      }
      return res
    }
  }, [macros])
  const searchSprint = useMemo(() => sprintLookup(currentProject?.sprints || []), [currentProject?.sprints])

  const searchTeamOptions = async (query: string): Promise<LookupOption[]> => {
    const found = await searchTrackerTeams(query)
    return found.filter(team => team.id).map(team => ({ id: team.id, label: team.name }))
  }

  const runBatch = async (kind: string, action: () => Promise<unknown>) => {
    setBatchBusy(kind)
    await action()
    setChecked({})
    setBatchSprint({ id: '', name: '' })
    setBatchTeam({ id: '', name: '' })
    setBatchMacro('')
    setBatchBusy(null)
  }

  if (!currentProject?.id) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-6 bg-[var(--bg-primary)]">
        <div className="p-3 rounded-2xl bg-[var(--accent-light)] border border-[var(--accent-color)]/30">
          <Inbox size={32} className="text-[var(--accent-color)]" />
        </div>
        <p className="text-sm font-bold text-[var(--text-primary)]">Sélectionnez un projet</p>
        <p className="text-xs text-[var(--text-secondary)] max-w-md">
          Le triage permet d'affecter en masse les Sprints, les Macros (Milestones) et les équipes sur vos tâches.
        </p>
      </div>
    )
  }

  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-hidden bg-[var(--bg-primary)] text-[var(--text-primary)]">
      {/* Top Header & Toolbar */}
      <div className="flex flex-col gap-2.5 px-4 py-3 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0">
        <div className="flex items-center justify-between gap-3 flex-wrap">
          <div className="flex items-center gap-2">
            <div className="p-1 rounded-lg bg-[var(--accent-light)] border border-[var(--accent-color)]/30 text-[var(--accent-color)]">
              <SlidersHorizontal size={16} />
            </div>
            <div>
              <h1 className="text-sm font-bold leading-tight">Triage du Backlog</h1>
              <p className="text-[11px] text-[var(--text-muted)]">
                Affectez rapidement vos tâches aux Sprints et aux Macros (Milestones).
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {/* Search Input */}
            <div className="relative w-56">
              <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
              <input
                type="text"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder="Filtrer clé, titre, membre…"
                className="w-full pl-8 pr-2.5 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => setSearchQuery('')}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                >
                  <X size={12} />
                </button>
              )}
            </div>

            {/* Sprints Filter Dropdown */}
            <div className="flex items-center gap-1">
              <CalendarRange size={13} className="text-[var(--text-muted)]" />
              <select
                value={filterSprint}
                onChange={e => setFilterSprint(e.target.value)}
                className="px-2 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer"
                title="Filtrer par Sprint"
              >
                <option value="all">Tous les sprints</option>
                <option value="none">Sans sprint (Backlog)</option>
                {sprintOptions.map(sp => (
                  <option key={sp.id} value={sp.name}>
                    {sp.name} ({sp.state})
                  </option>
                ))}
              </select>
            </div>

            {/* Macros Filter Dropdown */}
            <div className="flex items-center gap-1">
              <Target size={13} className="text-[var(--text-muted)]" />
              <select
                value={parentFilter === '__no_macro__' ? 'none' : parentFilter || filterMacro}
                onChange={e => {
                  const val = e.target.value
                  setFilterMacro(val)
                  setParentFilter(val === 'all' ? null : val === 'none' ? '__no_macro__' : val)
                }}
                className="px-2 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer"
                title="Filtrer par Macro"
              >
                <option value="all">Toutes les macros</option>
                <option value="none">Sans macro</option>
                {macroOptions.map(m => (
                  <option key={m.key} value={m.key}>
                    {m.key} - {m.title}
                  </option>
                ))}
              </select>
            </div>

            {/* Hide Done Toggle */}
            <button
              type="button"
              onClick={() => toggleHideDone()}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-semibold border cursor-pointer transition-colors"
              style={{
                color: hideDone ? 'var(--status-ok)' : 'var(--text-secondary)',
                background: hideDone ? 'rgb(var(--status-ok-rgb) / 0.12)' : 'var(--bg-tertiary)',
                borderColor: hideDone ? 'rgb(var(--status-ok-rgb) / 0.32)' : 'var(--border-color)',
              }}
              title="Masquer les tâches terminées"
            >
              {hideDone ? <EyeOff size={13} /> : <Eye size={13} />}
              <span>{hideDone ? 'Terminées masquées' : 'Terminées visibles'}</span>
            </button>
          </div>
        </div>

        {/* Dimension Filter Pills */}
        <div className="flex items-center justify-between gap-2 flex-wrap pt-1 border-t border-[var(--border-color)]/60">
          <div className="flex items-center gap-1.5 flex-wrap">
            <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mr-1">
              Manquants :
            </span>
            {(Object.keys(DIMENSION_LABELS) as Dimension[]).map(dimension => {
              const isActive = dimensions.includes(dimension)
              return (
                <button
                  key={dimension}
                  type="button"
                  onClick={() => toggleDimension(dimension)}
                  className="flex items-center gap-1.5 px-2.5 py-0.5 rounded-md text-[11px] font-bold border cursor-pointer transition-all"
                  style={{
                    color: isActive ? 'var(--accent-color)' : 'var(--text-secondary)',
                    background: isActive ? 'var(--accent-light)' : 'var(--bg-tertiary)',
                    borderColor: isActive ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
                  }}
                  title={`${counts[dimension]} tâche(s) ${DIMENSION_LABELS[dimension]}`}
                >
                  <span>{DIMENSION_LABELS[dimension]}</span>
                  <span className="font-mono text-[10px] px-1 rounded bg-[var(--bg-primary)]/80">
                    {counts[dimension]}
                  </span>
                </button>
              )
            })}

            {dimensions.length > 0 && (
              <button
                type="button"
                onClick={() => setDimensions([])}
                className="text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] underline cursor-pointer ml-1"
              >
                Afficher tout
              </button>
            )}
          </div>

          <div className="text-[11px] text-[var(--text-muted)] font-medium">
            <span className="font-bold text-[var(--text-primary)]">{filteredRows.length}</span> tâche(s) affichée(s)
          </div>
        </div>
      </div>

      {/* Batch Assignment Bar (Appears when items are checked) */}
      {selectedIds.length > 0 && (
        <div
          className="flex items-center gap-3 px-4 py-2 border-b shrink-0 flex-wrap"
          style={{ background: 'var(--accent-light)', borderColor: 'rgb(var(--accent-rgb) / 0.4)' }}
        >
          <div className="flex items-center gap-1.5 font-bold text-xs" style={{ color: 'var(--accent-color)' }}>
            <CheckCircle2 size={15} />
            <span>{selectedIds.length} sélectionné(s)</span>
          </div>

          {/* Quick Sprint Assignment */}
          <div className="flex items-center gap-1 w-[220px]">
            <div className="flex-1">
              <LookupField
                value={batchSprint.name}
                icon={<CalendarRange size={11} />}
                placeholder="Affecter Sprint…"
                clearLabel="Backlog (aucun sprint)"
                onSearch={searchSprint}
                onPick={option => setBatchSprint({ id: option?.id || '', name: option?.label || 'Backlog' })}
              />
            </div>
            <button
              type="button"
              disabled={!batchSprint.name || batchBusy === 'sprint'}
              onClick={() =>
                runBatch('sprint', () =>
                  setTasksSprint(currentProject.id, selectedIds, batchSprint.id, batchSprint.name)
                )
              }
              className="px-2 py-1 rounded-lg text-xs font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-primary)] border border-[var(--border-color)] shrink-0 hover:bg-[var(--bg-tertiary)]"
            >
              {batchBusy === 'sprint' ? '…' : 'OK'}
            </button>
          </div>

          {/* Quick Macro Assignment */}
          <div className="flex items-center gap-1 w-[230px]">
            <div className="flex-1">
              <LookupField
                value={batchMacro}
                icon={<Target size={11} />}
                placeholder="Affecter Macro…"
                allowClear={true}
                clearLabel="Retirer de la macro"
                onSearch={searchMacro}
                onPick={option => setBatchMacro(option?.id || '')}
              />
            </div>
            <button
              type="button"
              disabled={batchBusy === 'macro'}
              onClick={() =>
                runBatch('macro', () => moveTasksToMacro(currentProject.id, selectedIds, batchMacro))
              }
              className="px-2 py-1 rounded-lg text-xs font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-primary)] border border-[var(--border-color)] shrink-0 hover:bg-[var(--bg-tertiary)]"
            >
              {batchBusy === 'macro' ? '…' : 'OK'}
            </button>
          </div>

          {/* Quick Team Assignment */}
          <div className="flex items-center gap-1 w-[210px]">
            <div className="flex-1">
              <LookupField
                value={batchTeam.name}
                icon={<Layers size={11} />}
                placeholder="Affecter Équipe…"
                clearLabel="Aucune équipe"
                onSearch={searchTeamOptions}
                onPick={option => setBatchTeam({ id: option?.id || '', name: option?.label || 'Aucune équipe' })}
              />
            </div>
            <button
              type="button"
              disabled={!batchTeam.name || batchBusy === 'team'}
              onClick={() =>
                runBatch('team', () => setTasksTeam(currentProject.id, selectedIds, batchTeam.id, batchTeam.name))
              }
              className="px-2 py-1 rounded-lg text-xs font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-primary)] border border-[var(--border-color)] shrink-0 hover:bg-[var(--bg-tertiary)]"
            >
              {batchBusy === 'team' ? '…' : 'OK'}
            </button>
          </div>

          {/* Quick Project Migration */}
          {projects.length > 1 && (
            <div className="flex items-center gap-1">
              <select
                value={batchProjectId}
                onChange={(e) => setBatchProjectId(e.target.value)}
                className="px-2 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium max-w-[170px]"
              >
                <option value="">Déplacer projet…</option>
                {projects
                  .filter(p => p.id !== currentProject?.id)
                  .map(p => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.issueTracker || 'local'})
                    </option>
                  ))}
              </select>
              <button
                type="button"
                disabled={!batchProjectId || batchBusy === 'project'}
                onClick={() =>
                  runBatch('project', async () => {
                    const res = await migrateTasks(selectedIds, batchProjectId)
                    if (res.success) setChecked({})
                    return res.success
                  })
                }
                className="px-2 py-1 rounded-lg text-xs font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-primary)] border border-[var(--border-color)] shrink-0 hover:bg-[var(--bg-tertiary)]"
                title="Déplacer les tickets sélectionnés vers ce projet"
              >
                {batchBusy === 'project' ? '…' : 'OK'}
              </button>
            </div>
          )}

          <button
            type="button"
            onClick={() => startBatchPickup(selectedIds)}
            className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-colors shrink-0 shadow-xs"
            title="Run the selected tasks on the local agent"
          >
            <Sparkles size={13} />
            {t.batchLaunch}
          </button>

          <button
            type="button"
            onClick={() => setChecked({})}
            className="ml-auto text-xs font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            Désélectionner tout
          </button>
        </div>
      )}

      {/* Main Table */}
      <div className="flex-1 overflow-y-auto">
        <table className="w-full text-left border-collapse">
          <thead className="sticky top-0 z-10 bg-[var(--bg-secondary)] border-b border-[var(--border-color)] shadow-xs">
            <tr className="text-[10px] uppercase tracking-wider text-[var(--text-muted)] font-semibold">
              <th className="py-2.5 px-3 w-10">
                <button
                  type="button"
                  onClick={() =>
                    setChecked(prev => {
                      const allSelected = filteredRows.length > 0 && filteredRows.every(t => prev[t.id])
                      if (allSelected) return {}
                      const next: Record<string, boolean> = {}
                      filteredRows.forEach(t => {
                        next[t.id] = true
                      })
                      return next
                    })
                  }
                  className="w-4 h-4 rounded flex items-center justify-center cursor-pointer transition-colors"
                  style={{
                    background:
                      filteredRows.length > 0 && filteredRows.every(t => checked[t.id])
                        ? 'var(--accent-color)'
                        : 'transparent',
                    border: '1px solid var(--border-color)',
                  }}
                  title="Tout sélectionner / désélectionner"
                >
                  {filteredRows.length > 0 && filteredRows.every(t => checked[t.id]) && (
                    <Check size={10} className="text-white" />
                  )}
                </button>
              </th>
              <th className="py-2.5 px-3 w-24">Ticket</th>
              <th className="py-2.5 px-3 min-w-[280px]">Titre</th>
              <th className="py-2.5 px-3 w-[220px]">Macro (Milestone)</th>
              <th className="py-2.5 px-3 w-[200px]">Sprint</th>
              <th className="py-2.5 px-3 w-[180px]">Équipe</th>
              <th className="py-2.5 px-3 w-[180px]">Assigné</th>
              <th className="py-2.5 px-3 w-16 text-center">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border-color)]/60">
            {filteredRows.length === 0 ? (
              <tr>
                <td colSpan={8} className="py-12 px-4 text-center">
                  <div className="flex flex-col items-center justify-center gap-2 text-[var(--text-muted)]">
                    <CheckCircle2 size={24} className="text-emerald-400" />
                    <p className="text-sm font-semibold text-[var(--text-primary)]">Tout est trié !</p>
                    <p className="text-xs max-w-sm">
                      Aucune tâche ne correspond aux critères de triage actuels.
                    </p>
                  </div>
                </td>
              </tr>
            ) : (
              filteredRows.map(task => {
                const isSelected = Boolean(checked[task.id])

                return (
                  <tr
                    key={task.id}
                    className={`transition-colors hover:bg-[var(--bg-tertiary)]/40 ${
                      isSelected ? 'bg-[var(--accent-light)]/20' : ''
                    }`}
                  >
                    {/* Checkbox */}
                    <td className="py-2 px-3">
                      <button
                        type="button"
                        onClick={() => setChecked(prev => ({ ...prev, [task.id]: !prev[task.id] }))}
                        className="w-4 h-4 rounded flex items-center justify-center cursor-pointer transition-colors"
                        style={{
                          background: isSelected ? 'var(--accent-color)' : 'transparent',
                          border: `1px solid ${isSelected ? 'var(--accent-color)' : 'var(--border-color)'}`,
                        }}
                      >
                        {isSelected && <Check size={10} className="text-white" />}
                      </button>
                    </td>

                    {/* Key */}
                    <td className="py-2 px-3 whitespace-nowrap">
                      {task.externalUrl ? (
                        <a
                          href={task.externalUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 text-[11px] font-mono font-bold hover:underline"
                          style={{ color: 'var(--status-info)' }}
                          title={`Ouvrir ${task.key} sur le tracker distant`}
                        >
                          {task.key}
                          <ExternalLink size={10} />
                        </a>
                      ) : (
                        <span className="text-[11px] font-mono font-bold text-[var(--accent-color)]">
                          {task.key}
                        </span>
                      )}
                    </td>

                    {/* Title */}
                    <td className="py-2 px-3">
                      <button
                        type="button"
                        onClick={() => setSelectedTask(task)}
                        className="text-xs text-left font-medium text-[var(--text-primary)] hover:text-[var(--accent-color)] truncate block w-full max-w-[450px] cursor-pointer"
                        title={`${task.title} (cliquer pour ouvrir)`}
                      >
                        {task.title}
                      </button>
                    </td>

                    {/* Macro (Milestone) Lookup */}
                    <td className="py-2 px-3">
                      <LookupField
                        value={task.parentKey || task.parentTitle || ''}
                        icon={<Target size={11} />}
                        placeholder="Assigner Macro…"
                        clearLabel="Détacher de la macro"
                        emptyHint="Aucune macro ne correspond."
                        onSearch={searchMacro}
                        onPick={async (option) => {
                          if (!option?.id) {
                            await setTaskMacro(task.id, '')
                            return
                          }
                          if (option.id.startsWith('__create__:')) {
                            const title = option.id.replace('__create__:', '')
                            const created = await createMacro(task.projectId || currentProject?.id || 'default', title)
                            if (created) {
                              await setTaskMacro(task.id, created.key)
                              const macrosList = await fetchProjectMacros(task.projectId || currentProject?.id || 'default')
                              if (macrosList) setMacros(macrosList)
                            }
                          } else {
                            await setTaskMacro(task.id, option.id)
                          }
                        }}
                      />
                    </td>

                    {/* Sprint Lookup */}
                    <td className="py-2 px-3">
                      <LookupField
                        value={task.sprint || ''}
                        icon={<CalendarRange size={11} />}
                        placeholder="Assigner Sprint…"
                        clearLabel="Backlog (aucun sprint)"
                        emptyHint="Aucun sprint ne correspond."
                        onSearch={searchSprint}
                        onPick={option => setTaskSprint(task.id, option?.id || '', option?.label)}
                      />
                    </td>

                    {/* Team Lookup */}
                    <td className="py-2 px-3">
                      <LookupField
                        value={task.team || ''}
                        icon={<Layers size={11} />}
                        placeholder="Équipe…"
                        clearLabel="Aucune équipe"
                        onSearch={searchTeamOptions}
                        onPick={option => setTaskTeam(task.id, option?.id || '', option?.label)}
                      />
                    </td>

                    {/* Assignee Lookup */}
                    <td className="py-2 px-3">
                      <LookupField
                        value={task.assignee || ''}
                        icon={<User size={11} />}
                        placeholder="Assigné…"
                        clearLabel="Non assigné"
                        onSearch={async query => {
                          const people = await searchAssignableUsers(task.id, query)
                          return people.map(m => ({
                            id: m.accountId,
                            label: m.displayName,
                            sublabel: m.email,
                            avatarUrl: m.avatarUrl,
                            muted: !m.active,
                          }))
                        }}
                        onPick={option =>
                          updateTask(task.id, {
                            assignee: option?.label || '',
                            assigneeAccountId: option?.id || '',
                            assigneeAvatar: option?.avatarUrl || '',
                          })
                        }
                      />
                    </td>

                    {/* Quick Actions */}
                    <td className="py-2 px-3 text-center">

                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
