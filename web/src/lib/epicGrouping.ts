import type { EpicPriority } from '../types'
import { EPIC_PRIORITIES, normalizeQuarter } from './epicAxes.ts'
import type { HorizonTab } from './roadmap.ts'

/**
 * The sections a roadmap tab is split into (#628): one per value of the
 * epic's priority or quarter (#627), then one for the epics without a value.
 *
 * Kept out of `RoadmapView` so that the order, the always-present sections and
 * the drop plan can be tested without a DOM.
 */

/** What the sections are built on; `none` is the flat list. */
export type EpicGroupAxis = 'none' | 'priority' | 'quarter'

export const EPIC_GROUP_AXES: EpicGroupAxis[] = ['none', 'priority', 'quarter']

/** The axes a section can be built on. */
export type EpicSectionAxis = Exclude<EpicGroupAxis, 'none'>

/** The fields of an epic the sections read. */
export interface GroupableEpic {
  key: string
  priority?: EpicPriority | ''
  quarter?: string
}

export interface EpicSection<R> {
  /**
   * The folding key, `<axis>:<value>` or `<axis>:none`: the same value folds
   * the same section on every tab.
   */
  id: string
  axis: EpicSectionAxis
  /** The value a drop on this section sets; empty for the no-value section. */
  value: string
  rows: R[]
}

/**
 * The Hidden tab stays a flat list: nothing set aside is prioritized, which is
 * also why the condensed rows skip it.
 */
export const isGroupableTab = (tab: HorizonTab): boolean => tab !== 'hidden'

/** The quarter of a date, read in the browser's local time: "2026-Q3". */
export const quarterOf = (date: Date): string =>
  `${date.getFullYear()}-Q${Math.floor(date.getMonth() / 3) + 1}`

/** The quarter of the date and the ones after it, `count` in all. */
export const comingQuarters = (date: Date, count = 4): string[] => {
  const out: string[] = []
  let year = date.getFullYear()
  let q = Math.floor(date.getMonth() / 3) + 1
  for (let i = 0; i < count; i++) {
    out.push(`${year}-Q${q}`)
    q++
    if (q > 4) {
      q = 1
      year++
    }
  }
  return out
}

/**
 * The epic's value on the axis, empty when it has none. A quarter that does
 * not read as YYYY-Qn counts as none: the server normalizes it, so this only
 * guards against a value written by hand elsewhere.
 */
export const axisValueOf = (row: GroupableEpic, axis: EpicSectionAxis): string => {
  if (axis === 'priority') return row.priority || ''
  return normalizeQuarter(row.quarter || '') || ''
}

/**
 * Splits the rows into sections, keeping their order inside each one, so the
 * sort applied before grouping is the order within a section.
 *
 * - priority: P0 to P3 then "no priority", all always present, since each one
 *   is a drop target even when empty;
 * - quarter: the quarters the rows use, plus the current one and the next
 *   three, chronologically, then "no quarter", always present. A past quarter
 *   shows only while an epic carries it.
 */
export const groupEpics = <R extends GroupableEpic>(rows: R[], axis: EpicSectionAxis, today: Date): EpicSection<R>[] => {
  const values =
    axis === 'priority'
      ? [...EPIC_PRIORITIES]
      : [...new Set([...comingQuarters(today), ...rows.map(row => axisValueOf(row, 'quarter')).filter(Boolean)])].sort()
  const byValue = new Map<string, R[]>(values.map(value => [value, []]))
  const none: R[] = []
  rows.forEach(row => {
    const value = axisValueOf(row, axis)
    const bucket = value ? byValue.get(value) : undefined
    if (bucket) bucket.push(row)
    else none.push(row)
  })
  return [
    ...values.map(value => ({ id: `${axis}:${value}`, axis, value, rows: byValue.get(value) || [] })),
    { id: `${axis}:none`, axis, value: '', rows: none },
  ]
}

/**
 * What a drop of `keys` on a section of value `target` writes: the epics not
 * already at the target, in the order given. An unknown key is ignored.
 */
export const planAxisDrop = (
  rows: GroupableEpic[],
  keys: readonly string[],
  axis: EpicSectionAxis,
  target: string
): { toSave: string[]; skipped: string[] } => {
  const byKey = new Map(rows.map(row => [row.key, row]))
  const toSave: string[] = []
  const skipped: string[] = []
  keys.forEach(key => {
    const row = byKey.get(key)
    if (!row) return
    if (axisValueOf(row, axis) === target) skipped.push(key)
    else toSave.push(key)
  })
  return { toSave, skipped }
}

/** The drag payload of roadmap epics: their keys, as JSON. */
export const DRAG_EPIC_KEYS = 'application/x-sectile-epic-keys'

/** Reads a drag payload; anything but an array of strings carries nothing. */
export const parseDraggedEpicKeys = (raw: string): string[] => {
  try {
    const value: unknown = JSON.parse(raw)
    return Array.isArray(value) ? value.filter((key): key is string => typeof key === 'string' && key !== '') : []
  } catch {
    return []
  }
}
