/**
 * My Tasks (#468): who "me" is on each tracker of the board. The server
 * resolves it and filters on `mine=1`; the client only needs to know, for the
 * button's tooltip, on which trackers it had to fall back on the account's
 * name and e-mail.
 */

/** One tracker of the scope, as GET /api/me/assignee-identities answers it. */
export interface TrackerIdentity {
  tracker: string
  /** The account the tracker confirmed for the personal credential. */
  identity?: string
  known: boolean
}

export interface AssigneeIdentities {
  signedIn: boolean
  /** The account's name and e-mail, used wherever no tracker identity is known. */
  fallback: string[]
  /** The trackers of the tickets in scope, local tickets excluded. */
  trackers: TrackerIdentity[]
}

export const ASSIGNEE_IDENTITIES_PATH = '/api/me/assignee-identities'

const TRACKER_LABELS: Record<string, string> = { github: 'GitHub', jira: 'Jira', gitlab: 'GitLab' }

export function trackerLabel(tracker: string): string {
  return TRACKER_LABELS[tracker] ?? tracker
}

export interface MyTasksTooltipStrings {
  myTasks: string
  /** Names the trackers with `{trackers}`. */
  myTasksFallback: string
  myTasksSignedOut: string
}

/**
 * The button's title: its label alone when every tracker in scope knows who I
 * am, else the label and the trackers on which my name and e-mail are used,
 * with what fixes it. Signed out, "me" is the local profile everywhere.
 */
export function myTasksTooltip(identities: AssigneeIdentities | null, strings: MyTasksTooltipStrings): string {
  if (!identities) return strings.myTasks
  if (!identities.signedIn) return `${strings.myTasks}\n${strings.myTasksSignedOut}`
  const unknown = identities.trackers.filter(entry => !entry.known).map(entry => trackerLabel(entry.tracker))
  if (unknown.length === 0) return strings.myTasks
  return `${strings.myTasks}\n${strings.myTasksFallback.replace('{trackers}', unknown.join(', '))}`
}
