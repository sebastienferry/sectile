import type { TrackerSprint } from '../types'

/**
 * The tracker-backed sprint routes. Each one writes to the tracker before it
 * answers; the caller refreshes the project to show the mirrored list.
 */

export interface SprintPatch {
  name?: string
  start?: string
  end?: string
  state?: 'active' | 'future' | 'closed'
  moveOpenTo?: 'next' | 'backlog'
}

export interface SprintBatch {
  name: string
  count: number
  start: string
  weeks: number
}

/**
 * A refused sprint request. The message is the server's reason when it gave
 * one, else empty: the caller words the bare status in the UI language.
 */
export class SprintRequestError extends Error {
  readonly status: number

  constructor(status: number, reason?: string) {
    super(reason || '')
    this.name = 'SprintRequestError'
    this.status = status
  }
}

const base = (projectId: string) => `/api/projects/${encodeURIComponent(projectId)}/sprints`

async function send<T>(url: string, method: string, body?: unknown): Promise<{ status: number; data: T & { error?: string } }> {
  const res = await fetch(url, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok && res.status !== 207) throw new SprintRequestError(res.status, data.error)
  return { status: res.status, data }
}

/** Creates a batch; a partial failure resolves with what was created and the reason. */
export async function createSprints(projectId: string, batch: SprintBatch): Promise<{ created: TrackerSprint[]; error?: string }> {
  const { data } = await send<{ created?: TrackerSprint[] }>(base(projectId), 'POST', batch)
  return { created: data.created || [], error: data.error }
}

export async function updateSprint(projectId: string, sprintId: string, patch: SprintPatch): Promise<TrackerSprint> {
  const { data } = await send<TrackerSprint>(`${base(projectId)}/${encodeURIComponent(sprintId)}`, 'PATCH', patch)
  return data
}

export async function deleteSprint(projectId: string, sprintId: string): Promise<void> {
  await send(`${base(projectId)}/${encodeURIComponent(sprintId)}`, 'DELETE')
}
