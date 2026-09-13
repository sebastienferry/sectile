import React, { useEffect, useMemo, useRef, useState } from 'react'
import {
  Target,
  CalendarRange,
  Route,
  Compass,
  HelpCircle,
  EyeOff,
  Eye,
  CalendarDays,
  ExternalLink,
  Plus,
  Check,
  Trash2,
  AlertTriangle,
  Save,
  X,
  Filter,
  Scissors,
  Search,
  Loader2,
  Pencil,
  ArrowRightLeft,
  Sparkles,
  ListChecks,
  FolderGit2,
  Maximize2,
  Minimize2,
  ChevronDown,
  ChevronRight,
  MessageSquare,
} from 'lucide-react'
import type { RefineMacroResult } from '../types'
import { useApp } from '../context/AppContext'
import { LookupField } from './LookupField'
import { MarkdownEditor } from './Markdown'
import { sprintLookup, isProjectCompatible } from '../lib/lookups'
import {
  buildMacroRows,
  placementIssues,
  placementOf,
  matchesMacroSearch,
  HORIZON_META,
  MATURITY_META,
  PLACEMENT_META,
  PRIORITY_META,
  type MacroRow,
  type Horizon,
  type HorizonTab,
  tasksBySprintOrder,
  sprintLabelOf,
} from '../lib/roadmap'
import type { MacroHorizon, MacroMeta, MacroTodo } from '../types'

/**
 * Roadmap des macros, d'après le design « Roadmap Epics.dc.html ».
 *
 * Deux métiers, pas un seul. NOW et NEXT sont opérationnels : on y vérifie que
 * les stories d'une macro sont bien dans un sprint — actif pour NOW, à venir pour
 * NEXT — et tout ce qui ne l'est pas doit sauter aux yeux. LATER est du cadrage
 * de macro : description et TODO se travaillent avant qu'il y ait des stories.
 */

const TABS: { id: HorizonTab; label: string; icon: React.ReactNode }[] = [
  { id: 'now', label: 'NOW', icon: <Target size={14} /> },
  { id: 'next', label: 'NEXT', icon: <Route size={14} /> },
  { id: 'later', label: 'LATER', icon: <Compass size={14} /> },
  { id: 'unclassified', label: 'Non classés', icon: <HelpCircle size={14} /> },
  { id: 'hidden', label: 'Masqués', icon: <EyeOff size={14} /> },
]

