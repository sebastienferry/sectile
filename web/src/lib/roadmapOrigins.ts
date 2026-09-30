import { resolveStorage, type StorageLike } from './roadmapDisplayMode.ts'

/**
 * Which Jira projects the roadmap shows the epics of (#632).
 *
 * A Jira project can declare roadmap projects, whose epics the roadmap reads
 * beside its own. A declared project can carry hundreds of epics, so the
 * roadmap opens on the project's own key and the viewer ticks the others. The
 * choice is a way of reading the roadmap, like its tab, and is kept per browser
 * and per project: a key of one project means nothing in another.
 *
 * An epic's origin is the key prefix of its key, as the server computes it.
 */

/** The origin a row needs: its key and, when the server said so, its origin. */
export interface OriginRow {
  key: string
  meta?: { origin?: string }
}

/** The project fields the origins are read from. */
export interface OriginProject {
  issueTracker?: string
  jiraProject?: string
  roadmapProjects?: string[]
}

/** One entry of the origin selection. */
export interface OfferedOrigin {
  key: string
  count: number
  /** The project's own key, which an empty selection falls back to. */
  own: boolean
}

/** "DATA-12" reads "DATA"; a milestone ("M-4") or a key without a prefix has none. */
export const macroOrigin = (key: string): string => {
  const clean = key.trim().toUpperCase()
  if (/^M-\d+$/.test(clean)) return ''
  const dash = clean.indexOf('-')
  return dash > 0 ? clean.slice(0, dash) : ''
}

/** A row's origin: the server's when it gave one, the key's prefix otherwise. */
export const rowOrigin = (row: OriginRow): string => (row.meta?.origin || macroOrigin(row.key)).toUpperCase()

const ownKey = (project: OriginProject | null | undefined): string => (project?.jiraProject || '').trim().toUpperCase()

/**
 * The origins the selection offers: the own key first, then the declared keys
 * in their declared order, then any other origin the stored epics still carry,
 * alphabetically, so that a project removed from the declaration does not hide
 * the rows it left behind. A declared key with no epic is offered at zero.
 *
 * Nothing is offered on a project that is not a Jira project, or that declares
 * nothing and carries no epic of another project: there is nothing to choose.
 * Rows without an origin (milestones, local keys) count with the own key.
 *
 * The origins come from every row, and the counts from `counted`, the rows the
 * other filters let through: a filter changes a count, never the list.
 */
export function offeredOrigins(
  project: OriginProject | null | undefined,
  rows: OriginRow[],
  counted: OriginRow[] = rows
): OfferedOrigin[] {
  if (!project || project.issueTracker !== 'jira') return []
  const own = ownKey(project)
  const declared = (project.roadmapProjects || []).map(k => k.trim().toUpperCase()).filter(k => k && k !== own)
  const counts = originCounts(counted, own)
  const carried = [...originCounts(rows, own).keys()].filter(k => k !== own && !declared.includes(k)).sort()
  if (declared.length === 0 && carried.length === 0) return []
  const keys = [own, ...declared.filter((k, i) => declared.indexOf(k) === i), ...carried].filter(Boolean)
  return keys.map(key => ({ key, count: counts.get(key) || 0, own: key === own }))
}

/** How many rows each origin holds; a row without an origin counts with the own key. */
export function originCounts(rows: OriginRow[], own: string): Map<string, number> {
  const out = new Map<string, number>()
  for (const row of rows) {
    const origin = rowOrigin(row) || own
    out.set(origin, (out.get(origin) || 0) + 1)
  }
  return out
}

/**
 * The selection in force: the stored one cleaned of every origin no longer
 * offered, and the own key alone when nothing is left. Not showing anything is
 * not a way of reading a roadmap.
 */
export function normalizeOriginSelection(selection: string[], offered: OfferedOrigin[], own: string): string[] {
  const keys = offered.map(o => o.key)
  const kept = selection.map(k => k.trim().toUpperCase()).filter((k, i, all) => keys.includes(k) && all.indexOf(k) === i)
  if (kept.length > 0) return kept
  return own ? [own.toUpperCase()] : []
}

