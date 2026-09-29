import type { EpicPriority, Priority } from '../types'
import type { EpicRow } from './roadmap.ts'

/**
 * An epic's own priority and quarter (#627): what the roadmap reads, sorts
 * and filters on, and what the seeding reads from the epic titles.
 *
 * The server normalizes the same forms (internal/db/macroaxes.go), so a value
 * this module accepts is never refused on save.
 */

export const EPIC_PRIORITIES: EpicPriority[] = ['p0', 'p1', 'p2', 'p3']

/** The existing four-level scale lends its colours to P0 to P3. */
export const EPIC_PRIORITY_LEVEL: Record<EpicPriority, Priority> = {
  p0: 'urgent',
  p1: 'high',
  p2: 'medium',
  p3: 'low',
}

/** "p1" -> "P1", the way a badge reads it. */
export const epicPriorityLabel = (priority: EpicPriority): string => priority.toUpperCase()

const QUARTER_INPUT = /^(\d{4})[.\- ]q([1-4])$/i

/**
 * Normalizes a typed quarter: "2026-Q4", "2026.q4" and "2026 Q4" give
 * "2026-Q4", an empty field gives "" (clear), anything else gives null.
 */
export const normalizeQuarter = (input: string): string | null => {
  const clean = input.trim()
  if (!clean) return ''
  const m = QUARTER_INPUT.exec(clean)
  return m ? `${m[1]}-Q${m[2]}` : null
}

// A marker counts only on a word boundary, so that a key such as "PEP1" or a
// version such as "1.2026Q4" is not read as a priority or a quarter.
const TITLE_PRIORITY = /(?:^|[^\p{L}\p{N}])\[?P([0-3])\]?(?=$|[^\p{L}\p{N}])/iu
const TITLE_QUARTER = /(?:^|[^\p{L}\p{N}.])(\d{4})[.\- ]Q([1-4])(?=$|[^\p{L}\p{N}])/iu

/** The priority and the quarter a title carries, the first candidate of each. */
export const titleAxes = (title: string): { priority: EpicPriority | ''; quarter: string } => {
  const p = TITLE_PRIORITY.exec(title)
  const q = TITLE_QUARTER.exec(title)
  return {
    priority: p ? (`p${p[1]}` as EpicPriority) : '',
    quarter: q ? `${q[1]}-Q${q[2]}` : '',
  }
}

/** What the seeding proposes for one epic: only the axes it has no value on. */
export interface SeedLine {
  key: string
  title: string
  priority?: EpicPriority
  quarter?: string
}

/**
 * The seeding proposals: for each epic, the values its title carries on the
 * axes it has no value on yet. An epic with nothing to propose is left out.
 */
export const seedProposals = (rows: EpicRow[]): SeedLine[] => {
  const out: SeedLine[] = []
  rows.forEach(row => {
    const found = titleAxes(row.title)
    const line: SeedLine = { key: row.key, title: row.title }
    if (!row.priority && found.priority) line.priority = found.priority
    if (!row.quarter && found.quarter) line.quarter = found.quarter
    if (line.priority || line.quarter) out.push(line)
  })
  return out
}

export type PrioritySort = 'backlog' | 'priority-desc' | 'priority-asc'

const RANK: Record<EpicPriority, number> = { p0: 0, p1: 1, p2: 2, p3: 3 }

/**
 * Sorts the epics on their priority, keeping the backlog order between equal
 * ones. Epics without a priority come last in both directions: "not decided"
 * is neither the highest nor the lowest.
 */
export const sortByPriority = (rows: EpicRow[], sort: PrioritySort): EpicRow[] => {
  if (sort === 'backlog') return rows
  const dir = sort === 'priority-desc' ? 1 : -1
  return rows
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const pa = a.row.priority
      const pb = b.row.priority
      if (pa && pb && pa !== pb) return (RANK[pa] - RANK[pb]) * dir
      if (pa && !pb) return -1
      if (!pa && pb) return 1
      return a.index - b.index
    })
    .map(entry => entry.row)
}

/** A priority, "none" for epics without one, or null for no filter. */
export type PriorityFilter = EpicPriority | 'none' | null

export const matchesPriority = (row: EpicRow, filter: PriorityFilter): boolean => {
  if (!filter) return true
  if (filter === 'none') return !row.priority
  return row.priority === filter
}
