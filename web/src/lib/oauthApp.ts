import type { OAuthTracker } from './trackers'

/**
 * A tracker's OAuth app on the Administration page: Atlassian's for Jira
 * (#654), GitHub's and GitLab's (#804). The secret is write-only: the state
 * the server answers says whether one is set, never what it is, and nothing
 * here keeps it beyond the admin's own input.
 */
export function oauthAppPath(tracker: OAuthTracker): string {
  return `/api/admin/${tracker}-oauth`
}

/** The callback the admin registers on the provider's app, as a placeholder. */
export function oauthAppRedirectPlaceholder(tracker: OAuthTracker): string {
  return `https://sectile.example.com/auth/${tracker}/callback`
}

export type OAuthAppSource = 'database' | 'environment' | 'none'

export interface OAuthAppState {
  configured: boolean
  clientId: string
  secretSet: boolean
  redirectUrl: string
  source: OAuthAppSource
  unreadable?: boolean
  updatedAt?: string
}

export interface OAuthAppLabels {
  sourceStored: string
  sourceEnvironment: string
  sourceNone: string
}

export function oauthAppSourceLabel(source: OAuthAppSource, labels: OAuthAppLabels): string {
  switch (source) {
    case 'database':
      return labels.sourceStored
    case 'environment':
      return labels.sourceEnvironment
    default:
      return labels.sourceNone
  }
}

/** What a save sends: an empty secret keeps the saved one. */
export function oauthAppPayload(values: { clientId: string; clientSecret: string; redirectUrl: string }) {
  const payload: { clientId: string; redirectUrl: string; clientSecret?: string } = {
    clientId: values.clientId.trim(),
    redirectUrl: values.redirectUrl.trim(),
  }
  if (values.clientSecret.trim()) payload.clientSecret = values.clientSecret.trim()
  return payload
}

/**
 * The scopes the registered Atlassian app must declare, the same list the
 * server asks for (internal/atlassian.Scopes).
 */
export const JIRA_OAUTH_SCOPES = [
  'read:jira-work',
  'write:jira-work',
  'read:jira-user',
  'offline_access',
  'read:board-scope:jira-software',
  'read:board-scope.admin:jira-software',
  'write:board-scope:jira-software',
  'read:sprint:jira-software',
  'write:sprint:jira-software',
  'delete:sprint:jira-software',
  'read:project:jira',
]

/**
 * The scopes each tracker's app must declare or is asked for, the same lists
 * the server asks for (internal/atlassian.Scopes, internal/forgeoauth GitHub
 * and GitLab `Scopes`).
 */
export const OAUTH_APP_SCOPES: Record<OAuthTracker, readonly string[]> = {
  jira: JIRA_OAUTH_SCOPES,
  github: ['repo', 'read:project'],
  gitlab: ['api'],
}

async function answer(response: Response): Promise<OAuthAppState> {
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error((body as { error?: string }).error || `HTTP ${response.status}`)
  }
  return body as OAuthAppState
}

export async function fetchOAuthApp(tracker: OAuthTracker): Promise<OAuthAppState> {
  return answer(await fetch(oauthAppPath(tracker)))
}

export async function saveOAuthApp(
  tracker: OAuthTracker,
  values: { clientId: string; clientSecret: string; redirectUrl: string },
): Promise<OAuthAppState> {
  return answer(await fetch(oauthAppPath(tracker), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(oauthAppPayload(values)),
  }))
}

export async function clearOAuthApp(tracker: OAuthTracker): Promise<OAuthAppState> {
  return answer(await fetch(oauthAppPath(tracker), { method: 'DELETE' }))
}
