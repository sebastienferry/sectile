// A task row's skill result costs the agent two reads of the server: the task
// and every activity of it. The run list is polled every two seconds, and
// reading each visible row's result at that pace sent the server about ten
// requests a second for a sidebar of ten rows. A result is read again only
// when something can have changed it.

// A live run's result is read again at most this often, and an ended run's at
// this slower pace, which still follows a task moved on elsewhere.
export const LIVE_REFRESH_MS = 30_000
export const ENDED_REFRESH_MS = 120_000

const LIVE = new Set(['queued', 'preparing', 'running'])

// last is what was recorded when the result was last read: the run's status,
// its wait mark and when. A change of either is what a result follows.
export function skillResultDue(run, last, now) {
  if (!last) return true
  if (last.status !== run.status || last.waitingSince !== (run.waitingSince || '')) return true
  return now - last.at >= (LIVE.has(run.status) ? LIVE_REFRESH_MS : ENDED_REFRESH_MS)
}

export function skillResultStamp(run, now) {
  return { status: run.status, waitingSince: run.waitingSince || '', at: now }
}