export const RoadmapView: React.FC = () => {
  const {
    tasks,
    projects,
    currentProject,
    setSelectedTask,
    fetchProjectMacros,
    saveMacroMeta,
    createStoryFromMacroTodo,
    setTaskMacro,
    createStoryUnderMacro,
    createMacro,
    deleteMacro,
    moveTasksToMacro,
    setTaskSprint,
    setTasksSprint,
    assigneeFilter,
    setAssigneeFilter,
    sprintFilter,
    setSprintFilter,
    teamFilter,
    setTeamFilter,
    labelFilter,
    setLabelFilter,
    pinnedOnly,
    setPinnedOnly,
    searchQuery,
    setSearchQuery,
    activeJobCount,
    addToast,
    migrateMacro,
    refineMacro,
    createBatchTasks,
  } = useApp()

  const [tab, setTab] = useState<HorizonTab>('now')
  const [displayMode, setDisplayMode] = useState<'framing' | 'execution'>('execution')
  const [macroMeta, setMacroMeta] = useState<MacroMeta[]>([])
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [onlyIssues, setOnlyIssues] = useState(false)
  const [showClosed, setShowClosed] = useState(false)

  const [isRefining, setIsRefining] = useState(false)
  const [refinePreview, setRefinePreview] = useState<RefineMacroResult | null>(null)
  const [selectedProposedTasks, setSelectedProposedTasks] = useState<Record<number, boolean>>({})
  const [isCreatingBatch, setIsCreatingBatch] = useState(false)

  const [showMigrateModal, setShowMigrateModal] = useState(false)
  const [migrateTargetProjectId, setMigrateTargetProjectId] = useState('')
  const [migrateIncludeTasks, setMigrateIncludeTasks] = useState(true)
  const [isMigrating, setIsMigrating] = useState(false)

  // Synchronisation du mode par défaut selon l'horizon sélectionné
  useEffect(() => {
    if (tab === 'later') {
      setDisplayMode('framing')
    } else if (tab === 'now' || tab === 'next') {
      setDisplayMode('execution')
    }
  }, [tab])

  const [showCreateMacroModal, setShowCreateMacroModal] = useState(false)
  const [createMacroTitle, setCreateMacroTitle] = useState('')
  const [createMacroHorizon, setCreateMacroHorizon] = useState<MacroHorizon>('now')
  const [createMacroProjectId, setCreateMacroProjectId] = useState<string>('')

  const [isEditingTitle, setIsEditingTitle] = useState(false)
  const [editingTitleValue, setEditingTitleValue] = useState('')

  useEffect(() => {
    setIsEditingTitle(false)
    setEditingTitleValue('')
  }, [selectedKey])

  // Plein écran / Expand du panneau de droite et repli des zones de texte
  const [isPanelExpanded, setIsPanelExpanded] = useState(false)
  const [isDescExpanded, setIsDescExpanded] = useState(true)
  const [isFramingExpanded, setIsFramingExpanded] = useState(true)

  // Le cadrage n'est enregistré qu'à la demande
  const [draftDescription, setDraftDescription] = useState('')
  const [draftDirty, setDraftDirty] = useState(false)
  const [draftFramingComment, setDraftFramingComment] = useState('')
  const [draftFramingDirty, setDraftFramingDirty] = useState(false)
  const [newTodo, setNewTodo] = useState('')
  const [creatingTodoId, setCreatingTodoId] = useState<string | null>(null)
  // Prototypage de la macro : créer une story a la volée, ou y pousser un ticket existant
  const [newStory, setNewStory] = useState('')
  const [attachQuery, setAttachQuery] = useState('')
  const [busyKey, setBusyKey] = useState<string | null>(null)
  // Découpe : les stories cochées partent vers une autre macro
  const [checked, setChecked] = useState<Record<string, boolean>>({})
  const [cutAt, setCutAt] = useState<number>(-1)
  const [draggingCut, setDraggingCut] = useState(false)
  const rowRefs = useRef<(HTMLDivElement | null)[]>([])
  const [moveTarget, setMoveTarget] = useState('')
  const [newMacroTitle, setNewMacroTitle] = useState('')

  // Largeur du panneau de droite. C'est là que le travail se fait (les tickets de
  // l'épic, la découpe), alors que la colonne de gauche n'a qu'à lister : la
  // répartition par défaut donne donc la place au panneau, et la poignée permet
  // de la régler. La valeur est mémorisée par navigateur.
  // Sprints proposables : ceux du board du projet, actifs et futurs. Un sprint
  // clos n'accueille plus de travail, et l'API Agile déplace par identifiant, donc
  // ceux qui n'en portent pas (import antérieur) sont écartés.
  const sprintOptions = useMemo(
    () =>
      (currentProject?.sprints || [])
        .filter(sp => sp.id && sp.state !== 'closed')
        .map(sp => ({ id: sp.id as string, name: sp.name, state: sp.state })),
    [currentProject?.sprints]
  )
  const [sprintTarget, setSprintTarget] = useState<{ id: string; name: string }>({ id: '', name: '' })

  // Épics et sprints se cherchent au clavier : cette vue en liste cent quarante
  // et dix, et l'épic cible d'une découpe se choisissait dans un menu déroulant
  // de tout le projet.
  const searchSprint = useMemo(() => sprintLookup(currentProject?.sprints || []), [currentProject?.sprints])

  // Ce qui restreint la liste de tickets sur laquelle la roadmap est construite.
  const activeFilterChips = useMemo(() => {
    const chips: { label: string; clear: () => void }[] = []
    if (assigneeFilter) {
      chips.push({
        label: assigneeFilter === '__unassigned__' ? 'non assigné' : assigneeFilter,
        clear: () => setAssigneeFilter(null),
      })
    }
    if (sprintFilter) chips.push({ label: sprintFilter, clear: () => setSprintFilter(null) })
    if (teamFilter) chips.push({ label: teamFilter, clear: () => setTeamFilter(null) })
    if (labelFilter) chips.push({ label: `#${labelFilter.replace(/^#+/, '')}`, clear: () => setLabelFilter(null) })
    if (pinnedOnly) chips.push({ label: 'épinglés seulement', clear: () => setPinnedOnly(false) })
    if (searchQuery) chips.push({ label: `« ${searchQuery} »`, clear: () => setSearchQuery('') })
    return chips
  }, [
    assigneeFilter,
    sprintFilter,
    teamFilter,
    labelFilter,
    pinnedOnly,
    searchQuery,
    setAssigneeFilter,
    setSprintFilter,
    setTeamFilter,
    setLabelFilter,
    setPinnedOnly,
    setSearchQuery,
  ])

  const PANEL_MIN = 420
  const LIST_MIN = 280
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    const stored = Number(localStorage.getItem('taskflow_roadmap_panel_width') || localStorage.getItem('taskacao_roadmap_panel_width') || '')
    return Number.isFinite(stored) && stored >= PANEL_MIN ? stored : 720
  })
  const splitRef = useRef<HTMLDivElement>(null)
  const [isDraggingSplit, setIsDraggingSplit] = useState(false)

  const startSplitDrag = (e: React.PointerEvent) => {
    e.preventDefault()
    const container = splitRef.current
    if (!container) return
    setIsDraggingSplit(true)

    const onMove = (ev: PointerEvent) => {
      const rect = container.getBoundingClientRect()
      const next = Math.round(rect.right - ev.clientX)
      const max = Math.max(PANEL_MIN, rect.width - LIST_MIN)
      setPanelWidth(Math.min(max, Math.max(PANEL_MIN, next)))
    }
    const onUp = () => {
      setIsDraggingSplit(false)
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      setPanelWidth(current => {
        try {
          localStorage.setItem('taskflow_roadmap_panel_width', String(current))
        } catch {
          // stockage indisponible : la largeur vaut pour cette session
        }
        return current
      })
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  useEffect(() => {
    if (!currentProject?.id) {
      setMacroMeta([])
      return
    }
    fetchProjectMacros(currentProject.id).then(setMacroMeta)
  }, [currentProject?.id, fetchProjectMacros, activeJobCount])

  const allRows = useMemo(() => buildMacroRows(tasks, currentProject, macroMeta), [tasks, currentProject, macroMeta])

  const rows = useMemo(() => {
    let list = allRows
    if (!showClosed) list = list.filter(r => !r.closed)
    if (searchQuery.trim()) list = list.filter(r => matchesMacroSearch(r, searchQuery))
    return list
  }, [allRows, showClosed, searchQuery])

  const hiddenMatches = useMemo(() => {
    const q = searchQuery.trim()
    if (!q) return 0
    return allRows.filter(r => {
      if (!matchesMacroSearch(r, q)) return false
      if (!showClosed && r.closed) return true
      return false
    }).length
  }, [allRows, searchQuery, showClosed])

  const closedCount = allRows.filter(r => r.closed).length

  const counts = useMemo(
    () => ({
      now: rows.filter(r => r.horizon === 'now').length,
      next: rows.filter(r => r.horizon === 'next').length,
      later: rows.filter(r => r.horizon === 'later').length,
      unclassified: rows.filter(r => !r.horizon).length,
      hidden: rows.filter(r => r.horizon === 'hidden').length,
    }),
    [rows]
  )

  // Les onglets « non classés » et « masqués » n'ont pas d'horizon propre : le
  // panneau y montre le cadrage, pas la vérification de sprint.
  const horizonOfTab: Horizon =
    tab === 'next' ? 'next' : tab === 'later' ? 'later' : tab === 'hidden' ? 'hidden' : 'now'

  const visibleRows = useMemo(() => {
    const list = tab === 'unclassified' ? rows.filter(r => !r.horizon) : rows.filter(r => r.horizon === tab)
    if (displayMode === 'execution' && onlyIssues) {
      return list.filter(r => placementIssues(r, horizonOfTab).length > 0)
    }
    return list
  }, [rows, tab, displayMode, onlyIssues, horizonOfTab])

  // Chercher une macro et rester devant un onglet vide n'aide personne
  useEffect(() => {
    if (!searchQuery.trim() || visibleRows.length > 0) return
    const target = TABS.find(candidate =>
      candidate.id === 'unclassified'
        ? rows.some(r => !r.horizon)
        : rows.some(r => r.horizon === candidate.id)
    )
    if (target && target.id !== tab) setTab(target.id)
  }, [searchQuery, visibleRows.length, rows, tab])

  const selected: MacroRow | null = visibleRows.find(r => r.key === selectedKey) || visibleRows[0] || null

  // Les tickets de la macro dans l'ordre chronologique de leur sprint
  const orderedOpen = useMemo(
    () => (selected ? tasksBySprintOrder(selected.open, currentProject) : []),
    [selected?.key, selected?.open, currentProject?.id, currentProject?.sprints]
  )

  // Glissement du cran de coupe
  useEffect(() => {
    if (!draggingCut) return

    const onMove = (e: PointerEvent) => {
      const rows = rowRefs.current.slice(0, orderedOpen.length).filter(Boolean) as HTMLDivElement[]
      if (rows.length === 0) return

      let best = 0
      let bestDistance = Number.POSITIVE_INFINITY
      rows.forEach((row, index) => {
        const rect = row.getBoundingClientRect()
        const distance = Math.abs(rect.top - e.clientY)
        if (distance < bestDistance) {
          bestDistance = distance
          best = index
        }
      })

      const last = rows[rows.length - 1].getBoundingClientRect()
      if (e.clientY > last.bottom) {
        setCutAt(-1)
        return
      }
      setCutAt(best)
    }

    const onUp = () => setDraggingCut(false)
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
  }, [draggingCut, orderedOpen.length])

  const cutIds = useMemo(() => {
    const manual = Object.entries(checked).filter(([, on]) => on).map(([id]) => id)
    if (manual.length > 0) return manual
    if (cutAt >= 0) return orderedOpen.slice(cutAt).map(t => t.id)
    return []
  }, [checked, cutAt, orderedOpen])

  useEffect(() => {
    setDraftDescription(selected?.meta?.description || '')
    setDraftDirty(false)
    setDraftFramingComment(selected?.meta?.framingComment || '')
    setDraftFramingDirty(false)
    setNewTodo('')
    setChecked({})
    setCutAt(-1)
    rowRefs.current = []
    setMoveTarget('')
    setNewMacroTitle('')
  }, [selected?.key, selected?.meta?.description, selected?.meta?.framingComment])

  const attachCandidates = useMemo(() => {
    const q = attachQuery.trim().toLowerCase()
    if (!q || !selected) return []
    return tasks
      .filter(t => t.parentKey !== selected.key)
      .filter(t => t.key.toLowerCase().includes(q) || t.title.toLowerCase().includes(q))
      .slice(0, 6)
  }, [attachQuery, tasks, selected])

  const activeSprints = (currentProject?.sprints || []).filter(s => s.state === 'active')
  const futureSprints = (currentProject?.sprints || []).filter(s => s.state === 'future')

  const persist = async (
    key: string,
    patch: { horizon?: MacroHorizon | ''; description?: string; framingComment?: string; todos?: MacroTodo[] }
  ) => {
    if (!currentProject?.id) {
      addToast({
        type: 'error',
        title: 'Aucun projet sélectionné',
        description: 'Choisis un projet pour classer ses macros.',
      })
      return
    }
    const saved = await saveMacroMeta(currentProject.id, key, patch)
    if (saved) {
      setMacroMeta(prev => [...prev.filter(m => m.key !== saved.key), saved])
      if (patch.horizon) {
        setTab(patch.horizon)
      }
      setSelectedKey(saved.key)
    }
  }

  const todosOf = (row: MacroRow | null): MacroTodo[] => row?.meta?.todos || []

  const addTodo = (row: MacroRow) => {
    const text = newTodo.trim()
    if (!text) return
    setNewTodo('')
    persist(row.key, { todos: [...todosOf(row), { id: '', text, done: false }] })
  }

  const handleRefineMacro = async () => {
    if (!selected) return
    const text = (draftDescription || selected.meta?.description || '').trim()
    if (!text) {
      addToast({
        type: 'warning',
        title: 'Cadrage requis',
        description: 'Veuillez saisir un texte de cadrage (description) avant de raffiner la macro.',
      })
      return
    }

    if (draftDirty) {
      await persist(selected.key, { description: draftDescription })
      setDraftDirty(false)
    }

    setIsRefining(true)
    const result = await refineMacro(selected.key, currentProject?.id)
    setIsRefining(false)

    if (result && ((result.todos && result.todos.length > 0) || (result.proposedTasks && result.proposedTasks.length > 0))) {
      setRefinePreview(result)
      const initialMap: Record<number, boolean> = {}
      result.proposedTasks?.forEach((_: unknown, idx: number) => { initialMap[idx] = true })
      setSelectedProposedTasks(initialMap)
    } else if (result) {
      addToast({
        type: 'info',
        title: 'Aucune donnée générée',
        description: "Le texte de cadrage n'a pas permis de générer de nouveaux items.",
      })
    }
  }

  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-hidden bg-[var(--bg-primary)] text-[var(--text-primary)]">
      {/* Barre d'outils : classification, et en mode opérationnel les sprints visés */}
          <div className="flex items-center justify-between gap-3 flex-wrap px-4 py-2 border-b border-[var(--border-color)] bg-[var(--bg-secondary)]/50 shrink-0">
        <div className="flex items-center gap-3 flex-wrap min-w-0">
          <div className="flex items-center p-0.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
            {TABS.map(t => {
              const active = tab === t.id
              const meta = t.id === 'unclassified' ? null : HORIZON_META[t.id as Horizon]
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => setTab(t.id)}
                  className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] transition-colors cursor-pointer"
                  style={{
                    fontWeight: active ? 700 : 500,
                    background: active ? meta?.color || 'var(--text-muted)' : 'transparent',
                    color: active ? '#fff' : 'var(--text-secondary)',
                  }}
                  title={meta ? meta.hint : 'Macros pas encore arbitrées'}
                >
                  {t.icon}
                  <span>{t.label}</span>
                  <span
                    className="text-[10px] font-mono px-1.5 rounded-full"
                    style={{
                      background: active ? 'rgb(255 255 255 / 0.25)' : 'var(--bg-secondary)',
                      color: active ? '#fff' : 'var(--text-muted)',
                    }}
                  >
                    {counts[t.id]}
                  </span>
                </button>
              )
            })}
          </div>

          {closedCount > 0 && (
            <button
              type="button"
              onClick={() => setShowClosed(v => !v)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold cursor-pointer border"
              style={{
                background: showClosed ? 'var(--accent-light)' : 'var(--bg-tertiary)',
                borderColor: showClosed ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
                color: showClosed ? 'var(--accent-color)' : 'var(--text-secondary)',
              }}
              title={`${closedCount} macros terminées côté tracker`}
            >
              {showClosed ? <Eye size={12} /> : <EyeOff size={12} />}
              {showClosed ? `${closedCount} terminées affichées` : `${closedCount} terminées masquées`}
            </button>
          )}

          {/* Les filtres globaux */}
          {activeFilterChips.length > 0 && (
            <div className="flex items-center gap-1.5 flex-wrap">
              {activeFilterChips.map(chip => (
                <button
                  key={chip.label}
                  type="button"
                  onClick={chip.clear}
                  className="flex items-center gap-1 px-2 py-1 rounded-md text-[10.5px] font-semibold cursor-pointer border"
                  style={{
                    background: 'rgb(var(--status-warn-rgb) / 0.14)',
                    borderColor: 'rgb(var(--status-warn-rgb) / 0.34)',
                    color: 'var(--status-warn)',
                  }}
                  title={`Filtre actif : ${chip.label}. Cliquer pour l'enlever.`}
                >
                  <Filter size={10} />
                  {chip.label}
                  <X size={10} />
                </button>
              ))}
            </div>
          )}

          {/* Toggle Mode: Framing | Execution */}
          <div className="flex items-center p-0.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
            <button
              type="button"
              onClick={() => setDisplayMode('framing')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition-all cursor-pointer ${
                displayMode === 'framing'
                  ? 'bg-[var(--accent-color)] text-white shadow-xs'
                  : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
              title="Mode Framing : Cadrage et Checklist TODOs"
            >
              <Compass size={12} />
              <span>Framing</span>
            </button>
            <button
              type="button"
              onClick={() => setDisplayMode('execution')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition-all cursor-pointer ${
                displayMode === 'execution'
                  ? 'bg-[var(--accent-color)] text-white shadow-xs'
                  : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
              title="Mode Execution : Stories, Sprints et Découpe"
            >
              <Target size={12} />
              <span>Execution</span>
            </button>
          </div>

          <button
            type="button"
            disabled={busyKey === 'macro'}
            onClick={() => {
              const defaultProjId = currentProject?.id || projects.find(p => p.isDefault)?.id || projects[0]?.id || ''
              setCreateMacroProjectId(defaultProjId)
              setCreateMacroHorizon(tab === 'unclassified' || tab === 'hidden' ? 'now' : (tab as MacroHorizon))
              setCreateMacroTitle('')
              setShowCreateMacroModal(true)
            }}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold text-white accent-bg cursor-pointer disabled:opacity-40"
            title="Créer une macro / milestone GitHub"
          >
            <Plus size={12} /> Macro
          </button>

          {displayMode === 'execution' && (
            <button
              type="button"
              onClick={() => setOnlyIssues(v => !v)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold transition-colors cursor-pointer border"
              style={{
                background: onlyIssues ? 'rgb(var(--status-danger-rgb) / 0.14)' : 'var(--bg-tertiary)',
                borderColor: onlyIssues ? 'rgb(var(--status-danger-rgb) / 0.4)' : 'var(--border-color)',
                color: onlyIssues ? 'var(--status-danger)' : 'var(--text-secondary)',
              }}
              title="Ne garder que les macros ayant un ticket non terminé sans sprint, ou resté dans un sprint passé"
            >
              <AlertTriangle size={12} />
              À corriger
            </button>
          )}

          {/* La barre de recherche est dans l'en-tête, loin de la liste : sans
              ce rappel, on ne comprend pas pourquoi la roadmap est réduite. */}
          {searchQuery.trim() && (
            <span
              className="flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-semibold border"
              style={{
                background: 'var(--accent-light)',
                borderColor: 'rgb(var(--accent-rgb) / 0.4)',
                color: 'var(--accent-color)',
              }}
            >
              <Search size={11} />
              {rows.length} macro{rows.length > 1 ? 's' : ''} sur « {searchQuery.trim()} »
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                className="cursor-pointer opacity-70 hover:opacity-100"
                title="Effacer la recherche"
              >
                <X size={11} />
              </button>
            </span>
          )}
        </div>

        {displayMode === 'execution' && (tab === 'now' || tab === 'next') && (
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] bg-[var(--bg-tertiary)] border border-[var(--border-color)] shrink-0">
            <CalendarDays size={13} style={{ color: HORIZON_META[horizonOfTab].color }} />
            <span className="text-[var(--text-muted)]">{tab === 'now' ? 'Sprints actifs' : 'Sprints à venir'} :</span>
            {(tab === 'now' ? activeSprints : futureSprints).length === 0 ? (
              <span className="font-mono" style={{ color: 'var(--status-warn)' }}>aucun connu — lance une synchro</span>
            ) : (
              <span className="font-mono font-bold truncate max-w-[320px]">
                {(tab === 'now' ? activeSprints : futureSprints).map(s => s.name).join(' · ')}
              </span>
            )}
          </div>
        )}
      </div>

      <div className="flex-1 flex min-h-0 min-w-0 overflow-hidden" ref={splitRef}>
        {/* Macros de l'horizon courant (masqué si panneau en plein écran) */}
        {!isPanelExpanded && (
          <div className="flex-1 overflow-y-auto p-3 min-w-0 space-y-2">
            {visibleRows.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center gap-2 text-center px-6">
                <Compass size={26} className="text-[var(--text-muted)]" />
                <p className="text-sm font-semibold">
                  {searchQuery.trim() ? `Aucune macro ne correspond à « ${searchQuery.trim()} »` : 'Aucune macro ici'}
                </p>
                <p className="text-[11px] text-[var(--text-muted)] max-w-sm">
                  {searchQuery.trim()
                    ? hiddenMatches > 0
                      ? `${hiddenMatches} macro(s) correspondent mais sont écartées par les filtres d'affichage : affiche les macros closes ou celles d'un autre projet pour les voir.`
                      : 'La recherche porte sur la clé, le titre et l’équipe de la macro, ainsi que sur les clés et titres de ses tickets.'
                    : tab === 'hidden'
                    ? 'Aucune macro masquée. Classe en HIDDEN le tout-venant qui n’a pas vocation à apparaître dans la roadmap.'
                    : tab === 'unclassified'
                    ? 'Toutes les macros sont classées. Les nouvelles apparaîtront ici après une synchro.'
                    : onlyIssues
                      ? 'Aucune anomalie de placement sur cet horizon.'
                      : 'Classe des macros depuis l’onglet « Non classés » pour les voir apparaître ici.'}
                </p>
              </div>
            ) : (
              visibleRows.map(row => {
                const isSel = selected?.key === row.key
                const issues = placementIssues(row, horizonOfTab)
                const mat = MATURITY_META[row.maturity]
                const prio = PRIORITY_META[row.priority]
                return (
                  <div
                    key={row.key}
                    onClick={() => setSelectedKey(row.key)}
                    className="rounded-xl border p-2.5 cursor-pointer transition-colors"
                    style={{
                      background: isSel ? 'var(--accent-light)' : 'var(--bg-secondary)',
                      borderColor: isSel ? 'rgb(var(--accent-rgb) / 0.45)' : 'var(--border-color)',
                    }}
                  >
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-[11px] font-mono font-bold" style={{ color: 'var(--accent-color)' }}>{row.key}</span>
                      <span className="text-[9.5px] px-1 rounded font-mono truncate max-w-[150px] bg-[var(--bg-tertiary)] text-[var(--text-muted)] border border-[var(--border-color)]" title={row.squad}>
                        {row.squad}
                      </span>
                      <span className="text-[9.5px] px-1 rounded font-bold" style={{ color: prio.color, background: prio.bg }}>
                        {prio.label}
                      </span>
                      <span className="text-[9px] font-bold px-1.5 rounded uppercase tracking-[.06em]"
                        style={{ color: mat.color, background: mat.bg, border: `1px solid ${mat.border}` }}>
                        {row.maturity}
                      </span>

                      {displayMode === 'execution' ? (
                        issues.length > 0 ? (
                          <span className="ml-auto text-[10px] font-bold px-1.5 py-0.5 rounded inline-flex items-center gap-1"
                            style={{ color: 'var(--status-danger)', background: 'rgb(var(--status-danger-rgb) / 0.13)', border: '1px solid rgb(var(--status-danger-rgb) / 0.32)' }}>
                            <AlertTriangle size={10} />
                            {issues.length} à corriger
                          </span>
                        ) : (
                          <span className="ml-auto text-[10px] font-bold px-1.5 py-0.5 rounded inline-flex items-center gap-1"
                            style={{ color: 'var(--status-ok)', background: 'rgb(var(--status-ok-rgb) / 0.13)', border: '1px solid rgb(var(--status-ok-rgb) / 0.32)' }}>
                            <Check size={10} />
                            tout placé
                          </span>
                        )
                      ) : (
                        <span className="ml-auto text-[10px] font-mono text-[var(--text-muted)]">
                          {todosOf(row).filter(t => t.done).length}/{todosOf(row).length} todos
                        </span>
                      )}
                    </div>

                    <div className="text-[12px] font-semibold leading-snug mt-1.5">{row.title}</div>

                    <div className="flex items-center gap-2 mt-2 flex-wrap text-[10px] font-mono text-[var(--text-muted)]">
                      <span>{row.open.length} ouverts / {row.tasks.length}</span>
                      {row.inActiveSprint.length > 0 && (
                        <span style={{ color: 'var(--status-ok)' }}>{row.inActiveSprint.length} sprint actif</span>
                      )}
                      {row.inFutureSprint.length > 0 && (
                        <span style={{ color: 'var(--status-info)' }}>{row.inFutureSprint.length} sprint futur</span>
                      )}
                      {row.inStaleSprint.length > 0 && (
                        <span style={{ color: 'var(--status-warn)' }}>{row.inStaleSprint.length} sprint clos</span>
                      )}
                      {row.unscheduled.length > 0 && (
                        <span style={{ color: 'var(--status-danger)' }}>{row.unscheduled.length} sans sprint</span>
                      )}
                    </div>

                    {/* Classification : un clic, et la suggestion est mise en avant */}
                    <div className="flex items-center gap-1.5 mt-2">
                      {(['now', 'next', 'later', 'hidden'] as MacroHorizon[]).map(h => {
                        const active = row.horizon === h
                        const isSuggestion = !row.horizon && row.suggested === h
                        return (
                          <button
                            key={h}
                            type="button"
                            onClick={e => {
                              e.stopPropagation()
                              persist(row.key, { horizon: h })
                            }}
                            className="px-1.5 py-0.5 rounded text-[9.5px] font-bold uppercase tracking-[.06em] border transition-colors cursor-pointer"
                            style={{
                              color: active ? '#fff' : HORIZON_META[h].color,
                              background: active ? HORIZON_META[h].color : isSuggestion ? HORIZON_META[h].bg : 'transparent',
                              borderColor: active || isSuggestion ? HORIZON_META[h].border : 'var(--border-color)',
                            }}
                            title={isSuggestion ? `Suggéré d'après les sprints : ${HORIZON_META[h].label}` : `Classer en ${HORIZON_META[h].label}`}
                          >
                            {HORIZON_META[h].label}
                            {isSuggestion && ' ?'}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })
            )}
          </div>
        )}

        {/* Poignée de répartition (masquée si plein écran) */}
        {selected && !isPanelExpanded && (
          <div
            role="separator"
            aria-orientation="vertical"
            aria-label="Répartition entre la liste des macros et le panneau"
            onPointerDown={startSplitDrag}
            onDoubleClick={() => setPanelWidth(720)}
            title="Glisser pour répartir, double-clic pour revenir à la largeur par défaut"
            className="w-1.5 shrink-0 cursor-col-resize transition-colors"
            style={{ background: isDraggingSplit ? 'var(--accent-color)' : 'var(--border-color)' }}
          />
        )}

        {/* Panneau : vérification des sprints en NOW/NEXT, cadrage en LATER */}
        {selected && (
          <aside className="flex flex-col min-h-0 shrink-0 bg-[var(--bg-secondary)]"
            style={{ width: isPanelExpanded ? '100%' : panelWidth, flex: isPanelExpanded ? 1 : undefined }}>
            <div className="px-4 pt-3.5 pb-3 shrink-0 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 mb-1.5 flex-wrap">
                <span className="text-[11px] font-mono font-bold" style={{ color: 'var(--accent-color)' }}>{selected.key}</span>
                {selected.horizon && (
                  <span
                    className="px-1.5 py-0.5 rounded text-[9.5px] font-bold uppercase tracking-[.06em] text-white"
                    style={{ background: HORIZON_META[selected.horizon].color }}
                  >
                    {HORIZON_META[selected.horizon].label}
                  </span>
                )}
                <div className="ml-auto flex items-center gap-1.5 flex-wrap">
                  <button
                    type="button"
                    onClick={() => setIsPanelExpanded(prev => !prev)}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--accent-color)] hover:opacity-90 cursor-pointer border border-[var(--accent-color)]/40 bg-[var(--accent-light)] transition-colors"
                    title={isPanelExpanded ? "Réduire le panneau (Vue divisée)" : "Agrandir le panneau sur toute la surface (Mode Framing)"}
                  >
                    {isPanelExpanded ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
                    <span>{isPanelExpanded ? "Réduire" : "Plein écran"}</span>
                  </button>

                  {selected.tasks[0]?.externalUrl && (
                    <a href={selected.tasks[0].externalUrl} target="_blank" rel="noreferrer"
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold transition-colors"
                      style={{
                        color: 'var(--status-info)',
                        background: 'rgb(var(--status-info-rgb) / 0.12)',
                        border: '1px solid rgb(var(--status-info-rgb) / 0.32)',
                      }}
                      title={`Ouvrir ${selected.key} dans le tracker distant`}>
                      <ExternalLink size={13} />
                      <span>Lien</span>
                    </a>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      setEditingTitleValue(selected.title)
                      setIsEditingTitle(true)
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                    title="Modifier le nom de la macro / milestone"
                  >
                    <Pencil size={12} />
                    <span>Renommer</span>
                  </button>
                  <button
                    type="button"
                    disabled={busyKey === 'migrate' || isMigrating}
                    onClick={() => {
                      const compatible = projects.filter(p => isProjectCompatible(currentProject, p))
                      setMigrateTargetProjectId(compatible[0]?.id || '')
                      setMigrateIncludeTasks(true)
                      setShowMigrateModal(true)
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                    title="Migrer cette macro et ses tickets vers un autre projet compatible"
                  >
                    <ArrowRightLeft size={12} />
                    <span>Migrer</span>
                  </button>
                  <button
                    type="button"
                    disabled={busyKey === 'delete'}
                    onClick={async () => {
                      if (!confirm(`Supprimer la macro ${selected.key} (${selected.title}) ?\n(Les tickets associés seront détachés)`)) return
                      if (!currentProject?.id) return
                      setBusyKey('delete')
                      const ok = await deleteMacro(currentProject.id, selected.key)
                      if (ok) {
                        setMacroMeta(prev => prev.filter(m => m.key !== selected.key))
                        setSelectedKey(null)
                      }
                      setBusyKey(null)
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-muted)] hover:text-rose-400 cursor-pointer border border-[var(--border-color)] hover:border-rose-500/30 transition-colors disabled:opacity-50"
                    title={`Supprimer la macro ${selected.key} (et le milestone GitHub si applicable)`}
                  >
                    <Trash2 size={12} />
                    <span>Supprimer</span>
                  </button>
                </div>
              </div>

              <div className="mt-1">
                {isEditingTitle ? (
                  <form
                    onSubmit={async (e) => {
                      e.preventDefault()
                      if (!selected) return
                      const nextTitle = editingTitleValue.trim()
                      if (!nextTitle || nextTitle === selected.title) {
                        setIsEditingTitle(false)
                        return
                      }
                      const targetProjId = currentProject?.id || projects.find(p => p.isDefault)?.id || projects[0]?.id
                      if (!targetProjId) return
                      setBusyKey('editTitle')
                      const updated = await saveMacroMeta(targetProjId, selected.key, { title: nextTitle })
                      if (updated) {
                        setMacroMeta(prev => prev.map(m => m.key === selected.key ? { ...m, title: nextTitle } : m))
                        addToast({ type: 'success', title: `Macro ${selected.key} renommée`, description: nextTitle })
                      }
                      setIsEditingTitle(false)
                      setBusyKey(null)
                    }}
                    className="flex items-center gap-2"
                  >
                    <input
                      type="text"
                      autoFocus
                      value={editingTitleValue}
                      onChange={(e) => setEditingTitleValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Escape') {
                          setIsEditingTitle(false)
                        }
                      }}
                      className="flex-1 px-2.5 py-1 text-sm font-bold rounded-xl bg-[var(--bg-primary)] border border-[var(--accent-color)] text-[var(--text-primary)] focus:outline-none"
                    />
                    <button
                      type="submit"
                      disabled={!editingTitleValue.trim() || busyKey === 'editTitle'}
                      className="p-1.5 rounded-lg text-white accent-bg hover:opacity-90 transition-opacity cursor-pointer disabled:opacity-50"
                      title="Enregistrer le nom"
                    >
                      {busyKey === 'editTitle' ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                    </button>
                    <button
                      type="button"
                      onClick={() => setIsEditingTitle(false)}
                      className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                      title="Annuler"
                    >
                      <X size={13} />
                    </button>
                  </form>
                ) : (
                  <div className="group flex items-center gap-2">
                    <h2
                      onClick={() => {
                        setEditingTitleValue(selected.title)
                        setIsEditingTitle(true)
                      }}
                      className="text-[16px] font-bold leading-[1.25] cursor-pointer hover:text-[var(--accent-color)] transition-colors"
                      title="Cliquer pour modifier le nom du milestone"
                    >
                      {selected.title}
                    </h2>
                    <button
                      type="button"
                      onClick={() => {
                        setEditingTitleValue(selected.title)
                        setIsEditingTitle(true)
                      }}
                      className="opacity-0 group-hover:opacity-100 p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-all cursor-pointer"
                      title="Renommer la macro / milestone"
                    >
                      <Pencil size={12} />
                    </button>
                  </div>
                )}
              </div>

              {selected.meta?.status && (
                <div className="mt-1.5 text-[10px] font-mono" style={{ color: selected.closed ? 'var(--status-ok)' : 'var(--text-muted)' }}>
                  Macro {selected.closed ? 'terminée' : 'ouverte'} · {selected.meta.status}
                </div>
              )}
            </div>

            <div className="flex-1 overflow-y-auto px-4 pt-3.5 pb-7 flex flex-col gap-4">
              {displayMode === 'execution' ? (
                <>
                  {/* Prototypage : ajouter une story a la volée, ou pousser un
                      ticket existant dans la macro. */}
                  <div>
                    <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                      Composer la macro
                    </div>

                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={newStory}
                        onChange={e => setNewStory(e.target.value)}
                        onKeyDown={async e => {
                          if (e.key === 'Enter' && newStory.trim() && currentProject?.id) {
                            e.preventDefault()
                            setBusyKey('new')
                            await createStoryUnderMacro(currentProject.id, selected.key, newStory.trim())
                            setNewStory('')
                            setBusyKey(null)
                          }
                        }}
                        placeholder={`Nouvelle story sous ${selected.key}…`}
                        className="flex-1 px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <button
                        type="button"
                        disabled={!newStory.trim() || busyKey === 'new' || !currentProject?.id}
                        onClick={async () => {
                          setBusyKey('new')
                          await createStoryUnderMacro(currentProject!.id, selected.key, newStory.trim())
                          setNewStory('')
                          setBusyKey(null)
                        }}
                        className="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0"
                      >
                        <Plus size={12} /> {busyKey === 'new' ? '…' : 'Créer'}
                      </button>
                    </div>

                    <div className="mt-2">
                      <input
                        type="text"
                        value={attachQuery}
                        onChange={e => setAttachQuery(e.target.value)}
                        placeholder="Pousser un ticket existant : clé ou titre…"
                        className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      {attachCandidates.length > 0 && (
                        <div className="mt-1.5 flex flex-col gap-1">
                          {attachCandidates.map(candidate => (
                            <button
                              key={candidate.id}
                              type="button"
                              disabled={busyKey === candidate.id}
                              onClick={async () => {
                                setBusyKey(candidate.id)
                                await setTaskMacro(candidate.id, selected.key)
                                setBusyKey(null)
                                setAttachQuery('')
                              }}
                              className="flex items-center gap-2 px-2 py-1.5 rounded-lg text-left bg-[var(--bg-primary)] border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 cursor-pointer disabled:opacity-50"
                            >
                              <span className="text-[10px] font-mono font-bold shrink-0" style={{ color: 'var(--status-info)' }}>
                                {candidate.key}
                              </span>
                              <span className="text-[11px] truncate flex-1 text-[var(--text-secondary)]">{candidate.title}</span>
                              {candidate.parentKey && (
                                <span className="text-[9px] font-mono shrink-0 text-[var(--text-muted)]" title="Macro actuelle, qui sera remplacée">
                                  {candidate.parentKey} →
                                </span>
                              )}
                              <Plus size={11} className="shrink-0 text-[var(--text-muted)]" />
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <div className="px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <div className="text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">
                        {tab === 'now' ? 'Dans un sprint actif' : 'Dans un sprint à venir'}
                      </div>
                      <div className="text-[15px] font-bold mt-0.5" style={{ color: 'var(--status-ok)' }}>
                        {tab === 'now' ? selected.inActiveSprint.length : selected.inFutureSprint.length}
                        <span className="text-[11px] font-normal text-[var(--text-muted)]"> / {selected.open.length} ouverts</span>
                      </div>
                    </div>
                    <div className="px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <div className="text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">À corriger</div>
                      <div className="text-[15px] font-bold mt-0.5"
                        style={{ color: placementIssues(selected, horizonOfTab).length ? 'var(--status-danger)' : 'var(--status-ok)' }}>
                        {placementIssues(selected, horizonOfTab).length}
                      </div>
                    </div>
                  </div>

                  <div>
                    <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                      Stories et sprints ({selected.open.length} ouvertes)
                    </div>
                    <div className="flex flex-col gap-1.5">
                      {selected.open.length === 0 && (
                        <p className="text-[11px] text-[var(--text-muted)]">
                          Aucune story ouverte : cette macro ressemble plutôt à du LATER.
                        </p>
                      )}
                      {orderedOpen.map((task, index) => {
                        const state = placementOf(task, selected, horizonOfTab)
                        const meta = PLACEMENT_META[state]
                        const beyondCut = cutAt >= 0 && index >= cutAt
                        const sprintLabel = sprintLabelOf(task)
                        const startsSprint = index === 0 || sprintLabelOf(orderedOpen[index - 1]) !== sprintLabel
                        return (
                          <React.Fragment key={task.id}>
                            {startsSprint && (
                              <div className="flex items-center gap-1.5 mt-1 first:mt-0 pl-[20px]">
                                <span className="text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">
                                  {sprintLabel}
                                </span>
                                <span className="flex-1 h-px bg-[var(--border-color)]" />
                              </div>
                            )}
                          <div
                            ref={el => { rowRefs.current[index] = el }}
                            className="grid gap-1.5"
                            style={{ gridTemplateColumns: '14px 1fr' }}
                          >
                            {/* Gouttière de coupe : le cran est à la frontière
                                haute de la ligne, donc l'alignement suit les
                                lignes quel que soit leur hauteur. */}
                            <div className="relative">
                              <span
                                className="absolute left-1/2 top-0 bottom-0 w-px -translate-x-1/2"
                                style={{ background: beyondCut ? 'var(--accent-color)' : 'var(--border-color)' }}
                              />
                              <button
                                type="button"
                                onPointerDown={e => {
                                  e.preventDefault()
                                  setCutAt(index)
                                  setDraggingCut(true)
                                }}
                                onKeyDown={e => {
                                  if (e.key === 'ArrowUp') { e.preventDefault(); setCutAt(Math.max(0, index - 1)) }
                                  if (e.key === 'ArrowDown') {
                                    e.preventDefault()
                                    const next = index + 1
                                    setCutAt(next >= orderedOpen.length ? -1 : next)
                                  }
                                }}
                                className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full cursor-ns-resize"
                                style={{
                                  top: 0,
                                  width: cutAt === index ? 11 : 7,
                                  height: cutAt === index ? 11 : 7,
                                  background: cutAt === index ? 'var(--accent-color)' : 'var(--bg-secondary)',
                                  border: `1px solid ${cutAt === index ? 'var(--accent-color)' : 'var(--border-color)'}`,
                                }}
                                title={`Couper ici : ${orderedOpen.length - index} ticket(s) partent`}
                                aria-label={`Couper avant ${task.key}`}
                              />
                            </div>

                          <div className="px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border"
                            style={{
                              borderColor: beyondCut
                                ? 'var(--accent-color)'
                                : state === 'ok'
                                  ? 'var(--border-color)'
                                  : meta.border,
                              opacity: cutAt >= 0 && !beyondCut ? 0.55 : 1,
                            }}>
                            <div className="flex items-center gap-2">
                              <button
                                type="button"
                                onClick={() => setChecked(prev => ({ ...prev, [task.id]: !prev[task.id] }))}
                                className="w-3.5 h-3.5 rounded shrink-0 flex items-center justify-center cursor-pointer"
                                style={{
                                  background: checked[task.id] ? 'var(--accent-color)' : 'transparent',
                                  border: `1px solid ${checked[task.id] ? 'var(--accent-color)' : 'var(--border-color)'}`,
                                }}
                                title="Sélectionner pour déplacer vers une autre macro"
                              >
                                {checked[task.id] && <Check size={10} className="text-white" />}
                              </button>
                              <button type="button" onClick={() => setSelectedTask(task)}
                                className="text-[10.5px] font-mono font-bold hover:underline cursor-pointer"
                                style={{ color: 'var(--status-info)' }}>
                                {task.key}
                              </button>
                              {sprintOptions.length > 0 ? (
                                <div className="ml-auto shrink-0 w-[165px]">
                                  <LookupField
                                    value={task.sprint || ''}
                                    icon={<CalendarRange size={10} />}
                                    placeholder={meta.label}
                                    clearLabel="Backlog (aucun sprint)"
                                    onSearch={searchSprint}
                                    onPick={option => setTaskSprint(task.id, option?.id || '', option?.label)}
                                  />
                                </div>
                              ) : (
                                <span className="text-[9px] px-1 rounded font-mono ml-auto shrink-0 truncate max-w-[150px]"
                                  style={{ color: meta.color, background: meta.bg, border: `1px solid ${meta.border}` }}>
                                  {task.sprint || meta.label}
                                </span>
                              )}

                              <button
                                type="button"
                                disabled={busyKey === task.id}
                                onClick={async () => {
                                  setBusyKey(task.id)
                                  await setTaskMacro(task.id, '')
                                  setBusyKey(null)
                                }}
                                className="p-1 rounded text-[var(--text-muted)] hover:text-rose-400 cursor-pointer disabled:opacity-50"
                                title={`Retirer ${task.key} de la macro`}
                              >
                                <X size={12} />
                              </button>
                            </div>
                            <div className="text-[11px] mt-1 leading-snug text-[var(--text-secondary)]">{task.title}</div>
                            {state !== 'ok' && (
                              <div className="text-[9.5px] mt-1 font-mono" style={{ color: meta.color }}>
                                {state === 'missing' && 'Aucun sprint : à placer'}
                                {state === 'stale' && 'Sprint clos ou inconnu du board'}
                                {state === 'other-horizon' &&
                                  (tab === 'now' ? 'Dans un sprint futur, pas actif' : 'Dans un sprint actif, pas futur')}
                              </div>
                            )}
                          </div>
                          </div>
                          </React.Fragment>
                        )
                      })}
                    </div>
                  </div>

                  {/* Résumé de la coupe. Le curseur lui-même est vertical, dans
                      la gouttière de la liste : couper se lit alors comme une
                      ligne tracée entre deux tickets, et non comme un rail
                      horizontal détaché de ce qu'il découpe. */}
                  {orderedOpen.length > 1 && (
                    <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <Scissors size={11} style={{ color: cutAt >= 0 ? 'var(--accent-color)' : 'var(--text-muted)' }} />
                      <span className="text-[10px] text-[var(--text-secondary)]">
                        {cutAt < 0
                          ? 'Fais glisser un cran dans la marge pour couper'
                          : `${orderedOpen.length - cutAt} ticket(s) à partir de ${sprintLabelOf(orderedOpen[cutAt])}`}
                      </span>
                      {cutAt >= 0 && (
                        <button
                          type="button"
                          onClick={() => setCutAt(-1)}
                          className="ml-auto text-[10px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
                        >
                          Annuler la coupe
                        </button>
                      )}
                    </div>
                  )}

                  {/* Planification : la même sélection sert à replanifier, ce qui
                      est l'autre moitié du travail sur un épic trop gros. Couper
                      change de contenant, changer de sprint change la date. */}
                  {cutIds.length > 0 && sprintOptions.length > 0 && (
                    <div className="p-2.5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-primary)]">
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] mb-1.5 text-[var(--text-muted)]">
                        Replanifier {cutIds.length} ticket(s) dans…
                      </div>
                      <div className="flex items-center gap-2">
                        <div className="flex-1">
                          <LookupField
                            value={sprintTarget.name}
                            icon={<CalendarRange size={11} />}
                            placeholder="sprint cible..."
                            clearLabel="Backlog (retirer du sprint)"
                            onSearch={searchSprint}
                            onPick={option =>
                              setSprintTarget({ id: option?.id || '', name: option?.label || 'Backlog' })
                            }
                          />
                        </div>
                        <button
                          type="button"
                          disabled={!sprintTarget.name || busyKey === 'sprint'}
                          onClick={async () => {
                            setBusyKey('sprint')
                            await setTasksSprint(
                              currentProject!.id,
                              cutIds,
                              sprintTarget.id,
                              sprintTarget.name
                            )
                            setChecked({})
                            setCutAt(-1)
                            setSprintTarget({ id: '', name: '' })
                            setBusyKey(null)
                          }}
                          className="px-2.5 py-1.5 rounded-lg text-[11px] font-bold cursor-pointer shrink-0 disabled:opacity-40"
                          style={{ color: 'var(--status-ok)', background: 'rgb(var(--status-ok-rgb) / 0.12)', border: '1px solid rgb(var(--status-ok-rgb) / 0.32)' }}
                          title="Déplacer la sélection dans ce sprint, via la file d'activités"
                        >
                          {busyKey === 'sprint' ? '…' : 'Replanifier'}
                        </button>
                      </div>
                    </div>
                  )}

                  {/* Découpe : les stories cochées, ou celles au delà du
                      curseur, quittent la macro pour une autre. */}
                  {cutIds.length > 0 && (
                    <div className="p-2.5 rounded-xl border" style={{ background: 'var(--accent-light)', borderColor: 'rgb(var(--accent-rgb) / 0.4)' }}>
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] mb-1.5" style={{ color: 'var(--accent-color)' }}>
                        Couper {cutIds.length} ticket(s) vers…
                      </div>
                      <div className="flex items-center gap-2">
                        <div className="flex-1">
                          <LookupField
                            value={moveTarget}
                            icon={<Target size={11} />}
                            placeholder="macro existante..."
                            allowClear={false}
                            emptyHint="Aucune macro ne correspond."
                            onSearch={async query => {
                              const q = query.trim().toLowerCase()
                              return allRows
                                .filter(r => r.key !== selected.key && !r.closed)
                                .filter(r => !q || r.key.toLowerCase().includes(q) || r.title.toLowerCase().includes(q))
                                .slice(0, 40)
                                .map(r => ({ id: r.key, label: r.key, sublabel: r.title }))
                            }}
                            onPick={option => setMoveTarget(option?.id || '')}
                          />
                        </div>
                        <button
                          type="button"
                          disabled={!moveTarget || busyKey === 'move'}
                          onClick={async () => {
                            setBusyKey('move')
                            await moveTasksToMacro(currentProject!.id, cutIds, moveTarget)
                            setChecked({})
                            setCutAt(-1)
                            setBusyKey(null)
                          }}
                          className="px-2.5 py-1.5 rounded-lg text-[11px] font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0"
                        >
                          Déplacer
                        </button>
                      </div>
                      <div className="flex items-center gap-2 mt-2">
                        <input
                          type="text"
                          value={newMacroTitle}
                          onChange={e => setNewMacroTitle(e.target.value)}
                          placeholder="…ou vers une nouvelle macro : son titre"
                          className="flex-1 px-2 py-1.5 text-[11px] rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                        />
                        <button
                          type="button"
                          disabled={!newMacroTitle.trim() || busyKey === 'move'}
                          onClick={async () => {
                            setBusyKey('move')
                            await moveTasksToMacro(currentProject!.id, cutIds, '', newMacroTitle.trim())
                            setChecked({})
                            setCutAt(-1)
                            setNewMacroTitle('')
                            setBusyKey(null)
                          }}
                          title="Créer la macro cible et y déplacer les tickets sélectionnés"
                          className="px-2.5 py-1.5 rounded-lg text-[11px] font-bold cursor-pointer shrink-0 disabled:opacity-40"
                          style={{ color: 'var(--status-info)', background: 'rgb(var(--status-info-rgb) / 0.12)', border: '1px solid rgb(var(--status-info-rgb) / 0.32)' }}
                        >
                          {busyKey === 'move' ? '…' : 'Créer et couper'}
                        </button>
                      </div>
                    </div>
                  )}

</>
              ) : (
                <>
                  {/* Mode Framing : description de la macro, commentaire framing et checklist TODO / tickets */}
                  <div className="space-y-3">
                    {/* Textbox 1 : Description de la Macro */}
                    <div className="rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] overflow-hidden">
                      <div
                        className="flex items-center justify-between p-3 cursor-pointer select-none bg-[var(--bg-tertiary)]/40 hover:bg-[var(--bg-tertiary)]/70 transition-colors"
                        onClick={() => setIsDescExpanded(prev => !prev)}
                      >
                        <div className="flex items-center gap-1.5 min-w-0">
                          {isDescExpanded ? <ChevronDown size={14} className="text-[var(--accent-color)] shrink-0" /> : <ChevronRight size={14} className="text-[var(--text-muted)] shrink-0" />}
                          <span className="text-[11px] font-bold uppercase tracking-[.08em] text-[var(--text-primary)] shrink-0">
                            Description de la Macro
                          </span>
                          {!isDescExpanded && draftDescription.trim() && (
                            <span className="text-[10px] text-[var(--text-muted)] italic truncate max-w-[280px]">
                              — {draftDescription.trim().slice(0, 50)}…
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0" onClick={e => e.stopPropagation()}>
                          <button
                            type="button"
                            disabled={isRefining}
                            onClick={handleRefineMacro}
                            className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-orange-300 bg-orange-500/10 border border-orange-500/30 hover:bg-orange-500/20 disabled:opacity-50 cursor-pointer"
                            title="Raffiner le cadrage avec l'IA pour générer la checklist de TODOs"
                          >
                            {isRefining ? <Loader2 size={10} className="animate-spin text-orange-400" /> : <Sparkles size={10} className="text-orange-400" />}
                            <span>Raffiner AI</span>
                          </button>
                          {draftDirty && (
                            <button
                              type="button"
                              onClick={() => {
                                persist(selected.key, { description: draftDescription })
                                setDraftDirty(false)
                              }}
                              className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-white accent-bg cursor-pointer"
                            >
                              <Save size={10} /> Enregistrer
                            </button>
                          )}
                        </div>
                      </div>

                      {isDescExpanded && (
                        <div className="p-3 pt-1 border-t border-[var(--border-color)]">
                          <MarkdownEditor
                            value={draftDescription}
                            onChange={value => {
                              setDraftDescription(value)
                              setDraftDirty(true)
                            }}
                            minHeight={120}
                            placeholder="Le problème, le périmètre, la valeur attendue, ce qui est hors périmètre… Ce cadrage vit dans TaskFlow."
                          />
                        </div>
                      )}
                    </div>

                    {/* Textbox 2 : Commentaire Framing / Notes de Cadrage */}
                    <div className="rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] overflow-hidden">
                      <div
                        className="flex items-center justify-between p-3 cursor-pointer select-none bg-[var(--bg-tertiary)]/40 hover:bg-[var(--bg-tertiary)]/70 transition-colors"
                        onClick={() => setIsFramingExpanded(prev => !prev)}
                      >
                        <div className="flex items-center gap-1.5 min-w-0">
                          {isFramingExpanded ? <ChevronDown size={14} className="text-amber-400 shrink-0" /> : <ChevronRight size={14} className="text-[var(--text-muted)] shrink-0" />}
                          <span className="text-[11px] font-bold uppercase tracking-[.08em] text-[var(--text-primary)] flex items-center gap-1 shrink-0">
                            <MessageSquare size={12} className="text-amber-400" />
                            <span>Commentaire Framing / Notes de Cadrage</span>
                          </span>
                          {!isFramingExpanded && draftFramingComment.trim() && (
                            <span className="text-[10px] text-[var(--text-muted)] italic truncate max-w-[280px]">
                              — {draftFramingComment.trim().slice(0, 50)}…
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0" onClick={e => e.stopPropagation()}>
                          {draftFramingDirty && (
                            <button
                              type="button"
                              onClick={() => {
                                persist(selected.key, { framingComment: draftFramingComment })
                                setDraftFramingDirty(false)
                              }}
                              className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-white accent-bg cursor-pointer"
                            >
                              <Save size={10} /> Enregistrer
                            </button>
                          )}
                        </div>
                      </div>

                      {isFramingExpanded && (
                        <div className="p-3 pt-1 border-t border-[var(--border-color)]">
                          <MarkdownEditor
                            value={draftFramingComment}
                            onChange={value => {
                              setDraftFramingComment(value)
                              setDraftFramingDirty(true)
                            }}
                            minHeight={100}
                            placeholder="Commentaires et consignes de cadrage pour l'équipe ou l'agent (ex: architecture, contraintes techniques, notes de révision)..."
                          />
                        </div>
                      )}
                    </div>
                  </div>

                  <div>
                    <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                      Checklist TODOs ({todosOf(selected).filter(t => t.done).length}/{todosOf(selected).length})
                    </div>
                    <div className="flex flex-col gap-1.5">
                      {todosOf(selected).map(todo => (
                        <div key={todo.id} className="flex items-start gap-2 px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border"
                          style={{ borderColor: todo.done ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)' }}>
                          <button
                            type="button"
                            onClick={() =>
                              persist(selected.key, {
                                todos: todosOf(selected).map(t => (t.id === todo.id ? { ...t, done: !t.done } : t)),
                              })
                            }
                            className="w-3.5 h-3.5 mt-0.5 rounded shrink-0 flex items-center justify-center cursor-pointer"
                            style={{
                              background: todo.done ? 'var(--accent-color)' : 'transparent',
                              border: `1px solid ${todo.done ? 'var(--accent-color)' : 'var(--border-color)'}`,
                            }}
                            title={todo.done ? 'Rouvrir' : 'Cocher'}
                          >
                            {todo.done && <Check size={10} className="text-white" />}
                          </button>
                          <span className="text-[11.5px] leading-snug flex-1"
                            style={{
                              color: todo.done ? 'var(--text-muted)' : 'var(--text-primary)',
                              textDecoration: todo.done ? 'line-through' : 'none',
                            }}>
                            {todo.text}
                          </span>
                          {todo.storyKey ? (
                            <button
                              type="button"
                              onClick={() => {
                                const created = tasks.find(t => t.key === todo.storyKey)
                                if (created) setSelectedTask(created)
                              }}
                              className="text-[9.5px] font-mono font-bold px-1.5 py-0.5 rounded shrink-0 cursor-pointer"
                              style={{
                                color: 'var(--status-ok)',
                                background: 'rgb(var(--status-ok-rgb) / 0.13)',
                                border: '1px solid rgb(var(--status-ok-rgb) / 0.32)',
                              }}
                              title={`Story créée : ${todo.storyKey}`}
                            >
                              {todo.storyKey}
                            </button>
                          ) : (
                            <button
                              type="button"
                              disabled={creatingTodoId === todo.id}
                              onClick={async () => {
                                setCreatingTodoId(todo.id)
                                const result = await createStoryFromMacroTodo(currentProject!.id, selected.key, todo.id)
                                setCreatingTodoId(null)
                                if (result?.macro) {
                                  setMacroMeta(prev => [...prev.filter(m => m.key !== result.macro!.key), result.macro!])
                                }
                              }}
                              className="text-[9.5px] font-mono font-bold px-1.5 py-0.5 rounded shrink-0 cursor-pointer disabled:opacity-50"
                              style={{
                                color: 'var(--status-info)',
                                background: 'rgb(var(--status-info-rgb) / 0.12)',
                                border: '1px solid rgb(var(--status-info-rgb) / 0.32)',
                              }}
                              title={`Créer une story sous ${selected.key}`}
                            >
                              {creatingTodoId === todo.id ? '…' : 'Créer story'}
                            </button>
                          )}
                          <button
                            type="button"
                            onClick={() => persist(selected.key, { todos: todosOf(selected).filter(t => t.id !== todo.id) })}
                            className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 cursor-pointer shrink-0"
                            title="Retirer"
                          >
                            <Trash2 size={11} />
                          </button>
                        </div>
                      ))}
                      {todosOf(selected).length === 0 && (
                        <p className="text-[11px] text-[var(--text-muted)]">
                          Aucune ligne. Écris ici ce qu'il faudra faire, avant même de créer des stories.
                        </p>
                      )}
                    </div>

                    <div className="flex items-center gap-2 mt-2">
                      <input
                        type="text"
                        value={newTodo}
                        onChange={e => setNewTodo(e.target.value)}
                        onKeyDown={e => {
                          if (e.key === 'Enter') {
                            e.preventDefault()
                            addTodo(selected)
                          }
                        }}
                        placeholder="Ajouter une ligne de TODO…"
                        className="flex-1 px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <button type="button" onClick={() => addTodo(selected)} disabled={!newTodo.trim()}
                        className="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0">
                        <Plus size={12} /> Ajouter
                      </button>
                    </div>
                  </div>

                  {selected.tasks.length > 0 && (
                    <div className="pt-2 border-t border-[var(--border-color)]">
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                        Tickets créés sous cette macro ({selected.tasks.length})
                      </div>
                      <div className="flex flex-wrap gap-1.5">
                        {selected.tasks.map(task => (
                          <button
                            key={task.id}
                            type="button"
                            onClick={() => setSelectedTask(task)}
                            className="text-[10.5px] font-mono px-2 py-1 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--accent-color)]/40 cursor-pointer flex items-center gap-1.5 transition-colors"
                            title={task.title}
                          >
                            <span style={{ color: 'var(--status-info)' }}>{task.key}</span>
                            <span className="truncate max-w-[200px]">{task.title}</span>
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>
          </aside>
        )}
      </div>

      {showCreateMacroModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
          <div className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl w-full max-w-md shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 text-sm font-bold text-[var(--text-primary)]">
                <Target size={16} className="text-[var(--accent-color)]" />
                <span>Créer une Macro (Milestone)</span>
              </div>
              <button
                type="button"
                onClick={() => setShowCreateMacroModal(false)}
                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              >
                <X size={16} />
              </button>
            </div>

            <form
              onSubmit={async (e) => {
                e.preventDefault()
                const targetProjId = createMacroProjectId || currentProject?.id || projects[0]?.id
                if (!createMacroTitle.trim() || !targetProjId) return
                setBusyKey('macro')
                const created = await createMacro(
                  targetProjId,
                  createMacroTitle.trim(),
                  createMacroHorizon
                )
                if (created) {
                  setMacroMeta(prev => [...prev.filter(m => m.key !== created.key), created])
                  if (createMacroHorizon) {
                    setTab(createMacroHorizon)
                  }
                  setSelectedKey(created.key)
                  setShowCreateMacroModal(false)
                }
                setBusyKey(null)
              }}
              className="p-5 space-y-4"
            >
              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  Titre de la macro / milestone
                </label>
                <input
                  type="text"
                  autoFocus
                  value={createMacroTitle}
                  onChange={(e) => setCreateMacroTitle(e.target.value)}
                  placeholder="Ex : Refonte API v2, Authentification SSO, Q3 Release…"
                  className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                    Horizon
                  </label>
                  <select
                    value={createMacroHorizon}
                    onChange={(e) => setCreateMacroHorizon(e.target.value as MacroHorizon)}
                    className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                  >
                    <option value="now">NOW (En cours)</option>
                    <option value="next">NEXT (À venir)</option>
                    <option value="later">LATER (Plus tard / Design)</option>
                  </select>
                </div>

                {projects.length > 1 && (
                  <div>
                    <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                      Projet
                    </label>
                    <select
                      value={createMacroProjectId}
                      onChange={(e) => setCreateMacroProjectId(e.target.value)}
                      className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                    >
                      {projects.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                    </select>
                  </div>
                )}
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateMacroModal(false)}
                  className="px-3 py-1.5 text-xs font-semibold rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={!createMacroTitle.trim() || busyKey === 'macro'}
                  className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-bold text-white accent-bg rounded-xl cursor-pointer disabled:opacity-50"
                >
                  {busyKey === 'macro' ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
                  Créer la macro
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showMigrateModal && selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
          <div className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl w-full max-w-md shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 text-sm font-bold text-[var(--text-primary)]">
                <ArrowRightLeft size={16} className="text-[var(--accent-color)]" />
                <span>Migrer la Macro ({selected.key})</span>
              </div>
              <button
                type="button"
                onClick={() => setShowMigrateModal(false)}
                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              >
                <X size={16} />
              </button>
            </div>

            <form
              onSubmit={async (e) => {
                e.preventDefault()
                if (!migrateTargetProjectId || !currentProject?.id) return
                setIsMigrating(true)
                const res = await migrateMacro(
                  currentProject.id,
                  selected.key,
                  migrateTargetProjectId,
                  migrateIncludeTasks
                )
                setIsMigrating(false)
                if (res.success) {
                  setShowMigrateModal(false)
                  setSelectedKey(null)
                }
              }}
              className="p-5 space-y-4"
            >
              <div>
                <div className="text-xs text-[var(--text-muted)] mb-1 font-medium">Macro à déplacer :</div>
                <div className="p-3 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
                  <div className="text-xs font-bold text-[var(--text-primary)]">{selected.title}</div>
                  <div className="text-[11px] font-mono text-[var(--text-muted)] mt-0.5">
                    Projet actuel : {currentProject?.name} · {selected.tasks.length} ticket(s) associé(s)
                  </div>
                </div>
              </div>

              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  Projet de destination (compatible)
                </label>
                {projects.filter(p => isProjectCompatible(currentProject, p)).length > 0 ? (
                  <select
                    value={migrateTargetProjectId}
                    onChange={(e) => setMigrateTargetProjectId(e.target.value)}
                    className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                  >
                    {projects.filter(p => isProjectCompatible(currentProject, p)).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} ({p.issueTracker || 'local'}{p.githubRepo ? ` · ${p.githubRepo}` : ''})
                      </option>
                    ))}
                  </select>
                ) : (
                  <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs">
                    Aucun autre projet compatible trouvé (trackers compatibles requis).
                  </div>
                )}
              </div>

              {selected.tasks.length > 0 && (
                <label className="flex items-center gap-2.5 p-3 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] cursor-pointer">
                  <input
                    type="checkbox"
                    checked={migrateIncludeTasks}
                    onChange={(e) => setMigrateIncludeTasks(e.target.checked)}
                    className="rounded accent-[var(--accent-color)]"
                  />
                  <span className="text-xs text-[var(--text-primary)] font-medium">
                    Transférer aussi les <strong>{selected.tasks.length} ticket(s)</strong> rattaché(s) à cette macro
                  </span>
                </label>
              )}

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowMigrateModal(false)}
                  className="px-3 py-1.5 text-xs font-semibold rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={!migrateTargetProjectId || isMigrating}
                  className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-bold text-white accent-bg rounded-xl cursor-pointer disabled:opacity-50"
                >
                  {isMigrating ? <Loader2 size={13} className="animate-spin" /> : <ArrowRightLeft size={13} />}
                  <span>Confirmer la migration</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal d'aperçu du raffinage de macro (AI) */}
      {refinePreview && selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs">
          <div className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl p-5 max-w-lg w-full shadow-2xl flex flex-col max-h-[85vh]">
            <div className="flex items-start justify-between border-b border-[var(--border-color)] pb-3">
              <div>
                <div className="flex items-center gap-2">
                  <Sparkles size={16} className="text-orange-400" />
                  <h3 className="text-sm font-bold text-[var(--text-primary)]">
                    Raffinage de la macro {selected.key}
                  </h3>
                  <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${
                    refinePreview.specFramework === 'openspec'
                      ? 'bg-purple-500/10 text-purple-400 border border-purple-500/30'
                      : 'bg-blue-500/10 text-blue-400 border border-blue-500/30'
                  }`}>
                    {refinePreview.specFramework === 'openspec' ? 'OpenSpec SDD' : 'SpecKit SDD'}
                  </span>
                </div>
                <p className="text-[11px] text-[var(--text-muted)] mt-1">
                  {refinePreview.todos.length} item(s) de TODO généré(s) d'après le texte de cadrage.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setRefinePreview(null)}
                className="text-[var(--text-muted)] hover:text-[var(--text-primary)] p-1 rounded-lg cursor-pointer"
              >
                <X size={14} />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto py-3 space-y-3 my-2 pr-1">
              {refinePreview.todos && refinePreview.todos.length > 0 && (
                <div className="space-y-1.5">
                  <span className="text-xs font-bold text-[var(--text-muted)] flex items-center gap-1">
                    <ListChecks size={13} className="text-orange-400" />
                    Checklist TODOs de la macro ({refinePreview.todos.length})
                  </span>
                  {refinePreview.todos.map((todo, idx) => (
                    <div key={todo.id || idx} className="flex items-start gap-2 p-2 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <ListChecks size={13} className="text-orange-400 shrink-0 mt-0.5" />
                      <span className="text-[11.5px] leading-snug text-[var(--text-primary)] flex-1 font-mono">
                        {todo.text}
                      </span>
                    </div>
                  ))}
                </div>
              )}

              {refinePreview.proposedTasks && refinePreview.proposedTasks.length > 0 && (
                <div className="pt-2 border-t border-[var(--border-color)] space-y-1.5">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[var(--text-primary)] flex items-center gap-1.5">
                      <FolderGit2 size={13} className="text-cyan-400" />
                      Tickets TaskFlow déduits ({refinePreview.proposedTasks.length})
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        const total = refinePreview.proposedTasks?.length || 0
                        const allChecked = Object.keys(selectedProposedTasks).length === total
                        const newMap: Record<number, boolean> = {}
                        if (!allChecked) {
                          refinePreview.proposedTasks?.forEach((_, idx) => { newMap[idx] = true })
                        }
                        setSelectedProposedTasks(newMap)
                      }}
                      className="text-[10px] font-bold text-[var(--accent-color)] hover:underline cursor-pointer"
                    >
                      Tout sélectionner / Déselectionner
                    </button>
                  </div>
                  <div className="space-y-1.5 max-h-52 overflow-y-auto">
                    {refinePreview.proposedTasks.map((pt, idx) => (
                      <label
                        key={idx}
                        className="flex items-start gap-2.5 p-2 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] hover:border-cyan-500/30 cursor-pointer select-none"
                      >
                        <input
                          type="checkbox"
                          checked={Boolean(selectedProposedTasks[idx])}
                          onChange={e => setSelectedProposedTasks(prev => ({ ...prev, [idx]: e.target.checked }))}
                          className="mt-0.5 rounded border-[var(--border-color)] text-cyan-500 focus:ring-0 cursor-pointer"
                        />
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1.5">
                            <span className={`text-[9px] font-bold px-1.5 py-0.2 rounded uppercase ${
                              pt.issueType === 'Bug'
                                ? 'bg-rose-500/10 text-rose-400 border border-rose-500/30'
                                : pt.issueType === 'Task'
                                ? 'bg-indigo-500/10 text-indigo-400 border border-indigo-500/30'
                                : 'bg-blue-500/10 text-blue-400 border border-blue-500/30'
                            }`}>
                              {pt.issueType}
                            </span>
                            <span className="text-xs font-semibold text-[var(--text-primary)] truncate">{pt.title}</span>
                          </div>
                          {pt.description && (
                            <p className="text-[10.5px] text-[var(--text-muted)] line-clamp-1 mt-0.5">{pt.description}</p>
                          )}
                        </div>
                      </label>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-[var(--border-color)] pt-3 mt-auto flex-wrap">
              <button
                type="button"
                onClick={() => setRefinePreview(null)}
                className="px-3 py-1.5 rounded-xl text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] cursor-pointer"
              >
                Annuler
              </button>
              {refinePreview.todos && refinePreview.todos.length > 0 && (
                <button
                  type="button"
                  onClick={async () => {
                    const existing = todosOf(selected)
                    await persist(selected.key, { todos: [...existing, ...refinePreview.todos] })
                    setRefinePreview(null)
                    addToast({
                      type: 'success',
                      title: 'TODOs ajoutés',
                      description: `${refinePreview.todos.length} item(s) ajoutés à la checklist de ${selected.key}`,
                    })
                  }}
                  className="px-3 py-1.5 rounded-xl text-xs font-bold text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:border-[var(--accent-color)] cursor-pointer"
                >
                  Ajouter aux TODOs
                </button>
              )}
              {refinePreview.proposedTasks && refinePreview.proposedTasks.length > 0 && (
                <button
                  type="button"
                  disabled={isCreatingBatch}
                  onClick={async () => {
                    const selectedTasksToCreate = refinePreview.proposedTasks?.filter((_, idx) => selectedProposedTasks[idx]) || []
                    if (selectedTasksToCreate.length === 0) {
                      addToast({ type: 'warning', title: 'Aucun ticket sélectionné', description: 'Veuillez cocher au moins un ticket à générer.' })
                      return
                    }
                    setIsCreatingBatch(true)
                    const payload = selectedTasksToCreate.map(pt => ({
                      projectId: currentProject?.id || selected.meta?.projectId || '',
                      title: pt.title,
                      issueType: pt.issueType,
                      description: pt.description,
                      parentKey: selected.key,
                      parentTitle: selected.title,
                      parentType: 'Macro',
                      status: 'backlog' as const,
                    }))
                    await createBatchTasks(payload)
                    setIsCreatingBatch(false)
                    setRefinePreview(null)
                  }}
                  className="px-3.5 py-1.5 rounded-xl text-xs font-bold text-white bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 cursor-pointer flex items-center gap-1.5"
                >
                  {isCreatingBatch ? <Loader2 size={13} className="animate-spin" /> : <FolderGit2 size={13} />}
                  <span>Générer les tickets TaskFlow ({Object.values(selectedProposedTasks).filter(Boolean).length})</span>
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
