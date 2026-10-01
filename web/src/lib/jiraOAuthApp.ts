/**
 * The Jira OAuth app of the Administration page (#654). The secret is
 * write-only: the state the server answers says whether one is set, never
 * what it is, and nothing here keeps it beyond the admin's own input.
 */
export const JIRA_OAUTH_APP_PATH = '/api/admin/jira-oauth'

export type JiraOAuthAppSource = 'database' | 'environment' | 'none'

export interface JiraOAuthAppState {
  configured: boolean
  clientId: string
  secretSet: boolean
  redirectUrl: string
  source: JiraOAuthAppSource
  unreadable?: boolean
  updatedAt?: string
}

export interface JiraOAuthAppLabels {
  sourceStored: string
  sourceEnvironment: string
  sourceNone: string
}

export function jiraOAuthAppSourceLabel(source: JiraOAuthAppSource, labels: JiraOAuthAppLabels): string {
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
export function jiraOAuthAppPayload(values: { clientId: string; clientSecret: string; redirectUrl: string }) {
  const payload: { clientId: string; redirectUrl: string; clientSecret?: string } = {
    clientId: values.clientId.trim(),
    redirectUrl: values.redirectUrl.trim(),
  }
  if (values.clientSecret.trim()) payload.clientSecret = values.clientSecret.trim()
  return payload
}

/**
 * The scopes the registered app must declare, the same list the server asks
 * for (internal/atlassian.Scopes).
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

async function answer(response: Response): Promise<JiraOAuthAppState> {
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error((body as { error?: string }).error || `HTTP ${response.status}`)
  }
  return body as JiraOAuthAppState
}

export async function fetchJiraOAuthApp(): Promise<JiraOAuthAppState> {
  return answer(await fetch(JIRA_OAUTH_APP_PATH))
}

export async function saveJiraOAuthApp(values: { clientId: string; clientSecret: string; redirectUrl: string }): Promise<JiraOAuthAppState> {
  return answer(await fetch(JIRA_OAUTH_APP_PATH, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(jiraOAuthAppPayload(values)),
  }))
}

export async function clearJiraOAuthApp(): Promise<JiraOAuthAppState> {
  return answer(await fetch(JIRA_OAUTH_APP_PATH, { method: 'DELETE' }))
}
