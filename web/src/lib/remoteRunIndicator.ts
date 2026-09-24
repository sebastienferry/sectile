import type { TaskActivity } from '../types'
import { isSilentSummary } from '../../../shared/runStates.ts'

/** Remote executions are the only activities this indicator represents. */
const REMOTE_RUN_SKILL = 'remote_run'
/** Only a run started by the user's own agent can be stopped. */
const AGENT_OWNED_ACTION = 'Agent-owned remote execution'
/**
 * A run a connected client created has no agent to stop; it can only be closed,
 * the way a disconnection would close it (#319).
 */
const CLIENT_ACTION = 'Remote skill execution'
/** A cancellation stays visible long enough for the user to see its effect. */
export const CANCELED_VISIBILITY_MS = 30_000

export type RunIndicatorState = 'waiting' | 'silent' | 'running' | 'queued' | 'canceled'

export interface RunIndicator {
  state: RunIndicatorState
  runs: TaskActivity[]
  cancelableRunIds: string[]
  /** Client-created runs the viewer may close: their own, or any for an admin. */
  closableRunIds: string[]
  count: number
  /** When the displayed wait started, for the state 'waiting' alone. */
  waitingSince?: string
}

/**
 * A waiting run is a running run that reported it is blocked on the user, so
 * the mark is only ever read off a run still executing: a stale timestamp on a
 * finished run can never make it look alive.
 */
function isWaiting(activity: TaskActivity): boolean {
  return activity.status === 'running' && !!activity.waitingSince
}

function isVisibleCancellation(activity: TaskActivity, now: number): boolean {
  if (activity.status !== 'canceled') return false
  // Without an end timestamp the icon would never clear.
  if (!activity.completedAt) return false
  const endedAt = Date.parse(activity.completedAt)
  if (Number.isNaN(endedAt)) return false
  return now - endedAt <= CANCELED_VISIBILITY_MS
}

/** Who is looking at the badge, which decides what it may offer to close. */
export interface RunViewer {
  userId?: string
  role?: string
}

/**
 * A silent run is a running run whose session stopped speaking long enough for
 * the server to write it down. Its summary keeps the sentence, so it stays
 * silent until it ends.
 */
function isSilent(activity: TaskActivity): boolean {
  return activity.status === 'running' && !activity.waitingSince && isSilentSummary(activity.summary)
}

/**
 * Reduces a task's remote runs to a single displayable state:
 * waiting, then silent, then running, then queued, then recently canceled.
 *
 * Waiting outranks running because it is the only state that asks something of
 * the user: a task with one blocked run and one working run is a task to open.
 * Silent comes next: it asks nothing yet, but it is the run to look at first.
 */
export function deriveRunIndicator(
  activities: TaskActivity[],
  taskId: string,
  now: number = Date.now(),
  viewer?: RunViewer,
): RunIndicator | null {
  const runs = activities.filter(activity => activity.taskId === taskId && activity.skillId === REMOTE_RUN_SKILL)
  const waiting = runs.filter(isWaiting)
  // Running keeps every executing run, waiting ones included: the count and the
  // accessible label report the whole picture, not only the blocked part.
  const running = runs.filter(run => run.status === 'running')
  const silent = running.filter(isSilent)
  const queued = runs.filter(run => run.status === 'queued')
  const canceled = runs.filter(run => isVisibleCancellation(run, now))

  const selected: [RunIndicatorState, TaskActivity[]] | null =
    waiting.length > 0 ? ['waiting', running]
      : silent.length > 0 ? ['silent', running]
        : running.length > 0 ? ['running', running]
        : queued.length > 0 ? ['queued', queued]
          : canceled.length > 0 ? ['canceled', canceled]
              : null
  if (!selected) return null

  const [state, selectedRuns] = selected
  // An already canceled run offers no action. Waiting changes nothing here: a
  // blocked run is still a live run, and stopping it is still its owner's call.
  const cancelableRunIds = state === 'canceled'
    ? []
    : selectedRuns.filter(run => run.action === AGENT_OWNED_ACTION).map(run => run.id)
  // The server decides in the end; this only avoids offering a button that
  // would be refused. An ownerless run is an admin's, as on the server.
  const isAdmin = viewer?.role === 'admin'
  const closableRunIds = state === 'canceled'
    ? []
    : selectedRuns
      .filter(run => run.status === 'running' && run.action === CLIENT_ACTION)
      .filter(run => isAdmin || (!!viewer?.userId && run.userId === viewer.userId))
      .map(run => run.id)

  // The earliest wait is the one reported: it is the longest, and the one the
  // user has been keeping waiting.
  const waitingSince = waiting
    .map(run => run.waitingSince as string)
    .filter(value => !Number.isNaN(Date.parse(value)))
    .sort()[0]

  return { state, runs: selectedRuns, cancelableRunIds, closableRunIds, count: selectedRuns.length, waitingSince }
}

/**
 * The tasks a remote run is currently working on, as a set of task ids.
 *
 * It answers about many tasks what `deriveRunIndicator` answers about one, and
 * reads the same runs, so a filter built on it and the badge drawn from the
 * other cannot disagree. It is a pass of its own rather than a loop over the
 * indicator because a filter asks about every task at once: one scan of the
 * activities, then a constant-time membership test per rendered card.
 *
 * A waiting run needs no case: it is a running run carrying a mark, and a
 * blocked run is still a live one. A recently canceled run is the difference
 * with the indicator, which keeps it visible so the user sees the cancellation
 * land on the card being looked at. Here the work has stopped, so the task is
 * not active any more — and that is why this takes no `now`.
 */
export function activeTaskIds(activities: TaskActivity[]): Set<string> {
  const active = new Set<string>()
  for (const activity of activities) {
    if (activity.skillId !== REMOTE_RUN_SKILL) continue
    if (activity.status === 'running' || activity.status === 'queued') active.add(activity.taskId)
  }
  return active
}
