import type { Priority, PriorityMapping, PriorityMappingOption } from '../types'
import { PRIORITY_LEVELS } from './priority.ts'

/**
 * The priority mapping of a Jira project (#679): how each option of its
 * scheme maps to Sectile's levels. These helpers mirror the server's
 * `internal/models/prioritymapping.go`, so the table shows what a write will
 * actually send.
 */

const sure = (o: PriorityMappingOption) => !o.guessed

/** The sure options of a level, in the scheme's order. */
export function sureOptionsOf(mapping: PriorityMapping | undefined, level: Priority): PriorityMappingOption[] {
  return (mapping?.options ?? []).filter(o => o.level === level && sure(o))
}

/**
 * The option a write of the level sends: the preferred one when it is sure,
 * else the most urgent sure option. Undefined when the level cannot be
 * written.
 */
export function optionFor(mapping: PriorityMapping | undefined, level: Priority): PriorityMappingOption | undefined {
  const candidates = sureOptionsOf(mapping, level)
  const preferred = mapping?.preferred?.[level]
  return candidates.find(o => o.id === preferred) ?? candidates[0]
}

/** The levels no sure option carries, which a write is refused on. */
export function unwritableLevels(mapping: PriorityMapping | undefined): Priority[] {
  if (!mapping?.options?.length) return []
  return PRIORITY_LEVELS.filter(level => !optionFor(mapping, level))
}

/** A line set to a level by a person: sure, and never changed by discovery. */
export function setLevel(mapping: PriorityMapping, id: string, level: Priority): PriorityMapping {
  return {
    ...mapping,
    options: mapping.options?.map(o => (o.id === id ? { ...o, level, guessed: false, manual: true } : o)),
  }
}

/** A guessed line confirmed at the level it already has. */
export function confirmLine(mapping: PriorityMapping, id: string): PriorityMapping {
  const line = mapping.options?.find(o => o.id === id)
  return line ? setLevel(mapping, id, line.level) : mapping
}

/** The option a level sends when several sure options carry it. */
export function setPreferred(mapping: PriorityMapping, level: Priority, id: string): PriorityMapping {
  return { ...mapping, preferred: { ...mapping.preferred, [level]: id } }
}

/** What a bulk priority change did, ticket by ticket. */
export interface BulkOutcome {
  key: string
  written: boolean
}

/**
 * Splits a bulk change's outcomes into the count written and the keys refused:
 * one refused ticket must not hide the others, nor stop them.
 */
export function partitionBulkOutcomes(outcomes: BulkOutcome[]): { written: number; refused: string[] } {
  const refused = outcomes.filter(o => !o.written).map(o => o.key)
  return { written: outcomes.length - refused.length, refused }
}
