import type { ProjectTrackerRef, Task, TrackerSummary } from '../types'

/**
 * What a project selects its tickets with (#741): an ordered list of trackers,
 * the one its new tickets go to, and an optional label. The first tracker is
 * the default unless another is named; without a label the project shows
 * every ticket of its trackers.
 */

/** A tracker the project settings can offer, the project's own local board included. */
export interface TrackerOption {
  id: string
  identity: string
  provider: string
  name: string
  scope: string
  /** The project's own local board: never listed by GET /api/trackers. */
  local: boolean
}

/** A local board's identity: "local||<project id>". */
export function isLocalIdentity(identity: string | undefined): boolean {
  return (identity || '').startsWith('local|')
}

/**
 * The trackers the settings offer: every tracker a member can pick, plus the
 * project's own entries the list does not hold, which are its local board. A
 * save that forgot them would unlink the board its tickets live on.
 */
export function trackerOptions(summaries: TrackerSummary[], selected: ProjectTrackerRef[] = []): TrackerOption[] {
  const options: TrackerOption[] = summaries
    .filter(summary => summary.provider !== 'local')
    .map(summary => ({
      id: summary.id,
      identity: summary.identity,
      provider: summary.provider,
      name: summary.name,
      scope: summary.scope,
      local: false,
    }))
  for (const ref of selected) {
    if (options.some(option => option.id === ref.trackerId)) continue
    options.push({
      id: ref.trackerId,
      identity: ref.identity,
      provider: isLocalIdentity(ref.identity) ? 'local' : ref.identity.split('|')[0] || '',
      name: '',
      scope: ref.identity.split('|').slice(2).join('|'),
      local: isLocalIdentity(ref.identity),
    })
  }
  return options
}

/** The tracker ids a saved project selects, in order. */
export function selectedTrackerIds(trackers: ProjectTrackerRef[] | undefined): string[] {
  return (trackers ?? []).map(ref => ref.trackerId).filter(Boolean)
}

/** Adds a tracker at the end of the selection, once. */
export function addTracker(selected: string[], id: string): string[] {
  return !id || selected.includes(id) ? selected : [...selected, id]
}

export function removeTracker(selected: string[], id: string): string[] {
  return selected.filter(item => item !== id)
}

/** Moves the tracker at index by delta (-1 up, +1 down); out of range leaves the list as it is. */
export function moveTracker(selected: string[], index: number, delta: number): string[] {
  const target = index + delta
  if (index < 0 || index >= selected.length || target < 0 || target >= selected.length) return selected
  const next = [...selected]
  ;[next[index], next[target]] = [next[target], next[index]]
  return next
}

/**
 * Where the project's new tickets go: the named tracker while the project
 * selects it, else the first one, as the server falls back.
 */
export function effectiveDefaultTracker(selected: string[], defaultTrackerId: string | undefined): string {
  return defaultTrackerId && selected.includes(defaultTrackerId) ? defaultTrackerId : selected[0] || ''
}

export interface ProjectSelection {
  trackers?: ProjectTrackerRef[]
  defaultTrackerId?: string
  label: string
}

/**
 * The part of the project payload that names its trackers, its default and its
 * label. The label is always sent, trimmed: an empty one shows every ticket of
 * the trackers. The trackers are sent when they changed. A new project must
 * pick at least one (see missingTrackerReason): the server refuses a creation
 * naming none, and never creates a tracker, not even a local board.
 */
export function projectSelectionPayload(
  selected: string[],
  defaultTrackerId: string,
  label: string,
  options: TrackerOption[],
  original?: ProjectTrackerRef[]
): ProjectSelection {
  const payload: ProjectSelection = { label: label.trim() }
  if (selected.length === 0) return payload
  const unchanged = original !== undefined && selectedTrackerIds(original).join('\n') === selected.join('\n')
  if (!unchanged) {
    payload.trackers = selected.map(id => ({
      trackerId: id,
      identity: options.find(option => option.id === id)?.identity || '',
    }))
  }
  payload.defaultTrackerId = effectiveDefaultTracker(selected, defaultTrackerId)
  return payload
}

/**
 * Why the project cannot be saved for want of a tracker (#741), or null when
 * it can: a new project must select one an admin recorded ("pick"), and when
 * none is recorded at all there is nothing to pick ("noneRecorded"). A saved
 * project keeps the trackers it has, its own local board included.
 */
export function missingTrackerReason(isNew: boolean, selected: string[], options: TrackerOption[]): 'pick' | 'noneRecorded' | null {
  if (!isNew || selected.length > 0) return null
  return options.some(option => !option.local) ? 'pick' : 'noneRecorded'
}

/**
 * Whether a ticket belongs to a remote tracker (#741). Its projects follow
 * from its tracker and its labels, so it is not moved to a project: it is
 * labelled into one, from its tracker's backlog. Only a local ticket moves.
 */
export function isTrackerTicket(task: Pick<Task, 'trackerId' | 'source'> | null | undefined): boolean {
  return Boolean(task?.trackerId) && Boolean(task?.source) && task?.source !== 'local'
}
