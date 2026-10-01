import type { StoredUserCredential } from './trackers'

/**
 * Connecting Jira through Atlassian's consent screen (#654, ADR 0044). The
 * server says whether an OAuth app is configured and which Jira sites a grant
 * has to cover; these pure helpers decide what the Jira entry of the profile
 * shows and read the outcome the callback lands with.
 */
export interface JiraOAuthInfo {
  configured: boolean
  /** The Jira sites of the deployment, its own and every project's. */
  sites: string[]
}

export const NO_JIRA_OAUTH: JiraOAuthInfo = { configured: false, sites: [] }

/** Reads `jiraOAuth` from a credentials answer, tolerating its absence. */
export function jiraOAuthFrom(data: any): JiraOAuthInfo {
  const raw = data?.jiraOAuth
  if (!raw || typeof raw !== 'object') return NO_JIRA_OAUTH
  return {
    configured: raw.configured === true,
    sites: Array.isArray(raw.sites) ? raw.sites.filter((s: unknown) => typeof s === 'string') : [],
  }
}

/**
 * What the Jira entry shows:
 * - `form`: today's API token form, whenever OAuth is not configured;
 * - `connect`: no credential, *Connect Jira* first, the token form behind a link;
 * - `token-and-connect`: an API token is stored, its form plus *Connect Jira*;
 * - `connected` / `disconnected`: a grant, alive or refused by Atlassian.
 *
 * A grant keeps its own state when OAuth is no longer configured: the token
 * form would claim a token is stored that never was.
 */
export type JiraEntryState = 'form' | 'connect' | 'token-and-connect' | 'connected' | 'disconnected'

export function jiraEntryState(credential: StoredUserCredential | undefined, oauth: JiraOAuthInfo): JiraEntryState {
  if (credential?.kind === 'oauth') return credential.disconnected ? 'disconnected' : 'connected'
  if (!oauth.configured) return 'form'
  return credential ? 'token-and-connect' : 'connect'
}

export const JIRA_OAUTH_OUTCOMES = ['connected', 'cancelled', 'invalid', 'no_site', 'unreachable'] as const
export type JiraOAuthOutcome = (typeof JIRA_OAUTH_OUTCOMES)[number]

const OUTCOME_PARAMS = ['trackerCredentials', 'jiraOAuth']

/** The outcome the callback redirected with, or null when the address carries none. */
export function oauthOutcomeFromSearch(search: string): { outcome: JiraOAuthOutcome } | null {
  const params = new URLSearchParams(search)
  if (params.get('trackerCredentials') !== 'jira') return null
  const outcome = params.get('jiraOAuth') as JiraOAuthOutcome | null
  return outcome && (JIRA_OAUTH_OUTCOMES as readonly string[]).includes(outcome) ? { outcome } : null
}

/** The address search without the callback's parameters, so a reload shows nothing again. */
export function withoutOAuthOutcome(search: string): string {
  const params = new URLSearchParams(search)
  for (const name of OUTCOME_PARAMS) params.delete(name)
  const rest = params.toString()
  return rest ? `?${rest}` : ''
}

export type JiraOAuthOutcomeMessages = Record<JiraOAuthOutcome, string>

/** What the profile says about a callback's outcome, the sites named when none was covered. */
export function jiraOAuthOutcomeMessage(outcome: JiraOAuthOutcome, messages: JiraOAuthOutcomeMessages, sites: string[]): string {
  return messages[outcome].replace('{sites}', sites.length ? sites.join(', ') : '—')
}

/** How the outcome is shown: a success, a choice the person made, or a failure to act on. */
export function jiraOAuthOutcomeTone(outcome: JiraOAuthOutcome): 'success' | 'info' | 'error' {
  if (outcome === 'connected') return 'success'
  if (outcome === 'cancelled') return 'info'
  return 'error'
}
