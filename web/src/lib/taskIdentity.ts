import type { Task } from '../types'

// Public keys such as #39 are only unique within their tracker project.
export const sameTask = (left: Pick<Task, 'id'>, right: Pick<Task, 'id'>): boolean =>
  left.id === right.id

export const tasksInProject = <T extends Pick<Task, 'projectId' | 'projectIds'>>(
  tasks: T[],
  projectId: string,
  bookmarkedProjectIds?: Set<string> | string[]
): T[] => {
  const idsOf = (task: T): string[] =>
    task.projectIds && task.projectIds.length > 0 ? task.projectIds : task.projectId ? [task.projectId] : []
  if (projectId && projectId !== 'all') {
    return tasks.filter(task => idsOf(task).includes(projectId))
  }
  if (bookmarkedProjectIds) {
    const set = bookmarkedProjectIds instanceof Set ? bookmarkedProjectIds : new Set(bookmarkedProjectIds)
    if (set.size > 0) {
      return tasks.filter(task => idsOf(task).some(id => set.has(id)))
    }
  }
  return tasks
}

