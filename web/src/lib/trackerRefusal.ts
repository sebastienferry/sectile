import type { TrackerKind } from './trackers'

/**
 * Recognising a tracker write refused because the person has no token of
 * their own for the provider (#645).
 *
 * The server says so with a code and the provider, on the 403 of a write
 * answered at once and on the failed activity of a queued one. The message it
 * sends along is English and may be reworded: nothing here reads it.
 */

/** The code of the 403 a missing personal token is answered with. */
export const TRACKER_CREDENTIAL_MISSING = 'tracker_credential_missing'

const KNOWN_TRACKERS: readonly string[] = ['github', 'gitlab', 'jira']

function asTracker(value: unknown): TrackerKind | null {
  return typeof value === 'string' && KNOWN_TRACKERS.includes(value) ? (value as TrackerKind) : null
}

/** The provider a failed response was refused for, or null for any other failure. */
export function missingCredentialFromBody(status: number, body: unknown): TrackerKind | null {
  if (status !== 403 || !body || typeof body !== 'object') return null
  const { code, tracker } = body as { code?: unknown; tracker?: unknown }
  if (code !== TRACKER_CREDENTIAL_MISSING) return null
  return asTracker(tracker)
}

/** The provider a failed activity was refused for, or null. */
export function missingCredentialFromActivity(act: { status: string; credentialMissing?: string }): TrackerKind | null {
  if (act.status !== 'failed') return null
  return asTracker(act.credentialMissing)
}

/**
 * An error that remembers the refusal, so the catch that shows the
 * notification, often far from the fetch, can still offer the token.
 */
export class TrackerCredentialMissingError extends Error {
  readonly tracker: TrackerKind

  constructor(message: string, tracker: TrackerKind) {
    super(message)
    this.name = 'TrackerCredentialMissingError'
    this.tracker = tracker
  }
}
