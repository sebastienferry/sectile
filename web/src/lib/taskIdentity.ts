import type { Task } from '../types'

// Public keys such as #39 are only unique within their tracker project.
export const sameTask = (left: Pick<Task, 'id'>, right: Pick<Task, 'id'>): boolean =>
  left.id === right.id

export const tasksInProject = <T extends Pick<Task, 'projectId'>>(
  tasks: T[],
  projectId: string,
  bookmarkedProjectIds?: Set<string> | string[]
): T[] => {
  if (projectId && projectId !== 'all') {
    return tasks.filter(task => task.projectId === projectId)
  }
  if (bookmarkedProjectIds) {
    const set = bookmarkedProjectIds instanceof Set ? bookmarkedProjectIds : new Set(bookmarkedProjectIds)
    if (set.size > 0) {
      return tasks.filter(task => (task.projectId ? set.has(task.projectId) : false))
    }
  }
  return tasks
}
