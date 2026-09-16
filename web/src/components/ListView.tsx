import { RemoteRunBadge } from './RemoteRunBadge'
import React, { useState, useMemo, useRef, useEffect } from "react"
import {
  Flame,
  Clock,
  CheckCircle2,
  Calendar,
  Trash2,
  ArrowUpDown,
  Sparkles,
  Loader2,
  GitBranch,
  GitPullRequest,
  ExternalLink,
  FolderGit2,
  Eye,
  EyeOff,
  Layers,
  Pin,
  HelpCircle,
  FileCode,
  ShieldCheck,
  Kanban,
  Tag,
  Plus,
  CopyPlus,
  X,
} from "lucide-react"
import { useApp } from "../context/AppContext"
import { TaskFilters } from "./TaskFilters"
import { issueTypeStyle } from "../lib/issueTypes"
import { Avatar } from "./Avatar"
import { shortElapsed, isElapsedStale } from "../lib/elapsed"
import { resolveTaskStage } from "../lib/workflow"
import type { Task, Status, Priority, WorkflowStage } from "../types"

export const ListView: React.FC = () => {
  const {
    tasks,
    isPinned,
    togglePin,
    setSelectedTask,
    updateTask,
    deleteTask,
    openCloneModal,
    activities,
    hideDone,
    toggleHideDone,
    boardGrouping,
    setBoardGrouping,
    moveTaskWorkflowStage,
    moveTaskToTrackerStatus,
    moveTask,
    currentProject,
    addToast,
    t,
  } = useApp()


  // Par défaut, le plus urgent en premier.
  const [sortField, setSortField] = useState<"key" | "title" | "status" | "priority" | "dueDate" | "createdAt">("priority")
  const [sortAsc, setSortAsc] = useState(false)
  const [groupByStatus, setGroupByStatus] = useState(true)

  // -------------------------------------------------------------
  // Bulk Multi-Selection State
  // -------------------------------------------------------------
  const [selectedTaskIds, setSelectedTaskIds] = useState<Set<string>>(new Set())
  const [isBulkProcessing, setIsBulkProcessing] = useState(false)
  const [activeBulkDropdown, setActiveBulkDropdown] = useState<"status" | "workflow" | "priority" | "labels" | null>(null)
  const [bulkLabelInput, setBulkLabelInput] = useState("")
  const bulkDropdownRef = useRef<HTMLDivElement>(null)

  // Close bulk popover when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (bulkDropdownRef.current && !bulkDropdownRef.current.contains(e.target as Node)) {
        setActiveBulkDropdown(null)
      }
    }
    if (activeBulkDropdown) {
      document.addEventListener("mousedown", handleClickOutside)
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside)
    }
  }, [activeBulkDropdown])

  const handleSort = (field: typeof sortField) => {
    if (sortField === field) {
      setSortAsc(prev => !prev)
    } else {
      setSortField(field)
      setSortAsc(field !== "priority")
    }
  }

  const sortedTasks = useMemo(() => {
    return [...tasks].sort((a, b) => {
      let result = 0
      if (sortField === "key") {
        result = a.key.localeCompare(b.key, undefined, { numeric: true })
      } else if (sortField === "title") {
        result = a.title.localeCompare(b.title)
      } else if (sortField === "status") {
        result = a.status.localeCompare(b.status)
      } else if (sortField === "priority") {
        const priorityOrder: Record<Priority, number> = { urgent: 4, high: 3, medium: 2, low: 1 }
        result = (priorityOrder[a.priority] || 0) - (priorityOrder[b.priority] || 0)
      } else if (sortField === "dueDate") {
        result = (a.dueDate || "").localeCompare(b.dueDate || "")
      } else {
        result = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()
      }
      return sortAsc ? result : -result
    })
  }, [tasks, sortField, sortAsc])

  const doneTasksCount = tasks.filter(t => t.status === "finished" || t.status === "done").length

  const visibleTasks = useMemo(() => {
    return hideDone
      ? sortedTasks.filter(t => t.status !== "finished" && t.status !== "done")
      : sortedTasks
  }, [sortedTasks, hideDone])

  // -------------------------------------------------------------
  // Workflow Stages & Statuses
  // -------------------------------------------------------------
  const WORKFLOW_STAGES: { id: WorkflowStage; label: string; stageLabel: string; stageColor: string; icon: React.ReactNode; color: string }[] = [
    { id: "new", label: "New / À cadrer", stageLabel: "#new", stageColor: "bg-cyan-500/15 text-cyan-400 border-cyan-500/30", icon: <Sparkles size={14} />, color: "text-cyan-400" },
    { id: "clarified", label: "Clarified / À spécifier", stageLabel: "#clarified", stageColor: "bg-amber-500/15 text-amber-400 border-amber-500/30", icon: <HelpCircle size={14} />, color: "text-amber-400" },
    { id: "specified", label: "Specified / À développer", stageLabel: "#specified", stageColor: "bg-blue-500/15 text-blue-400 border-blue-500/30", icon: <FileCode size={14} />, color: "text-blue-400" },
    { id: "implemented", label: "Implemented / À tester", stageLabel: "#implemented", stageColor: "bg-indigo-500/15 text-indigo-400 border-indigo-500/30", icon: <Flame size={14} />, color: "text-indigo-400" },
    { id: "reviewed", label: "Reviewed / À clôturer", stageLabel: "#reviewed", stageColor: "bg-purple-500/15 text-purple-400 border-purple-500/30", icon: <ShieldCheck size={14} />, color: "text-purple-400" },
    { id: "finished", label: "Finished / Terminé", stageLabel: "#finished", stageColor: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30", icon: <CheckCircle2 size={14} />, color: "text-emerald-400" },
  ]

  const trackerStatusOptions = useMemo(() => {
    const cols = currentProject?.trackerColumns || []
    if (cols.length > 0) {
      const list: string[] = []
      cols.forEach(c => {
        if (c.statuses && c.statuses.length > 0) {
          c.statuses.forEach(s => {
            if (!list.includes(s)) list.push(s)
          })
        } else if (!list.includes(c.name)) {
          list.push(c.name)
        }
      })
      return list
    }
    return ["To Do", "In Progress", "In Review", "Done"]
  }, [currentProject?.trackerColumns])

  const statusList: { id: Status; label: string; stageLabel: string; stageColor: string; icon: React.ReactNode; color: string }[] = [
    { id: "to_clarify", label: t.status.to_clarify, stageLabel: "Backlog", stageColor: "bg-cyan-500/15 text-cyan-400 border-cyan-500/30", icon: <Sparkles size={14} />, color: "text-cyan-400" },
    { id: "clarified", label: t.status.clarified, stageLabel: "Spécification", stageColor: "bg-amber-500/15 text-amber-400 border-amber-500/30", icon: <HelpCircle size={14} />, color: "text-amber-400" },
    { id: "to_implement", label: t.status.to_implement, stageLabel: "En cours", stageColor: "bg-blue-500/15 text-blue-400 border-blue-500/30", icon: <FileCode size={14} />, color: "text-blue-400" },
    { id: "to_test", label: t.status.to_test, stageLabel: "Tests", stageColor: "bg-indigo-500/15 text-indigo-400 border-indigo-500/30", icon: <Flame size={14} />, color: "text-indigo-400" },
    { id: "to_close", label: t.status.to_close, stageLabel: "Revue", stageColor: "bg-purple-500/15 text-purple-400 border-purple-500/30", icon: <ShieldCheck size={14} />, color: "text-purple-400" },
    { id: "finished", label: t.status.finished, stageLabel: "Terminé", stageColor: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30", icon: <CheckCircle2 size={14} />, color: "text-emerald-400" },
  ]

  // Pastilles flat (sans emoji ni bordure 3D)
  const PRIORITY_OPTIONS: { id: Priority; label: string; color: string }[] = [
    { id: "urgent", label: t.priority.urgent, color: "var(--status-danger)" },
    { id: "high", label: t.priority.high, color: "var(--status-warn)" },
    { id: "medium", label: t.priority.medium, color: "var(--status-info)" },
    { id: "low", label: t.priority.low, color: "var(--text-muted)" },
  ]

  // -------------------------------------------------------------
  // Bulk Actions Selection Utilities
  // -------------------------------------------------------------
  const selectedTasks = useMemo(() => {
    return tasks.filter(t => selectedTaskIds.has(t.id))
  }, [tasks, selectedTaskIds])

  const isAllVisibleSelected = visibleTasks.length > 0 && visibleTasks.every(t => selectedTaskIds.has(t.id))
  const isSomeVisibleSelected = visibleTasks.some(t => selectedTaskIds.has(t.id)) && !isAllVisibleSelected

  const toggleSelectTask = (id: string) => {
    setSelectedTaskIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAllVisible = () => {
    if (isAllVisibleSelected) {
      setSelectedTaskIds(new Set())
    } else {
      setSelectedTaskIds(new Set(visibleTasks.map(t => t.id)))
    }
  }

  const toggleSelectGroup = (groupTasks: Task[]) => {
    const allGroupSelected = groupTasks.length > 0 && groupTasks.every(t => selectedTaskIds.has(t.id))
    setSelectedTaskIds(prev => {
      const next = new Set(prev)
      if (allGroupSelected) {
        groupTasks.forEach(t => next.delete(t.id))
      } else {
        groupTasks.forEach(t => next.add(t.id))
      }
      return next
    })
  }

  const clearSelection = () => {
    setSelectedTaskIds(new Set())
    setActiveBulkDropdown(null)
  }

  // All non-workflow project labels for quick-add suggestions
  const allProjectLabels = useMemo(() => {
    const set = new Set<string>()
    tasks.forEach(t => {
      ;(t.labels || []).forEach(l => {
        const clean = l.replace(/^#+/, "").trim()
        if (clean && !["new", "clarified", "specified", "implemented", "reviewed", "finished", "untouched", "closed"].includes(clean.toLowerCase())) {
          set.add(clean)
        }
      })
    })
    return Array.from(set).sort()
  }, [tasks])

  // Labels present across the selected tasks
  const selectedTasksLabels = useMemo(() => {
    const map = new Map<string, number>()
    selectedTasks.forEach(t => {
      ;(t.labels || []).forEach(l => {
        const clean = l.replace(/^#+/, "").trim()
        if (clean) {
          map.set(clean, (map.get(clean) || 0) + 1)
        }
      })
    })
    return Array.from(map.entries()).map(([label, count]) => ({ label, count }))
  }, [selectedTasks])

  // -------------------------------------------------------------
  // Bulk Execution Handlers
  // -------------------------------------------------------------
  const handleBulkPriority = async (priority: Priority) => {
    if (selectedTasks.length === 0) return
    setIsBulkProcessing(true)
    try {
      for (const task of selectedTasks) {
        await updateTask(task.id, { priority })
      }
      addToast({
        type: "success",
        title: "Priorités mises à jour",
        description: `${selectedTasks.length} tâche(s) passée(s) en priorité ${priority}`,
      })
      setActiveBulkDropdown(null)
    } catch (err: any) {
      addToast({ type: "error", title: "Erreur", description: err.message })
    } finally {
      setIsBulkProcessing(false)
    }
  }

  const handleBulkWorkflowStage = async (stage: WorkflowStage) => {
    if (selectedTasks.length === 0) return
    setIsBulkProcessing(true)
    try {
      for (const task of selectedTasks) {
        await moveTaskWorkflowStage(task.id, stage)
      }
      addToast({
        type: "success",
        title: "Étape workflow mise à jour",
        description: `${selectedTasks.length} tâche(s) passée(s) à #${stage}`,
      })
      setActiveBulkDropdown(null)
    } catch (err: any) {
      addToast({ type: "error", title: "Erreur", description: err.message })
    } finally {
      setIsBulkProcessing(false)
    }
  }

  const handleBulkTrackerStatus = async (status: string) => {
    if (selectedTasks.length === 0) return
    setIsBulkProcessing(true)
    try {
      for (const task of selectedTasks) {
        if (currentProject?.trackerColumns && currentProject.trackerColumns.length > 0) {
          await moveTaskToTrackerStatus(task.id, status)
        } else {
          await moveTask(task.id, status as Status, 0)
        }
      }
      addToast({
        type: "success",
        title: "Statut mis à jour",
        description: `${selectedTasks.length} tâche(s) passée(s) à « ${status} »`,
      })
      setActiveBulkDropdown(null)
    } catch (err: any) {
      addToast({ type: "error", title: "Erreur", description: err.message })
    } finally {
      setIsBulkProcessing(false)
    }
  }

  const handleBulkAddLabel = async (labelName: string) => {
    const clean = labelName.replace(/^#+/, "").trim()
    if (!clean || selectedTasks.length === 0) return
    setIsBulkProcessing(true)
    try {
      for (const task of selectedTasks) {
        const current = task.labels || []
        if (!current.some(l => l.toLowerCase().replace(/^#+/, "") === clean.toLowerCase())) {
          const next = [...current, clean]
          await updateTask(task.id, { labels: next })
        }
      }
      addToast({
        type: "success",
        title: "Label ajouté",
        description: `Label #${clean} ajouté à ${selectedTasks.length} tâche(s)`,
      })
      setBulkLabelInput("")
    } catch (err: any) {
      addToast({ type: "error", title: "Erreur", description: err.message })
    } finally {
      setIsBulkProcessing(false)
    }
  }

  const handleBulkRemoveLabel = async (labelName: string) => {
    const clean = labelName.replace(/^#+/, "").trim().toLowerCase()
    if (selectedTasks.length === 0) return
    setIsBulkProcessing(true)
    try {
      for (const task of selectedTasks) {
        const current = task.labels || []
        const next = current.filter(l => l.toLowerCase().replace(/^#+/, "") !== clean)
        if (next.length !== current.length) {
          await updateTask(task.id, { labels: next })
        }
      }
      addToast({
        type: "success",
        title: "Label retiré",
        description: `Label #${labelName} retiré des tâches sélectionnées`,
      })
    } catch (err: any) {
      addToast({ type: "error", title: "Erreur", description: err.message })
    } finally {
      setIsBulkProcessing(false)
    }
  }

  const handleBulkDelete = async () => {
    if (selectedTasks.length === 0) return
    if (!window.confirm(`Supprimer définitivement ${selectedTasks.length} tâche(s) sélectionnée(s) ?`)) return
    setIsBulkProcessing(true)
    try {
      for (const task of selectedTasks) {
        await deleteTask(task.id)
      }
      addToast({
        type: "success",
        title: "Tâches supprimées",
        description: `${selectedTasks.length} tâche(s) supprimée(s)`,
      })
      clearSelection()
    } catch (err: any) {
      addToast({ type: "error", title: "Erreur", description: err.message })
    } finally {
      setIsBulkProcessing(false)
    }
  }

  const renderTaskRow = (task: Task) => {
    const taskActs = activities?.filter(a => a.taskId === task.id) || []
    const latestActivity = taskActs.find(a => a.status === "running" || a.status === "queued" || a.status === "pending") || taskActs[0]

    const priorityOpt = PRIORITY_OPTIONS.find(p => p.id === task.priority) || PRIORITY_OPTIONS[2]
    const taskStage = resolveTaskStage(task, currentProject)
    const isSelected = selectedTaskIds.has(task.id)

    return (
      <tr
        key={task.id}
        onClick={() => setSelectedTask(task)}
        className={`border-b border-[var(--border-color)]/60 hover:bg-[var(--bg-tertiary)]/50 transition-colors cursor-pointer group ${
          isSelected ? "bg-[var(--accent-light)]/20" : ""
        }`}
      >
        {/* Selection Checkbox */}
        <td className="py-2.5 px-3 w-10 text-center whitespace-nowrap" onClick={e => e.stopPropagation()}>
          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => toggleSelectTask(task.id)}
            className="rounded text-[var(--accent-color)] focus:ring-0 cursor-pointer w-4 h-4"
          />
        </td>

        {/* Source & Key */}
        <td className="py-2.5 px-3 whitespace-nowrap" onClick={e => e.stopPropagation()}>
          <div className="flex items-center gap-1.5 font-mono text-xs font-bold">
            {task.externalUrl ? (
              <a
                href={task.externalUrl}
                target="_blank"
                rel="noreferrer"
                className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[11px] font-mono transition-all shadow-2xs hover:scale-105 ${
                  task.source === "github"
                    ? "bg-purple-500/15 text-purple-400 hover:bg-purple-500/25 border border-purple-500/30"
                    : task.source === "jira"
                    ? "bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 border border-blue-500/30"
                    : "bg-[var(--accent-light)] text-[var(--accent-color)] hover:opacity-80"
                }`}
                title={
                  task.source === "github"
                    ? `Ouvrir ${task.key} sur GitHub`
                    : task.source === "jira"
                    ? `Ouvrir ${task.key} sur Jira`
                    : `Ouvrir ${task.key}`
                }
              >
                {task.source === "github" && <FolderGit2 size={11} className="text-purple-400" />}
                {task.source === "jira" && <span className="text-blue-400 font-sans font-black text-[9px]">J</span>}
                <span>{task.key}</span>
                <ExternalLink size={9} />
              </a>
            ) : (
              <span className="text-[var(--accent-color)] font-mono">
                {task.key}
              </span>
            )}

            {task.branchName && (
              <span title={`Branche: ${task.branchName}`} className="text-indigo-400">
                <GitBranch size={11} />
              </span>
            )}
          </div>
        </td>

        {/* Title & Activity */}
        <td className="py-2.5 px-3 max-w-[560px]">
          <div className="flex items-center gap-1.5">
            {task.issueType && (
              <span
                className="shrink-0 px-1.5 py-0.5 rounded text-[9px] font-bold uppercase tracking-wide"
                style={{
                  color: issueTypeStyle(task.issueType).color,
                  background: issueTypeStyle(task.issueType).background,
                  border: `1px solid ${issueTypeStyle(task.issueType).border}`,
                }}
                title={`Type de ticket : ${task.issueType}`}
              >
                {issueTypeStyle(task.issueType).short}
              </span>
            )}
            <span className="text-xs font-semibold text-[var(--text-primary)] truncate group-hover:text-[var(--accent-color)] transition-colors">
              {task.title}
            </span>
          </div>

          {/* Parent (Macro ou Story) */}
          {task.parentKey && (
            <div
              className="inline-flex items-center gap-1 px-1.5 py-0.2 rounded text-[10px] mt-1 mr-1 text-violet-300 bg-violet-500/10 border border-violet-500/25 max-w-[220px]"
              title={`${task.parentType || "Parent"} ${task.parentKey}${task.parentTitle ? ` — ${task.parentTitle}` : ""}`}
            >
              <Layers size={9} className="shrink-0 opacity-80" />
              <span className="font-mono font-bold shrink-0">{task.parentKey}</span>
              {task.parentTitle && <span className="truncate opacity-80">{task.parentTitle}</span>}
            </div>
          )}

          <RemoteRunBadge taskId={task.id} />
          {/* Activity badge if exists */}


          {task.description && !latestActivity && (
            <div className="text-[11px] text-[var(--text-muted)] line-clamp-1 mt-0.5">
              {task.description}
            </div>
          )}
        </td>

        {/* Dynamic Column: Status vs Agentic Workflow Stage */}
        <td className="py-2.5 px-3 whitespace-nowrap" onClick={e => e.stopPropagation()}>
          {boardGrouping === "workflow" ? (
            /* Agentic Workflow Stage Select */
            <select
              value={taskStage}
              onChange={e => moveTaskWorkflowStage(task.id, e.target.value as WorkflowStage)}
              className="text-[11px] font-bold bg-[var(--bg-secondary)] text-[var(--text-primary)] border border-[var(--border-color)] rounded-md px-2 py-1 focus:outline-none focus:border-[var(--accent-color)] cursor-pointer shadow-2xs"
              title="Modifier l'étape du workflow agentique (met à jour le statut selon le mapping du projet)"
            >
              {WORKFLOW_STAGES.map(s => (
                <option key={s.id} value={s.id}>
                  {s.stageLabel} — {s.label}
                </option>
              ))}
            </select>
          ) : (
            /* Tracker / Native Status Select */
            <select
              value={task.trackerStatus || task.status}
              onChange={e => {
                if (currentProject?.trackerColumns && currentProject.trackerColumns.length > 0) {
                  moveTaskToTrackerStatus(task.id, e.target.value)
                } else {
                  moveTask(task.id, e.target.value as Status, 0)
                }
              }}
              className="text-[11px] font-medium bg-[var(--bg-secondary)] text-[var(--text-secondary)] border border-[var(--border-color)] rounded-md px-2 py-1 focus:outline-none focus:border-[var(--accent-color)] cursor-pointer shadow-2xs"
              title="Modifier le statut (met à jour l'étape agentique selon le mapping du projet)"
            >
              {currentProject?.trackerColumns && currentProject.trackerColumns.length > 0 ? (
                trackerStatusOptions.map(st => (
                  <option key={st} value={st}>
                    {st}
                  </option>
                ))
              ) : (
                statusList.map(s => (
                  <option key={s.id} value={s.id}>
                    {s.label}
                  </option>
                ))
              )}
            </select>
          )}

          {task.statusChangedAt && shortElapsed(task.statusChangedAt) && (
            <div
              className="flex items-center gap-0.5 mt-1 text-[9.5px] font-medium"
              style={{ color: isElapsedStale(task.statusChangedAt) ? "var(--status-warn)" : "var(--text-muted)" }}
              title={`Dans cet état depuis le ${new Date(task.statusChangedAt).toLocaleDateString()}`}
            >
              <Clock size={9} />
              <span>{shortElapsed(task.statusChangedAt)}</span>
            </div>
          )}
        </td>

        {/* Priority (Flat colored dot, sans contour) */}
        <td className="py-2.5 px-3 w-8 text-center whitespace-nowrap" onClick={e => e.stopPropagation()}>
          <div className="relative inline-flex items-center justify-center group/prio">
            <select
              value={task.priority}
              onChange={e => updateTask(task.id, { priority: e.target.value as Priority })}
              className="opacity-0 absolute inset-0 w-full h-full cursor-pointer z-10"
              title={`${t.taskModal.priority} : ${priorityOpt.label} (Cliquer pour changer)`}
            >
              {PRIORITY_OPTIONS.map(p => (
                <option key={p.id} value={p.id}>
                  {p.label}
                </option>
              ))}
            </select>
            <span
              className="w-2.5 h-2.5 rounded-full inline-block transition-transform group-hover/prio:scale-125 cursor-pointer"
              style={{ backgroundColor: priorityOpt.color }}
              title={`${t.taskModal.priority} : ${priorityOpt.label}`}
            />
          </div>
        </td>

        {/* Labels */}
        <td className="py-2.5 px-3">
          <div className="flex flex-wrap gap-1 max-w-[180px]">
            {task.labels && task.labels.map(lbl => {
              const clean = lbl.replace(/^#+/, "")
              const lower = clean.toLowerCase()
              let badgeStyle = "bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)]"
              if (lower === "new" || lower === "untouched") badgeStyle = "bg-cyan-500/15 text-cyan-400 border-cyan-500/30 font-semibold"
              else if (lower === "clarified") badgeStyle = "bg-amber-500/15 text-amber-400 border-amber-500/30 font-semibold"
              else if (lower === "specified") badgeStyle = "bg-blue-500/15 text-blue-400 border-blue-500/30 font-semibold"
              else if (lower === "implemented") badgeStyle = "bg-indigo-500/15 text-indigo-400 border-indigo-500/30 font-semibold"
              else if (lower === "reviewed") badgeStyle = "bg-purple-500/15 text-purple-400 border-purple-500/30 font-semibold"
              else if (lower === "finished" || lower === "closed") badgeStyle = "bg-emerald-500/15 text-emerald-400 border-emerald-500/30 font-semibold"

              return (
                <span
                  key={lbl}
                  className={`text-[10px] px-1.5 py-0.2 rounded border whitespace-nowrap ${badgeStyle}`}
                >
                  #{clean}
                </span>
              )
            })}
          </div>
        </td>

        {/* Assignee */}
        <td className="py-2.5 px-3 whitespace-nowrap text-xs text-[var(--text-secondary)]">
          {task.assignee ? (
            <div className="flex items-center gap-1.5">
              <Avatar name={task.assignee} url={task.assigneeAvatar} size={20} />
              <span className="truncate max-w-[90px]">{task.assignee}</span>
            </div>
          ) : (
            <span className="text-[var(--text-muted)] text-[11px]">-</span>
          )}
        </td>

        {/* Due Date & Git / PR / MR */}
        <td className="py-2.5 px-3 whitespace-nowrap text-xs text-[var(--text-muted)]">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              {task.dueDate ? (
                <span className="flex items-center gap-1 text-[11px] text-amber-400 font-medium">
                  <Calendar size={12} />
                  {task.dueDate}
                </span>
              ) : (
                <span className="text-[10px] opacity-60 font-mono">
                  {new Date(task.createdAt).toLocaleDateString()}
                </span>
              )}


            </div>

            {task.prUrl && (
              <a
                href={task.prUrl}
                target="_blank"
                rel="noreferrer"
                onClick={e => e.stopPropagation()}
                className={`inline-flex items-center gap-1 px-1.5 py-0.2 rounded text-[10px] font-mono font-bold transition-all w-fit ${
                  task.prUrl.includes("gitlab")
                    ? "bg-orange-500/15 text-orange-400 hover:bg-orange-500/25 border border-orange-500/30"
                    : "bg-purple-500/15 text-purple-400 hover:bg-purple-500/25 border border-purple-500/30"
                }`}
                title={task.prUrl.includes("gitlab") ? `Voir MR GitLab : ${task.prUrl}` : `Voir PR GitHub : ${task.prUrl}`}
              >
                <GitPullRequest size={10} className={task.prUrl.includes("gitlab") ? "text-orange-400" : "text-purple-400"} />
                <span>{task.prUrl.includes("gitlab") ? "GitLab MR" : "GitHub PR"}</span>
                <ExternalLink size={8} />
              </a>
            )}
          </div>
        </td>

        {/* Actions */}
        <td className="py-2.5 px-3 text-right whitespace-nowrap" onClick={e => e.stopPropagation()}>
          <div className="flex items-center justify-end gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
            <button
              onClick={() => togglePin(task.id)}
              className={`inline-flex items-center justify-center p-1 rounded-md border transition-colors cursor-pointer ${
                isPinned(task.id)
                  ? "accent-text bg-[var(--accent-light)] border-[var(--accent-color)]/40"
                  : "text-[var(--text-muted)] hover:text-[var(--accent-color)] border-transparent hover:border-[var(--accent-color)]/30"
              }`}
              title={isPinned(task.id) ? "Retirer des épinglés" : "Épingler"}
            >
              <Pin size={13} />
            </button>


            <button
              type="button"
              onClick={() => openCloneModal(task)}
              className="p-1 rounded text-[var(--text-muted)] hover:text-cyan-400 hover:bg-cyan-500/10 transition-colors cursor-pointer"
              title="Cloner cette story"
            >
              <CopyPlus size={13} />
            </button>

            <button
              type="button"
              onClick={() => {
                if (window.confirm(t.taskModal.deleteConfirm)) {
                  deleteTask(task.id)
                }
              }}
              className="p-1 rounded text-[var(--text-muted)] hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
              title={t.taskModal.delete}
            >
              <Trash2 size={13} />
            </button>
          </div>
        </td>
      </tr>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto p-4 bg-[var(--bg-primary)] pb-24">
      <div className="space-y-4">
        {/* Backlog Toolbar */}
        <div className="flex items-center justify-between gap-4 pb-2 border-b border-[var(--border-color)] flex-wrap">
          <div className="flex items-center gap-4 flex-wrap">
            <span className="text-xs font-semibold text-[var(--text-secondary)]">
              {visibleTasks.length} {visibleTasks.length > 1 ? "tâches dans le backlog" : "tâche dans le backlog"}
            </span>

            {/* Toggle Status vs Workflow Mode */}
            <div className="flex items-center gap-0.5 p-0.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-xs">
              <button
                type="button"
                onClick={() => setBoardGrouping("status")}
                className={`flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-semibold transition-all cursor-pointer ${
                  boardGrouping === "status"
                    ? "bg-[var(--accent-color)] text-white shadow-xs font-bold"
                    : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                }`}
                title="Vue groupée et colonne orientée Statuts"
              >
                <Kanban size={12} />
                <span>Statuts</span>
              </button>
              <button
                type="button"
                onClick={() => setBoardGrouping("workflow")}
                className={`flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-semibold transition-all cursor-pointer ${
                  boardGrouping === "workflow"
                    ? "bg-[var(--accent-color)] text-white shadow-xs font-bold"
                    : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                }`}
                title="Vue groupée et colonne orientée Workflow Agentique"
              >
                <Sparkles size={12} />
                <span>Workflow Agentique</span>
              </button>
            </div>

            <label className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] cursor-pointer hover:text-[var(--text-primary)] transition-colors">
              <input
                type="checkbox"
                checked={groupByStatus}
                onChange={e => setGroupByStatus(e.target.checked)}
                className="rounded text-[var(--accent-color)] focus:ring-0 cursor-pointer"
              />
              {boardGrouping === "workflow" ? "Grouper par étape workflow" : "Grouper par statut"}
            </label>
            <label className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] cursor-pointer hover:text-[var(--text-primary)] transition-colors">
              <input
                type="checkbox"
                checked={hideDone}
                onChange={() => toggleHideDone()}
                className="rounded text-[var(--accent-color)] focus:ring-0 cursor-pointer"
              />
              Masquer terminées ({doneTasksCount})
            </label>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <TaskFilters />
            <button
              type="button"
              onClick={() => handleSort("priority")}
              title="Trier par priorité"
              className={`flex items-center gap-1 px-2 py-1 rounded-md text-xs font-semibold border transition-colors cursor-pointer ${
                sortField === "priority"
                  ? "bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/40"
                  : "bg-[var(--bg-secondary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]"
              }`}
            >
              <ArrowUpDown size={11} />
              <span>Priorité</span>
              {sortField === "priority" && <span className="font-mono">{sortAsc ? "↑" : "↓"}</span>}
            </button>
          </div>
        </div>

        {visibleTasks.length === 0 ? (
          <div className="py-16 text-center text-[var(--text-muted)] space-y-3">
            <Clock size={32} className="mx-auto opacity-40" />
            <p className="text-sm font-medium">{t.list.empty}</p>
            {hideDone && doneTasksCount > 0 && (
              <button
                onClick={toggleHideDone}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/30 transition-colors cursor-pointer"
              >
                <Eye size={13} />
                <span>Afficher les {doneTasksCount} tâches terminées</span>
              </button>
            )}
          </div>
        ) : groupByStatus ? (
          <div className="space-y-6">
            {/* Mode 1: Grouped by Agentic Workflow Stages */}
            {boardGrouping === "workflow" ? (
              WORKFLOW_STAGES.map(stage => {
                const groupTasks = sortedTasks.filter(t => resolveTaskStage(t, currentProject) === stage.id)
                if (groupTasks.length === 0) return null
                if (stage.id === "finished" && hideDone) return null

                const isGroupAllSelected = groupTasks.length > 0 && groupTasks.every(t => selectedTaskIds.has(t.id))
                const isGroupSomeSelected = groupTasks.some(t => selectedTaskIds.has(t.id)) && !isGroupAllSelected

                return (
                  <div key={stage.id} className="space-y-2">
                    {/* Stage Group Header */}
                    <div className="flex items-center justify-between px-1">
                      <div className="flex items-center gap-2">
                        <span className={stage.color}>{stage.icon}</span>
                        <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
                          {stage.label}
                        </h3>
                        <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded border ${stage.stageColor}`}>
                          {stage.stageLabel}
                        </span>
                        <span className="text-xs font-mono text-[var(--text-muted)]">
                          ({groupTasks.length})
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => toggleSelectGroup(groupTasks)}
                        className="text-[11px] font-semibold text-[var(--text-muted)] hover:text-[var(--accent-color)] transition-colors cursor-pointer"
                      >
                        {isGroupAllSelected ? "Tout désélectionner" : "Sélectionner le groupe"}
                      </button>
                    </div>

                    {/* Table */}
                    <div className="overflow-x-auto rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] shadow-xs">
                      <table className="w-full text-left border-collapse">
                        <thead>
                          <tr className="border-b border-[var(--border-color)]/60 text-[11px] uppercase tracking-wider text-[var(--text-muted)] font-semibold bg-[var(--bg-tertiary)]/40">
                            <th className="py-2 px-3 w-10 text-center">
                              <input
                                type="checkbox"
                                checked={isGroupAllSelected}
                                ref={el => {
                                  if (el) el.indeterminate = isGroupSomeSelected
                                }}
                                onChange={() => toggleSelectGroup(groupTasks)}
                                className="rounded text-[var(--accent-color)] focus:ring-0 cursor-pointer w-4 h-4"
                                title="Sélectionner toutes les tâches de ce groupe"
                              />
                            </th>
                            <th className="py-2 px-3 w-28">{t.list.columns.key}</th>
                            <th className="py-2 px-3 min-w-[240px]">{t.list.columns.title}</th>
                            <th className="py-2 px-3 w-44">Étape Workflow</th>
                            <th className="py-2 px-3 w-8 text-center">{t.list.columns.priority}</th>
                            <th className="py-2 px-3 w-36">{t.list.columns.labels}</th>
                            <th className="py-2 px-3 w-32">{t.list.columns.assignee}</th>
                            <th className="py-2 px-3 w-32">{t.list.columns.dueDate}</th>
                            <th className="py-2 px-3 text-right w-24">Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {groupTasks.map(task => renderTaskRow(task))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )
              })
            ) : (
              /* Mode 2: Grouped by Status */
              statusList.map(st => {
                if ((st.id === "finished" || st.id === "done") && hideDone) return null
                const groupTasks = sortedTasks.filter(t => t.status === st.id)
                if (groupTasks.length === 0) return null

                const isGroupAllSelected = groupTasks.length > 0 && groupTasks.every(t => selectedTaskIds.has(t.id))
                const isGroupSomeSelected = groupTasks.some(t => selectedTaskIds.has(t.id)) && !isGroupAllSelected

                return (
                  <div key={st.id} className="space-y-2">
                    {/* Status Group Header */}
                    <div className="flex items-center justify-between px-1">
                      <div className="flex items-center gap-2">
                        <span className={st.color}>{st.icon}</span>
                        <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
                          {st.label}
                        </h3>
                        <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded border ${st.stageColor}`}>
                          {st.stageLabel}
                        </span>
                        <span className="text-xs font-mono text-[var(--text-muted)]">
                          ({groupTasks.length})
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => toggleSelectGroup(groupTasks)}
                        className="text-[11px] font-semibold text-[var(--text-muted)] hover:text-[var(--accent-color)] transition-colors cursor-pointer"
                      >
                        {isGroupAllSelected ? "Tout désélectionner" : "Sélectionner le groupe"}
                      </button>
                    </div>

                    {/* Table */}
                    <div className="overflow-x-auto rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] shadow-xs">
                      <table className="w-full text-left border-collapse">
                        <thead>
                          <tr className="border-b border-[var(--border-color)]/60 text-[11px] uppercase tracking-wider text-[var(--text-muted)] font-semibold bg-[var(--bg-tertiary)]/40">
                            <th className="py-2 px-3 w-10 text-center">
                              <input
                                type="checkbox"
                                checked={isGroupAllSelected}
                                ref={el => {
                                  if (el) el.indeterminate = isGroupSomeSelected
                                }}
                                onChange={() => toggleSelectGroup(groupTasks)}
                                className="rounded text-[var(--accent-color)] focus:ring-0 cursor-pointer w-4 h-4"
                                title="Sélectionner toutes les tâches de ce groupe"
                              />
                            </th>
                            <th className="py-2 px-3 w-28">{t.list.columns.key}</th>
                            <th className="py-2 px-3 min-w-[240px]">{t.list.columns.title}</th>
                            <th className="py-2 px-3 w-40">{t.list.columns.status}</th>
                            <th className="py-2 px-3 w-8 text-center">{t.list.columns.priority}</th>
                            <th className="py-2 px-3 w-36">{t.list.columns.labels}</th>
                            <th className="py-2 px-3 w-32">{t.list.columns.assignee}</th>
                            <th className="py-2 px-3 w-32">{t.list.columns.dueDate}</th>
                            <th className="py-2 px-3 text-right w-24">Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {groupTasks.map(task => renderTaskRow(task))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )
              })
            )}

            {/* Notice if done tasks are hidden */}
            {hideDone && doneTasksCount > 0 && (
              <div className="flex items-center justify-between p-3 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-xs text-[var(--text-muted)]">
                <div className="flex items-center gap-2">
                  <EyeOff size={14} className="text-slate-400" />
                  <span>{doneTasksCount} {doneTasksCount > 1 ? "tâches terminées sont masquées" : "tâche terminée est masquée"}</span>
                </div>
                <button
                  onClick={toggleHideDone}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-bold text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/30 transition-colors cursor-pointer"
                >
                  <Eye size={12} />
                  <span>Afficher</span>
                </button>
              </div>
            )}
          </div>
        ) : (
          /* Flat Table */
          <div className="space-y-4">
            <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] overflow-hidden shadow-xs">
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="border-b border-[var(--border-color)] text-[11px] uppercase tracking-wider text-[var(--text-muted)] font-semibold bg-[var(--bg-tertiary)]/60">
                      <th className="py-2.5 px-3 w-10 text-center">
                        <input
                          type="checkbox"
                          checked={isAllVisibleSelected}
                          ref={el => {
                            if (el) el.indeterminate = isSomeVisibleSelected
                          }}
                          onChange={toggleSelectAllVisible}
                          className="rounded text-[var(--accent-color)] focus:ring-0 cursor-pointer w-4 h-4"
                          title="Sélectionner toutes les tâches visibles"
                        />
                      </th>
                      <th className="py-2.5 px-3 cursor-pointer hover:text-[var(--text-primary)] w-28" onClick={() => handleSort("key")}>
                        <div className="flex items-center gap-1">{t.list.columns.key} <ArrowUpDown size={12} /></div>
                      </th>
                      <th className="py-2.5 px-3 cursor-pointer hover:text-[var(--text-primary)] min-w-[240px]" onClick={() => handleSort("title")}>
                        <div className="flex items-center gap-1">{t.list.columns.title} <ArrowUpDown size={12} /></div>
                      </th>
                      <th className="py-2.5 px-3 cursor-pointer hover:text-[var(--text-primary)] w-44" onClick={() => handleSort("status")}>
                        <div className="flex items-center gap-1">
                          {boardGrouping === "workflow" ? "Étape Workflow" : t.list.columns.status} <ArrowUpDown size={12} />
                        </div>
                      </th>
                      <th className="py-2.5 px-3 cursor-pointer hover:text-[var(--text-primary)] w-8 text-center" onClick={() => handleSort("priority")}>
                        <div className="flex items-center justify-center gap-1">{t.list.columns.priority} <ArrowUpDown size={12} /></div>
                      </th>
                      <th className="py-2.5 px-3 w-36">{t.list.columns.labels}</th>
                      <th className="py-2.5 px-3 w-32">{t.list.columns.assignee}</th>
                      <th className="py-2.5 px-3 cursor-pointer hover:text-[var(--text-primary)] w-32" onClick={() => handleSort("dueDate")}>
                        <div className="flex items-center gap-1">{t.list.columns.dueDate} <ArrowUpDown size={12} /></div>
                      </th>
                      <th className="py-2.5 px-3 text-right w-24">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleTasks.map(task => renderTaskRow(task))}
                  </tbody>
                </table>
              </div>
            </div>

            {hideDone && doneTasksCount > 0 && (
              <div className="flex items-center justify-between p-3 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-xs text-[var(--text-muted)]">
                <div className="flex items-center gap-2">
                  <EyeOff size={14} className="text-slate-400" />
                  <span>{doneTasksCount} {doneTasksCount > 1 ? "tâches terminées sont masquées" : "tâche terminée est masquée"}</span>
                </div>
                <button
                  onClick={toggleHideDone}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-bold text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/30 transition-colors cursor-pointer"
                >
                  <Eye size={12} />
                  <span>Afficher</span>
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* ------------------------------------------------------------- */}
      {/* Floating Bulk Action Bar */}
      {/* ------------------------------------------------------------- */}
      {selectedTaskIds.size > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 animate-in slide-in-from-bottom-5 duration-200">
          <div
            ref={bulkDropdownRef}
            className="flex items-center gap-2.5 px-4 py-2.5 rounded-2xl bg-[var(--bg-secondary)]/95 backdrop-blur-md border border-[var(--border-color)] shadow-2xl text-xs max-w-[95vw] flex-wrap justify-center sm:justify-start"
          >
            {/* Selection Counter & Clear */}
            <div className="flex items-center gap-2 pr-3 border-r border-[var(--border-color)]">
              <span className="flex items-center justify-center min-w-[22px] h-[22px] px-1.5 rounded-full bg-[var(--accent-color)] text-white text-[11px] font-bold shadow-xs">
                {selectedTaskIds.size}
              </span>
              <span className="font-semibold text-[var(--text-primary)] whitespace-nowrap">
                {selectedTaskIds.size > 1 ? "sélectionnées" : "sélectionnée"}
              </span>
              <button
                type="button"
                onClick={clearSelection}
                className="text-[var(--text-muted)] hover:text-[var(--text-primary)] p-1 rounded hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                title="Désélectionner tout"
              >
                <X size={14} />
              </button>
            </div>

            {/* Action 1: Status / Stage in Bulk */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActiveBulkDropdown(prev => prev === "status" ? null : "status")}
                disabled={isBulkProcessing}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl font-semibold border transition-all cursor-pointer ${
                  activeBulkDropdown === "status"
                    ? "bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/40 shadow-xs"
                    : "bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)]"
                }`}
              >
                {boardGrouping === "workflow" ? <Sparkles size={13} className="text-cyan-400" /> : <Kanban size={13} className="text-indigo-400" />}
                <span>{boardGrouping === "workflow" ? "Étape" : "Statut"}</span>
              </button>

              {activeBulkDropdown === "status" && (
                <div className="absolute bottom-full mb-2 left-0 w-60 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-xl p-1.5 z-50 space-y-1 animate-in fade-in-50 zoom-in-95">
                  <div className="px-2 py-1 text-[10px] uppercase font-bold text-[var(--text-muted)] tracking-wider border-b border-[var(--border-color)]/50 pb-1 mb-1">
                    {boardGrouping === "workflow" ? "Changer l'étape agentique" : "Changer le statut"}
                  </div>
                  {boardGrouping === "workflow" ? (
                    WORKFLOW_STAGES.map(s => (
                      <button
                        key={s.id}
                        type="button"
                        onClick={() => handleBulkWorkflowStage(s.id)}
                        className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-[var(--bg-tertiary)] text-left text-xs text-[var(--text-primary)] transition-colors cursor-pointer"
                      >
                        <div className="flex items-center gap-2">
                          <span className={s.color}>{s.icon}</span>
                          <span className="font-medium">{s.label}</span>
                        </div>
                        <span className={`text-[9px] font-bold px-1 py-0.2 rounded border ${s.stageColor}`}>
                          {s.stageLabel}
                        </span>
                      </button>
                    ))
                  ) : currentProject?.trackerColumns && currentProject.trackerColumns.length > 0 ? (
                    trackerStatusOptions.map(st => (
                      <button
                        key={st}
                        type="button"
                        onClick={() => handleBulkTrackerStatus(st)}
                        className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-[var(--bg-tertiary)] text-left text-xs text-[var(--text-primary)] transition-colors cursor-pointer"
                      >
                        <span className="font-medium">{st}</span>
                      </button>
                    ))
                  ) : (
                    statusList.map(s => (
                      <button
                        key={s.id}
                        type="button"
                        onClick={() => handleBulkTrackerStatus(s.id)}
                        className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-[var(--bg-tertiary)] text-left text-xs text-[var(--text-primary)] transition-colors cursor-pointer"
                      >
                        <div className="flex items-center gap-2">
                          <span className={s.color}>{s.icon}</span>
                          <span className="font-medium">{s.label}</span>
                        </div>
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>

            {/* Action 2: Priority in Bulk */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActiveBulkDropdown(prev => prev === "priority" ? null : "priority")}
                disabled={isBulkProcessing}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl font-semibold border transition-all cursor-pointer ${
                  activeBulkDropdown === "priority"
                    ? "bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/40 shadow-xs"
                    : "bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)]"
                }`}
              >
                <Flame size={13} className="text-amber-400" />
                <span>Priorité</span>
              </button>

              {activeBulkDropdown === "priority" && (
                <div className="absolute bottom-full mb-2 left-0 w-44 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-xl p-1.5 z-50 space-y-1 animate-in fade-in-50 zoom-in-95">
                  <div className="px-2 py-1 text-[10px] uppercase font-bold text-[var(--text-muted)] tracking-wider border-b border-[var(--border-color)]/50 pb-1 mb-1">
                    Définir la priorité
                  </div>
                  {PRIORITY_OPTIONS.map(p => (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => handleBulkPriority(p.id)}
                      className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-[var(--bg-tertiary)] text-left text-xs text-[var(--text-primary)] transition-colors cursor-pointer font-medium"
                    >
                      <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: p.color }} />
                      <span>{p.label}</span>
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Action 3: Labels in Bulk */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActiveBulkDropdown(prev => prev === "labels" ? null : "labels")}
                disabled={isBulkProcessing}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl font-semibold border transition-all cursor-pointer ${
                  activeBulkDropdown === "labels"
                    ? "bg-[var(--accent-light)] accent-text border-[var(--accent-color)]/40 shadow-xs"
                    : "bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)]"
                }`}
              >
                <Tag size={13} className="text-blue-400" />
                <span>Labels</span>
              </button>

              {activeBulkDropdown === "labels" && (
                <div className="absolute bottom-full mb-2 -left-16 sm:left-0 w-72 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl p-3 z-50 space-y-3 animate-in fade-in-50 zoom-in-95">
                  <div className="flex items-center justify-between border-b border-[var(--border-color)] pb-1.5">
                    <span className="text-[11px] font-bold text-[var(--text-primary)] uppercase tracking-wider">
                      Gestion des Labels
                    </span>
                    <button
                      type="button"
                      onClick={() => setActiveBulkDropdown(null)}
                      className="text-[var(--text-muted)] hover:text-[var(--text-primary)] p-0.5 rounded"
                    >
                      <X size={13} />
                    </button>
                  </div>

                  {/* Add Label Form */}
                  <form
                    onSubmit={e => {
                      e.preventDefault()
                      if (bulkLabelInput.trim()) {
                        handleBulkAddLabel(bulkLabelInput.trim())
                      }
                    }}
                    className="flex items-center gap-1.5"
                  >
                    <input
                      type="text"
                      value={bulkLabelInput}
                      onChange={e => setBulkLabelInput(e.target.value)}
                      placeholder="Nouveau label..."
                      className="flex-1 text-xs px-2.5 py-1.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)]"
                    />
                    <button
                      type="submit"
                      disabled={!bulkLabelInput.trim() || isBulkProcessing}
                      className="px-2.5 py-1.5 rounded-lg bg-[var(--accent-color)] text-white text-xs font-bold hover:opacity-90 disabled:opacity-50 transition-all cursor-pointer flex items-center gap-1 shrink-0"
                    >
                      <Plus size={12} />
                      <span>Ajouter</span>
                    </button>
                  </form>

                  {/* Existing project labels */}
                  {allProjectLabels.length > 0 && (
                    <div className="space-y-1">
                      <div className="text-[10px] uppercase font-bold text-[var(--text-muted)]">
                        Labels du projet (+ ajouter)
                      </div>
                      <div className="flex flex-wrap gap-1 max-h-24 overflow-y-auto pr-1">
                        {allProjectLabels.map(l => (
                          <button
                            key={l}
                            type="button"
                            onClick={() => handleBulkAddLabel(l)}
                            className="text-[10px] px-1.5 py-0.5 rounded-md border border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:text-cyan-400 hover:border-cyan-500/40 hover:bg-cyan-500/10 transition-colors cursor-pointer flex items-center gap-0.5"
                          >
                            <Plus size={9} />
                            <span>#{l}</span>
                          </button>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Labels present on selected tasks to remove */}
                  {selectedTasksLabels.length > 0 && (
                    <div className="space-y-1 pt-2 border-t border-[var(--border-color)]">
                      <div className="text-[10px] uppercase font-bold text-[var(--text-muted)]">
                        Labels sur la sélection (cliquer pour retirer)
                      </div>
                      <div className="flex flex-wrap gap-1 max-h-24 overflow-y-auto pr-1">
                        {selectedTasksLabels.map(({ label, count }) => (
                          <button
                            key={label}
                            type="button"
                            onClick={() => handleBulkRemoveLabel(label)}
                            className="text-[10px] px-1.5 py-0.5 rounded-md border border-rose-500/30 bg-rose-500/10 text-rose-300 hover:bg-rose-500/20 transition-colors cursor-pointer flex items-center gap-1 group/chip"
                            title={`Retirer #${label} de ${count} tâche(s)`}
                          >
                            <span>#{label}</span>
                            <span className="text-[9px] opacity-70">({count})</span>
                            <X size={10} className="text-rose-400 group-hover/chip:scale-110" />
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Action 4: Delete in Bulk */}
            <button
              type="button"
              onClick={handleBulkDelete}
              disabled={isBulkProcessing}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl font-semibold text-rose-400 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/25 transition-all cursor-pointer"
              title="Supprimer les tâches sélectionnées"
            >
              <Trash2 size={13} />
              <span className="hidden sm:inline">Supprimer</span>
            </button>

            {/* Loading Indicator */}
            {isBulkProcessing && (
              <div className="flex items-center gap-1.5 text-xs text-[var(--accent-color)] font-medium pl-2 border-l border-[var(--border-color)]">
                <Loader2 size={13} className="animate-spin" />
                <span>En cours...</span>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
