import React, { useState, useMemo, useRef, useCallback } from 'react'
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
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { useEscapeKey } from '../hooks/useEscapeKey'
import {
  calculateSprintDates,
  shiftSubsequentSprintDates,
  generateDefaultSprints,
  formatDateISO,
  formatDateInput,
  getMonday,
  getSprintRelativeInfo,
  nextBatchStart,
  nextSprintAfter,
  sprintManagementOf,
  sprintTarget,
} from '../lib/sprints'
import { createSprints, deleteSprint, updateSprint, SprintRequestError, type SprintPatch } from '../lib/sprintApi'
import { format, plural, formatDate } from '../lib/i18n'
import type { Task, TrackerSprint, WorkflowStage } from '../types'
import { resolveTaskStage } from '../lib/workflow'
import { Avatar } from './Avatar'
import { EpicBar, useEpicColors } from './EpicMarker'

const DRAG_TASK_ID = 'application/x-sectile-task-id'
const DRAG_TASK_IDS = 'application/x-sectile-task-ids'

export const SprintTimelineView: React.FC = () => {
  const {
    tasks,
    currentProject,
    updateProject,
    fetchProjects,
    refreshTasks,
    setTaskSprint,
    setTasksSprint,
    setSelectedTask,
    addToast,
    startBatchPickup,
    t,
    settings,
  } = useApp()
  const st = t.sprints
  const lang = settings.language
  const showDate = (value?: string) => (value ? formatDate(lang, value) : '')
  const showsEpicColors = useEpicColors()
  // True when the task carries its epic's bar: its project asks for it and it
  // has an epic.
  const showsEpicBarOf = (task: Task) =>
    showsEpicColors(task.projectId) && Boolean(task.parentKey?.trim())

  // Who owns the sprints: the tracker (written there first), nobody (GitHub:
  // read-only), or the local board, which keeps its own.
  const management = sprintManagementOf(currentProject)
  const trackerOwned = management === 'tracker'
  const readOnly = management === 'readonly'

  // Project sprints. Only the local board invents default ones: a tracker's
  // sprints are the tracker's, and made-up ids could never receive a ticket.
  const sprints: TrackerSprint[] = useMemo(() => {
    if (currentProject?.sprints && currentProject.sprints.length > 0) {
      return currentProject.sprints
    }
    return management === 'local' ? generateDefaultSprints(4) : []
  }, [currentProject?.sprints, management])

  // Tracker-owned batch creation form.
  const [isCreatingBatch, setIsCreatingBatch] = useState(false)
  const [batchPattern, setBatchPattern] = useState('Sprint {n}')
  const [batchCount, setBatchCount] = useState(1)
  const [batchBusyCreate, setBatchBusyCreate] = useState(false)

  // Runs one tracker write, then re-reads the project to show the tracker's
  // answer. A refusal is shown with the tracker's reason, and nothing changes.
  const trackerWrite = async <T,>(title: string, write: () => Promise<T>): Promise<T | null> => {
    try {
      const result = await write()
      // The server re-links the tasks a rename or a close moved, so both are
      // read back, not only the sprint list.
      await Promise.all([fetchProjects(), refreshTasks()])
      return result
    } catch (err) {
      const description =
        err instanceof SprintRequestError && !err.message
          ? format(st.feedback.serverRefused, { status: err.status })
          : err instanceof Error
          ? err.message
          : String(err)
      addToast({ type: 'error', title, description })
      return null
    }
  }
  const moveTarget = (value: string) => sprintTarget(sprints, value, management)

  // Configuration state
  const [durationDays, setDurationDays] = useState<number>(14)
  const [startDateStr, setStartDateStr] = useState<string>(() => {
    if (sprints.length > 0 && sprints[0].startDate) {
      // A tracker writes RFC3339; the date input and the batch route take a day.
      return formatDateInput(sprints[0].startDate)
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

  const closeSprintDialog = useCallback(() => setClosingSprint(null), [])
  const closeSprintBackdrop = useBackdropDismiss(closeSprintDialog)
  useEscapeKey(closingSprint !== null, closeSprintDialog)

  // Fold/Collapse Closed Sprints State
  const [collapsedSprints, setCollapsedSprints] = useState<Record<string, boolean>>({})
  const [hideClosedSprints, setHideClosedSprints] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem('sectile_sprint_hide_closed')
      if (stored !== null) return stored === 'true'
    } catch {}
    return true // Masqués par défaut
  })

  const handleToggleHideClosedSprints = () => {
    setHideClosedSprints(prev => {
      const next = !prev
      try {
        localStorage.setItem('sectile_sprint_hide_closed', String(next))
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
      const stored = localStorage.getItem('sectile_sprint_display_mode')
      return stored === 'chips' ? 'chips' : 'cards'
    } catch {
      return 'cards'
    }
  })

  const handleSetDisplayMode = (mode: 'cards' | 'chips') => {
    setDisplayMode(mode)
    try {
      localStorage.setItem('sectile_sprint_display_mode', mode)
    } catch {}
  }

  // Backlog Width Resizing State
  const [backlogWidth, setBacklogWidth] = useState<number>(() => {
    try {
      const stored = localStorage.getItem('sectile_sprint_backlog_width')
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
        localStorage.setItem('sectile_sprint_backlog_width', String(backlogWidth))
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
      <span className={`px-1.5 py-0.2 rounded font-mono font-bold text-[8.5px] border shrink-0 ${info.style}`} title={format(st.timeline.stageTitle, { stage: info.label })}>
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
        title: st.feedback.saved,
        description: plural(lang, newSprints.length, st.feedback.savedDescription),
      })
    }
  }

  // Recalculate all dates automatically
  const handleRecalculateAll = async () => {
    const updated = calculateSprintDates(sprints, startDateStr, durationDays)
    await saveSprints(updated)
  }

  // Add next consecutive sprint. On a tracker, a batch is created there: the
  // form opens, starting the day after the last sprint.
  const handleAddSprint = async () => {
    if (trackerOwned) {
      setStartDateStr(nextBatchStart(sprints, startDateStr))
      setIsCreatingBatch(true)
      return
    }
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

  const handleCreateBatch = async () => {
    if (!currentProject?.id) return
    setBatchBusyCreate(true)
    const result = await trackerWrite(st.batch.createFailed, () =>
      createSprints(currentProject.id, { name: batchPattern, count: batchCount, start: startDateStr, weeks: Math.round(durationDays / 7) })
    )
    setBatchBusyCreate(false)
    if (!result) return
    if (result.error) {
      addToast({ type: 'warning', title: st.batch.interrupted, description: result.error })
    } else {
      addToast({ type: 'success', title: st.batch.created, description: plural(lang, result.created.length, st.batch.createdDescription) })
      setIsCreatingBatch(false)
    }
  }

  // Delete a sprint
  const handleDeleteSprint = async (index: number) => {
    const sprintToDelete = sprints[index]
    if (!window.confirm(format(st.dialogs.deleteConfirm, { name: sprintToDelete.name }))) {
      return
    }
    if (trackerOwned) {
      if (!currentProject?.id || !sprintToDelete.id) return
      const done = await trackerWrite(st.dialogs.deleteFailed, () => deleteSprint(currentProject.id, sprintToDelete.id!).then(() => true))
      if (done) addToast({ type: 'success', title: format(st.dialogs.deleted, { name: sprintToDelete.name }), description: st.dialogs.deletedOnTracker })
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

    if (trackerOwned) {
      // The tracker renames and re-dates; the server re-links the tasks that
      // carried the old name. Only what changed is sent.
      const current = sprints[index]
      if (!currentProject?.id || !current.id) return
      const patch: SprintPatch = {}
      if (newName !== oldName) patch.name = newName
      if (current.state === 'closed') {
        // Jira keeps a closed sprint's dates and state; only its name changes.
        setEditingSprintIndex(null)
        if (patch.name) await trackerWrite(st.dialogs.editFailed, () => updateSprint(currentProject.id, current.id!, patch))
        return
      }
      if (editSprintStartDate && editSprintStartDate !== formatDateInput(current.startDate)) patch.start = editSprintStartDate
      if (editSprintEndDate && editSprintEndDate !== formatDateInput(current.endDate)) patch.end = editSprintEndDate
      if (editSprintState !== current.state) patch.state = editSprintState
      setEditingSprintIndex(null)
      if (Object.keys(patch).length === 0) return
      await trackerWrite(st.dialogs.editFailed, () => updateSprint(currentProject.id, current.id!, patch))
      return
    }

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
    setCloseSprintDestination((trackerOwned ? nextSprintAfter(sprints, sprint) !== null : index < sprints.length - 1) ? 'next' : 'backlog')
    setCloseSprintActivateNext(true)
  }

  const handleConfirmCloseSprint = async () => {
    if (!closingSprint || !currentProject?.id) return
    const { sprint, index } = closingSprint
    setIsClosingSprintBusy(true)

    if (trackerOwned) {
      // The server moves the unfinished work on the tracker, then closes; the
      // sprint it hands over to is the next one by start date, as here.
      const nextSprint = nextSprintAfter(sprints, sprint)
      const patch: SprintPatch = { state: 'closed' }
      if (closeSprintDestination === 'next' || closeSprintDestination === 'backlog') patch.moveOpenTo = closeSprintDestination
      const closed = sprint.id ? await trackerWrite(st.close.failed, () => updateSprint(currentProject.id, sprint.id!, patch)) : null
      if (closed && closeSprintActivateNext && nextSprint?.id && nextSprint.state === 'future') {
        await trackerWrite(format(st.close.startFailed, { name: nextSprint.name }), () => updateSprint(currentProject.id, nextSprint.id!, { state: 'active' }))
      }
      if (closed) {
        addToast({ type: 'success', title: format(st.close.closed, { name: sprint.name }), description: st.close.closedOnTracker })
        setClosingSprint(null)
      }
      setIsClosingSprintBusy(false)
      return
    }

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
        title: format(st.close.closed, { name: sprint.name }),
        description: unfinishedTasks.length > 0
          ? closeSprintDestination === 'next' && nextSprint
            ? plural(lang, unfinishedTasks.length, st.close.movedToNext, { name: nextSprint.name })
            : closeSprintDestination === 'backlog'
            ? plural(lang, unfinishedTasks.length, st.close.sentToBacklog)
            : plural(lang, unfinishedTasks.length, st.close.keptInSprint)
          : st.close.allDone,
      })

      setClosingSprint(null)
    } catch (err: any) {
      addToast({
        type: 'error',
        title: st.close.error,
        description: err.message,
      })
    } finally {
      setIsClosingSprintBusy(false)
    }
  }

  const handleReopenSprint = async (index: number) => {
    if (trackerOwned) {
      const sprint = sprints[index]
      if (!currentProject?.id || !sprint.id) return
      const reopened = await trackerWrite(st.dialogs.reopenFailed, () => updateSprint(currentProject.id, sprint.id!, { state: 'active' }))
      if (reopened) addToast({ type: 'info', title: format(st.dialogs.reopened, { name: sprint.name }), description: st.dialogs.reopenedOnTracker })
      return
    }
    const updated = sprints.map((sp, i) => (i === index ? { ...sp, state: 'active' } : sp))
    await saveSprints(updated)
    addToast({
      type: 'info',
      title: format(st.dialogs.reopened, { name: sprints[index].name }),
      description: st.dialogs.reopenedLocal,
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
    if (taskIds.length === 0 || readOnly) return
    const target = moveTarget(sprintName)

    if (currentProject?.id && taskIds.length > 1) {
      await setTasksSprint(currentProject.id, taskIds, target.id, target.name)
      addToast({
        type: 'success',
        title: st.backlog.planned,
        description: plural(lang, taskIds.length, st.backlog.plannedDescription, { name: sprintName }),
      })
    } else {
      for (const id of taskIds) {
        await setTaskSprint(id, target.id, target.name)
      }
    }

    setCheckedTaskIds({})
  }

  const handleRemoveTaskFromSprint = async (taskId: string) => {
    if (readOnly) return
    await setTaskSprint(taskId, '', '')
  }

  // Batch assign from Backlog
  const handleApplyBatchSprint = async () => {
    if (readOnly) return
    const ids = Object.keys(checkedTaskIds).filter(id => checkedTaskIds[id])
    if (ids.length === 0 || !batchTargetSprint) return

    setBatchBusy(true)
    const target = moveTarget(batchTargetSprint)
    if (currentProject?.id) {
      await setTasksSprint(currentProject.id, ids, target.id, target.name)
      addToast({
        type: 'success',
        title: st.backlog.assigned,
        description: plural(lang, ids.length, st.backlog.assignedDescription, { name: batchTargetSprint }),
      })
    } else {
      for (const id of ids) {
        await setTaskSprint(id, target.id, target.name)
      }
    }
    setCheckedTaskIds({})
    setBatchTargetSprint('')
    setBatchBusy(false)
  }

  // Batch remove tasks from sprint (send back to backlog)
  const handleBatchRemoveFromSprint = async () => {
    if (readOnly) return
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
                  {st.timeline.title}
                </h2>
                <span className="text-[10px] text-[var(--text-muted)]">
                  {st.timeline.subtitle}
                </span>
              </div>
            </div>

            <div className="h-4 w-px bg-[var(--border-color)] hidden sm:block mx-1" />

            {/* Sprint Duration Selector */}
            <div className="flex items-center gap-1 bg-[var(--bg-tertiary)] p-0.5 rounded-lg border border-[var(--border-color)] text-xs">
              <span className="text-[10px] text-[var(--text-muted)] font-medium px-2 flex items-center gap-1">
                <Clock size={11} /> {st.timeline.duration}
              </span>
              {[7, 14, 21, 28].map(days => ({ label: format(st.timeline.durationOption, { weeks: days / 7, days }), days })).map(d => (
                <button
                  key={d.days}
                  type="button"
                  onClick={() => {
                    setDurationDays(d.days)
                    if (management !== 'local') return
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
              <span className="text-[10px] text-[var(--text-muted)] font-medium">{st.timeline.startS1}</span>
              <input
                type="date"
                value={startDateStr}
                onChange={e => {
                  setStartDateStr(e.target.value)
                  if (management !== 'local') return
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
                title={st.timeline.cardsTitle}
              >
                <LayoutGrid size={12} />
                <span className="hidden sm:inline">{st.timeline.cards}</span>
              </button>
              <button
                type="button"
                onClick={() => handleSetDisplayMode('chips')}
                className={`px-2 py-1 rounded flex items-center gap-1 text-[11px] font-semibold transition-all cursor-pointer ${
                  displayMode === 'chips'
                    ? 'bg-[var(--accent-color)] text-white shadow-xs font-bold'
                    : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                }`}
                title={st.timeline.chipsTitle}
              >
                <Tag size={12} />
                <span className="hidden sm:inline">{st.timeline.chips}</span>
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
                title={hideClosedSprints ? st.timeline.showClosed : st.timeline.hideClosed}
              >
                <Archive size={12} className={hideClosedSprints ? "text-slate-400" : "text-[var(--text-muted)]"} />
                <span>
                  {format(hideClosedSprints ? st.timeline.closedHidden : st.timeline.closedShown, {
                    count: sprints.filter(s => s.state === 'closed').length,
                  })}
                </span>
              </button>
            )}

            {management === 'local' && <button
              type="button"
              onClick={handleRecalculateAll}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-semibold bg-[var(--bg-tertiary)] hover:bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] transition-all cursor-pointer shadow-xs"
              title={st.timeline.recalculateTitle}
            >
              <RefreshCw size={12} className="text-cyan-400" />
              <span>{st.timeline.recalculate}</span>
            </button>}

            {!readOnly && <button
              type="button"
              disabled={trackerOwned && !currentProject?.boardId}
              onClick={handleAddSprint}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold bg-[var(--accent-color)] hover:opacity-90 text-white transition-all cursor-pointer shadow-xs"
              title={trackerOwned && !currentProject?.boardId ? st.timeline.chooseBoardFirst : st.timeline.addSprintTitle}
            >
              <Plus size={13} />
              <span>{trackerOwned ? st.timeline.addSprints : st.timeline.addSprint}</span>
            </button>}

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
                  ? st.timeline.hideBacklogTitle
                  : st.timeline.showBacklogTitle
              }
            >
              <Layers size={13} />
              <span>
                {isBacklogOpen ? st.timeline.backlogOpen : format(st.timeline.showBacklog, { count: unscheduledTasks.length })}
              </span>
            </button>
          </div>
        </div>
      </div>

      {/* Tracker-owned batch creation: written to the tracker, shown from its answer. */}
      {trackerOwned && isCreatingBatch && (
        <div className="border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 px-4 py-2 flex items-center gap-2 flex-wrap text-xs shrink-0">
          <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">{st.batch.heading}</span>
          <input
            aria-label={st.batch.patternLabel}
            value={batchPattern}
            onChange={e => setBatchPattern(e.target.value)}
            placeholder="Sprint {n}"
            className="px-2 py-1 rounded bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono text-[11px] w-36"
            title={st.batch.patternTitle}
          />
          <label className="flex items-center gap-1 text-[var(--text-secondary)]">
            {st.batch.count}
            <input
              type="number"
              min={1}
              max={12}
              value={batchCount}
              onChange={e => setBatchCount(Math.max(1, Math.min(12, Number(e.target.value) || 1)))}
              className="w-14 px-1.5 py-1 rounded bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono text-[11px]"
            />
          </label>
          <span className="text-[var(--text-muted)] text-[10.5px]">
            {plural(lang, Math.round(durationDays / 7), st.batch.summary, { date: showDate(startDateStr) })}
          </span>
          <button
            type="button"
            disabled={batchBusyCreate}
            onClick={handleCreateBatch}
            className="px-3 py-1 rounded-lg text-xs font-bold bg-[var(--accent-color)] text-white hover:opacity-90 disabled:opacity-40 cursor-pointer"
          >
            {batchBusyCreate ? <Loader2 size={12} className="animate-spin" /> : st.batch.create}
          </button>
          <button
            type="button"
            onClick={() => setIsCreatingBatch(false)}
            className="px-2 py-1 rounded-lg text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            {st.batch.cancel}
          </button>
        </div>
      )}

      {/* Global Batch Action Bar for Selected Tasks */}
      {selectedTaskIds.length > 0 && (
        <div className="bg-[var(--accent-light)]/30 border-b border-[var(--accent-color)]/30 px-4 py-2 flex items-center justify-between gap-3 text-xs animate-in fade-in duration-150 shrink-0 flex-wrap">
          <div className="flex items-center gap-2">
            <span className="font-bold text-[var(--accent-color)] flex items-center gap-1.5">
              <CheckSquare size={14} />
              <span>{plural(lang, selectedTaskIds.length, st.backlog.selectedTasks)}</span>
            </span>
            {selectedSprintTaskIds.length > 0 && (
              <span className="text-[10.5px] text-[var(--text-muted)] font-medium">
                {plural(lang, selectedSprintTaskIds.length, st.backlog.selectedInSprint)}
              </span>
            )}
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {/* Target sprint dropdown */}
            <select
              value={batchTargetSprint}
              disabled={readOnly}
              onChange={e => setBatchTargetSprint(e.target.value)}
              className="text-xs px-2.5 py-1 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer font-medium"
            >
              <option value="">{st.backlog.assignPlaceholder}</option>
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
              {batchBusy ? <Loader2 size={12} className="animate-spin" /> : st.backlog.assign}
            </button>

            {/* Remove from the sprint, back to the backlog */}
            <button
              type="button"
              disabled={batchBusy || readOnly}
              onClick={handleBatchRemoveFromSprint}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold bg-amber-500/15 text-amber-400 hover:bg-amber-500/25 border border-amber-500/30 transition-all cursor-pointer shadow-xs disabled:opacity-40"
              title={st.backlog.removeSelectedTitle}
            >
              {batchBusy ? <Loader2 size={12} className="animate-spin" /> : <X size={13} />}
              <span>
                {st.backlog.removeFromSprint}{selectedSprintTaskIds.length > 0 ? ` (${selectedSprintTaskIds.length})` : ''}
              </span>
            </button>

            {/* Run the batch on autopilot */}
            <button
              type="button"
              onClick={() => startBatchPickup(selectedTaskIds)}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-colors cursor-pointer shadow-xs"
              title={st.backlog.runSelectedTitle}
            >
              <Sparkles size={13} />
              <span>{t.batchLaunch}</span>
            </button>

            <button
              type="button"
              onClick={() => setCheckedTaskIds({})}
              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              title={st.backlog.clearSelection}
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
              const rel = getSprintRelativeInfo(sprint, st.timeline.relative, lang)
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
                    onDrop={isBacklogOpen && !readOnly ? e => handleDropOnSprint(e, sprint.name) : undefined}
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
                            <span>{format(st.dialogs.editHeading, { name: sprint.name })}</span>
                          </span>
                          <div className="flex items-center gap-1.5">
                            <button
                              type="button"
                              onClick={() => handleSaveEditSprint(index)}
                              className="flex items-center gap-1 px-3 py-1 text-xs font-bold text-white bg-emerald-600 hover:bg-emerald-500 rounded-lg transition-all cursor-pointer shadow-xs"
                              title={st.dialogs.saveTitle}
                            >
                              <Check size={13} />
                              <span>{st.dialogs.save}</span>
                            </button>
                            <button
                              type="button"
                              onClick={() => setEditingSprintIndex(null)}
                              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                              title={st.dialogs.cancel}
                            >
                              <X size={14} />
                            </button>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 sm:grid-cols-4 gap-2.5 text-xs">
                          {/* Nom */}
                          <div>
                            <label className="block text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1">
                              {st.dialogs.nameLabel}
                            </label>
                            <input
                              type="text"
                              value={editSprintName}
                              onChange={e => setEditSprintName(e.target.value)}
                              placeholder={st.dialogs.namePlaceholder}
                              className="w-full px-2.5 py-1.5 text-xs font-bold rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                            />
                          </div>

                          {/* Date Début */}
                          <div>
                            <label className="block text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider mb-1">
                              {st.dialogs.startDate}
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
                              {st.dialogs.endDate}
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
                              {st.dialogs.stateLabel}
                            </label>
                            <select
                              value={editSprintState}
                              onChange={e => setEditSprintState(e.target.value as any)}
                              className="w-full px-2.5 py-1.5 text-xs font-semibold rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
                            >
                              <option value="active">{st.dialogs.stateActive}</option>
                              <option value="future">{st.dialogs.stateFuture}</option>
                              <option value="closed">{st.dialogs.stateClosed}</option>
                            </select>
                          </div>
                        </div>

                        {/* Décalage option */}
                        {management === 'local' && <div className="flex items-center gap-2 pt-0.5">
                          <label className="flex items-center gap-1.5 text-[11px] text-[var(--text-secondary)] cursor-pointer select-none">
                            <input
                              type="checkbox"
                              checked={editShiftSubsequent}
                              onChange={e => setEditShiftSubsequent(e.target.checked)}
                              className="rounded text-[var(--accent-color)] w-3.5 h-3.5 cursor-pointer"
                            />
                            <span>{st.dialogs.shiftSubsequent}</span>
                          </label>
                        </div>}
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
                              {st.timeline.badgeClosed}
                            </span>
                          ) : sprint.state === 'active' || rel.type === 'current' ? (
                            <span className="text-[9px] font-bold px-2 py-0.5 rounded-full border bg-emerald-500/15 text-emerald-400 border-emerald-500/30 flex items-center gap-1">
                              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-ping inline-block" />
                              {st.timeline.badgeActive}
                            </span>
                          ) : (
                            <span className="text-[9px] font-bold px-2 py-0.5 rounded-full border bg-cyan-500/15 text-cyan-400 border-cyan-500/30">
                              {st.timeline.badgeFuture}
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
                            disabled={readOnly}
                            onClick={() => handleStartEditSprint(index, sprint)}
                            className="flex items-center gap-1 text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] px-2 py-0.5 rounded-lg border border-transparent hover:border-[var(--border-color)] font-mono text-[10.5px] transition-colors cursor-pointer"
                            title={st.timeline.editDatesTitle}
                          >
                            <CalendarDays size={11} className="text-[var(--accent-color)]" />
                            <span>
                              {showDate(sprint.startDate)} ➔ {showDate(sprint.endDate)}
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
                            {/* Jira never reopens a closed sprint, so neither does a tracker-owned timeline. */}
                            {readOnly || (trackerOwned && sprint.state === 'closed') ? null : sprint.state === 'closed' ? (
                              <button
                                type="button"
                                onClick={() => handleReopenSprint(index)}
                                className="flex items-center gap-1 px-2 py-1 rounded-lg text-[10.5px] font-semibold text-cyan-400 bg-cyan-500/10 hover:bg-cyan-500/20 border border-cyan-500/30 transition-all cursor-pointer shadow-xs"
                                title={st.timeline.reopenTitle}
                              >
                                <RotateCcw size={11} />
                                <span>{st.timeline.reopen}</span>
                              </button>
                            ) : (
                              <button
                                type="button"
                                onClick={() => handleStartCloseSprint(sprint, index)}
                                className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[10.5px] font-bold text-emerald-300 bg-emerald-500/15 hover:bg-emerald-500/25 border border-emerald-500/30 transition-all cursor-pointer shadow-xs active:scale-95"
                                title={st.timeline.closeTitle}
                              >
                                <CheckCircle2 size={12} className="text-emerald-400" />
                                <span>{st.timeline.close}</span>
                              </button>
                            )}

                            {/* Edit Sprint Button */}
                            {!readOnly && <button
                              type="button"
                              onClick={() => handleStartEditSprint(index, sprint)}
                              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                              title={st.timeline.editTitle}
                            >
                              <SlidersHorizontal size={12} />
                            </button>}

                            {/* Delete Sprint Button */}
                            {!readOnly && <button
                              type="button"
                              onClick={() => handleDeleteSprint(index)}
                              className="p-1 rounded-lg text-[var(--text-muted)] hover:text-rose-400 transition-colors cursor-pointer"
                              title={st.timeline.deleteTitle}
                            >
                              <Trash2 size={12} />
                            </button>}

                            {/* Collapse/Expand for Closed Sprints */}
                            {sprint.state === 'closed' && (
                              <button
                                type="button"
                                onClick={() => toggleCollapseSprint(sprint.name)}
                                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                                title={isCollapsed ? st.timeline.expand : st.timeline.collapse}
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
                          {format(st.timeline.collapsedSummary, {
                            tasks: plural(lang, sprintTasks.length, st.timeline.collapsedTasks),
                            done: plural(lang, doneTasks.length, st.timeline.collapsedDone),
                          })}
                        </span>
                        <button
                          type="button"
                          onClick={() => toggleCollapseSprint(sprint.name)}
                          className="text-[11px] text-[var(--accent-color)] hover:underline font-semibold cursor-pointer"
                        >
                          {st.timeline.showTasks}
                        </button>
                      </div>
                    ) : !isBacklogOpen ? (
                      <div className="p-2.5">
                        {sprintTasks.length === 0 ? (
                          <p className="text-[11px] text-[var(--text-muted)] italic px-1">
                            {st.timeline.emptyCompact}
                          </p>
                        ) : displayMode === 'chips' ? (
                          <div className="flex flex-wrap gap-1.5">
                            {sprintTasks.map(task => (
                              <div
                                key={task.id}
                                onClick={() => setSelectedTask(task)}
                                className={`relative inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border text-xs cursor-pointer transition-colors group shadow-2xs ${
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
                                {showsEpicBarOf(task) && <EpicBar parentKey={task.parentKey} />}
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
                                  hidden={readOnly}
                                  className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer ml-0.5"
                                  title={st.timeline.removeTaskTitle}
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
                                className={`relative flex items-center justify-between gap-2 py-1 px-1.5 rounded cursor-pointer transition-colors group ${
                                  checkedTaskIds[task.id]
                                    ? 'bg-[var(--accent-light)]/20 font-medium'
                                    : 'hover:bg-[var(--bg-tertiary)]/60'
                                }`}
                              >
                                {showsEpicBarOf(task) && <EpicBar parentKey={task.parentKey} />}
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
                                      handleRemoveTaskFromSprint(task.id)
                                    }}
                                    hidden={readOnly}
                                    className="text-[var(--text-muted)] hover:text-rose-400 p-0.5 transition-colors opacity-0 group-hover:opacity-100 cursor-pointer"
                                    title={st.timeline.removeTaskTitle}
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
                            <p className="font-medium">{st.timeline.emptyTitle}</p>
                            <p className="text-[10px] opacity-70 mt-0.5">
                              {st.timeline.emptyHint}
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
                                className={`relative inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg border transition-all cursor-grab active:cursor-grabbing group text-xs select-none shadow-2xs ${
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
                                {showsEpicBarOf(task) && <EpicBar parentKey={task.parentKey} />}
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
                                  hidden={readOnly}
                                  className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                                  title={st.backlog.removeFromSprint}
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
                                className={`relative p-2.5 rounded-xl border shadow-xs transition-all cursor-grab active:cursor-grabbing group space-y-1.5 ${
                                  checkedTaskIds[task.id]
                                    ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)]'
                                    : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/50'
                                }`}
                              >
                                {showsEpicBarOf(task) && <EpicBar parentKey={task.parentKey} />}
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
                                      hidden={readOnly}
                                      className="p-1 rounded text-[var(--text-muted)] hover:text-rose-400 hover:bg-rose-500/10 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                                      title={st.timeline.removeTaskTitle}
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
              title={st.backlog.resizeTitle}
            >
              <div className="w-0.5 h-6 bg-[var(--border-color)] group-hover:bg-white rounded-full" />
            </div>

            {/* Backlog Header */}
            <div className="p-3 border-b border-[var(--border-color)] flex items-center justify-between gap-2 shrink-0">
              <div className="flex items-center gap-2">
                <Layers size={14} className="text-cyan-400" />
                <h3 className="text-xs font-bold text-[var(--text-primary)]">
                  {st.backlog.title}
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
                  title={st.backlog.closeTitle}
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
                  placeholder={st.backlog.filterPlaceholder}
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
                  title={showDoneInBacklog ? st.backlog.hideDoneTitle : plural(lang, finishedUnscheduledCount, st.backlog.showDoneTitle)}
                >
                  <CheckCircle2 size={11} />
                  <span>{showDoneInBacklog ? st.backlog.done : format(st.backlog.doneCount, { count: finishedUnscheduledCount })}</span>
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
                    <span>{st.backlog.deselectAll}</span>
                  </>
                ) : (
                  <>
                    <Square size={13} />
                    <span>{st.backlog.selectAll}</span>
                  </>
                )}
              </button>

              {selectedBacklogList.length > 0 && (
                <span className="text-[10px] font-bold text-[var(--accent-color)] bg-[var(--accent-light)] px-1.5 py-0.5 rounded border border-[var(--accent-color)]/30">
                  {plural(lang, selectedBacklogList.length, st.backlog.selectedCount)}
                </span>
              )}
            </div>

            {/* Batch Action Bar (if multiple items selected) */}
            {selectedBacklogList.length > 0 && (
              <div className="p-2 border-b border-[var(--border-color)] bg-[var(--accent-light)]/25 flex items-center gap-1.5 shrink-0">
                <select
                  value={batchTargetSprint}
                  disabled={readOnly}
                  onChange={e => setBatchTargetSprint(e.target.value)}
                  className="flex-1 text-xs px-2 py-1 rounded bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer"
                >
                  <option value="">{st.backlog.movePlaceholder}</option>
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
                  {batchBusy ? '…' : st.backlog.move}
                </button>
                <button
                  type="button"
                  onClick={() => startBatchPickup(selectedBacklogList.map(t => t.id))}
                  className="flex items-center gap-1 px-2.5 py-1 rounded text-xs font-semibold bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-colors cursor-pointer shadow-xs shrink-0"
                  title={st.backlog.runBacklogTitle}
                >
                  <Sparkles size={12} />
                  <span>{t.batchLaunch}</span>
                </button>
              </div>
            )}

            {/* Backlog List */}
            <div className="flex-1 overflow-y-auto p-2.5 space-y-1.5">
              {unscheduledTasks.length === 0 ? (
                <div className="py-12 text-center text-xs text-[var(--text-muted)]">
                  {backlogSearch ? st.backlog.noResults : st.backlog.allPlanned}
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
                      className={`relative inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg border transition-all cursor-grab active:cursor-grabbing group text-xs select-none shadow-2xs max-w-full ${
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
                      {showsEpicBarOf(task) && <EpicBar parentKey={task.parentKey} />}
                      <span className="font-mono font-bold text-[10px] text-[var(--accent-color)] shrink-0">
                        {task.key}
                      </span>
                      <span className="truncate max-w-[160px] text-[11px] font-medium">
                        {task.title}
                      </span>
                      <select
                        value=""
                        disabled={readOnly}
                        onChange={e => {
                          if (e.target.value) {
                            const target = moveTarget(e.target.value)
                            setTaskSprint(task.id, target.id, target.name)
                          }
                        }}
                        onClick={e => e.stopPropagation()}
                        className="text-[9px] font-semibold px-1 py-0.2 rounded bg-[var(--bg-tertiary)] text-[var(--accent-color)] border border-[var(--border-color)] focus:outline-none cursor-pointer ml-auto"
                      >
                        <option value="">{st.timeline.addSprint}</option>
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
                    className={`relative p-2.5 rounded-xl border shadow-xs transition-all cursor-grab active:cursor-grabbing space-y-1.5 group ${
                      checkedTaskIds[task.id]
                        ? 'bg-[var(--accent-light)]/20 border-[var(--accent-color)]'
                        : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/60'
                    }`}
                  >
                    {showsEpicBarOf(task) && <EpicBar parentKey={task.parentKey} />}
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
                        <GripVertical size={11} className="text-[var(--text-muted)]" /> {st.backlog.drag}
                      </span>

                      {/* Quick Assign Dropdown */}
                      <select
                        value=""
                        disabled={readOnly}
                        onChange={e => {
                          if (e.target.value) {
                            const target = moveTarget(e.target.value)
                            setTaskSprint(task.id, target.id, target.name)
                          }
                        }}
                        onClick={e => e.stopPropagation()}
                        className="text-[9.5px] font-semibold px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--accent-color)] border border-[var(--border-color)] focus:outline-none cursor-pointer"
                      >
                        <option value="">{st.timeline.addSprint}</option>
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
        <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150" {...closeSprintBackdrop}>
          <div className="relative w-full max-w-lg rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-150">
            {/* Modal Header */}
            <div className="flex items-center justify-between px-5 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
              <div className="flex items-center gap-2.5">
                <div className="p-2 rounded-xl bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                  <CheckCircle2 size={18} />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-[var(--text-primary)]">
                    {format(st.close.heading, { name: closingSprint.sprint.name })}
                  </h3>
                  <span className="text-[10.5px] text-[var(--text-muted)]">
                    {st.close.subtitle}
                  </span>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setClosingSprint(null)}
                aria-label={st.close.dismiss}
                title={st.close.dismiss}
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
                          {st.close.doneHeading}
                        </span>
                        <span className="text-2xl font-extrabold text-emerald-300 mt-1">
                          {doneTasks.length}
                        </span>
                        <span className="text-[10px] text-[var(--text-muted)] mt-1">
                          {st.close.doneHint}
                        </span>
                      </div>

                      <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 flex flex-col">
                        <span className="text-[10px] uppercase tracking-wider font-bold text-amber-400">
                          {st.close.unfinishedHeading}
                        </span>
                        <span className="text-2xl font-extrabold text-amber-300 mt-1">
                          {unfinishedTasks.length}
                        </span>
                        <span className="text-[10px] text-[var(--text-muted)] mt-1">
                          {unfinishedTasks.length === 0 ? st.close.nothingLeft : st.close.toMove}
                        </span>
                      </div>
                    </div>

                    {unfinishedTasks.length > 0 ? (
                      <div className="space-y-2 pt-1">
                        <label className="block text-[11.5px] font-bold text-[var(--text-primary)]">
                          {plural(lang, unfinishedTasks.length, st.close.question)}
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
                                  {format(st.close.toNext, { name: nextSprint.name })}
                                </span>
                                <span className="text-[10.5px] text-[var(--text-muted)] block leading-snug">
                                  {st.close.toNextHint}
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
                                {st.close.toBacklog}
                              </span>
                              <span className="text-[10.5px] text-[var(--text-muted)] block leading-snug">
                                {st.close.toBacklogHint}
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
                                {st.close.keep}
                              </span>
                              <span className="text-[10.5px] text-[var(--text-muted)] block leading-snug">
                                {format(st.close.keepHint, { name: closingSprint.sprint.name })}
                              </span>
                            </div>
                          </label>
                        </div>
                      </div>
                    ) : (
                      <div className="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-center space-y-1">
                        <p className="font-bold text-emerald-300">{st.close.congrats}</p>
                        <p className="text-[11px] text-[var(--text-muted)]">
                          {st.close.allDoneHint}
                        </p>
                      </div>
                    )}

                    {nextSprint && nextSprint.state === 'future' && (
                      <div className="p-3 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] flex items-center justify-between">
                        <div>
                          <span className="font-semibold text-[var(--text-primary)] block text-xs">
                            {format(st.close.activateNext, { name: nextSprint.name })}
                          </span>
                          <span className="text-[10px] text-[var(--text-muted)] block">
                            {st.close.activateNextHint}
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
                {st.close.cancel}
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
                <span>{st.close.confirm}</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
