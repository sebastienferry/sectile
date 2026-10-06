import type { EpicAxisField, EpicFieldCandidate } from '../types'

/**
 * The Jira custom fields of the epic priority and quarter (#680). Nothing
 * here knows a field: the candidates come from an epic's edit screen, and the
 * maps are what the server deduced and the person corrected.
 */

export type EpicFieldAxis = 'priority' | 'quarter'

export const EPIC_PRIORITY_VALUES = ['p0', 'p1', 'p2', 'p3'] as const

/** One option a map line can pick, its path and the label a person reads. */
export interface EpicFieldChoice {
  path: string
  label: string
}

/** The options of a field a line can pick: a cascade lists its leaves. */
export function optionChoices(candidate: EpicFieldCandidate | undefined): EpicFieldChoice[] {
  if (!candidate) return []
  if (candidate.kind !== 'cascade') {
    return candidate.options.map(o => ({ path: o.id, label: o.value }))
  }
  return candidate.options.flatMap(parent =>
    (parent.children ?? []).map(child => ({ path: `${parent.id}/${child.id}`, label: `${parent.value} / ${child.value}` })),
  )
}

/** A fresh mapping of an axis to a candidate, its map prefilled by the deductions. */
export function pickField(candidate: EpicFieldCandidate, axis: EpicFieldAxis): EpicAxisField {
  return {
    id: candidate.id,
    name: candidate.name,
    kind: candidate.kind,
    options: { ...(candidate.deduced?.[axis] ?? {}) },
  }
}

/** Sets or clears (empty path) the option of one axis value. */
export function setOption(field: EpicAxisField, value: string, path: string): EpicAxisField {
  const options = { ...(field.options ?? {}) }
  if (path) options[value] = path
  else delete options[value]
  return { ...field, options }
}

/** The quarter values a map shows: those it maps, then the one being added, sorted. */
export function quarterRows(field: EpicAxisField | undefined, extra: string[] = []): string[] {
  const values = new Set([...Object.keys(field?.options ?? {}), ...extra])
  return [...values].sort()
}

/** "2026-Q4" from "2026 q4", "2026-q4" or "2026.Q4"; null when it is not a quarter. */
export function normalizeQuarterValue(input: string): string | null {
  const m = /^(\d{4})[.\- ]q([1-4])$/.exec(input.trim().toLowerCase())
  return m ? `${m[1]}-Q${m[2]}` : null
}

/** The label of a stored path, the path itself when the field was not read. */
export function choiceLabel(choices: EpicFieldChoice[], path: string | undefined): string {
  if (!path) return ''
  return choices.find(c => c.path === path)?.label ?? path
}
