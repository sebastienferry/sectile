import type { ActivityStats, Priority, Status, Task, TaskActivity, ViewMode } from '../types'
import type { AssigneeIdentities } from './myTasks'
import { sameTask } from './taskIdentity.ts'
import { sendsServerSearch } from './taskQuery.ts'

export interface TaskBoardScope {
  selectedProjectId: string
  selectedProjectSlug?: string
  selectedViewId?: string | null
  viewProjectIds?: readonly string[] | null
  viewLabels?: readonly string[] | null
  bookmarkedProjectIds?: readonly string[] | null
  activeView?: ViewMode
  searchQuery?: string
  statusFilter?: Status | null
  priorityFilter?: Priority | null
  labelFilter?: string | null
  sprintFilter?: string | null
  teamFilter?: string | null
  parentFilter?: string | null
  assigneeFilter?: string | null
  myTasksOnly?: boolean
  myTasksIdentities?: AssigneeIdentities | null
  trackerStatusFilters?: readonly string[]
  issueTypeFilters?: readonly string[]
  pinnedOnly?: boolean
}

function taskProjectIds(task: Pick<Task, 'projectId' | 'projectIds'>): string[] {
  if (Array.isArray(task.projectIds) && task.projectIds.length > 0) {
    return task.projectIds.filter(Boolean)
  }
  return task.projectId ? [task.projectId] : []
}

function taskMatchesProjectScope(task: Task, scope: TaskBoardScope, wasInList: boolean): boolean {
  const ids = taskProjectIds(task)
  if (scope.selectedViewId) {
    const viewProjects = scope.viewProjectIds ?? []
    if (viewProjects.length > 0 && !ids.some(id => viewProjects.includes(id))) {
      return false
    }
    const viewLabels = (scope.viewLabels ?? []).map(label => label.trim().toLowerCase()).filter(Boolean)
    if (viewLabels.length > 0) {
      const taskLabels = (task.labels ?? []).map(label => label.trim().toLowerCase())
      if (!viewLabels.some(label => taskLabels.includes(label))) return false
    }
    return true
  }
  if (scope.selectedProjectId && scope.selectedProjectId !== 'all') {
    return ids.includes(scope.selectedProjectId) || (Boolean(scope.selectedProjectSlug) && ids.includes(scope.selectedProjectSlug!))
  }
  const bookmarked = scope.bookmarkedProjectIds ?? []
  if (bookmarked.length > 0) {
    return ids.some(id => bookmarked.includes(id))
  }
  return wasInList || ids.length > 0
}

function taskMatchesMyTasks(task: Task, identities: AssigneeIdentities | null | undefined, wasInList: boolean): boolean {
  const assignee = (task.assignee || '').trim().toLowerCase()
  if (!assignee) return false
  if (!identities) return wasInList
  const source = (task.source || '').toLowerCase()
  const candidates = new Set<string>()
  const add = (value?: string) => {
    const trimmed = (value || '').trim().toLowerCase()
    if (trimmed) candidates.add(trimmed)
  }
  const trackerMatch = source
    ? (identities.trackers ?? []).find(entry => entry.tracker.toLowerCase() === source)
    : undefined
  if (trackerMatch && trackerMatch.known && trackerMatch.identity) {
    add(trackerMatch.identity)
  } else {
    ;(identities.trackers ?? []).forEach(entry => add(entry.identity))
    ;(identities.fallback ?? []).forEach(add)
  }
  return candidates.size === 0 ? wasInList : candidates.has(assignee)
}

/**
 * Whether an updated task from `task_updated` belongs on the board currently
 * displayed (#722), respecting the active project or saved view and filters.
 */
export function taskMatchesBoardScope(task: Task, scope: TaskBoardScope, wasInList = false): boolean {
  if (!taskMatchesProjectScope(task, scope, wasInList)) return false
  if (scope.statusFilter && task.status !== scope.statusFilter) return false
  if (scope.priorityFilter && task.priority !== scope.priorityFilter) return false
  if (scope.labelFilter) {
    const wanted = scope.labelFilter.trim().toLowerCase()
    if (!(task.labels ?? []).some(label => label.trim().toLowerCase() === wanted)) return false
  }
  if (scope.sprintFilter && (task.sprint || '') !== scope.sprintFilter) return false
  if (scope.teamFilter && (task.team || '') !== scope.teamFilter) return false
  if (scope.parentFilter && scope.activeView !== 'roadmap' && (task.parentKey || '') !== scope.parentFilter) {
    return false
  }
  if (scope.assigneeFilter && (task.assignee || '') !== scope.assigneeFilter) return false
  if (scope.myTasksOnly && !taskMatchesMyTasks(task, scope.myTasksIdentities, wasInList)) return false
  if (scope.trackerStatusFilters && scope.trackerStatusFilters.length > 0) {
    if (!task.trackerStatus || !scope.trackerStatusFilters.includes(task.trackerStatus)) return false
  }
  if (scope.issueTypeFilters && scope.issueTypeFilters.length > 0) {
    if (!task.issueType || !scope.issueTypeFilters.includes(task.issueType)) return false
  }
  if (scope.pinnedOnly) {
    const pinned = (task.labels ?? []).some(label => {
      const lower = label.trim().toLowerCase()
      return lower === '#pinned' || lower === 'pinned'
    })
    if (!pinned) return false
  }
  if (scope.searchQuery && (!scope.activeView || sendsServerSearch(scope.activeView))) {
    const q = scope.searchQuery.trim().toLowerCase()
    if (q) {
      const haystacks = [
        task.key || '',
        task.title || '',
        task.description || '',
        task.assignee || '',
        ...(task.labels ?? []),
      ]
      if (!haystacks.some(value => value.toLowerCase().includes(q))) return false
    }
  }
  return true
}

