import React, { useEffect, useState } from 'react'
import {
  HelpCircle,
  FileCode,
  Flame,
  ShieldCheck,
  CheckCircle2,
  Plus,
  Eye,
  EyeOff,
  Sparkles,
  Kanban,
  ListFilter,
  List,
  X
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { TaskCard } from './TaskCard'
import { TaskFilters } from './TaskFilters'
import type { Task, Status, WorkflowStage } from '../types'
import { resolveTaskStage, stageFromLabels } from '../lib/workflow'
import { BoardGroupingToggle } from './BoardGroupingToggle'
import { BoardSortSelect } from './BoardSortSelect'
import { sortTasks } from '../lib/boardSort'
import {
  isSelectableStage,
  orderSelection,
  pruneSelection,
  shouldEscapeClearSelection,
  toggleSelected,
} from '../lib/boardSelection'

interface WorkflowColumnConfig {
  id: WorkflowStage
  title: string
  stageLabel: string
  stageColor: string
  icon: React.ReactNode
  color: string
  borderColor: string
  badgeBg: string
  description: string
}

interface StatusColumnConfig {
  id: Status
  title: string
  stageLabel: string
  stageColor: string
  icon: React.ReactNode
  color: string
  borderColor: string
  badgeBg: string
  description: string
}

// Conservée pour l'API publique du module : la déduction par labels vit
// désormais dans lib/workflow, aux côtés de la déduction par colonne.
export const getTaskWorkflowStage = stageFromLabels

export const BoardView: React.FC = () => {
  const {
    tasks,
    activeOnly,
    setActiveOnly,
    activeTasks,
    moveTask,
    moveTaskWorkflowStage,
    boardGrouping,
    boardSort,
    boardCardDisplayMode,
    toggleBoardCardDisplayMode,
    hideDone,
    toggleHideDone,
    currentProject,
    moveTaskToTrackerStatus,
    setIsQuickAddOpen,
    setQuickAddInitialStatus,
    startBatchPickup,
    selectedTask,
    selectedActivity,
    isQuickAddOpen,
    isCommandPaletteOpen,
    isProfileOpen,
    searchQuery,
    t,
  } = useApp()

  const [showHiddenColumns, setShowHiddenColumns] = useState(false)
  const isCondensed = boardCardDisplayMode === 'condensed'
  const [dragOverColumn, setDragOverColumn] = useState<string | null>(null)
  const [draggingTaskId, setDraggingTaskId] = useState<string | null>(null)
  // The cards selected for a batch pickup (#109). Local to the board: no other
  // view reads it, and it never outlives the project it was made in.
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(() => new Set())
  const [launchingBatch, setLaunchingBatch] = useState(false)

  // Every column follows the sort chosen in the toolbar (#402). Drag and drop
  // never reorders inside a column — it changes the stage — so sorting here
  // overrides no manual order.
  const sortColumn = (list: Task[]) => sortTasks(list, boardSort)

  // -------------------------------------------------------------
  // MODE 1: Workflow Labels Columns (Pipeline IA)
  // new -> clarified -> specified -> implemented -> reviewed -> finished
  // -------------------------------------------------------------
  const workflowColumns: WorkflowColumnConfig[] = [
    {
      id: 'new',
      title: 'New',
      stageLabel: '#new',
      stageColor: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30',
      icon: <Sparkles size={16} className="text-cyan-400" />,
      color: 'text-cyan-400',
      borderColor: 'border-cyan-500/30',
      badgeBg: 'bg-cyan-500/20 text-cyan-300',
      description: 'Nouveaux tickets non encore traités par IA',
    },
    {
      id: 'clarified',
      title: 'Clarified',
      stageLabel: '#clarified',
      stageColor: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
      icon: <HelpCircle size={16} className="text-amber-400" />,
      color: 'text-amber-400',
      borderColor: 'border-amber-500/30',
      badgeBg: 'bg-amber-500/20 text-amber-300',
      description: 'Cadrage & Q&A validés',
    },
    {
      id: 'specified',
      title: 'Specified',
      stageLabel: '#specified',
      stageColor: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
      icon: <FileCode size={16} className="text-blue-400" />,
      color: 'text-blue-400',
      borderColor: 'border-blue-500/30',
      badgeBg: 'bg-blue-500/20 text-blue-300',
      description: 'Spécification technique prête',
    },
    {
      id: 'implemented',
      title: 'Implemented',
      stageLabel: '#implemented',
      stageColor: 'bg-indigo-500/15 text-indigo-400 border-indigo-500/30',
      icon: <Flame size={16} className="text-indigo-400" />,
      color: 'text-indigo-400',
      borderColor: 'border-indigo-500/30',
      badgeBg: 'bg-indigo-500/20 text-indigo-300',
      description: 'Code développé & commité',
    },
    {
      id: 'reviewed',
      title: 'Reviewed',
      stageLabel: '#reviewed',
      stageColor: 'bg-purple-500/15 text-purple-400 border-purple-500/30',
      icon: <ShieldCheck size={16} className="text-purple-400" />,
      color: 'text-purple-400',
      borderColor: 'border-purple-500/30',
      badgeBg: 'bg-purple-500/20 text-purple-300',
      description: 'Pull Request & revue prêtes',
    },
    {
      id: 'finished',
      title: 'Finished',
      stageLabel: '#finished',
      stageColor: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
      icon: <CheckCircle2 size={16} className="text-emerald-400" />,
      color: 'text-emerald-400',
      borderColor: 'border-emerald-500/30',
      badgeBg: 'bg-emerald-500/20 text-emerald-300',
      description: 'Tâche finalisée & mergée',
    },
  ]

  // -------------------------------------------------------------
  // MODE 2: Operational Status Columns (Kanban Agile)
  // Todo -> In Progress -> In Review & Testing -> Done
  // -------------------------------------------------------------
  const isTaskInStatusColumn = (taskStatus: Status, colId: Status) => {
    const st = (taskStatus || '').toLowerCase()
    if (colId === 'to_clarify') {
      return st === 'to_clarify' || st === 'backlog' || st === 'open' || st === 'todo' || st === 'new'
    }
    if (colId === 'in_progress') {
      return st === 'in_progress' || st === 'clarified' || st === 'to_implement' || st === 'specified'
    }
    if (colId === 'to_test') {
      return st === 'to_test' || st === 'to_validate' || st === 'to_close' || st === 'in_review' || st === 'testing'
    }
    if (colId === 'to_close' || colId === 'finished') {
      return st === 'finished' || st === 'done' || st === 'closed'
    }
    return st === colId
  }

  const statusColumns: StatusColumnConfig[] = [
    {
      id: 'to_clarify',
      title: 'Todo',
      stageLabel: 'Backlog',
      stageColor: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30',
      icon: <ListFilter size={16} className="text-cyan-400" />,
      color: 'text-cyan-400',
      borderColor: 'border-cyan-500/30',
      badgeBg: 'bg-cyan-500/20 text-cyan-300',
      description: 'Tâches à faire',
    },
    {
      id: 'in_progress',
      title: 'In Progress',
      stageLabel: 'En cours',
      stageColor: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
      icon: <Flame size={16} className="text-blue-400" />,
      color: 'text-blue-400',
      borderColor: 'border-blue-500/30',
      badgeBg: 'bg-blue-500/20 text-blue-300',
      description: 'En cours de dev / spec',
    },
    {
      id: 'to_test',
      title: 'In Review & Testing',
      stageLabel: 'Revue & Tests',
      stageColor: 'bg-purple-500/15 text-purple-400 border-purple-500/30',
      icon: <ShieldCheck size={16} className="text-purple-400" />,
      color: 'text-purple-400',
      borderColor: 'border-purple-500/30',
      badgeBg: 'bg-purple-500/20 text-purple-300',
      description: 'Prêt pour test & merge',
    },
    {
      id: 'finished',
      title: 'Done',
      stageLabel: 'Terminé',
      stageColor: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
      icon: <CheckCircle2 size={16} className="text-emerald-400" />,
      color: 'text-emerald-400',
      borderColor: 'border-emerald-500/30',
      badgeBg: 'bg-emerald-500/20 text-emerald-300',
      description: 'Tickets fermés / validés',
    },
  ]

  const handleDragOver = (e: React.DragEvent, colId: string) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    if (dragOverColumn !== colId) {
      setDragOverColumn(colId)
    }
  }

  const handleDragLeave = (e: React.DragEvent, colId: string) => {
    e.preventDefault()
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    if (
      e.clientX < rect.left ||
      e.clientX >= rect.right ||
      e.clientY < rect.top ||
      e.clientY >= rect.bottom
    ) {
      if (dragOverColumn === colId) {
        setDragOverColumn(null)
      }
    }
  }

  const handleDropWorkflow = async (e: React.DragEvent, targetStage: WorkflowStage) => {
    e.preventDefault()
    setDragOverColumn(null)
    const taskId = e.dataTransfer.getData('text/plain') || draggingTaskId
    setDraggingTaskId(null)

    if (!taskId) return
    const task = tasks.find(t => t.id === taskId || t.key === taskId)
    if (!task) return

    const currentStage = resolveTaskStage(task, currentProject)
    if (currentStage === targetStage) return

    await moveTaskWorkflowStage(task.id, targetStage)
  }

  // Les colonnes du projet, à la façon de Jira : un nom et les statuts du tracker
  // qu'elles regroupent. Dès qu'un projet en définit, elles remplacent les
  // colonnes de statuts génériques — ce mode n'est qu'une extension de celui-ci.
  // Extract distinct trackerStatus from tasks as fallback columns if project trackerColumns is empty
  const fallbackTrackerColumns = React.useMemo(() => {
    const seen = new Set<string>()
    const cols: string[] = []
    tasks.forEach(t => {
      const st = (t.trackerStatus || '').trim()
      if (st && !seen.has(st.toLowerCase())) {
        seen.add(st.toLowerCase())
        cols.push(st)
      }
    })
    return cols.map(name => ({ name, statuses: [name], hidden: false }))
  }, [tasks])

  const trackerColumns = (currentProject?.trackerColumns && currentProject.trackerColumns.length > 0)
    ? currentProject.trackerColumns
    : fallbackTrackerColumns

  const useTrackerBoard = trackerColumns.length > 0
  const stageColumns = currentProject?.stageColumns || {}

  // Étapes agentiques affectées à une colonne, pour l'afficher dans son en-tête.
  const stagesForColumn = (columnName: string): string[] =>
    Object.entries(stageColumns)
      .filter(([, cols]) => (cols || []).includes(columnName))
      .map(([stage]) => stage)

  // Les colonnes masquées sortent du board sans perdre leur affectation de
  // statuts : leurs tickets ne sont simplement pas affichés, et un bouton dans
  // la barre d'outils permet de les remontrer sans passer par les réglages.
  const hiddenColumns = trackerColumns.filter(c => c.hidden)
  const visibleTrackerColumns = showHiddenColumns
    ? trackerColumns
    : trackerColumns.filter(c => !c.hidden)

  const effectiveStatusColumns: StatusColumnConfig[] = visibleTrackerColumns.length > 0
    ? visibleTrackerColumns.map(col => ({
        id: col.name as unknown as Status,
        title: col.name,
        stageLabel: stagesForColumn(col.name).map(st => `#${st}`).join(' ') || col.statuses.join(', '),
        stageColor: 'bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/30',
        icon: <Kanban size={16} className="text-[var(--accent-color)]" />,
        color: 'text-[var(--text-primary)]',
        borderColor: 'border-[var(--border-color)]',
        badgeBg: 'bg-[var(--bg-tertiary)] text-[var(--text-secondary)]',
        description: col.statuses.join(', '),
      }))
    : statusColumns

  const columnStatuses = (columnName: string): string[] =>
    trackerColumns.find(c => c.name === columnName)?.statuses || []

  /**
   * Une colonne de tracker est terminale quand tous ses statuts ferment le
   * ticket. Le nom seul ne suffit pas (une équipe appelle sa dernière colonne
   * « TO MERGE/DEPLOY »), et les tickets seuls non plus (une colonne vide n'a
   * rien à dire) : les deux se complètent.
   */
  const CLOSED_MARKERS = ['done', 'closed', 'terminé', 'termine', 'fini', 'resolved', 'wontdo', "won't do", 'cancel', 'annul', 'rejet', 'reject']

  const isClosedColumn = (col: StatusColumnConfig): boolean => {
    if (!useTrackerBoard) return col.id === 'finished'
    const statuses = columnStatuses(col.title)
    if (statuses.length === 0) return false
    const allClosedByName = statuses.every(st => {
      const lower = st.toLowerCase()
      return CLOSED_MARKERS.some(marker => lower.includes(marker))
    })
    if (allClosedByName) return true
    const lowered = statuses.map(st => st.toLowerCase())
    const inColumn = tasks.filter(t => lowered.includes((t.trackerStatus || '').toLowerCase()))
    return inColumn.length > 0 && inColumn.every(t => t.status === 'finished' || t.status === 'done')
  }

  // Les tickets que les colonnes affichent. Le filtre « en cours » s'arrête ici :
  // `tasks` continue de servir aux résolutions par id du glisser-déposer, et la
  // structure du board — ses colonnes — ne bouge pas parce qu'un filtre la vide.
  const displayedTasks = React.useMemo(
    () => (activeOnly ? tasks.filter(t => activeTasks.has(t.id)) : tasks),
    [tasks, activeOnly, activeTasks],
  )

  const tasksForColumn = (col: StatusColumnConfig): Task[] => {
    const statuses = columnStatuses(col.title).map(st => st.toLowerCase())
    if (statuses.length === 0) {
      statuses.push(col.title.toLowerCase())
    }
    return sortColumn(
      displayedTasks.filter(t => {
        const st = (t.trackerStatus || '').toLowerCase()
        if (statuses.includes(st)) return true
        if (st === '' && col.title.toLowerCase() === 'todo') return true
        return false
      })
    )
  }

  // Aucun ticket ne doit disparaître : ceux dont le statut n'est réclamé par
  // aucune colonne atterrissent dans une colonne dédiée, affichée seulement si
  // elle contient quelque chose.
  const unassignedTasks = useTrackerBoard
    ? sortColumn(
        displayedTasks.filter(t => {
          const st = (t.trackerStatus || '').toLowerCase()
          if (!st) return true
          // Une colonne masquée réclame toujours ses statuts : ses tickets sont
          // cachés, pas orphelins.
          return !trackerColumns.some(c => c.statuses.some(cs => cs.toLowerCase() === st))
        })
      )
    : []

  // The cards of every column, computed once so that the columns render from
  // the same lists the selection is ordered and pruned against.
  const workflowColumnTasks = new Map<WorkflowStage, Task[]>(
    boardGrouping === 'workflow'
      ? workflowColumns.map(col => [col.id, sortColumn(displayedTasks.filter(t => resolveTaskStage(t, currentProject) === col.id))])
      : [],
  )
  const statusColumnTasks = new Map<Status, Task[]>(
    boardGrouping === 'status'
      ? effectiveStatusColumns.map(col => [col.id, tasksForColumn(col)])
      : [],
  )

  // The cards on screen, in board order: columns left to right, cards top to
  // bottom. A collapsed column shows no card, so it contributes none. A task
  // two tracker columns both claim keeps its first place only.
  const boardCards: Task[] = boardGrouping === 'workflow'
    ? workflowColumns
        .filter(col => !(col.id === 'finished' && hideDone))
        .flatMap(col => workflowColumnTasks.get(col.id) || [])
    : [
        ...effectiveStatusColumns
          .filter(col => !(hideDone && isClosedColumn(col)))
          .flatMap(col => statusColumnTasks.get(col.id) || []),
        ...unassignedTasks,
      ]
  const boardOrder = Array.from(new Set(boardCards.map(task => task.id)))
  const selectableIds = Array.from(new Set(
    boardCards
      .filter(task => isSelectableStage(resolveTaskStage(task, currentProject)))
      .map(task => task.id),
  ))
  const selectableIdSet = new Set(selectableIds)

  // Both adjustments happen while rendering, React's pattern for state that
  // follows other state, rather than in an effect that would paint the stale
  // selection first. A card that leaves the screen or moves past `clarified`
  // leaves the selection for good: it does not come back selected when it
  // reappears. pruneSelection returns the same Set when nothing drops out, so
  // this settles in one pass.
  const prunedIds = pruneSelection(selectedIds, selectableIds)
  if (prunedIds !== selectedIds) setSelectedIds(prunedIds)

  const [selectionProjectId, setSelectionProjectId] = useState(currentProject?.id)
  if (selectionProjectId !== currentProject?.id) {
    setSelectionProjectId(currentProject?.id)
    setSelectedIds(new Set())
  }

  const hasSelection = selectedIds.size > 0

  // Escape clears the selection only when nothing else would take the key.
  // Listening on window lets the card menus, which handle it on document and
  // call preventDefault, and the dialogs that stop it on the way down, spend
  // it first. The surfaces AppContext closes on Escape are excluded by state.
  const appSurfaceOpen = Boolean(
    isCommandPaletteOpen || isQuickAddOpen || selectedTask || selectedActivity || isProfileOpen || searchQuery,
  )
  useEffect(() => {
    if (!hasSelection) return
    const handleKeyDown = (e: KeyboardEvent) => {
      const active = document.activeElement
      const activeTag = (active?.tagName || '').toLowerCase()
      const inTerminal = Boolean(active?.closest('.xterm') || active?.classList.contains('xterm-helper-textarea'))
      const clear = shouldEscapeClearSelection({
        key: e.key,
        defaultPrevented: e.defaultPrevented,
        appSurfaceOpen,
        inputFocused: activeTag === 'input' || activeTag === 'textarea' || activeTag === 'select' || inTerminal,
        modalOpen: Boolean(document.querySelector('[aria-modal="true"]')),
      })
      if (clear) setSelectedIds(new Set())
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [hasSelection, appSurfaceOpen])

  const toggleCardSelection = (taskId: string) => setSelectedIds(prev => toggleSelected(prev, taskId))

  // The selection is cleared only once the launch is accepted: after a
  // failure, already reported by a toast, it stays so that it can be retried.
  const launchSelectedBatch = async () => {
    if (launchingBatch) return
    setLaunchingBatch(true)
    try {
      const accepted = await startBatchPickup(orderSelection(selectedIds, boardOrder))
      if (accepted) setSelectedIds(new Set())
    } finally {
      setLaunchingBatch(false)
    }
  }

  const renderCard = (task: Task) => (
    <TaskCard
      key={task.id}
      task={task}
      compact={isCondensed}
      isDragging={draggingTaskId === task.id}
      onDragStart={() => setDraggingTaskId(task.id)}
      selectable={selectableIdSet.has(task.id)}
      selected={selectedIds.has(task.id)}
      selectionActive={hasSelection}
      onToggleSelect={() => toggleCardSelection(task.id)}
    />
  )

  // Déplacement sur un board de tracker : la colonne cible impose son premier
  // statut, et la transition part dans le tracker.
  const handleDropTrackerColumn = async (e: React.DragEvent, columnName: string) => {
    e.preventDefault()
    setDragOverColumn(null)
    const taskId = e.dataTransfer.getData('text/plain') || draggingTaskId
    setDraggingTaskId(null)
    if (!taskId) return

    const task = tasks.find(t => t.id === taskId || t.key === taskId)
    const statuses = columnStatuses(columnName)
    if (!task || statuses.length === 0) return
    // Déjà dans la colonne : rien à transitionner.
    if (statuses.some(st => st.toLowerCase() === (task.trackerStatus || '').toLowerCase())) return

    await moveTaskToTrackerStatus(task.id, statuses[0])
  }

  const handleDropStatus = async (e: React.DragEvent, targetStatus: Status) => {
    e.preventDefault()
    setDragOverColumn(null)
    const taskId = e.dataTransfer.getData('text/plain') || draggingTaskId
    setDraggingTaskId(null)

    if (!taskId) return
    const task = tasks.find(t => t.id === taskId || t.key === taskId)
    if (!task || isTaskInStatusColumn(task.status, targetStatus)) return

    const targetTasks = tasks.filter(t => isTaskInStatusColumn(t.status, targetStatus))
    const newPos = targetTasks.length

    await moveTask(task.id, targetStatus, newPos)
  }

  const openQuickAddForWorkflow = (stage: WorkflowStage) => {
    let initialSt: Status = 'to_clarify'
    if (stage === 'specified') initialSt = 'to_implement'
    else if (stage === 'implemented' || stage === 'reviewed') initialSt = 'to_test'
    else if (stage === 'finished') initialSt = 'to_close'
    setQuickAddInitialStatus(initialSt)
    setIsQuickAddOpen(true)
  }

  const openQuickAddForStatus = (status: Status) => {
    setQuickAddInitialStatus(status)
    setIsQuickAddOpen(true)
  }

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-[var(--bg-primary)] select-none">
      {/* Board Top Toolbar: Grouping Mode Switcher */}
      {/* View Subheader: View Mode Switcher (Icon-only) & Hide/Show Done Filter */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-color)] bg-[var(--bg-secondary)]/50 shrink-0">
        <div className="flex items-center gap-3">
          <BoardGroupingToggle size="md" />
          <BoardSortSelect size="md" />
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={toggleBoardCardDisplayMode}
            aria-pressed={isCondensed}
            aria-label={isCondensed ? 'Afficher les cartes détaillées' : 'Afficher les cartes sur une ligne'}
            className={`flex items-center justify-center p-1.5 rounded-lg border transition-colors cursor-pointer ${
              isCondensed
                ? 'bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/40 shadow-2xs'
                : 'bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
            }`}
            title={isCondensed ? 'Afficher les cartes détaillées' : 'Afficher les cartes sur une ligne'}
          >
            {isCondensed ? <List size={14} /> : <Kanban size={14} />}
          </button>

          <TaskFilters />

          {hiddenColumns.length > 0 && (
            <button
              onClick={() => setShowHiddenColumns(v => !v)}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors cursor-pointer ${
                showHiddenColumns
                  ? 'bg-amber-500/15 text-amber-300 border-amber-500/40'
                  : 'bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
              }`}
              title={hiddenColumns.map(c => c.name).join(', ')}
            >
              {showHiddenColumns ? <EyeOff size={13} /> : <Eye size={13} />}
              <span>{showHiddenColumns ? 'Masquer' : `${hiddenColumns.length} colonne${hiddenColumns.length > 1 ? 's' : ''} masquée${hiddenColumns.length > 1 ? 's' : ''}`}</span>
            </button>
          )}

          <button
            onClick={toggleHideDone}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors cursor-pointer ${
              hideDone
                ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/40 shadow-2xs'
                : 'bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
            }`}
            title={hideDone ? 'Afficher les tâches terminées' : 'Masquer les tâches terminées'}
          >
            {hideDone ? <Eye size={13} className="text-emerald-400" /> : <EyeOff size={13} />}
            <span>{hideDone ? 'Afficher Terminé' : 'Masquer Terminé'}</span>
          </button>
        </div>
      </div>

      {/* Le board garde ses colonnes quand le filtre les vide : leur disparition
          ferait croire à une perte de structure. Reste à dire pourquoi elles sont
          toutes vides, ce qu'une colonne vide seule n'explique pas. */}
      {activeOnly && displayedTasks.length === 0 && (
        <div className="mx-4 mt-4 flex items-center justify-between gap-3 rounded-lg border border-[var(--border-color)] bg-[var(--bg-secondary)] px-3 py-2 text-[11px] text-[var(--text-secondary)]">
          <span>Aucun ticket en cours d'exécution.</span>
          <button
            type="button"
            onClick={() => setActiveOnly(false)}
            className="font-semibold accent-text hover:opacity-80 transition-opacity cursor-pointer"
          >
            Afficher tous les tickets
          </button>
        </div>
      )}

      {/* Board Scrollable Columns Container */}
      <div className="flex-1 overflow-x-auto overflow-y-hidden p-4">
        <div className="flex gap-4 h-full min-w-max pb-2">
          {/* ========================================================= */}
          {/* RENDER MODE 1: WORKFLOW LABELS PIPELINE                   */}
          {/* ========================================================= */}
          {boardGrouping === 'workflow' &&
            workflowColumns.map(col => {
              const colTasks = workflowColumnTasks.get(col.id) || []
              const isOver = dragOverColumn === col.id

              // Collapsed Finished Column when hideDone is enabled
              if (col.id === 'finished' && hideDone) {
                return (
                  <div
                    key={col.id}
                    onDragOver={e => handleDragOver(e, col.id)}
                    onDragLeave={e => handleDragLeave(e, col.id)}
                    onDrop={e => handleDropWorkflow(e, col.id)}
                    onClick={toggleHideDone}
                    className={`w-14 shrink-0 flex flex-col items-center justify-between py-4 rounded-2xl bg-[var(--bg-secondary)]/60 border transition-all duration-200 cursor-pointer group hover:bg-[var(--bg-secondary)] select-none ${
                      isOver
                        ? 'border-emerald-500 ring-2 ring-emerald-500/30 bg-emerald-500/15'
                        : 'border-[var(--border-color)] hover:border-emerald-500/40'
                    }`}
                    title="Colonne Finished masquée - Glissez une tâche ici pour la terminer, ou cliquez pour l'afficher"
                  >
                    <div className="flex flex-col items-center gap-1.5">
                      <CheckCircle2 size={18} className="text-emerald-400" />
                      <span className="text-[10px] font-mono font-bold px-1.5 py-0.2 rounded-full bg-emerald-500/20 text-emerald-300">
                        {colTasks.length}
                      </span>
                    </div>

                    <div className="flex-1 flex items-center justify-center my-4">
                      <span className="[writing-mode:vertical-lr] rotate-180 text-xs font-bold text-[var(--text-secondary)] group-hover:text-emerald-400 tracking-wider transition-colors">
                        Finished ({colTasks.length})
                      </span>
                    </div>

                    <div className="p-1.5 rounded-lg bg-[var(--bg-tertiary)] text-[var(--text-muted)] group-hover:text-emerald-400 transition-colors">
                      <Eye size={14} />
                    </div>
                  </div>
                )
              }

              return (
                <div
                  key={col.id}
                  onDragOver={e => handleDragOver(e, col.id)}
                  onDragLeave={e => handleDragLeave(e, col.id)}
                  onDrop={e => handleDropWorkflow(e, col.id)}
                  className={`kanban-column w-[320px] min-w-[290px] shrink-0 flex flex-col rounded-2xl border transition-all duration-200 ${
                    isOver
                      ? 'border-[var(--accent-color)] ring-2 ring-[var(--accent-glow)] bg-[var(--accent-light)]/20'
                      : 'border-[var(--border-color)] bg-[var(--bg-primary)]'
                  }`}
                >
                  {/* Column Header */}
                  <div className="flex items-center justify-between p-3 border-b border-[var(--border-color)]">
                    <div className="flex items-center gap-2">
                      <span>{col.icon}</span>
                      <h3 className="text-xs font-bold text-[var(--text-primary)] tracking-wide">
                        {col.title}
                      </h3>
                      <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded border ${col.stageColor}`}>
                        {col.stageLabel}
                      </span>
                      <span className={`text-[11px] font-mono px-1.5 py-0.2 rounded-full font-bold ${col.badgeBg}`}>
                        {colTasks.length}
                      </span>
                    </div>

                    <div className="flex items-center gap-1">
                      {col.id === 'finished' && (
                        <button
                          onClick={toggleHideDone}
                          className="p-1 rounded-md text-[var(--text-muted)] hover:text-emerald-400 hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                          title="Masquer la colonne Finished"
                        >
                          <EyeOff size={14} />
                        </button>
                      )}
                      <button
                        onClick={() => openQuickAddForWorkflow(col.id)}
                        className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                        title={`Ajouter un ticket (#${col.id})`}
                      >
                        <Plus size={15} />
                      </button>
                    </div>
                  </div>

                  {/* Task Cards Column Body */}
                  <div className="flex-1 overflow-y-auto p-2.5 space-y-2.5">
                    {colTasks.map(renderCard)}

                    {colTasks.length === 0 && (
                      <div className="h-32 flex flex-col items-center justify-center text-center p-4 border border-dashed border-[var(--border-color)]/60 rounded-xl">
                        <p className="text-xs text-[var(--text-muted)]">
                          {t.board.emptyColumn}
                        </p>
                      </div>
                    )}
                  </div>
                </div>
              )
            })}

          {/* ========================================================= */}
          {/* RENDER MODE 2: OPERATIONAL STATUS COLUMNS                 */}
          {/* ========================================================= */}
          {boardGrouping === 'status' &&
            effectiveStatusColumns.map(col => {
              const colTasks = statusColumnTasks.get(col.id) || []
              const isOver = dragOverColumn === col.id
              const onDropColumn = useTrackerBoard
                ? (e: React.DragEvent) => handleDropTrackerColumn(e, col.title)
                : (e: React.DragEvent) => handleDropStatus(e, col.id)

              // Colonne terminale repliée quand « masquer terminé » est actif,
              // board interne comme board de tracker : ses tickets sont sortis
              // des autres colonnes, la laisser ouverte et vide n'apprend rien.
              if (hideDone && isClosedColumn(col)) {
                return (
                  <div
                    key={col.id}
                    onDragOver={e => handleDragOver(e, col.id)}
                    onDragLeave={e => handleDragLeave(e, col.id)}
                    onDrop={onDropColumn}
                    onClick={toggleHideDone}
                    className={`w-14 shrink-0 flex flex-col items-center justify-between py-4 rounded-2xl bg-[var(--bg-secondary)]/60 border transition-all duration-200 cursor-pointer group hover:bg-[var(--bg-secondary)] select-none ${
                      isOver
                        ? 'border-emerald-500 ring-2 ring-emerald-500/30 bg-emerald-500/15'
                        : 'border-[var(--border-color)] hover:border-emerald-500/40'
                    }`}
                    title={`Colonne ${col.title} masquée : glissez une tâche ici pour la fermer, ou cliquez pour la rouvrir`}
                  >
                    <div className="flex flex-col items-center gap-1.5">
                      <CheckCircle2 size={18} className="text-emerald-400" />
                      <span className="text-[10px] font-mono font-bold px-1.5 py-0.2 rounded-full bg-emerald-500/20 text-emerald-300">
                        {colTasks.length}
                      </span>
                    </div>

                    <div className="flex-1 flex items-center justify-center my-4">
                      <span className="[writing-mode:vertical-lr] rotate-180 text-xs font-bold text-[var(--text-secondary)] group-hover:text-emerald-400 tracking-wider transition-colors">
                        {col.title} ({colTasks.length})
                      </span>
                    </div>

                    <div className="p-1.5 rounded-lg bg-[var(--bg-tertiary)] text-[var(--text-muted)] group-hover:text-emerald-400 transition-colors">
                      <Eye size={14} />
                    </div>
                  </div>
                )
              }

              return (
                <div
                  key={col.id}
                  onDragOver={e => handleDragOver(e, col.id)}
                  onDragLeave={e => handleDragLeave(e, col.id)}
                  onDrop={onDropColumn}
                  className={`kanban-column w-[320px] min-w-[290px] shrink-0 flex flex-col rounded-2xl border transition-all duration-200 ${
                    isOver
                      ? 'border-[var(--accent-color)] ring-2 ring-[var(--accent-glow)] bg-[var(--accent-light)]/20'
                      : 'border-[var(--border-color)] bg-[var(--bg-primary)]'
                  }`}
                >
                  {/* Column Header */}
                  <div className="flex items-center justify-between p-3 border-b border-[var(--border-color)]">
                    <div className="flex items-center gap-2">
                      <span>{col.icon}</span>
                      <h3 className="text-xs font-bold text-[var(--text-primary)] tracking-wide">
                        {col.title}
                      </h3>
                      <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded border ${col.stageColor}`}>
                        {col.stageLabel}
                      </span>
                      <span className={`text-[11px] font-mono px-1.5 py-0.2 rounded-full font-bold ${col.badgeBg}`}>
                        {colTasks.length}
                      </span>
                    </div>

                    <div className="flex items-center gap-1">
                      {col.id === 'to_close' && (
                        <button
                          onClick={toggleHideDone}
                          className="p-1 rounded-md text-[var(--text-muted)] hover:text-emerald-400 hover:bg-[var(--bg-tertiary)] transition-colors"
                          title="Masquer la colonne Done"
                        >
                          <EyeOff size={14} />
                        </button>
                      )}
                      <button
                        onClick={() => openQuickAddForStatus(col.id)}
                        className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors"
                        title={`Ajouter une tâche (${col.title})`}
                      >
                        <Plus size={15} />
                      </button>
                    </div>
                  </div>

                  {/* Task Cards Column Body */}
                  <div className="flex-1 overflow-y-auto p-2.5 space-y-2.5">
                    {colTasks.map(renderCard)}

                    {colTasks.length === 0 && (
                      <div className="h-32 flex flex-col items-center justify-center text-center p-4 border border-dashed border-[var(--border-color)]/60 rounded-xl">
                        <p className="text-xs text-[var(--text-muted)]">
                          {t.board.emptyColumn}
                        </p>
                      </div>
                    )}
                  </div>
                </div>
              )
            })}

          {/* Colonne de repli : les tickets dont le statut n'est réclamé par
              aucune colonne. Visible seulement si elle contient quelque chose,
              pour qu'un statut oublié se remarque au lieu de faire disparaître
              des tickets. */}
          {boardGrouping === 'status' && unassignedTasks.length > 0 && (
            <div className="kanban-column w-[320px] min-w-[290px] shrink-0 flex flex-col rounded-2xl bg-[var(--bg-secondary)]/70 border border-amber-500/40">
              <div className="flex items-center justify-between p-3 border-b border-[var(--border-color)]">
                <div className="flex items-center gap-2">
                  <ListFilter size={16} className="text-amber-400" />
                  <h3 className="text-xs font-bold text-[var(--text-primary)] tracking-wide">Non classé</h3>
                  <span className="text-[11px] font-mono px-1.5 py-0.2 rounded-full font-bold bg-amber-500/20 text-amber-300">
                    {unassignedTasks.length}
                  </span>
                </div>
              </div>
              <div className="px-3 py-1.5 text-[10px] text-[var(--text-muted)] border-b border-[var(--border-color)]/60">
                Statuts non affectés à une colonne : {Array.from(new Set(unassignedTasks.map(t => t.trackerStatus || 'sans statut'))).join(', ')}
              </div>
              <div className="flex-1 overflow-y-auto p-2.5 space-y-2.5">
                {unassignedTasks.map(renderCard)}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* The selection bar: the batch pickup the other views offer, and a way
          out. Nothing else, the board is not a bulk editor. */}
      {hasSelection && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 animate-in slide-in-from-bottom-5 duration-200">
          <div className="flex items-center gap-2.5 px-4 py-2.5 rounded-2xl bg-[var(--bg-secondary)]/95 backdrop-blur-md border border-[var(--border-color)] shadow-2xl text-xs max-w-[95vw] flex-wrap justify-center sm:justify-start">
            <div className="flex items-center gap-2 pr-3 border-r border-[var(--border-color)]">
              <span className="flex items-center justify-center min-w-[22px] h-[22px] px-1.5 rounded-full bg-[var(--accent-color)] text-white text-[11px] font-bold shadow-xs">
                {selectedIds.size}
              </span>
              <span className="font-semibold text-[var(--text-primary)] whitespace-nowrap">
                {selectedIds.size > 1 ? 'sélectionnées' : 'sélectionnée'}
              </span>
            </div>

            <button
              type="button"
              onClick={launchSelectedBatch}
              disabled={launchingBatch}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-colors shrink-0 shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
              title="Run the selected tasks on the local agent"
            >
              <Sparkles size={13} />
              {t.batchLaunch}
            </button>

            <button
              type="button"
              onClick={() => setSelectedIds(new Set())}
              className="flex items-center gap-1 text-xs font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
            >
              <X size={13} />
              Désélectionner tout
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
