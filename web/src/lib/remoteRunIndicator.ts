import type { TaskActivity } from '../types'

/** Remote executions are the only activities this indicator represents. */
const REMOTE_RUN_SKILL = 'remote_run'
/** Only a run started by the user's own agent can be stopped. */
const AGENT_OWNED_ACTION = 'Agent-owned remote execution'
/** A cancellation stays visible long enough for the user to see its effect. */
export const CANCELED_VISIBILITY_MS = 30_000

export type RunIndicatorState = 'running' | 'queued' | 'canceled'

export interface RunIndicator {
  state: RunIndicatorState
  runs: TaskActivity[]
  cancelableRunIds: string[]
  count: number
}

function isVisibleCancellation(activity: TaskActivity, now: number): boolean {
  if (activity.status !== 'canceled') return false
  // Without an end timestamp the icon would never clear.
  if (!activity.completedAt) return false
  const endedAt = Date.parse(activity.completedAt)
  if (Number.isNaN(endedAt)) return false
  return now - endedAt <= CANCELED_VISIBILITY_MS
}

/**
 * Reduces a task's remote runs to a single displayable state:
 * running, then queued, then recently canceled.
 */
export function deriveRunIndicator(
  activities: TaskActivity[],
  taskId: string,
  now: number = Date.now(),
): RunIndicator | null {
  const runs = activities.filter(activity => activity.taskId === taskId && activity.skillId === REMOTE_RUN_SKILL)
  const running = runs.filter(run => run.status === 'running')
  const queued = runs.filter(run => run.status === 'queued')
  const canceled = runs.filter(run => isVisibleCancellation(run, now))

  const selected: [RunIndicatorState, TaskActivity[]] | null =
    running.length > 0 ? ['running', running]
      : queued.length > 0 ? ['queued', queued]
        : canceled.length > 0 ? ['canceled', canceled]
          : null
  if (!selected) return null

  const [state, selectedRuns] = selected
  // An already canceled run offers no action.
  const cancelableRunIds = state === 'canceled'
    ? []
    : selectedRuns.filter(run => run.action === AGENT_OWNED_ACTION).map(run => run.id)

  return { state, runs: selectedRuns, cancelableRunIds, count: selectedRuns.length }
}
