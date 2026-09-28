/**
 * Dropping remembered filters whose value left the board (#581).
 *
 * A remembered sprint can close and a team can be renamed: kept, such a filter
 * would empty the board behind a selector that shows nothing selected. The
 * values it is checked against must be those of the project or view the filter
 * belongs to. Right after a switch the board still holds the previous scope's
 * values, and checking against them dropped, and stored as dropped, every
 * filter the new scope had just restored.
 */

/** The filters that are dropped once their value is no longer on the board. */
export interface PrunableFilters {
  sprint: string | null
  team: string | null
  assignee: string | null
}

/** The values the board carries, with the scope they were read for. */
export interface ScopedFacets {
  /** The filterScopeKey of the project or view, null before the first read. */
  scope: string | null
  sprints: string[]
  teams: string[]
  assignees: string[]
}

const leftTheBoard = (value: string | null, values: string[]): boolean =>
  !!value && values.length > 0 && !values.includes(value)

/**
 * The filters to drop because their value left the board of their own scope.
 * Facets read for another scope, or not read yet, drop nothing; an empty list
 * drops nothing either, since a tracker that feeds no value is not a value
 * that disappeared. "Unassigned" is a filter value of its own and never drops.
 */
export const staleFilters = (
  filters: PrunableFilters,
  facets: ScopedFacets,
  scope: string,
  unassignedValue: string,
): Array<keyof PrunableFilters> => {
  if (facets.scope !== scope) return []
  const stale: Array<keyof PrunableFilters> = []
  if (leftTheBoard(filters.sprint, facets.sprints)) stale.push('sprint')
  if (leftTheBoard(filters.team, facets.teams)) stale.push('team')
  if (filters.assignee !== unassignedValue && leftTheBoard(filters.assignee, facets.assignees)) stale.push('assignee')
  return stale
}
