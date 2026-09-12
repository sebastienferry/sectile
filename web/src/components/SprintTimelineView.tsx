import React, { useState, useMemo, useRef } from 'react'
import {
  CalendarDays,
  Clock,
  Plus,
  RefreshCw,
  CheckCircle2,
  Trash2,
  Edit2,
  Layers,
  Search,
  Check,
  X,
  MessageSquare,
  LayoutGrid,
  Tag,
  CheckSquare,
  Square,
  GripVertical,
  ChevronDown,
  ChevronUp,
  Loader2,
  SlidersHorizontal,
  Archive,
  RotateCcw,
  Sparkles,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import {
  calculateSprintDates,
  shiftSubsequentSprintDates,
  generateDefaultSprints,
  formatDateFR,
  formatDateISO,
  formatDateInput,
  getMonday,
  getSprintRelativeInfo,
} from '../lib/sprints'
import type { Task, TrackerSprint, WorkflowStage } from '../types'
import { resolveTaskStage } from '../lib/workflow'
import { Avatar } from './Avatar'

const DRAG_TASK_ID = 'application/x-taskflow-task-id'
const DRAG_TASK_IDS = 'application/x-taskflow-task-ids'

export const SprintTimelineView: React.FC = () => {
  const {
    tasks,
    currentProject,
    updateProject,
    setTaskSprint,
    setTasksSprint,
    setSelectedTask,
    setChatTask,
    addToast,
    startBatchPickup,
  } = useApp()

  // Project sprints (or defaults if none yet)
  const sprints: TrackerSprint[] = useMemo(() => {
    if (currentProject?.sprints && currentProject.sprints.length > 0) {
      return currentProject.sprints
    }
    return generateDefaultSprints(4)
  }, [currentProject?.sprints])

  // Configuration state
  const [durationDays, setDurationDays] = useState<number>(14)
  const [startDateStr, setStartDateStr] = useState<string>(() => {
    if (sprints.length > 0 && sprints[0].startDate) {
      return sprints[0].startDate
    }
    return formatDateISO(getMonday(new Date()))
  })

  // Sprint Details & Dates Edit State
  const [editingSprintIndex, setEditingSprintIndex] = useState<number | null>(null)
  const [editSprintName, setEditSprintName] = useState<string>('')
  const [editSprintStartDate, setEditSprintStartDate] = useState<string>('')
  const [editSprintEndDate, setEditSprintEndDate] = useState<string>('')
  const [editSprintState, setEditSprintState] = useState<'active' | 'future' | 'closed'>('future')
  const [editShiftSubsequent, setEditShiftSubsequent] = useState<boolean>(true)

  // Close Sprint Modal State
  const [closingSprint, setClosingSprint] = useState<{ sprint: TrackerSprint; index: number } | null>(null)
  const [closeSprintDestination, setCloseSprintDestination] = useState<'next' | 'backlog' | 'keep'>('next')
  const [closeSprintActivateNext, setCloseSprintActivateNext] = useState<boolean>(true)
  const [isClosingSprintBusy, setIsClosingSprintBusy] = useState<boolean>(false)

  // Fold/Collapse Closed Sprints State
  const [collapsedSprints, setCollapsedSprints] = useState<Record<string, boolean>>({})
  const [hideClosedSprints, setHideClosedSprints] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem('taskflow_sprint_hide_closed')
      if (stored !== null) return stored === 'true'
    } catch {}
    return true // Masqués par défaut
  })

  const handleToggleHideClosedSprints = () => {
    setHideClosedSprints(prev => {
      const next = !prev
      try {
        localStorage.setItem('taskflow_sprint_hide_closed', String(next))
      } catch {}
      return next
    })
  }

  const [backlogSearch, setBacklogSearch] = useState<string>('')
  const [isBacklogOpen, setIsBacklogOpen] = useState<boolean>(true)
  const [dragOverSprint, setDragOverSprint] = useState<string | null>(null)

  // Toggle Display Mode: 'cards' vs 'chips'
  const [displayMode, setDisplayMode] = useState<'cards' | 'chips'>(() => {
    try {
      const stored = localStorage.getItem('taskflow_sprint_display_mode')
      return stored === 'chips' ? 'chips' : 'cards'
    } catch {
      return 'cards'
    }
  })

  const handleSetDisplayMode = (mode: 'cards' | 'chips') => {
    setDisplayMode(mode)
    try {
      localStorage.setItem('taskflow_sprint_display_mode', mode)
    } catch {}
  }

  // Backlog Width Resizing State
  const [backlogWidth, setBacklogWidth] = useState<number>(() => {
    try {
      const stored = localStorage.getItem('taskflow_sprint_backlog_width')
      const parsed = Number(stored)
      return parsed >= 280 && parsed <= 900 ? parsed : 360
    } catch {
      return 360
    }
  })
  const isResizingBacklog = useRef(false)

  const handleStartResizeBacklog = (e: React.PointerEvent) => {
    e.preventDefault()
    isResizingBacklog.current = true

    const startX = e.clientX
    const startWidth = backlogWidth

    const onPointerMove = (moveEvent: PointerEvent) => {
      if (!isResizingBacklog.current) return
      // Moving left increases width because drawer is on the right
      const deltaX = startX - moveEvent.clientX
      const newWidth = Math.min(Math.max(280, startWidth + deltaX), 850)
      setBacklogWidth(newWidth)
    }

    const onPointerUp = () => {
      isResizingBacklog.current = false
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerup', onPointerUp)
      try {
        localStorage.setItem('taskflow_sprint_backlog_width', String(backlogWidth))
      } catch {}
    }

    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', onPointerUp)
  }

  // Multi-Selection State
  const [checkedTaskIds, setCheckedTaskIds] = useState<Record<string, boolean>>({})
  const [batchTargetSprint, setBatchTargetSprint] = useState<string>('')
  const [batchBusy, setBatchBusy] = useState(false)

  const getTaskStageBadge = (task: Task) => {
    const stage = resolveTaskStage(task, currentProject)
    const stageInfo: Record<WorkflowStage, { label: string; style: string }> = {
      new: { label: '#new', style: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30' },
      clarified: { label: '#clarified', style: 'bg-amber-500/15 text-amber-400 border-amber-500/30' },
      specified: { label: '#specified', style: 'bg-blue-500/15 text-blue-400 border-blue-500/30' },
      implemented: { label: '#implemented', style: 'bg-indigo-500/15 text-indigo-400 border-indigo-500/30' },
      reviewed: { label: '#reviewed', style: 'bg-purple-500/15 text-purple-400 border-purple-500/30' },
      finished: { label: '#finished', style: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' },
    }
    const info = stageInfo[stage] || stageInfo.new
    return (
      <span className={`px-1.5 py-0.2 rounded font-mono font-bold text-[8.5px] border shrink-0 ${info.style}`} title={`Étape : ${info.label}`}>
        {info.label}
      </span>
    )
  }

  // Map tasks to sprints
  const tasksBySprint = useMemo(() => {
    const map = new Map<string, Task[]>()
    sprints.forEach(sp => map.set(sp.name.toLowerCase().trim(), []))

    tasks.forEach(task => {
      const spName = (task.sprint || '').toLowerCase().trim()
      if (spName && map.has(spName)) {
        map.get(spName)!.push(task)
      }
    })
    return map
  }, [tasks, sprints])

  const [showDoneInBacklog, setShowDoneInBacklog] = useState<boolean>(false)

  // Finished unscheduled count
  const finishedUnscheduledCount = useMemo(() => {
    return tasks.filter(t => {
      const sp = (t.sprint || '').trim()
      const hasNoSprint = !sp || !sprints.some(s => s.name.toLowerCase() === sp.toLowerCase())
      if (!hasNoSprint) return false
      return resolveTaskStage(t, currentProject) === 'finished' || t.status === 'finished' || t.status === 'done'
    }).length
  }, [tasks, sprints, currentProject])

  // Unscheduled tasks (Backlog)
  const unscheduledTasks = useMemo(() => {
    const q = backlogSearch.toLowerCase().trim()
    return tasks.filter(t => {
      const sp = (t.sprint || '').trim()
      const hasNoSprint = !sp || !sprints.some(s => s.name.toLowerCase() === sp.toLowerCase())
      if (!hasNoSprint) return false

      const isFinished = resolveTaskStage(t, currentProject) === 'finished' || t.status === 'finished' || t.status === 'done'
      if (isFinished && !showDoneInBacklog) return false

      if (!q) return true
      return (
        t.key.toLowerCase().includes(q) ||
        t.title.toLowerCase().includes(q) ||
        (t.parentTitle && t.parentTitle.toLowerCase().includes(q)) ||
        (t.assignee && t.assignee.toLowerCase().includes(q))
      )
    })
  }, [tasks, sprints, backlogSearch, currentProject, showDoneInBacklog])

  const selectedTaskIds = useMemo(
    () => Object.keys(checkedTaskIds).filter(id => checkedTaskIds[id]),
    [checkedTaskIds]
  )

  const selectedSprintTaskIds = useMemo(
    () =>
      selectedTaskIds.filter(id => {
        const t = tasks.find(item => item.id === id)
        return Boolean(t && (t.sprint || '').trim() !== '')
      }),
    [selectedTaskIds, tasks]
  )

  const selectedBacklogList = useMemo(
    () => unscheduledTasks.filter(t => checkedTaskIds[t.id]),
    [unscheduledTasks, checkedTaskIds]
  )

  const toggleTaskCheck = (taskId: string) => {
    setCheckedTaskIds(prev => ({
      ...prev,
      [taskId]: !prev[taskId],
    }))
  }

  const handleSelectAllBacklog = () => {
    const allSelected = unscheduledTasks.length > 0 && unscheduledTasks.every(t => checkedTaskIds[t.id])
    if (allSelected) {
      setCheckedTaskIds({})
    } else {
      const next: Record<string, boolean> = {}
      unscheduledTasks.forEach(t => {
        next[t.id] = true
      })
      setCheckedTaskIds(next)
    }
  }

  // Save updated sprints to project
  const saveSprints = async (newSprints: TrackerSprint[]) => {
    if (!currentProject?.id) return
    const updated = await updateProject(currentProject.id, { sprints: newSprints })
    if (updated) {
      addToast({
        type: 'success',
        title: 'Sprints mis à jour',
        description: `${newSprints.length} sprint(s) enregistrés pour ce projet.`,
      })
    }
  }

  // Recalculate all dates automatically
  const handleRecalculateAll = async () => {
    const updated = calculateSprintDates(sprints, startDateStr, durationDays)
    await saveSprints(updated)
  }

  // Add next consecutive sprint
  const handleAddSprint = async () => {
    let nextStart = new Date(startDateStr)
    if (sprints.length > 0) {
      const last = sprints[sprints.length - 1]
      if (last.endDate) {
        const lastEnd = new Date(last.endDate)
        lastEnd.setDate(lastEnd.getDate() + 1)
        nextStart = lastEnd
      }
    }

    const newIndex = sprints.length + 1
    const newSprintObj: TrackerSprint = {
      id: `sprint-${newIndex}`,
      name: `Sprint ${newIndex}`,
      state: 'future',
      startDate: formatDateISO(nextStart),
      endDate: formatDateISO(
        new Date(nextStart.getTime() + (durationDays - 1) * 24 * 60 * 60 * 1000)
      ),
    }

    const updated = [...sprints, newSprintObj]
    await saveSprints(updated)
  }

  // Delete a sprint
  const handleDeleteSprint = async (index: number) => {
    const sprintToDelete = sprints[index]
    if (!window.confirm(`Supprimer le ${sprintToDelete.name} ? Les tâches associées seront renvoyées au backlog.`)) {
      return
    }
    const updated = sprints.filter((_, i) => i !== index)
    await saveSprints(updated)
  }

  // Edit Sprint Details & Dates
  const handleStartEditSprint = (index: number, sprint: TrackerSprint) => {
    setEditingSprintIndex(index)
    setEditSprintName(sprint.name || `Sprint ${index + 1}`)
    setEditSprintStartDate(formatDateInput(sprint.startDate))
    setEditSprintEndDate(formatDateInput(sprint.endDate))
    setEditSprintState((sprint.state as any) || 'future')
    setEditShiftSubsequent(true)
  }

  const handleSaveEditSprint = async (index: number) => {
    if (!editSprintName.trim()) {
      setEditingSprintIndex(null)
      return
    }
    const oldName = sprints[index].name
    const newName = editSprintName.trim()

    let updated = sprints.map((sp, i) => {
      if (i === index) {
        return {
          ...sp,
          name: newName,
          startDate: editSprintStartDate || sp.startDate,
          endDate: editSprintEndDate || sp.endDate,
          state: editSprintState,
        }
      }
      return sp
    })

    if (editShiftSubsequent && editSprintEndDate) {
      updated = shiftSubsequentSprintDates(updated, index, durationDays)
    }

    setEditingSprintIndex(null)
    await saveSprints(updated)

    // Re-link tasks to new name if renamed
    if (oldName.toLowerCase() !== newName.toLowerCase()) {
      const tasksToUpdate = tasks.filter(
        t => (t.sprint || '').toLowerCase() === oldName.toLowerCase()
      )
      for (const t of tasksToUpdate) {
        await setTaskSprint(t.id, newName, newName)
      }
    }
  }

  // Close Sprint Handlers
  const handleStartCloseSprint = (sprint: TrackerSprint, index: number) => {
    setClosingSprint({ sprint, index })
    setCloseSprintDestination(index < sprints.length - 1 ? 'next' : 'backlog')
    setCloseSprintActivateNext(true)
  }

  const handleConfirmCloseSprint = async () => {
    if (!closingSprint || !currentProject?.id) return
    const { sprint, index } = closingSprint
    setIsClosingSprintBusy(true)

    try {
      const sprintKey = sprint.name.toLowerCase().trim()
      const sprintTasks = tasksBySprint.get(sprintKey) || []
      const unfinishedTasks = sprintTasks.filter(
        t => !(t.status === 'finished' || t.status === 'done' || resolveTaskStage(t, currentProject) === 'finished')
      )

      const nextSprint = index < sprints.length - 1 ? sprints[index + 1] : null

      // 1. Move unfinished tasks based on choice
      if (unfinishedTasks.length > 0) {
        const unfinishedIds = unfinishedTasks.map(t => t.id)
        if (closeSprintDestination === 'next' && nextSprint) {
          await setTasksSprint(currentProject.id, unfinishedIds, nextSprint.id || nextSprint.name, nextSprint.name)
        } else if (closeSprintDestination === 'backlog') {
          await setTasksSprint(currentProject.id, unfinishedIds, '', '')
        }
      }

      // 2. Update sprint states
      const updatedSprints = sprints.map((sp, i) => {
        if (i === index) {
          return { ...sp, state: 'closed' }
        }
        if (i === index + 1 && closeSprintActivateNext && sp.state !== 'closed') {
          return { ...sp, state: 'active' }
        }
        return sp
      })

      await saveSprints(updatedSprints)

      addToast({
        type: 'success',
        title: `${sprint.name} clôturé !`,
        description: unfinishedTasks.length > 0
          ? `${unfinishedTasks.length} tâche(s) non terminée(s) ${
              closeSprintDestination === 'next' && nextSprint
                ? `déplacée(s) vers ${nextSprint.name}`
                : closeSprintDestination === 'backlog'
                ? 'renvoyée(s) au backlog'
                : 'conservée(s) dans le sprint'
            }.`
          : 'Toutes les tâches étaient complétées.',
      })

      setClosingSprint(null)
    } catch (err: any) {
      addToast({
        type: 'error',
        title: 'Erreur lors de la clôture',
        description: err.message,
      })
    } finally {
      setIsClosingSprintBusy(false)
    }
  }

  const handleReopenSprint = async (index: number) => {
    const updated = sprints.map((sp, i) => (i === index ? { ...sp, state: 'active' } : sp))
    await saveSprints(updated)
    addToast({
      type: 'info',
      title: `${sprints[index].name} réouvert`,
      description: 'Le statut du sprint a été repassé en Actif.',
    })
  }

  const toggleCollapseSprint = (sprintName: string) => {
    setCollapsedSprints(prev => ({
      ...prev,
      [sprintName]: !prev[sprintName],
    }))
  }

  // Drag & Drop Handlers with Multi-selection support
  const handleDragStart = (e: React.DragEvent, taskId: string) => {
    const isTargetSelected = Boolean(checkedTaskIds[taskId])
    const idsToDrag = isTargetSelected
      ? Object.keys(checkedTaskIds).filter(id => checkedTaskIds[id])
      : [taskId]

    e.dataTransfer.setData(DRAG_TASK_IDS, JSON.stringify(idsToDrag))
    e.dataTransfer.setData(DRAG_TASK_ID, taskId)
    e.dataTransfer.effectAllowed = 'move'
  }

  const handleDragOver = (e: React.DragEvent, sprintName: string) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOverSprint(sprintName)
  }

  const handleDropOnSprint = async (e: React.DragEvent, sprintName: string) => {
    e.preventDefault()
    setDragOverSprint(null)

    const idsJson = e.dataTransfer.getData(DRAG_TASK_IDS)
    let taskIds: string[] = []
    if (idsJson) {
      try {
        taskIds = JSON.parse(idsJson)
      } catch {}
    }
    if (taskIds.length === 0) {
      const singleId = e.dataTransfer.getData(DRAG_TASK_ID)
      if (singleId) taskIds = [singleId]
    }
    if (taskIds.length === 0) return

    if (currentProject?.id && taskIds.length > 1) {
      await setTasksSprint(currentProject.id, taskIds, sprintName, sprintName)
      addToast({
        type: 'success',
        title: 'Tâches planifiées',
        description: `${taskIds.length} tâches déplacées dans le ${sprintName}.`,
      })
    } else {
      for (const id of taskIds) {
        await setTaskSprint(id, sprintName, sprintName)
      }
    }

    setCheckedTaskIds({})
  }

  const handleRemoveTaskFromSprint = async (taskId: string) => {
    await setTaskSprint(taskId, '', '')
  }

  // Batch assign from Backlog
  const handleApplyBatchSprint = async () => {
    const ids = Object.keys(checkedTaskIds).filter(id => checkedTaskIds[id])
    if (ids.length === 0 || !batchTargetSprint) return

    setBatchBusy(true)
    if (currentProject?.id) {
      await setTasksSprint(currentProject.id, ids, batchTargetSprint, batchTargetSprint)
      addToast({
        type: 'success',
        title: 'Tâches affectées',
        description: `${ids.length} tâche(s) affectée(s) au ${batchTargetSprint}.`,
      })
    } else {
      for (const id of ids) {
        await setTaskSprint(id, batchTargetSprint, batchTargetSprint)
      }
    }
    setCheckedTaskIds({})
    setBatchTargetSprint('')
    setBatchBusy(false)
  }

  // Batch remove tasks from sprint (send back to backlog)
  const handleBatchRemoveFromSprint = async () => {
    const ids = selectedSprintTaskIds.length > 0 ? selectedSprintTaskIds : selectedTaskIds
    if (ids.length === 0) return

    setBatchBusy(true)
    if (currentProject?.id) {
      await setTasksSprint(currentProject.id, ids, '', '')
    } else {
      for (const id of ids) {
        await setTaskSprint(id, '', '')
      }
    }
    setCheckedTaskIds({})
    setBatchBusy(false)
  }

  return (
    <div className="flex-1 flex flex-col h-full bg-[var(--bg-primary)] overflow-hidden">
      {/* Top Controls & Auto-Calculation Bar */}
      <div className="border-b border-[var(--border-color)] bg-[var(--bg-secondary)] px-4 py-2.5 shrink-0">
        <div className="flex flex-col lg:flex-row items-start lg:items-center justify-between gap-3">
          {/* Title & Durations */}
          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <div className="p-1.5 rounded-lg bg-[var(--accent-light)] text-[var(--accent-color)] border border-[var(--accent-color)]/30">
                <CalendarDays size={16} />
              </div>
              <div>
                <h2 className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
                  SPRINTS
                </h2>
                <span className="text-[10px] text-[var(--text-muted)]">
                  Découpage temporel & calcul automatique des cycles
                </span>
              </div>
            </div>

            <div className="h-4 w-px bg-[var(--border-color)] hidden sm:block mx-1" />

            {/* Sprint Duration Selector */}
            <div className="flex items-center gap-1 bg-[var(--bg-tertiary)] p-0.5 rounded-lg border border-[var(--border-color)] text-xs">
              <span className="text-[10px] text-[var(--text-muted)] font-medium px-2 flex items-center gap-1">
                <Clock size={11} /> Durée :
              </span>
              {[
                { label: '1 sem (7j)', days: 7 },
                { label: '2 sem (14j)', days: 14 },
                { label: '3 sem (21j)', days: 21 },
                { label: '4 sem (28j)', days: 28 },
              ].map(d => (
                <button
                  key={d.days}
                  type="button"
                  onClick={() => {
                    setDurationDays(d.days)
                    const updated = calculateSprintDates(sprints, startDateStr, d.days)
                    saveSprints(updated)
                  }}
                  className={`px-2.5 py-1 rounded-md text-[11px] font-medium transition-all cursor-pointer ${
                    durationDays === d.days
                      ? 'bg-[var(--accent-color)] text-white font-bold shadow-xs'
                      : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)]'
                  }`}
                >
                  {d.label}
                </button>
              ))}
            </div>

            {/* Start Date of Sprint 1 */}
            <div className="flex items-center gap-1.5 text-xs bg-[var(--bg-tertiary)] px-2.5 py-1 rounded-lg border border-[var(--border-color)]">
              <span className="text-[10px] text-[var(--text-muted)] font-medium">Début S1 :</span>
              <input
                type="date"
                value={startDateStr}
                onChange={e => {
                  setStartDateStr(e.target.value)
                  const updated = calculateSprintDates(sprints, e.target.value, durationDays)
                  saveSprints(updated)
                }}
                className="bg-transparent text-[11px] text-[var(--text-primary)] font-mono focus:outline-none cursor-pointer"
              />
            </div>
          </div>

          {/* Action Buttons & Display Mode Switcher */}
          <div className="flex items-center gap-2 self-end lg:self-auto flex-wrap">
            {/* Display Mode: Cards vs Chips */}
            <div className="flex items-center gap-0.5 p-0.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-xs">
              <button
                type="button"
                onClick={() => handleSetDisplayMode('cards')}
                className={`px-2 py-1 rounded flex items-center gap-1 text-[11px] font-semibold transition-all cursor-pointer ${
                  displayMode === 'cards'
                    ? 'bg-[var(--accent-color)] text-white shadow-xs font-bold'
                    : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                }`}
                title="Affichage sous forme de cartes complètes"
              >
                <LayoutGrid size={12} />
                <span className="hidden sm:inline">Cartes</span>
              </button>
              <button
                type="button"
                onClick={() => handleSetDisplayMode('chips')}
                className={`px-2 py-1 rounded flex items-center gap-1 text-[11px] font-semibold transition-all cursor-pointer ${
                  displayMode === 'chips'
                    ? 'bg-[var(--accent-color)] text-white shadow-xs font-bold'
                    : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                }`}
                title="Affichage sous forme de chips compactes"
              >
                <Tag size={12} />
                <span className="hidden sm:inline">Chips</span>
              </button>
            </div>

            {/* Toggle Closed Sprints */}
            {sprints.some(s => s.state === 'closed') && (
              <button
                type="button"
                onClick={handleToggleHideClosedSprints}
                className={`flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-semibold border transition-all cursor-pointer shadow-xs ${
                  hideClosedSprints
                    ? 'bg-slate-500/15 text-slate-300 border-slate-500/30 hover:bg-slate-500/25'
                    : 'bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
                }`}
                title={hideClosedSprints ? "Afficher les sprints clôturés" : "Masquer les sprints clôturés"}
              >
                <Archive size={12} className={hideClosedSprints ? "text-slate-400" : "text-[var(--text-muted)]"} />
                <span>
                  {hideClosedSprints
                    ? `Clôturés masqués (${sprints.filter(s => s.state === 'closed').length})`
                    : `Clôturés (${sprints.filter(s => s.state === 'closed').length})`}
                </span>
              </button>
            )}

            <button
              type="button"
              onClick={handleRecalculateAll}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-semibold bg-[var(--bg-tertiary)] hover:bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] transition-all cursor-pointer shadow-xs"
              title="Réaligner automatiquement les dates de tous les sprints"
            >
              <RefreshCw size={12} className="text-cyan-400" />
              <span>Recalculer</span>
            </button>

            <button
              type="button"
              onClick={handleAddSprint}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold bg-[var(--accent-color)] hover:opacity-90 text-white transition-all cursor-pointer shadow-xs"
              title="Ajouter un nouveau sprint consécutif"
            >
              <Plus size={13} />
              <span>+ Sprint</span>
            </button>

            <button
              type="button"
              onClick={() => setIsBacklogOpen(!isBacklogOpen)}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold border transition-all cursor-pointer ${
                isBacklogOpen
                  ? 'bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/40 shadow-xs'
                  : 'bg-[var(--bg-tertiary)] text-[var(--text-muted)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
              }`}
              title={
                isBacklogOpen
                  ? 'Masquer le Backlog (Vue synthétique compacte sans Drag & Drop)'
                  : 'Afficher le Backlog (Mode planification interactive avec Drag & Drop)'
              }
            >
              <Layers size={13} />
              <span>
                {isBacklogOpen ? 'Backlog (Glisser-Déposer actif)' : `Afficher Backlog (${unscheduledTasks.length})`}
              </span>
            </button>
          </div>
        </div>
      </div>

      {/* Global Batch Action Bar for Selected Tasks */}
      {selectedTaskIds.length > 0 && (
        <div className="bg-[var(--accent-light)]/30 border-b border-[var(--accent-color)]/30 px-4 py-2 flex items-center justify-between gap-3 text-xs animate-in fade-in duration-150 shrink-0 flex-wrap">
          <div className="flex items-center gap-2">
            <span className="font-bold text-[var(--accent-color)] flex items-center gap-1.5">
              <CheckSquare size={14} />
              <span>{selectedTaskIds.length} tâche(s) sélectionnée(s)</span>
            </span>
            {selectedSprintTaskIds.length > 0 && (
              <span className="text-[10.5px] text-[var(--text-muted)] font-medium">
                ({selectedSprintTaskIds.length} affectée(s) à un sprint)
              </span>
            )}
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {/* Target sprint dropdown */}
            <select
              value={batchTargetSprint}
              onChange={e => setBatchTargetSprint(e.target.value)}
              className="text-xs px-2.5 py-1 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer font-medium"
            >
              <option value="">Affecter à un sprint...</option>
              {sprints.map(sp => (
                <option key={sp.name} value={sp.name}>
                  {sp.name}
                </option>
              ))}
            </select>

            <button
              type="button"
              disabled={!batchTargetSprint || batchBusy}
              onClick={handleApplyBatchSprint}
              className="px-3 py-1 rounded-lg text-xs font-bold bg-[var(--accent-color)] text-white hover:opacity-90 disabled:opacity-40 transition-all cursor-pointer shadow-xs"
            >
              {batchBusy ? <Loader2 size={12} className="animate-spin" /> : 'Affecter'}
            </button>

            {/* Retirer du sprint / Renvoyer au backlog */}
            <button
              type="button"
              disabled={batchBusy}
              onClick={handleBatchRemoveFromSprint}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold bg-amber-500/15 text-amber-400 hover:bg-amber-500/25 border border-amber-500/30 transition-all cursor-pointer shadow-xs disabled:opacity-40"
              title="Retirer les tâches sélectionnées du sprint et les renvoyer au backlog"
            >
              {batchBusy ? <Loader2 size={12} className="animate-spin" /> : <X size={13} />}
              <span>
                Retirer du sprint{selectedSprintTaskIds.length > 0 ? ` (${selectedSprintTaskIds.length})` : ''}
              </span>
            </button>

            {/* Lancer le lot en auto-pilot */}
            <button
              type="button"
              onClick={() => startBatchPickup(selectedTaskIds)}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold text-purple-300 bg-purple-950/60 hover:bg-purple-900/80 border border-purple-700/50 cursor-pointer shadow-xs"
              title="Démarrer un unique Git Worktree et exécuter le lot de tâches en auto-pilot"
            >
              <Sparkles size={13} className="text-purple-400 animate-pulse" />
              <span>Lancer le lot (Git tree + Auto-pilot)</span>
            </button>

            <button
              type="button"
              onClick={() => setCheckedTaskIds({})}
              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              title="Désélectionner tout"
            >
              <X size={14} />
            </button>
          </div>
        </div>
      )}

      {/* Main Timeline Workspace */}
      <div className="flex-1 flex min-h-0 overflow-hidden relative">
        {/* Vertical Chronological Timeline */}
        <div className={`flex-1 overflow-y-auto ${isBacklogOpen ? 'p-6 space-y-5' : 'p-4 space-y-3'}`}>
          <div className={`${isBacklogOpen ? 'max-w-4xl space-y-5' : 'max-w-5xl space-y-3'} mx-auto relative`}>
            {/* Continuous Vertical Timeline Line */}
            <div
              className={`absolute ${isBacklogOpen ? 'left-6 top-6 bottom-6' : 'left-4 top-4 bottom-4'} w-0.5 bg-gradient-to-b from-indigo-500/40 via-cyan-500/40 to-emerald-500/40 hidden md:block`}
            />

            {sprints.map((sprint, index) => {
              const sprintKey = sprint.name.toLowerCase().trim()
              const sprintTasks = tasksBySprint.get(sprintKey) || []
              const rel = getSprintRelativeInfo(sprint)
              const isOver = isBacklogOpen && dragOverSprint === sprint.name

              // Completed stats
              const doneTasks = sprintTasks.filter(
                t => t.status === 'finished' || t.status === 'done'
              )
              const progressPct =
                sprintTasks.length > 0 ? Math.round((doneTasks.length / sprintTasks.length) * 100) : 0

              if (hideClosedSprints && sprint.state === 'closed') {
                return null
              }

              const isCollapsed = Boolean(collapsedSprints[sprint.name])

              return (
                <div
                  key={sprint.id || sprint.name}
                  className={`relative flex flex-col md:flex-row items-start ${isBacklogOpen ? 'gap-4' : 'gap-3'}`}
                >
                  {/* Timeline Node Icon (Desktop) */}
                  <div
                    className={`hidden md:flex items-center justify-center rounded-2xl bg-[var(--bg-secondary)] border-2 border-[var(--border-color)] z-10 shrink-0 shadow-md ${
                      isBacklogOpen ? 'w-12 h-12' : 'w-8 h-8 rounded-xl'
                    }`}
                  >
                    {sprint.state === 'closed' ? (
                      <CheckCircle2 size={isBacklogOpen ? 20 : 15} className="text-slate-400" />
                    ) : rel.type === 'current' ? (
                      <span className="relative flex h-3.5 w-3.5">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                        <span className="relative inline-flex rounded-full h-3.5 w-3.5 bg-emerald-500"></span>
                      </span>
                    ) : rel.type === 'past' ? (
                      <CheckCircle2 size={isBacklogOpen ? 20 : 15} className="text-amber-400" />
                    ) : (
                      <CalendarDays size={isBacklogOpen ? 18 : 14} className="text-cyan-400" />
                    )}
                  </div>

                  {/* Sprint Container Card */}
                  <div
                    onDragOver={isBacklogOpen ? e => handleDragOver(e, sprint.name) : undefined}
                    onDragLeave={isBacklogOpen ? () => setDragOverSprint(null) : undefined}
                    onDrop={isBacklogOpen ? e => handleDropOnSprint(e, sprint.name) : undefined}
                    className={`flex-1 w-full border bg-[var(--bg-secondary)]/90 shadow-sm transition-all duration-200 overflow-hidden ${
                      isBacklogOpen ? 'rounded-2xl' : 'rounded-xl'
                    } ${
                      isOver
                        ? 'border-[var(--accent-color)] ring-2 ring-[var(--accent-glow)] bg-[var(--accent-light)]/20'
                        : sprint.state === 'closed'
                        ? 'border-[var(--border-color)]/60 opacity-90'
                        : rel.type === 'current'
                        ? 'border-emerald-500/50 shadow-[0_0_15px_rgba(16,185,129,0.1)]'
                        : 'border-[var(--border-color)] hover:border-[var(--border-color)]/80'
                    }`}
                  >
                    {/* Sprint Header (Edit Mode vs Display Mode) */}
                    {editingSprintIndex === index ? (
                      <div className="p-3.5 bg-[var(--bg-primary)] border-b border-[var(--border-color)] space-y-2.5 animate-in fade-in duration-150">
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-bold text-[var(--accent-color)] flex items-center gap-1.5">
                            <SlidersHorizontal size={13} />
                            <span>Modifier les dates et paramètres de {sprint.name}</span>
                          </span>
                          <div className="flex items-center gap-1.5">
                            <button
                              type="button"
                              onClick={() => handleSaveEditSprint(index)}
                              className="flex items-center gap-1 px-3 py-1 text-xs font-bold text-white bg-emerald-600 hover:bg-emerald-500 rounded-lg transition-all cursor-pointer shadow-xs"
                              title="Enregistrer les modifications"
                            >
                              <Check size={13} />
                              <span>Enregistrer</span>
                            </button>
                            <button
                              type="button"
                              onClick={() => setEditingSprintIndex(null)}
                              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                              title="Annuler"
                            >
                              <X size={14} />
                            </button>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 sm:grid-cols-4 gap-2.5 text-xs">
                          {/* Nom */}
                          <div>
                            <label className="block text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1">
                              Nom du sprint
                            </label>
                            <input
                              type="text"
                              value={editSprintName}
                              onChange={e => setEditSprintName(e.target.value)}
                              placeholder="Ex: Sprint 1"
                              className="w-full px-2.5 py-1.5 text-xs font-bold rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                            />
                          </div>

                          {/* Date Début */}
                          <div>
                            <label className="block text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1">
                              Date de début
                            </label>
                            <input
                              type="date"
                              value={editSprintStartDate}
                              onChange={e => setEditSprintStartDate(e.target.value)}
                              className="w-full px-2.5 py-1.5 text-xs font-mono rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
                            />
                          </div>

                          {/* Date Fin */}
                          <div>
                            <label className="block text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1">
                              Date de fin
                            </label>
                            <input
                              type="date"
                              value={editSprintEndDate}
                              onChange={e => setEditSprintEndDate(e.target.value)}
                              className="w-full px-2.5 py-1.5 text-xs font-mono rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
                            />
                          </div>

                          {/* Statut */}
                          <div>
                            <label className="block text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1">
                              Statut du cycle
                            </label>
                            <select
                              value={editSprintState}
                              onChange={e => setEditSprintState(e.target.value as any)}
                              className="w-full px-2.5 py-1.5 text-xs font-semibold rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
                            >
                              <option value="active">🟢 En cours (Active)</option>
                              <option value="future">🔵 À venir (Future)</option>
                              <option value="closed">⚪ Clôturé (Closed)</option>
                            </select>
                          </div>
                        </div>

                        {/* Décalage option */}
                        <div className="flex items-center gap-2 pt-0.5">
                          <label className="flex items-center gap-1.5 text-[11px] text-[var(--text-secondary)] cursor-pointer select-none">
                            <input
                              type="checkbox"
                              checked={editShiftSubsequent}
                              onChange={e => setEditShiftSubsequent(e.target.checked)}
                              className="rounded text-[var(--accent-color)] w-3.5 h-3.5 cursor-pointer"
                            />
                            <span>Décaler automatiquement les dates des sprints suivants pour préserver l'enchaînement</span>
                          </label>
                        </div>
                      </div>
                    ) : (
                      <div
                        className={`border-b border-[var(--border-color)] flex flex-col sm:flex-row sm:items-center justify-between gap-2 ${
                          sprint.state === 'closed'
                            ? 'bg-slate-500/10'
                            : 'bg-[var(--bg-tertiary)]/30'
                        } ${isBacklogOpen ? 'p-3.5' : 'px-3 py-2'}`}
                      >
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3
                            className={`${
                              isBacklogOpen ? 'text-sm' : 'text-xs'
                            } font-extrabold text-[var(--text-primary)]`}
                          >
                            {sprint.name}
                          </h3>

                          {/* State Badges */}
                          {sprint.state === 'closed' ? (
                            <span className="text-[9px] font-bold px-2 py-0.5 rounded-full border bg-slate-500/15 text-slate-400 border-slate-500/30 flex items-center gap-1">
                              <CheckCircle2 size={10} className="text-slate-400" />
                              Clôturé
                            </span>
                          ) : sprint.state === 'active' || rel.type === 'current' ? (
                            <span className="text-[9px] font-bold px-2 py-0.5 rounded-full border bg-emerald-500/15 text-emerald-400 border-emerald-500/30 flex items-center gap-1">
                              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-ping inline-block" />
                              En cours
                            </span>
                          ) : (
                            <span className="text-[9px] font-bold px-2 py-0.5 rounded-full border bg-cyan-500/15 text-cyan-400 border-cyan-500/30">
                              À venir
                            </span>
                          )}

                          <span
                            className={`text-[9px] font-bold px-2 py-0.5 rounded-full border ${
                              rel.type === 'current'
                                ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30 animate-pulse'
                                : rel.type === 'past'
                                ? 'bg-slate-500/15 text-slate-400 border-slate-500/30'
                                : 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30'
                            }`}
                          >
                            {rel.label}
                          </span>
                        </div>

                        {/* Dates & Quick Stats & Actions */}
                        <div className="flex items-center gap-2 flex-wrap text-xs">
                          {/* Clickable Date Range Button */}
                          <button
                            type="button"
                            onClick={() => handleStartEditSprint(index, sprint)}
                            className="flex items-center gap-1 text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] px-2 py-0.5 rounded-lg border border-transparent hover:border-[var(--border-color)] font-mono text-[10.5px] transition-colors cursor-pointer"
                            title="Cliquer pour modifier les dates du sprint"
                          >
                            <CalendarDays size={11} className="text-[var(--accent-color)]" />
                            <span>
                              {formatDateFR(sprint.startDate)} ➔ {formatDateFR(sprint.endDate)}
                            </span>
                            <Edit2 size={9} className="opacity-40 hover:opacity-100 ml-0.5" />
                          </button>

                          {/* Progress bar */}
                          {sprintTasks.length > 0 && (
                            <div className="flex items-center gap-1.5 pl-2 border-l border-[var(--border-color)]">
                              <div className="w-14 h-1.5 rounded-full bg-[var(--bg-primary)] overflow-hidden border border-[var(--border-color)]">
                                <div
                                  className={`h-full transition-all duration-300 ${
                                    sprint.state === 'closed' ? 'bg-slate-400' : 'bg-emerald-500'
                                  }`}
                                  style={{ width: `${progressPct}%` }}
                                />
                              </div>
                              <span className="text-[9.5px] font-mono font-bold text-[var(--text-secondary)]">
                                {progressPct}% ({doneTasks.length}/{sprintTasks.length})
                              </span>
                            </div>
                          )}

                          <div className="flex items-center gap-1 pl-1">
                            {/* Close Sprint / Reopen Sprint Button */}
                            {sprint.state === 'closed' ? (
                              <button
                                type="button"
                                onClick={() => handleReopenSprint(index)}
                                className="flex items-center gap-1 px-2 py-1 rounded-lg text-[10.5px] font-semibold text-cyan-400 bg-cyan-500/10 hover:bg-cyan-500/20 border border-cyan-500/30 transition-all cursor-pointer shadow-xs"
                                title="Réouvrir ce sprint (repasser en statut Actif)"
                              >
                                <RotateCcw size={11} />
                                <span>Réouvrir</span>
                              </button>
                            ) : (
                              <button
                                type="button"
                                onClick={() => handleStartCloseSprint(sprint, index)}
                                className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[10.5px] font-bold text-emerald-300 bg-emerald-500/15 hover:bg-emerald-500/25 border border-emerald-500/30 transition-all cursor-pointer shadow-xs active:scale-95"
                                title="Clôturer ce sprint et choisir la destination des tâches non terminées"
                              >
                                <CheckCircle2 size={12} className="text-emerald-400" />
                                <span>Clôturer</span>
                              </button>
                            )}

                            {/* Edit Sprint Button */}
                            <button
                              type="button"
                              onClick={() => handleStartEditSprint(index, sprint)}
                              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                              title="Modifier les dates et le statut"
                            >
                              <SlidersHorizontal size={12} />
                            </button>

                            {/* Delete Sprint Button */}
                            <button
                              type="button"
                              onClick={() => handleDeleteSprint(index)}
                              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-rose-400 transition-colors cursor-pointer"
                              title="Supprimer ce sprint"
                            >
                              <Trash2 size={12} />
                            </button>

                            {/* Collapse/Expand for Closed Sprints */}
                            {sprint.state === 'closed' && (
                              <button
                                type="button"
                                onClick={() => toggleCollapseSprint(sprint.name)}
                                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                                title={isCollapsed ? "Déplier le sprint" : "Replier le sprint"}
                              >
                                {isCollapsed ? <ChevronDown size={13} /> : <ChevronUp size={13} />}
                              </button>
                            )}
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Collapsed State for Closed Sprints */}
                    {sprint.state === 'closed' && isCollapsed ? (
                      <div className="p-3 text-center text-xs text-[var(--text-muted)] bg-[var(--bg-primary)]/40 flex items-center justify-between">
                        <span>
                          Sprint clôturé ({sprintTasks.length} tâche{sprintTasks.length > 1 ? 's' : ''}, {doneTasks.length} terminée{doneTasks.length > 1 ? 's' : ''})
                        </span>
                        <button
                          type="button"
                          onClick={() => toggleCollapseSprint(sprint.name)}
                          className="text-[11px] text-[var(--accent-color)] hover:underline font-semibold cursor-pointer"
                        >
                          Afficher les tâches
                        </button>
                      </div>
                    ) : !isBacklogOpen ? (
                      <div className="p-2.5">
                        {sprintTasks.length === 0 ? (
                          <p className="text-[11px] text-[var(--text-muted)] italic px-1">
                            0 tâche planifiée dans ce sprint.
                          </p>
                        ) : displayMode === 'chips' ? (
                          <div className="flex flex-wrap gap-1.5">
                            {sprintTasks.map(task => (
                              <div
                                key={task.id}
                                onClick={() => setSelectedTask(task)}
                                className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border text-xs cursor-pointer transition-colors group shadow-2xs ${
                                  checkedTaskIds[task.id]
                                    ? 'bg-[var(--accent-light)] border-[var(--accent-color)] text-[var(--accent-color)] font-semibold'
                                    : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)] text-[var(--text-primary)]'
                                }`}
                                title={`${task.key}: ${task.title}`}
                              >
                                <input
                                  type="checkbox"
                                  checked={Boolean(checkedTaskIds[task.id])}
                                  onChange={e => {
                                    e.stopPropagation()
                                    toggleTaskCheck(task.id)
                                  }}
                                  className="rounded text-[var(--accent-color)] w-3 h-3 cursor-pointer"
                                />
                                <span className="font-mono font-bold text-[10px] text-[var(--accent-color)]">
                                  {task.key}
                                </span>
                                <span className="truncate max-w-[160px] text-[11px] font-medium text-[var(--text-primary)]">
                                  {task.title}
                                </span>
                                {getTaskStageBadge(task)}
                                {task.assignee && (
                                  <Avatar name={task.assignee} url={task.assigneeAvatar} size={14} />
                                )}
                                <button
                                  type="button"
                                  onClick={e => {
                                    e.stopPropagation()
                                    handleRemoveTaskFromSprint(task.id)
                                  }}
                                  className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer ml-0.5"
                                  title="Retirer du sprint (renvoyer au backlog)"
                                >
                                  <X size={11} />
                                </button>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="flex flex-col divide-y divide-[var(--border-color)]/50">
                            {sprintTasks.map(task => (
                              <div
                                key={task.id}
                                onClick={() => setSelectedTask(task)}
                                className={`flex items-center justify-between gap-2 py-1 px-1.5 rounded cursor-pointer transition-colors group ${
                                  checkedTaskIds[task.id]
                                    ? 'bg-[var(--accent-light)]/20 font-medium'
                                    : 'hover:bg-[var(--bg-tertiary)]/60'
                                }`}
                              >
                                <div className="flex items-center gap-2 min-w-0">
                                  <input
                                    type="checkbox"
                                    checked={Boolean(checkedTaskIds[task.id])}
                                    onChange={e => {
                                      e.stopPropagation()
                                      toggleTaskCheck(task.id)
                                    }}
                                    className="rounded text-[var(--accent-color)] w-3 h-3 cursor-pointer"
                                  />
                                  <span className="text-[10px] font-mono font-bold text-[var(--accent-color)] shrink-0">
                                    {task.key}
                                  </span>
                                  {task.parentTitle && (
                                    <span
                                      className="text-[9px] px-1.5 py-0.2 rounded font-bold bg-[var(--bg-tertiary)] text-[var(--text-muted)] truncate max-w-[100px]"
                                      title={task.parentTitle}
                                    >
                                      {task.parentTitle}
                                    </span>
                                  )}
                                  <span className="text-xs text-[var(--text-primary)] truncate max-w-[360px] font-medium">
                                    {task.title}
                                  </span>
                                </div>

                                <div className="flex items-center gap-2 shrink-0">
                                  {getTaskStageBadge(task)}
                                  {task.assignee && (
                                    <Avatar name={task.assignee} url={task.assigneeAvatar} size={15} />
                                  )}
                                  <button
                                    type="button"
                                    onClick={e => {
                                      e.stopPropagation()
                                      setChatTask(task)
                                    }}
                                    className="text-[var(--text-muted)] hover:text-cyan-300 p-0.5 transition-colors opacity-0 group-hover:opacity-100"
                                    title="Discuter avec l'agent IA"
                                  >
                                    <MessageSquare size={11} />
                                  </button>
                                  <button
                                    type="button"
                                    onClick={e => {
                                      e.stopPropagation()
                                      handleRemoveTaskFromSprint(task.id)
                                    }}
                                    className="text-[var(--text-muted)] hover:text-rose-400 p-0.5 transition-colors opacity-0 group-hover:opacity-100 cursor-pointer"
                                    title="Retirer du sprint (renvoyer au backlog)"
                                  >
                                    <X size={11} />
                                  </button>
                                </div>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    ) : (
                      /* Mode 2: Large Planning View (Backlog Open, ACTIVE Drag & Drop) */
                      <div className="p-3.5 space-y-2">
                        {sprintTasks.length === 0 ? (
                          <div className="p-5 rounded-xl border-2 border-dashed border-[var(--border-color)]/70 text-center text-xs text-[var(--text-muted)] bg-[var(--bg-primary)]/40 hover:bg-[var(--bg-primary)]/60 transition-colors">
                            <p className="font-medium">Aucune tâche dans ce sprint</p>
                            <p className="text-[10px] opacity-70 mt-0.5">
                              Glissez des tâches depuis le Backlog ou un autre sprint ici.
                            </p>
                          </div>
                        ) : displayMode === 'chips' ? (
                          /* Chips display in planning mode */
                          <div className="flex flex-wrap gap-1.5">
                            {sprintTasks.map(task => (
                              <div
                                key={task.id}
                                draggable
                                onDragStart={e => handleDragStart(e, task.id)}
                                onClick={() => setSelectedTask(task)}
                                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg border transition-all cursor-grab active:cursor-grabbing group text-xs select-none shadow-2xs ${
                                  checkedTaskIds[task.id]
                                    ? 'bg-[var(--accent-light)] border-[var(--accent-color)] text-[var(--accent-color)] font-semibold'
                                    : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/60 text-[var(--text-primary)]'
                                }`}
                                title={`${task.key}: ${task.title}`}
                              >
                                <input
                                  type="checkbox"
                                  checked={Boolean(checkedTaskIds[task.id])}
                                  onChange={e => {
                                    e.stopPropagation()
                                    toggleTaskCheck(task.id)
                                  }}
                                  className="rounded text-[var(--accent-color)] w-3 h-3 cursor-pointer"
                                />
                                <span className="font-mono font-bold text-[10px] text-[var(--accent-color)] shrink-0">
                                  {task.key}
                                </span>
                                <span className="truncate max-w-[170px] text-[11px] font-medium">
                                  {task.title}
                                </span>
                                {getTaskStageBadge(task)}
                                {task.assignee && (
                                  <Avatar name={task.assignee} url={task.assigneeAvatar} size={14} />
                                )}
                                <button
                                  type="button"
                                  onClick={e => {
                                    e.stopPropagation()
                                    handleRemoveTaskFromSprint(task.id)
                                  }}
                                  className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                                  title="Retirer du sprint"
                                >
                                  <X size={11} />
                                </button>
                              </div>
                            ))}
                          </div>
                        ) : (
                          /* Cards display in planning mode */
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                            {sprintTasks.map(task => (
                              <div
                                key={task.id}
                                draggable
                                onDragStart={e => handleDragStart(e, task.id)}
                                onClick={() => setSelectedTask(task)}
                                className={`p-2.5 rounded-xl border shadow-xs transition-all cursor-grab active:cursor-grabbing group space-y-1.5 ${
                                  checkedTaskIds[task.id]
                                    ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)]'
                                    : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/50'
                                }`}
                              >
                                <div className="flex items-center justify-between gap-2">
                                  <div className="flex items-center gap-1.5 min-w-0">
                                    <input
                                      type="checkbox"
                                      checked={Boolean(checkedTaskIds[task.id])}
                                      onChange={e => {
                                        e.stopPropagation()
                                        toggleTaskCheck(task.id)
                                      }}
                                      className="rounded text-[var(--accent-color)] w-3 h-3 cursor-pointer"
                                    />
                                    <span className="text-[10px] font-mono font-bold text-[var(--accent-color)] shrink-0">
                                      {task.key}
                                    </span>
                                    {task.parentTitle && (
                                      <span
                                        className="text-[9px] px-1.5 py-0.2 rounded font-bold bg-[var(--bg-tertiary)] text-[var(--text-muted)] truncate max-w-[110px]"
                                        title={task.parentTitle}
                                      >
                                        {task.parentTitle}
                                      </span>
                                    )}
                                  </div>

                                  <div className="flex items-center gap-1 shrink-0">
                                    {task.assignee && (
                                      <Avatar name={task.assignee} url={task.assigneeAvatar} size={16} />
                                    )}
                                    <button
                                      type="button"
                                      onClick={e => {
                                        e.stopPropagation()
                                        handleRemoveTaskFromSprint(task.id)
                                      }}
                                      className="p-1 rounded text-[var(--text-muted)] hover:text-rose-400 hover:bg-rose-500/10 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                                      title="Retirer du sprint (renvoyer au backlog)"
                                    >
                                      <X size={11} />
                                    </button>
                                  </div>
                                </div>

                                <h4 className="text-xs font-semibold text-[var(--text-primary)] line-clamp-1 leading-snug">
                                  {task.title}
                                </h4>

                                <div className="flex items-center justify-between text-[10px] pt-1 border-t border-[var(--border-color)]/40 text-[var(--text-muted)]">
                                  {getTaskStageBadge(task)}

                                  <button
                                    type="button"
                                    onClick={e => {
                                      e.stopPropagation()
                                      setChatTask(task)
                                    }}
                                    className="text-[var(--text-muted)] hover:text-cyan-300 p-0.5 transition-colors cursor-pointer"
                                    title="Discuter avec l'agent IA"
                                  >
                                    <MessageSquare size={11} />
                                  </button>
                                </div>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Backlog / Unscheduled Drawer (Right Side with Resizer) */}
        {isBacklogOpen && (
          <div
            style={{ width: `${backlogWidth}px` }}
            className="border-l border-[var(--border-color)] bg-[var(--bg-secondary)] flex flex-col shrink-0 min-h-0 relative select-none"
          >
            {/* Left Edge Resizer Handle */}
            <div
              onPointerDown={handleStartResizeBacklog}
              className="absolute left-0 top-0 bottom-0 w-2 -ml-1 cursor-col-resize hover:bg-[var(--accent-color)]/50 active:bg-[var(--accent-color)] transition-colors z-30 flex items-center justify-center group"
              title="Glisser pour redimensionner le panneau Backlog"
            >
              <div className="w-0.5 h-6 bg-[var(--border-color)] group-hover:bg-white rounded-full" />
            </div>

            {/* Backlog Header */}
            <div className="p-3 border-b border-[var(--border-color)] flex items-center justify-between gap-2 shrink-0">
              <div className="flex items-center gap-2">
                <Layers size={14} className="text-cyan-400" />
                <h3 className="text-xs font-bold text-[var(--text-primary)]">
                  Backlog non planifié
                </h3>
                <span className="text-[10px] font-mono font-bold px-1.5 py-0.2 rounded-full bg-[var(--bg-tertiary)] text-[var(--text-secondary)]">
                  {unscheduledTasks.length}
                </span>
              </div>

              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => setIsBacklogOpen(false)}
                  className="p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
                  title="Fermer le Backlog"
                >
                  <X size={14} />
                </button>
              </div>
            </div>

            {/* Search filter in Backlog */}
            <div className="p-2 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 flex items-center gap-2 shrink-0">
              <div className="relative flex-1">
                <input
                  type="text"
                  value={backlogSearch}
                  onChange={e => setBacklogSearch(e.target.value)}
                  placeholder="Filtrer le backlog..."
                  className="w-full pl-7 pr-2 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                />
                <Search size={12} className="absolute left-2.5 top-2 text-[var(--text-muted)]" />
              </div>

              {finishedUnscheduledCount > 0 && (
                <button
                  type="button"
                  onClick={() => setShowDoneInBacklog(prev => !prev)}
                  className={`flex items-center gap-1 px-2 py-1 rounded-lg text-[10.5px] font-semibold border transition-all cursor-pointer shrink-0 ${
                    showDoneInBacklog
                      ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                      : 'bg-[var(--bg-primary)] text-[var(--text-muted)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
                  }`}
                  title={showDoneInBacklog ? 'Masquer les tâches terminées (#finished)' : `Afficher les ${finishedUnscheduledCount} tâches terminées`}
                >
                  <CheckCircle2 size={11} />
                  <span>{showDoneInBacklog ? 'Terminées' : `Terminées (${finishedUnscheduledCount})`}</span>
                </button>
              )}
            </div>

            {/* Multi-Selection Action Strip */}
            <div className="px-3 py-1.5 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] flex items-center justify-between gap-2 text-xs shrink-0">
              <button
                type="button"
                onClick={handleSelectAllBacklog}
                className="flex items-center gap-1.5 text-[11px] font-semibold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {unscheduledTasks.length > 0 && unscheduledTasks.every(t => checkedTaskIds[t.id]) ? (
                  <>
                    <CheckSquare size={13} className="text-[var(--accent-color)]" />
                    <span>Tout désélectionner</span>
                  </>
                ) : (
                  <>
                    <Square size={13} />
                    <span>Tout cocher</span>
                  </>
                )}
              </button>

              {selectedBacklogList.length > 0 && (
                <span className="text-[10px] font-bold text-[var(--accent-color)] bg-[var(--accent-light)] px-1.5 py-0.5 rounded border border-[var(--accent-color)]/30">
                  {selectedBacklogList.length} sélectionné(s)
                </span>
              )}
            </div>

            {/* Batch Action Bar (if multiple items selected) */}
            {selectedBacklogList.length > 0 && (
              <div className="p-2 border-b border-[var(--border-color)] bg-[var(--accent-light)]/25 flex items-center gap-1.5 shrink-0">
                <select
                  value={batchTargetSprint}
                  onChange={e => setBatchTargetSprint(e.target.value)}
                  className="flex-1 text-xs px-2 py-1 rounded bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer"
                >
                  <option value="">Déplacer vers un sprint...</option>
                  {sprints.map(sp => (
                    <option key={sp.name} value={sp.name}>
                      {sp.name}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  disabled={!batchTargetSprint || batchBusy}
                  onClick={handleApplyBatchSprint}
                  className="px-2.5 py-1 rounded text-xs font-bold bg-[var(--accent-color)] text-white hover:opacity-90 disabled:opacity-40 cursor-pointer shadow-xs"
                >
                  {batchBusy ? '…' : 'Déplacer'}
                </button>
                <button
                  type="button"
                  onClick={() => startBatchPickup(selectedBacklogList.map(t => t.id))}
                  className="flex items-center gap-1 px-2.5 py-1 rounded text-xs font-bold text-purple-300 bg-purple-950/60 hover:bg-purple-900/80 border border-purple-700/50 cursor-pointer shadow-xs shrink-0"
                  title="Démarrer un unique Git Worktree et exécuter le lot de tâches du backlog"
                >
                  <Sparkles size={12} className="text-purple-400 animate-pulse" />
                  <span>Auto-pilot</span>
                </button>
              </div>
            )}

            {/* Backlog List */}
            <div className="flex-1 overflow-y-auto p-2.5 space-y-1.5">
              {unscheduledTasks.length === 0 ? (
                <div className="py-12 text-center text-xs text-[var(--text-muted)]">
                  {backlogSearch ? 'Aucun résultat' : 'Toutes les tâches sont planifiées ! 🎉'}
                </div>
              ) : displayMode === 'chips' ? (
                /* Chips representation in Backlog */
                <div className="flex flex-wrap gap-1.5">
                  {unscheduledTasks.map(task => (
                    <div
                      key={task.id}
                      draggable
                      onDragStart={e => handleDragStart(e, task.id)}
                      onClick={() => setSelectedTask(task)}
                      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg border transition-all cursor-grab active:cursor-grabbing group text-xs select-none shadow-2xs max-w-full ${
                        checkedTaskIds[task.id]
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)] text-[var(--accent-color)] font-semibold'
                          : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/60 text-[var(--text-primary)]'
                      }`}
                      title={`${task.key}: ${task.title}`}
                    >
                      <input
                        type="checkbox"
                        checked={Boolean(checkedTaskIds[task.id])}
                        onChange={e => {
                          e.stopPropagation()
                          toggleTaskCheck(task.id)
                        }}
                        className="rounded text-[var(--accent-color)] w-3 h-3 cursor-pointer"
                      />
                      <span className="font-mono font-bold text-[10px] text-[var(--accent-color)] shrink-0">
                        {task.key}
                      </span>
                      <span className="truncate max-w-[160px] text-[11px] font-medium">
                        {task.title}
                      </span>
                      <select
                        value=""
                        onChange={e => {
                          if (e.target.value) {
                            setTaskSprint(task.id, e.target.value, e.target.value)
                          }
                        }}
                        onClick={e => e.stopPropagation()}
                        className="text-[9px] font-semibold px-1 py-0.2 rounded bg-[var(--bg-tertiary)] text-[var(--accent-color)] border border-[var(--border-color)] focus:outline-none cursor-pointer ml-auto"
                      >
                        <option value="">+ Sprint</option>
                        {sprints.map(sp => (
                          <option key={sp.name} value={sp.name}>
                            {sp.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
              ) : (
                /* Cards representation in Backlog */
                unscheduledTasks.map(task => (
                  <div
                    key={task.id}
                    draggable
                    onDragStart={e => handleDragStart(e, task.id)}
                    onClick={() => setSelectedTask(task)}
                    className={`p-2.5 rounded-xl border shadow-xs transition-all cursor-grab active:cursor-grabbing space-y-1.5 group ${
                      checkedTaskIds[task.id]
                        ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)]'
                        : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/60'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-1.5">
                      <div className="flex items-center gap-1.5 min-w-0">
                        <input
                          type="checkbox"
                          checked={Boolean(checkedTaskIds[task.id])}
                          onChange={e => {
                            e.stopPropagation()
                            toggleTaskCheck(task.id)
                          }}
                          className="rounded text-[var(--accent-color)] w-3 h-3 cursor-pointer"
                        />
                        <span className="text-[10px] font-mono font-bold text-[var(--accent-color)] shrink-0">
                          {task.key}
                        </span>
                        {task.parentTitle && (
                          <span className="text-[9px] px-1.5 py-0.2 rounded font-bold bg-[var(--bg-tertiary)] text-[var(--text-muted)] truncate max-w-[120px]">
                            {task.parentTitle}
                          </span>
                        )}
                      </div>

                      {task.assignee && (
                        <Avatar name={task.assignee} url={task.assigneeAvatar} size={15} />
                      )}
                    </div>

                    <h4 className="text-xs font-medium text-[var(--text-primary)] line-clamp-2 leading-snug">
                      {task.title}
                    </h4>

                    <div className="flex items-center justify-between pt-1 border-t border-[var(--border-color)]/40 text-[10px]">
                      <span className="text-[var(--text-muted)] flex items-center gap-1">
                        <GripVertical size={11} className="text-[var(--text-muted)]" /> Glisser
                      </span>

                      {/* Quick Assign Dropdown */}
                      <select
                        value=""
                        onChange={e => {
                          if (e.target.value) {
                            setTaskSprint(task.id, e.target.value, e.target.value)
                          }
                        }}
                        onClick={e => e.stopPropagation()}
                        className="text-[9.5px] font-semibold px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--accent-color)] border border-[var(--border-color)] focus:outline-none cursor-pointer"
                      >
                        <option value="">+ Sprint</option>
                        {sprints.map(sp => (
                          <option key={sp.name} value={sp.name}>
                            {sp.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>

      {/* Close Sprint Modal Dialog */}
      {closingSprint && (
        <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="relative w-full max-w-lg rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-150">
            {/* Modal Header */}
            <div className="flex items-center justify-between px-5 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
              <div className="flex items-center gap-2.5">
                <div className="p-2 rounded-xl bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                  <CheckCircle2 size={18} />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-[var(--text-primary)]">
                    Clôturer {closingSprint.sprint.name}
                  </h3>
                  <span className="text-[10.5px] text-[var(--text-muted)]">
                    Bilan du cycle et réaffectation des tâches restantes
                  </span>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setClosingSprint(null)}
                className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              >
                <X size={16} />
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-5 space-y-4 text-xs">
              {(() => {
                const sprintKey = closingSprint.sprint.name.toLowerCase().trim()
                const sprintTasks = tasksBySprint.get(sprintKey) || []
                const doneTasks = sprintTasks.filter(
                  t => t.status === 'finished' || t.status === 'done' || resolveTaskStage(t, currentProject) === 'finished'
                )
                const unfinishedTasks = sprintTasks.filter(
                  t => !(t.status === 'finished' || t.status === 'done' || resolveTaskStage(t, currentProject) === 'finished')
                )
                const nextSprint = closingSprint.index < sprints.length - 1 ? sprints[closingSprint.index + 1] : null

                return (
                  <>
                    {/* Stats summary cards */}
                    <div className="grid grid-cols-2 gap-3">
                      <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex flex-col">
                        <span className="text-[10px] uppercase tracking-wider font-bold text-emerald-400">
                          Tâches Terminées
                        </span>
                        <span className="text-2xl font-extrabold text-emerald-300 mt-1">
                          {doneTasks.length}
                        </span>
                        <span className="text-[10px] text-[var(--text-muted)] mt-1">
                          Livrées dans ce cycle
                        </span>
                      </div>

                      <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 flex flex-col">
                        <span className="text-[10px] uppercase tracking-wider font-bold text-amber-400">
                          Tâches Non Terminées
                        </span>
                        <span className="text-2xl font-extrabold text-amber-300 mt-1">
                          {unfinishedTasks.length}
                        </span>
                        <span className="text-[10px] text-[var(--text-muted)] mt-1">
                          {unfinishedTasks.length === 0 ? 'Aucun reliquat !' : 'À transférer ou reporter'}
                        </span>
                      </div>
                    </div>

                    {unfinishedTasks.length > 0 ? (
                      <div className="space-y-2 pt-1">
                        <label className="block text-[11.5px] font-bold text-[var(--text-primary)]">
                          Que faire des {unfinishedTasks.length} tâche(s) non terminée(s) ?
                        </label>

                        <div className="space-y-2">
                          {nextSprint && (
                            <label
                              className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all ${
                                closeSprintDestination === 'next'
                                  ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)] ring-1 ring-[var(--accent-color)] shadow-xs'
                                  : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--border-color)]/80'
                              }`}
                            >
                              <input
                                type="radio"
                                name="close_dest"
                                checked={closeSprintDestination === 'next'}
                                onChange={() => setCloseSprintDestination('next')}
                                className="mt-0.5 text-[var(--accent-color)] cursor-pointer"
                              />
                              <div className="space-y-0.5">
                                <span className="font-bold text-[var(--text-primary)] block">
                                  Transférer vers {nextSprint.name} (Recommandé)
                                </span>
                                <span className="text-[10.5px] text-[var(--text-muted)] block leading-snug">
                                  Les tâches ouvertes rejoindront la liste des priorités du prochain cycle.
                                </span>
                              </div>
                            </label>
                          )}

                          <label
                            className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all ${
                              closeSprintDestination === 'backlog'
                                ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)] ring-1 ring-[var(--accent-color)] shadow-xs'
                                : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--border-color)]/80'
                            }`}
                          >
                            <input
                              type="radio"
                              name="close_dest"
                              checked={closeSprintDestination === 'backlog'}
                              onChange={() => setCloseSprintDestination('backlog')}
                              className="mt-0.5 text-[var(--accent-color)] cursor-pointer"
                            />
                            <div className="space-y-0.5">
                              <span className="font-bold text-[var(--text-primary)] block">
                                Renvoyer vers le Backlog général
                              </span>
                              <span className="text-[10.5px] text-[var(--text-muted)] block leading-snug">
                                Les tâches redeviendront non planifiées dans le backlog sans assignation de sprint.
                              </span>
                            </div>
                          </label>

                          <label
                            className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all ${
                              closeSprintDestination === 'keep'
                                ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)] ring-1 ring-[var(--accent-color)] shadow-xs'
                                : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--border-color)]/80'
                            }`}
                          >
                            <input
                              type="radio"
                              name="close_dest"
                              checked={closeSprintDestination === 'keep'}
                              onChange={() => setCloseSprintDestination('keep')}
                              className="mt-0.5 text-[var(--accent-color)] cursor-pointer"
                            />
                            <div className="space-y-0.5">
                              <span className="font-bold text-[var(--text-primary)] block">
                                Conserver dans ce sprint clôturé
                              </span>
                              <span className="text-[10.5px] text-[var(--text-muted)] block leading-snug">
                                Les tâches resteront associées à {closingSprint.sprint.name} pour l'historique.
                              </span>
                            </div>
                          </label>
                        </div>
                      </div>
                    ) : (
                      <div className="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-center space-y-1">
                        <p className="font-bold text-emerald-300">Félicitations ! 🎉</p>
                        <p className="text-[11px] text-[var(--text-muted)]">
                          Toutes les tâches de ce sprint ont été menées à terme. Aucune réaffectation n'est nécessaire.
                        </p>
                      </div>
                    )}

                    {nextSprint && nextSprint.state === 'future' && (
                      <div className="p-3 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] flex items-center justify-between">
                        <div>
                          <span className="font-semibold text-[var(--text-primary)] block text-xs">
                            Activer automatiquement {nextSprint.name}
                          </span>
                          <span className="text-[10px] text-[var(--text-muted)] block">
                            Passe le statut du prochain sprint à "En cours".
                          </span>
                        </div>
                        <input
                          type="checkbox"
                          checked={closeSprintActivateNext}
                          onChange={e => setCloseSprintActivateNext(e.target.checked)}
                          className="rounded text-[var(--accent-color)] w-4 h-4 cursor-pointer"
                        />
                      </div>
                    )}
                  </>
                )
              })()}
            </div>

            {/* Modal Footer */}
            <div className="flex items-center justify-end gap-2 px-5 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/30">
              <button
                type="button"
                onClick={() => setClosingSprint(null)}
                disabled={isClosingSprintBusy}
                className="px-3.5 py-1.5 rounded-xl text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] border border-[var(--border-color)] transition-colors cursor-pointer"
              >
                Annuler
              </button>
              <button
                type="button"
                onClick={handleConfirmCloseSprint}
                disabled={isClosingSprintBusy}
                className="flex items-center gap-1.5 px-4 py-1.5 rounded-xl text-xs font-bold text-white bg-emerald-600 hover:bg-emerald-500 transition-all shadow-xs cursor-pointer disabled:opacity-50"
              >
                {isClosingSprintBusy ? (
                  <Loader2 size={13} className="animate-spin" />
                ) : (
                  <CheckCircle2 size={13} />
                )}
                <span>Confirmer la clôture</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
