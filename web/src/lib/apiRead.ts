/**
 * Reading the API without losing the failures.
 *
 * The interface used to read with `if (res.ok) { … }` and drop everything
 * else: a 500 on `/api/projects` left the sidebar empty, the board empty, and
 * nothing anywhere saying the server had answered at all. A read has three
 * outcomes, not two, and this module keeps them apart so the caller can say
 * which one happened.
 */

/** The reads this module names, so a failure can be reported in the user's language. */
export type ReadResource = 'projects' | 'tasks' | 'settings' | 'boardViews'

/** The reads whose failure degrades the whole interface rather than one panel. */
export const CORE_READS: ReadResource[] = ['projects', 'tasks']

/**
 * What a read came back as:
 * - `ok`: the body was fetched and parsed.
 * - `silent`: the server refused the session (401). Signing in is mandatory and
 *   `installUnauthorizedRedirect` already sends the browser to the sign-in
 *   screen; saying it twice would turn every start-up into noise.
 * - `failed`: anything else — a status the server refused on, an unparseable
 *   body, or a request that never reached it (`status: null`).
 */
export type ReadOutcome<T> =
  | { kind: 'ok'; data: T }
  | { kind: 'silent'; status: number }
  | { kind: 'failed'; status: number | null; detail: string }

/** A failing read, as the banner and the toast need it. */
export interface ReadFailure {
  resource: ReadResource
  detail: string
}

/**
 * The single sentence that says what the server answered: `HTTP 500`, the
 * message it sent along with it, or — when no response came back at all — the
 * network error alone, since there is no status to quote.
 */
export function failureDetail(outcome: { status: number | null; detail: string }): string {
  const detail = (outcome.detail || '').trim()
  if (outcome.status === null) return detail || 'network error'
  const status = `HTTP ${outcome.status}`
  return detail && detail !== status ? `${status} — ${detail}` : status
}

/**
 * Fills `{resource}` and `{detail}` in a translated sentence. Placeholders are
 * replaced once each, so a detail that happens to contain `{resource}` cannot
 * expand a second time.
 */
export function formatReadFailure(template: string, resource: string, detail: string): string {
  return template
    .replace('{resource}', resource)
    .replace('{detail}', detail)
}

/**
 * The error message a non-ok response carries, when it carries one: the API
 * answers `{"error": "…"}` on its refusals. A body that is not that shape adds
 * nothing the status does not already say, so it is dropped.
 */
async function messageFrom(res: Response): Promise<string> {
  try {
    const body = await res.json()
    return body && typeof body.error === 'string' ? body.error.trim() : ''
  } catch {
    return ''
  }
}

/**
 * Reads a JSON resource and reports which of the three outcomes happened. It
 * never throws: a caller that only wants the body checks `kind === 'ok'`, and
 * the failure stays available for whoever shows it.
 */
export async function readJson<T>(url: string, init?: RequestInit): Promise<ReadOutcome<T>> {
  let res: Response
  try {
    res = await fetch(url, init)
  } catch (err: any) {
    return { kind: 'failed', status: null, detail: err?.message || String(err) }
  }
  if (res.status === 401) return { kind: 'silent', status: 401 }
  if (!res.ok) {
    return { kind: 'failed', status: res.status, detail: await messageFrom(res) }
  }
  try {
    return { kind: 'ok', data: (await res.json()) as T }
  } catch (err: any) {
    // A 200 whose body is not the JSON we asked for is a failed read too: the
    // proxy that answers an HTML error page with a 200 used to look like an
    // empty deployment exactly like a 500 did.
    return { kind: 'failed', status: res.status, detail: err?.message || 'invalid JSON' }
  }
}

/**
 * The failing reads after this outcome: a resource that came back drops out of
 * the list, one that failed joins it with what the server answered. The list
 * keeps its order so the banner does not reshuffle as reads recover.
 */
export function withOutcome<T>(
  failures: ReadFailure[],
  resource: ReadResource,
  outcome: ReadOutcome<T>,
): ReadFailure[] {
  if (outcome.kind === 'silent') return failures
  const index = failures.findIndex(failure => failure.resource === resource)
  if (outcome.kind === 'ok') {
    return index === -1 ? failures : failures.filter((_, position) => position !== index)
  }
  const detail = failureDetail(outcome)
  if (index === -1) return [...failures, { resource, detail }]
  if (failures[index].detail === detail) return failures
  const next = failures.slice()
  next[index] = { resource, detail }
  return next
}

/** Whether any read the whole interface depends on is currently failing. */
export function coreFailures(failures: ReadFailure[]): ReadFailure[] {
  return failures.filter(failure => CORE_READS.includes(failure.resource))
}
