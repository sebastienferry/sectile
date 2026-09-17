import { runState, runStateIconDataUrl } from '../../shared/runStates.ts'

/** Statuses that end a run, and therefore announce a finished turn. */
const TERMINAL = new Set(['completed', 'failed', 'canceled'])

/** The state a run is in, in the vocabulary both surfaces share. */
export function stateOf(run) {
  if (TERMINAL.has(run.status)) return run.status
  if (run.waitingSince) return 'waiting'
  return run.status === 'queued' || run.status === 'preparing' ? 'queued' : 'running'
}

/** What the notification calls this session: its task if it has one, else its directory. */
export function nameOf(run) {
  if (run.taskKey) return run.taskKey + (run.skill ? ' · ' + run.skill : '')
  if (run.directory) return run.directory.split('/').filter(Boolean).pop() || 'Session'
  return run.skill || 'Session'
}

/**
 * Compares two polls and returns what deserves a banner.
 *
 * The transition is what notifies, never the state: the run list is polled
 * every two seconds, so announcing the state would raise the same banner thirty
 * times a minute. Only a run that has just entered a notifiable state is
 * reported, and a run seen for the first time is not — the desktop opening
 * should not replay every session's history.
 */
export function transitions(previous, next) {
  const before = new Map(previous.map(run => [run.id, stateOf(run)]))
  const raised = []
  for (const run of next) {
    const now = stateOf(run)
    if (!before.has(run.id)) continue
    if (before.get(run.id) === now) continue
    if (now !== 'waiting' && !TERMINAL.has(run.status)) continue
    raised.push({ id: run.id, state: now, name: nameOf(run) })
  }
  return raised
}

/** Turns a transition, or a session-level alert, into the notification payload. */
export function notification({ state, name }) {
  const definition = runState(state)
  if (!definition) return null
  return {
    title: name,
    body: name + ' ' + definition.announcement,
    icon: runStateIconDataUrl(state),
  }
}

/**
 * Raises the banner for each transition, through the renderer's Notification
 * API. Electron maps it onto the platform's own facility — Notification Center,
 * Windows toasts, the freedesktop spec — so this is a real system notification
 * attributed to the application, not an in-window widget.
 *
 * Availability is checked once. A workstation that denies notifications must
 * not be asked again on every event, and must not break the poll that called
 * this: the list still carries the state.
 */
let permitted = null
export function announce(events, Notifier = globalThis.Notification) {
  if (!events.length || !Notifier) return 0
  if (permitted === null) {
    permitted = Notifier.permission !== 'denied'
    if (Notifier.permission === 'default' && Notifier.requestPermission) {
      try { Notifier.requestPermission() } catch { /* asking is best effort */ }
    }
  }
  if (!permitted) return 0
  let raised = 0
  for (const event of events) {
    const payload = notification(event)
    if (!payload) continue
    try {
      new Notifier(payload.title, { body: payload.body, icon: payload.icon, tag: event.id })
      raised++
    } catch {
      // A failure here is the notification channel's, never the poll's.
      permitted = false
      return raised
    }
  }
  return raised
}

/** Test seam: forgets the availability decision taken on the first call. */
export function resetNotificationAvailability() { permitted = null }
