/**
 * The server tracker credentials of the Administration page (#464): one per
 * provider, set by an admin, sealed on the server. The token never comes back;
 * what the page gets is where each credential comes from and who it
 * authenticates as.
 */

export type ServerCredentialTracker = 'github' | 'jira' | 'gitlab'

export type ServerCredentialSource = 'database' | 'environment' | 'none'

/** One provider's state, as GET /api/admin/tracker-credentials answers it. */
export interface ServerCredentialState {
  tracker: ServerCredentialTracker
  source: ServerCredentialSource
  /** Stored, but the server key does not open it: it has to be saved again. */
  unreadable?: boolean
  /** Jira only: the account e-mail the token goes with. */
  email?: string
  /** The account the last successful check named. */
  account?: string
  checkedAt?: string
  updatedAt?: string
}

export const SERVER_CREDENTIALS_PATH = '/api/admin/tracker-credentials'

/** The label keys a state reads as, one per situation the page tells apart. */
export type ServerCredentialLabel = 'stored' | 'storedUnreadable' | 'environment' | 'none'

export function serverCredentialLabel(state: Pick<ServerCredentialState, 'source' | 'unreadable'>): ServerCredentialLabel {
  if (state.source === 'database') return state.unreadable ? 'storedUnreadable' : 'stored'
  if (state.source === 'environment') return 'environment'
  return 'none'
}

/** The environment variables that supply a provider when nothing is stored. */
export function serverCredentialVariables(tracker: ServerCredentialTracker): string[] {
  switch (tracker) {
    case 'jira':
      return ['SECTILE_JIRA_EMAIL', 'SECTILE_JIRA_TOKEN']
    case 'gitlab':
      return ['SECTILE_GITLAB_TOKEN']
    default:
      return ['SECTILE_GITHUB_TOKEN']
  }
}

async function answer<T>(response: Response): Promise<T> {
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error((body as { error?: string }).error || `HTTP ${response.status}`)
  }
  return body as T
}

export async function fetchServerCredentials(): Promise<ServerCredentialState[]> {
  return answer<ServerCredentialState[]>(await fetch(SERVER_CREDENTIALS_PATH))
}

/** Checks a credential without storing it; an empty token checks the one in use. */
export async function checkServerCredential(tracker: ServerCredentialTracker, email: string, token: string): Promise<string> {
  const result = await answer<{ account?: string }>(await fetch(`${SERVER_CREDENTIALS_PATH}/${tracker}/check`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, token }),
  }))
  return result.account || ''
}

/** Checks, then stores; a refused check stores nothing. */
export async function saveServerCredential(tracker: ServerCredentialTracker, email: string, token: string): Promise<ServerCredentialState> {
  return answer<ServerCredentialState>(await fetch(`${SERVER_CREDENTIALS_PATH}/${tracker}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, token }),
  }))
}

export async function clearServerCredential(tracker: ServerCredentialTracker): Promise<ServerCredentialState> {
  return answer<ServerCredentialState>(await fetch(`${SERVER_CREDENTIALS_PATH}/${tracker}`, { method: 'DELETE' }))
}
