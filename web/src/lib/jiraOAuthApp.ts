import {
  clearOAuthApp,
  fetchOAuthApp,
  oauthAppPath,
  saveOAuthApp,
  type OAuthAppState,
} from './oauthApp.ts'

/**
 * The Jira OAuth app of the Administration page (#654). The logic now lives
 * in oauthApp.ts, shared with GitHub and GitLab (#804); this module keeps the
 * Jira names it shipped with. The secret stays write-only.
 */
export {
  JIRA_OAUTH_SCOPES,
  oauthAppPayload as jiraOAuthAppPayload,
  oauthAppSourceLabel as jiraOAuthAppSourceLabel,
  type OAuthAppLabels as JiraOAuthAppLabels,
  type OAuthAppSource as JiraOAuthAppSource,
} from './oauthApp.ts'

export const JIRA_OAUTH_APP_PATH = oauthAppPath('jira')

export interface JiraOAuthAppState extends OAuthAppState {}

export function fetchJiraOAuthApp(): Promise<JiraOAuthAppState> {
  return fetchOAuthApp('jira')
}

export function saveJiraOAuthApp(values: { clientId: string; clientSecret: string; redirectUrl: string }): Promise<JiraOAuthAppState> {
  return saveOAuthApp('jira', values)
}

export function clearJiraOAuthApp(): Promise<JiraOAuthAppState> {
  return clearOAuthApp('jira')
}