/** Whether a selection is the default one: the own key alone. */
export const isDefaultOriginSelection = (selection: string[], own: string): boolean =>
  selection.length === 1 && selection[0] === own.toUpperCase()

/**
 * Whether a row passes the selection. With nothing offered every row passes:
 * the filter only exists once there is something to choose.
 */
export function matchesOrigins(row: OriginRow, selection: string[], own: string): boolean {
  if (selection.length === 0) return true
  return selection.includes(rowOrigin(row) || own.toUpperCase())
}

/** The key the selection of one project is kept under. */
export const roadmapOriginsStorageKey = (projectId: string): string => `sectile_roadmap_origins:${projectId}`

/** The stored selection of a project, empty when none or unreadable. */
export function loadOriginSelection(projectId: string, customStorage?: StorageLike): string[] {
  const storage = resolveStorage(customStorage)
  if (!storage || !projectId) return []
  try {
    const parsed: unknown = JSON.parse(storage.getItem(roadmapOriginsStorageKey(projectId)) || '[]')
    return Array.isArray(parsed) ? parsed.filter((k): k is string => typeof k === 'string') : []
  } catch {
    return []
  }
}

export function saveOriginSelection(projectId: string, selection: string[], customStorage?: StorageLike): void {
  const storage = resolveStorage(customStorage)
  if (!storage || !projectId) return
  try {
    storage.setItem(roadmapOriginsStorageKey(projectId), JSON.stringify(selection))
  } catch {
    // Storage unavailable: the selection holds for the page, and no longer.
  }
}

/**
 * The roadmap projects a slicing line's story may be created in (#632): the
 * declared keys of a Jira project, none elsewhere. The story then stays in Jira
 * and is never imported.
 */
export const roadmapTargetOptions = (project: OriginProject | null | undefined): string[] => {
  if (!project || project.issueTracker !== 'jira') return []
  const own = ownKey(project)
  return (project.roadmapProjects || [])
    .map(k => k.trim().toUpperCase())
    .filter((k, i, all) => k && k !== own && all.indexOf(k) === i)
}

/** The prefix a target picker value carries when it names a roadmap project. */
export const TRACKER_TARGET_PREFIX = 'tracker:'

/** The line fields a target picker reads and writes. */
export interface TargetLine {
  targetProjectId?: string
  targetTrackerProject?: string
}

/**
 * The picker value of a line: "tracker:DATA" for a roadmap project, a project
 * id for another Sectile project, "" for the macro's own project.
 */
export function targetPickerValue(line: TargetLine, macroProjectId: string): string {
  const remote = (line.targetTrackerProject || '').trim().toUpperCase()
  if (remote) return TRACKER_TARGET_PREFIX + remote
  return line.targetProjectId && line.targetProjectId !== macroProjectId ? line.targetProjectId : ''
}

/** The line once the picker says `value`: one target at most, the other cleared. */
export function applyTargetPickerValue<T extends TargetLine>(line: T, value: string): T {
  if (value.startsWith(TRACKER_TARGET_PREFIX)) {
    return { ...line, targetTrackerProject: value.slice(TRACKER_TARGET_PREFIX.length), targetProjectId: undefined }
  }
  return { ...line, targetProjectId: value || undefined, targetTrackerProject: undefined }
}

/**
 * The selection that shows an epic of `origin` (#630 opening an epic from one
 * of its tickets): the current one when it already does, otherwise the current
 * one plus that origin. The own key stays in, a selection that showed it
 * before showing it still. Null when nothing has to change.
 */
export function selectionRevealing(selection: string[], own: string, origin: string): string[] | null {
  const key = origin.trim().toUpperCase()
  const ownKey = own.trim().toUpperCase()
  const current = selection.length > 0 ? selection.map(k => k.toUpperCase()) : ownKey ? [ownKey] : []
  if (!key || current.includes(key)) return null
  return [...current, key]
}
