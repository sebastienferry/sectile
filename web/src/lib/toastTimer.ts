/**
 * How long a toast stays, and the timer that dismisses it.
 *
 * A toast that only reports something can go quickly. One that offers a link
 * has to stay long enough to be reached, and must not vanish under the pointer
 * or while the keyboard is on it: the timer pauses for as long as something
 * holds it, and resumes with the time that was left.
 */

export const TOAST_DURATION_MS = 3500
export const TOAST_WITH_LINK_DURATION_MS = 8000

/** The explicit duration when there is one, else a longer default for a toast with a link. */
export function toastDuration(toast: { duration?: number; link?: unknown }): number {
  if (toast.duration && toast.duration > 0) return toast.duration
  return toast.link ? TOAST_WITH_LINK_DURATION_MS : TOAST_DURATION_MS
}

/** What holds a toast on screen. Each reason is released on its own. */
export type PauseReason = 'hover' | 'focus'

export interface DismissTimer {
  pause: (reason: PauseReason) => void
  resume: (reason: PauseReason) => void
  cancel: () => void
}

export interface TimerClock {
  now: () => number
  setTimeout: (callback: () => void, ms: number) => unknown
  clearTimeout: (handle: unknown) => void
}

const systemClock: TimerClock = {
  now: () => Date.now(),
  setTimeout: (callback, ms) => setTimeout(callback, ms),
  clearTimeout: handle => clearTimeout(handle as ReturnType<typeof setTimeout>),
}

/**
 * Calls onElapsed once `duration` ms have run, not counting the time spent
 * paused. The timer runs only while no reason holds it, so leaving the toast
 * with the pointer does not restart it while it still has the focus.
 */
export function createDismissTimer(
  duration: number,
  onElapsed: () => void,
  clock: TimerClock = systemClock,
): DismissTimer {
  let remaining = duration
  let startedAt = 0
  let handle: unknown = null
  let done = false
  const holds = new Set<PauseReason>()

  const start = () => {
    if (done || handle !== null) return
    startedAt = clock.now()
    handle = clock.setTimeout(() => {
      handle = null
      done = true
      onElapsed()
    }, Math.max(0, remaining))
  }

  const stop = () => {
    if (handle === null) return
    clock.clearTimeout(handle)
    handle = null
    remaining -= clock.now() - startedAt
  }

  start()

  return {
    pause(reason) {
      holds.add(reason)
      stop()
    },
    resume(reason) {
      holds.delete(reason)
      if (holds.size === 0) start()
    },
    cancel() {
      done = true
      stop()
    },
  }
}
