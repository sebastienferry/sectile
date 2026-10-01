import type { Project, Task, ViewMode } from '../types'
import { isViewAvailable } from './optionalViews.ts'
import type { HorizonTab, MacroRow } from './roadmap'

/**
 * The trip between a ticket and its epic on the roadmap (#630).
 *
 * A ticket with a parent opens the roadmap on that epic, and the epic's panel
 * leads back to the ticket view the user came from, filtered on it. The
 * decisions that need no React live here: whether a ticket may make the trip,
 * where the epic sits on the roadmap, and which view the way back lands on.
 */

/** The views that list tickets, the ones the way back can return to. */
export const TICKET_VIEWS: ViewMode[] = ['board', 'list', 'triage', 'timeline']

export const isTicketView = (view: ViewMode | null | undefined): boolean =>
  Boolean(view) && TICKET_VIEWS.includes(view as ViewMode)

/**
 * True when the ticket has a parent key and its project shows the roadmap.
 * Offering the entry on a project without the view would open a screen the
 * sidebar does not even list.
 */
export function canOpenEpicInRoadmap(
  task: Pick<Task, 'parentKey'>,
  project: Project | null | undefined
): boolean {
  if (!(task.parentKey || '').trim()) return false
  if (!project) return false
  return isViewAvailable(project, 'roadmap')
}

/**
 * The project a ticket belongs to. A ticket without a project id is one of the
 * project on screen; under "all projects" such a ticket has none.
 */
export function projectOfTask(
  task: Pick<Task, 'projectId'>,
  projects: Project[],
  currentProject: Project | null | undefined
): Project | null {
  if (!task.projectId) return currentProject || null
  return projects.find(p => p.id === task.projectId || p.slug === task.projectId) || null
}

/**
 * Where the epic sits on the roadmap: the tab of its horizon, and whether it
 * is closed, which decides if closed epics must be shown. Null when the
 * roadmap holds no row for the key, which the caller refuses rather than
 * opening another epic's panel. The key is matched exactly: it is the very key
 * the rows are built from.
 */
export function locateEpic(rows: MacroRow[], key: string): { tab: HorizonTab; closed: boolean } | null {
  const row = rows.find(r => r.key === key)
  if (!row) return null
  return { tab: row.horizon || 'unclassified', closed: row.closed }
}

/**
 * The view the way back lands on: the ticket view left to reach the roadmap
 * when the project still shows it, the board otherwise.
 */
export function returnView(origin: ViewMode | null, project: Project | null | undefined): ViewMode {
  if (origin && isTicketView(origin) && isViewAvailable(project, origin)) return origin
  return 'board'
}
