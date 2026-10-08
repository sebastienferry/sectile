import type { Project, ProjectTrackerRef, TrackerColumn } from '../types'

/**
 * The stage→columns mapping of a project, per tracker it selects (#741). A
 * tracker carries the default an admin sets in Administration; a project may
 * map the stages onto the same columns its own way, from its settings, a member
 * as well as an admin. An empty project mapping reads the tracker's.
 */

export type StageMapping = Record<string, string[]>

/** The project's own mapping per tracker id; null reads the tracker's. */
export type OwnStageMappings = Record<string, StageMapping | null>

/** The own mappings a saved project holds, by tracker id. */
export function ownStageMappings(trackers: ProjectTrackerRef[] | undefined): OwnStageMappings {
  const own: OwnStageMappings = {}
  for (const ref of trackers ?? []) {
    own[ref.trackerId] = ref.ownStageColumns && ref.stageColumns ? cleanStageMapping(ref.stageColumns) : null
  }
  return own
}

/** A mapping without its empty stages. */
export function cleanStageMapping(mapping: StageMapping | null | undefined): StageMapping {
  const out: StageMapping = {}
  for (const [stage, columns] of Object.entries(mapping ?? {})) {
    if (columns && columns.length > 0) out[stage] = [...columns]
  }
  return out
}

/** Whether two mappings assign the same columns, in the same order, to the same stages. */
export function sameStageMapping(a: StageMapping | null | undefined, b: StageMapping | null | undefined): boolean {
  const left = cleanStageMapping(a)
  const right = cleanStageMapping(b)
  const stages = new Set([...Object.keys(left), ...Object.keys(right)])
  for (const stage of stages) {
    if ((left[stage] || []).join('\n') !== (right[stage] || []).join('\n')) return false
  }
  return true
}

/**
 * The project's own mapping after an edit in the stage editor, which starts
 * from the mapping shown, the tracker's while the project inherits it. A
 * mapping left with no column reads the tracker's, as the server does.
 */
export function editedStageMapping(mapping: StageMapping): StageMapping | null {
  const cleaned = cleanStageMapping(mapping)
  return Object.keys(cleaned).length > 0 ? cleaned : null
}

/**
 * The trackerStageColumns a save sends: the mappings of the selected trackers
 * that changed since the project was read, an inherited one as {} so that the
 * server goes back to the tracker's. Undefined when none changed.
 */
export function stageMappingPayload(
  edited: OwnStageMappings,
  original: OwnStageMappings,
  selected: string[]
): Record<string, StageMapping> | undefined {
  const payload: Record<string, StageMapping> = {}
  for (const id of selected) {
    if (!(id in edited)) continue
    const now = edited[id]
    const before = original[id] ?? null
    if (now === null && before === null) continue
    if (now !== null && before !== null && sameStageMapping(now, before)) continue
    payload[id] = now === null ? {} : cleanStageMapping(now)
  }
  return Object.keys(payload).length > 0 ? payload : undefined
}

/**
 * The board a ticket of a tracker reads in a project: that tracker's columns
 * and the mapping that applies in the project. The project's own fields, its
 * default tracker's, when the tracker is not one it lists.
 */
export function trackerBoard(
  project: Project | null | undefined,
  trackerId: string | undefined
): { trackerColumns: TrackerColumn[]; stageColumns: StageMapping } {
  const ref = trackerId ? project?.trackers?.find(item => item.trackerId === trackerId) : undefined
  if (ref) return { trackerColumns: ref.trackerColumns || [], stageColumns: ref.stageColumns || {} }
  return { trackerColumns: project?.trackerColumns || [], stageColumns: project?.stageColumns || {} }
}

/** The project as a ticket of the tracker reads it: its columns and mapping are that tracker's. */
export function projectForTracker<T extends Project>(project: T, trackerId: string | undefined): T
export function projectForTracker<T extends Project>(project: T | null | undefined, trackerId: string | undefined): T | null | undefined
export function projectForTracker<T extends Project>(project: T | null | undefined, trackerId: string | undefined): T | null | undefined {
  if (!project || !trackerId) return project
  if (!project.trackers?.some(item => item.trackerId === trackerId)) return project
  return { ...project, ...trackerBoard(project, trackerId) }
}
