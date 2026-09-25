import type { Priority, Task } from '../types'
import type { StorageLike } from './boardDisplayMode'

/**
 * The order of the Board columns and of the Backlog lists (#402). One sort is
 * shared by both views and remembered per browser; drag and drop changes a
 * ticket's stage and never reorders a column, so no manual order is lost.
 */
export type BoardSortField = 'priority' | 'epic' | 'key' | 'updated'

export interface BoardSort {
  field: BoardSortField
  asc: boolean
}

export const BOARD_SORT_STORAGE_KEY = 'sectile_board_sort'
export const DEFAULT_BOARD_SORT: BoardSort = { field: 'priority', asc: false }

export interface BoardSortOption {
  id: BoardSortField
  /** Key in `translations.board.sort.fields`. */
  labelKey: BoardSortField
  /** The direction a pick of this criterion starts in. */
  naturalAsc: boolean
}

/** Order of the selector: priority first, as it is the default. */
export const BOARD_SORT_OPTIONS: readonly BoardSortOption[] = [
  { id: 'priority', labelKey: 'priority', naturalAsc: false },
  { id: 'epic', labelKey: 'epic', naturalAsc: false },
  { id: 'key', labelKey: 'key', naturalAsc: true },
  { id: 'updated', labelKey: 'updated', naturalAsc: false },
]

export type SortableTask = Pick<Task, 'id' | 'key' | 'priority' | 'parentKey' | 'trackerUpdatedAt' | 'updatedAt'>

const PRIORITY_RANK: Record<Priority, number> = { urgent: 4, high: 3, medium: 2, low: 1 }

const rank = (task: SortableTask): number => PRIORITY_RANK[task.priority] || 0

const compareKeys = (a: string, b: string): number => a.localeCompare(b, undefined, { numeric: true })

/**
 * The tie-break every criterion ends with, never reversed: priority highest
 * first, then key oldest first, then the id, so that two tickets sharing a key
 * in a multi-project view keep one order whatever order the list arrived in.
 */
export function compareSortTail(a: SortableTask, b: SortableTask): number {
  return rank(b) - rank(a) || compareKeys(a.key, b.key) || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
}

/** The tracker update date, else the Sectile one; NaN when neither parses. */
const updatedTime = (task: SortableTask): number => Date.parse(task.trackerUpdatedAt || task.updatedAt)

/** Returns the criterion picked in the selector, in its natural direction. */
export function pickBoardSortField(field: BoardSortField): BoardSort {
  const option = BOARD_SORT_OPTIONS.find(candidate => candidate.id === field)
  return { field, asc: option ? option.naturalAsc : DEFAULT_BOARD_SORT.asc }
}

/**
 * Epic mode keeps the tickets of one parent together. A card's place depends
 * on its whole group, so this is a partition rather than a pairwise
 * comparator: groups are ordered by their highest priority (the only part the
 * direction reverses), then by parent key; cards inside a group, and the
 * tickets without a parent that close the list, are ordered by the tail.
 */
function sortByEpic<T extends SortableTask>(tasks: readonly T[], dir: number): T[] {
  const groups = new Map<string, T[]>()
  const orphans: T[] = []
  for (const task of tasks) {
    if (!task.parentKey) {
      orphans.push(task)
      continue
    }
    const group = groups.get(task.parentKey)
    if (group) {
      group.push(task)
    } else {
      groups.set(task.parentKey, [task])
    }
  }

  const ordered = Array.from(groups, ([parentKey, members]) => ({
    parentKey,
    top: Math.max(...members.map(rank)),
    members: members.sort(compareSortTail),
  })).sort((a, b) => dir * (a.top - b.top) || compareKeys(a.parentKey, b.parentKey))

  return [...ordered.flatMap(group => group.members), ...orphans.sort(compareSortTail)]
}

/** Returns a new array in the order the sort asks for; the input is untouched. */
export function sortTasks<T extends SortableTask>(tasks: readonly T[], sort: BoardSort): T[] {
  const dir = sort.asc ? 1 : -1
  switch (sort.field) {
    case 'epic':
      return sortByEpic(tasks, dir)
    case 'key':
      return [...tasks].sort((a, b) => dir * compareKeys(a.key, b.key) || compareSortTail(a, b))
    case 'updated':
      return [...tasks].sort((a, b) => {
        const timeA = updatedTime(a)
        const timeB = updatedTime(b)
        const undatedA = Number.isNaN(timeA)
        const undatedB = Number.isNaN(timeB)
        // Tickets without a date close the list in both directions.
        if (undatedA || undatedB) {
          return undatedA === undatedB ? compareSortTail(a, b) : undatedA ? 1 : -1
        }
        return dir * (timeA - timeB) || compareSortTail(a, b)
      })
    case 'priority':
    default:
      return [...tasks].sort((a, b) => dir * (rank(a) - rank(b)) || compareSortTail(a, b))
  }
}

function resolveStorage(customStorage?: StorageLike): StorageLike | null {
  if (customStorage) {
    return customStorage
  }
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      return window.localStorage
    }
  } catch {
    return null
  }
  return null
}

const isBoardSortField = (value: unknown): value is BoardSortField =>
  BOARD_SORT_OPTIONS.some(option => option.id === value)

/**
 * Loads the remembered sort. Anything missing, unreadable or invalid means the
 * default, priority highest first, which is the order the Board always had.
 */
export function loadBoardSort(customStorage?: StorageLike): BoardSort {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return DEFAULT_BOARD_SORT
  }

  try {
    const raw = storage.getItem(BOARD_SORT_STORAGE_KEY)
    if (!raw) {
      return DEFAULT_BOARD_SORT
    }
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) {
      return DEFAULT_BOARD_SORT
    }
    const { field, asc } = parsed as { field?: unknown; asc?: unknown }
    if (!isBoardSortField(field) || typeof asc !== 'boolean') {
      return DEFAULT_BOARD_SORT
    }
    return { field, asc }
  } catch {
    return DEFAULT_BOARD_SORT
  }
}

/** Persists the sort shared by the Board and the Backlog. */
export function saveBoardSort(sort: BoardSort, customStorage?: StorageLike): void {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return
  }

  try {
    storage.setItem(BOARD_SORT_STORAGE_KEY, JSON.stringify({ field: sort.field, asc: sort.asc }))
  } catch {
    // Storage unavailable, ignore gracefully
  }
}
