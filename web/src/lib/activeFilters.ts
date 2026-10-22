/**
 * The active filters of a view, as the chip row under its filter bar shows them
 * (#866). Each view builds the list from the state it already holds, global or
 * local; this module only gives the list one shape and the operations every
 * view needs on it.
 */
export interface ActiveFilter {
  /** Stable key: the dimension, plus the value for a multi-select picker. Two
   *  chips may share a label (a sprint and a label named alike), never an id. */
  id: string
  /** Already localised. */
  label: string
  clear: () => void
}

/** Clears every listed filter once, in order. */
export function clearAll(filters: ActiveFilter[]): void {
  filters.forEach(filter => filter.clear())
}

/** One entry per chosen value of a multi-select picker, so each value is
 *  cleared on its own. */
export function multiValueFilters(
  dimension: string,
  values: string[],
  label: (value: string) => string,
  remove: (value: string) => void
): ActiveFilter[] {
  return values.map(value => ({ id: `${dimension}:${value}`, label: label(value), clear: () => remove(value) }))
}

/** How many pickers are set, for the badge of the "Filters" button: one per
 *  dimension, so three ticked statuses still count as one filter. */
export function countSetPickers(values: Array<string | string[] | boolean | null | undefined>): number {
  return values.filter(value => (Array.isArray(value) ? value.length > 0 : Boolean(value))).length
}
