import type { ViewMode } from '../types'

/**
 * Views that filter their own rows on complete data, so the header search
 * must not reach the server for them (#636). The roadmap is searched by epic,
 * not by ticket, and the timeline counts every ticket of a sprint: a server
 * search would empty sprints, counters and epics instead of narrowing them.
 */
export const SELF_FILTERING_VIEWS: ReadonlySet<ViewMode> = new Set<ViewMode>(['roadmap', 'timeline'])

/** True when the ticket query of the view carries the header search. */
export const sendsServerSearch = (view: ViewMode): boolean => !SELF_FILTERING_VIEWS.has(view)
