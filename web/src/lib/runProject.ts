import type { Project, Task } from '../types'

/**
 * The project a run works for (#741). A ticket can belong to several projects;
 * a run works for one of them, which gives it its repositories and rules. A run
 * launched from a project board works for that project; anywhere else the
 * server takes the ticket's only project, or answers with the candidates so
 * the person picks one.
 */

/** One project a run on a ticket could work for, as the server lists it. */
export interface RunProjectCandidate {
  id: string
  name: string
}

/**
 * The candidates of a refusal that asks which project a run works for, or null
 * when the refusal is something else. Only the interactive 409 asks: a busy
 * ticket is a 409 too, and carries no candidates; an unattended launch
 * (autonomous, a batch) is refused with a 400 that lists the candidates but
 * must not be retried for one of them, so it is shown as the refusal it is.
 */
export function runProjectCandidates(status: number, body: unknown): RunProjectCandidate[] | null {
  if (status !== 409 || !body || typeof body !== 'object') return null
  if ((body as { unattended?: unknown }).unattended === true) return null
  const candidates = (body as { candidates?: unknown }).candidates
  if (!Array.isArray(candidates) || candidates.length === 0) return null
  const list = candidates
    .filter((c): c is { id: unknown; name?: unknown } => Boolean(c) && typeof c === 'object' && 'id' in c)
    .map(c => ({ id: String(c.id), name: typeof c.name === 'string' && c.name ? c.name : String(c.id) }))
    .filter(c => c.id !== '')
  return list.length > 0 ? list : null
}

/**
 * The project a run launched from the board works for: the selected project
 * when the ticket belongs to it. "All projects", a saved view, or a ticket the
 * selected project does not show leave the choice to the server.
 */
export function boardRunProject(
  task: Pick<Task, 'projectId' | 'projectIds'> | undefined,
  selectedProjectId: string,
  projects: Pick<Project, 'id' | 'slug'>[],
): string | undefined {
  if (!selectedProjectId || selectedProjectId === 'all') return undefined
  const project = projects.find(p => p.id === selectedProjectId || p.slug === selectedProjectId)
  if (!project) return undefined
  if (task?.projectIds && task.projectIds.length > 0 && !task.projectIds.includes(project.id)) return undefined
  return project.id
}