/**
 * Updates `tasks` in place from a `task_updated` SSE event (#722): replaces an
 * existing task when it stays in scope, removes it when it leaves scope, or
 * inserts it when it enters scope.
 */
export function applyUpdatedTask(tasks: Task[], incoming: Task, scope: TaskBoardScope): Task[] {
  const index = tasks.findIndex(existing => sameTask(existing, incoming))
  const ids = taskProjectIds(incoming)
  const scopedProjectId =
    scope.selectedProjectId &&
    scope.selectedProjectId !== 'all' &&
    !scope.selectedViewId &&
    ids.includes(scope.selectedProjectId)
      ? scope.selectedProjectId
      : index >= 0 && tasks[index].projectId && ids.includes(tasks[index].projectId!)
        ? tasks[index].projectId
        : incoming.projectId
  const normalized: Task = scopedProjectId && scopedProjectId !== incoming.projectId
    ? { ...incoming, projectId: scopedProjectId }
    : incoming

  const matches = taskMatchesBoardScope(normalized, scope, index >= 0)
  if (index >= 0) {
    if (!matches) {
      return tasks.filter((_, i) => i !== index)
    }
    const next = [...tasks]
    next[index] = normalized
    return next
  }
  if (!matches) return tasks
  return [normalized, ...tasks]
}

export function activityMatchesProjectScope(
  activity: Pick<TaskActivity, 'projectId' | 'runProjectId'>,
  selectedProjectId: string,
  task?: Pick<Task, 'projectId' | 'projectIds'> | null
): boolean {
  if (!selectedProjectId || selectedProjectId === 'all') return true
  if (activity.projectId === selectedProjectId || activity.runProjectId === selectedProjectId) return true
  if (task && taskProjectIds(task).includes(selectedProjectId)) return true
  return false
}

function statusBucket(status: TaskActivity['status']): keyof Omit<ActivityStats, 'total'> | null {
  switch (status) {
    case 'queued':
    case 'pending':
      return 'queued'
    case 'running':
      return 'running'
    case 'completed':
      return 'completed'
    case 'failed':
      return 'failed'
    case 'canceled':
      return 'canceled'
    default:
      return null
  }
}

/**
 * Updates `activities` and `activityStats` in place from a `task_updated` SSE
 * event (#722), avoiding full `/api/activities` and `/api/activities/stats`
 * reloads on every event.
 */
export function applyUpdatedActivity(
  activities: TaskActivity[],
  stats: ActivityStats,
  incoming: TaskActivity,
  selectedProjectId: string,
  task?: Pick<Task, 'projectId' | 'projectIds'> | null
): { activities: TaskActivity[]; stats: ActivityStats } {
  const index = activities.findIndex(existing => existing.id === incoming.id)
  if (index >= 0) {
    const previous = activities[index]
    const nextActivities = [...activities]
    nextActivities[index] = incoming
    if (previous.status === incoming.status) {
      return { activities: nextActivities, stats }
    }
    const nextStats = { ...stats }
    const oldBucket = statusBucket(previous.status)
    const newBucket = statusBucket(incoming.status)
    if (oldBucket) nextStats[oldBucket] = Math.max(0, nextStats[oldBucket] - 1)
    if (newBucket) nextStats[newBucket] = nextStats[newBucket] + 1
    return { activities: nextActivities, stats: nextStats }
  }
  if (!activityMatchesProjectScope(incoming, selectedProjectId, task)) {
    return { activities, stats }
  }
  const nextStats = { ...stats, total: stats.total + 1 }
  const bucket = statusBucket(incoming.status)
  if (bucket) nextStats[bucket] = nextStats[bucket] + 1
  return {
    activities: [incoming, ...activities],
    stats: nextStats,
  }
}

/**
 * Coalesces a burst of bare reload triggers into a single call after `delayMs`.
 */
export function createCoalescedRefetch(delayMs: number, run: () => void) {
  let timer: ReturnType<typeof setTimeout> | null = null
  return {
    schedule() {
      if (timer !== null) return
      timer = setTimeout(() => {
        timer = null
        run()
      }, delayMs)
    },
    cancel() {
      if (timer !== null) {
        clearTimeout(timer)
        timer = null
      }
    },
  }
}
