// A task row's skill result costs the agent two reads of the server: the task
// and every activity of it. The run list is polled every two seconds, and
// reading each visible row's result at that pace sent the server about ten
// requests a second for a sidebar of ten rows. A result is read again only
// when something can have changed it.

// Right after a run changes, its result is read at every poll for a while:
// a skill ends its run, then moves its task, and the second step lands a
// moment after the first. Past that, a live run's result is read again at
// most every LIVE_REFRESH_MS and an ended run's every ENDED_REFRESH_MS, which
// still follows a task moved on elsewhere.
export const SETTLE_MS = 30_000
export const LIVE_REFRESH_MS = 30_000
export const ENDED_REFRESH_MS = 120_000

const LIVE = new Set(['queued', 'preparing', 'running'])

// last is what was recorded when the result was last read: the run's status,
// its wait mark, when it was read and when the run last changed. A change of
// either is what a result follows.
export function skillResultDue(run, last, now) {
  if (!last) return true
  if (last.status !== run.status || last.waitingSince !== (run.waitingSince || '')) return true
  if (now - last.changedAt < SETTLE_MS) return true
  return now - last.at >= (LIVE.has(run.status) ? LIVE_REFRESH_MS : ENDED_REFRESH_MS)
}

// The stamp of a read. The run's change keeps its time while the run stays as
// it was, so the settling window starts at the change, not at each read.
export function skillResultStamp(run, now, last) {
  const same = last && last.status === run.status && last.waitingSince === (run.waitingSince || '')
  return { status: run.status, waitingSince: run.waitingSince || '', at: now, changedAt: same ? last.changedAt : now }
}
