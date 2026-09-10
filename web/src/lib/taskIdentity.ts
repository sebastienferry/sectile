import type { Task } from '../types'

// Public keys such as #39 are only unique within their tracker project.
export const sameTask = (left: Pick<Task, 'id'>, right: Pick<Task, 'id'>): boolean =>
  left.id === right.id

export const tasksInProject = <T extends Pick<Task, 'projectId'>>(tasks: T[], projectId: string): T[] =>
  projectId && projectId !== 'all' ? tasks.filter(task => task.projectId === projectId) : tasks
