import type { ForgeOAuthStrings, OAuthEntryStrings, TranslationSchema } from '../locales/translations'
import type { OAuthTracker, StoredUserCredential, UserCredentialsAnswer } from './trackers'

/**
 * Connecting a tracker through its provider's consent screen: Jira through
 * Atlassian (#654, ADR 0044), GitHub and GitLab through their OAuth app
 * (#804). The server says, per tracker, whether an OAuth app is configured
 * (and for Jira which sites a grant has to cover); these pure helpers decide
 * what a tracker's entry of the profile shows and read the outcome the
 * callback lands with.
 */
export const OAUTH_TRACKERS: readonly OAuthTracker[] = ['jira', 'github', 'gitlab']

export function isOAuthTracker(tracker: string | null | undefined): tracker is OAuthTracker {
  return (OAUTH_TRACKERS as readonly string[]).includes(tracker ?? '')
}

export interface TrackerOAuthInfo {
  configured: boolean
  /** The Jira sites of the deployment, its own and every project's. Always empty for a forge. */
  sites: string[]
}

/** Kept under its Jira name: Jira was the first tracker to connect through OAuth. */
export type JiraOAuthInfo = TrackerOAuthInfo

export const NO_TRACKER_OAUTH: TrackerOAuthInfo = { configured: false, sites: [] }
export const NO_JIRA_OAUTH: JiraOAuthInfo = NO_TRACKER_OAUTH

/** Every tracker's OAuth availability, as the credentials answer gives it. */
export type TrackerOAuthMap = Record<OAuthTracker, TrackerOAuthInfo>

export const NO_OAUTH: TrackerOAuthMap = { jira: NO_TRACKER_OAUTH, github: NO_TRACKER_OAUTH, gitlab: NO_TRACKER_OAUTH }

/** The field of the credentials answer that carries a tracker's OAuth availability. */
const ANSWER_FIELD: Record<OAuthTracker, keyof UserCredentialsAnswer> = {
  jira: 'jiraOAuth',
  github: 'githubOAuth',
  gitlab: 'gitlabOAuth',
}

/** Reads one tracker's OAuth availability from a credentials answer, tolerating its absence. */
export function trackerOAuthFrom(data: any, tracker: OAuthTracker): TrackerOAuthInfo {
  const raw = data?.[ANSWER_FIELD[tracker]]
  if (!raw || typeof raw !== 'object') return NO_TRACKER_OAUTH
  return {
    configured: raw.configured === true,
    // Only Jira names sites: a forge grant covers its public instance alone.
    sites: tracker === 'jira' && Array.isArray(raw.sites) ? raw.sites.filter((s: unknown) => typeof s === 'string') : [],
  }
}

/** Reads `jiraOAuth` from a credentials answer, tolerating its absence. */
export function jiraOAuthFrom(data: any): JiraOAuthInfo {
  return trackerOAuthFrom(data, 'jira')
}

/** Every tracker's OAuth availability from a credentials answer. */
export function oauthFrom(data: any): TrackerOAuthMap {
  return { jira: trackerOAuthFrom(data, 'jira'), github: trackerOAuthFrom(data, 'github'), gitlab: trackerOAuthFrom(data, 'gitlab') }
}

/** A tracker's OAuth availability, or undefined for a tracker that never connects through OAuth. */
export function oauthFor(oauth: TrackerOAuthMap, tracker: string): TrackerOAuthInfo | undefined {
  return isOAuthTracker(tracker) ? oauth[tracker] : undefined
}

/**
 * What a tracker's entry shows:
 * - `form`: the token form, whenever OAuth is not configured;
 * - `connect`: no credential, *Connect* first, the token form behind a link;
 * - `token-and-connect`: a token is stored, its form plus *Connect*;
 * - `connected` / `disconnected`: a grant, alive or refused by the provider.
 *
 * A grant keeps its own state when OAuth is no longer configured: the token
 * form would claim a token is stored that never was.
 */
export type EntryState = 'form' | 'connect' | 'token-and-connect' | 'connected' | 'disconnected'
export type JiraEntryState = EntryState

export function entryState(credential: StoredUserCredential | undefined, oauth: TrackerOAuthInfo | undefined): EntryState {
  if (credential?.kind === 'oauth') return credential.disconnected ? 'disconnected' : 'connected'
  if (!oauth?.configured) return 'form'
  return credential ? 'token-and-connect' : 'connect'
}

export const JIRA_OAUTH_OUTCOMES = ['connected', 'cancelled', 'invalid', 'no_site', 'unreachable'] as const
export type JiraOAuthOutcome = (typeof JIRA_OAUTH_OUTCOMES)[number]

/** A forge grant covers its public instance alone, so no consent can miss a site. */
export const FORGE_OAUTH_OUTCOMES = ['connected', 'cancelled', 'invalid', 'unreachable'] as const
export type ForgeOAuthOutcome = (typeof FORGE_OAUTH_OUTCOMES)[number]

export type OAuthOutcome = JiraOAuthOutcome | ForgeOAuthOutcome

export function outcomesFor(tracker: OAuthTracker): readonly OAuthOutcome[] {
  return tracker === 'jira' ? JIRA_OAUTH_OUTCOMES : FORGE_OAUTH_OUTCOMES
}

export interface OAuthCallbackOutcome {
  tracker: OAuthTracker
  outcome: OAuthOutcome
}

/**
 * The parameter each callback carries its outcome in, next to
 * `trackerCredentials=<tracker>`: Jira kept the one it shipped with.
 */
function outcomeParam(tracker: OAuthTracker): string {
  return tracker === 'jira' ? 'jiraOAuth' : 'oauth'
}

const OUTCOME_PARAMS = ['trackerCredentials', 'jiraOAuth', 'oauth']

/** The outcome a callback redirected with, or null when the address carries none. */
export function oauthOutcomeFromSearch(search: string): OAuthCallbackOutcome | null {
  const params = new URLSearchParams(search)
  const tracker = params.get('trackerCredentials')
  if (!isOAuthTracker(tracker)) return null
  const outcome = params.get(outcomeParam(tracker))
  return outcome && (outcomesFor(tracker) as readonly string[]).includes(outcome) ? { tracker, outcome: outcome as OAuthOutcome } : null
}

/** The address search without the callbacks' parameters, so a reload shows nothing again. */
export function withoutOAuthOutcome(search: string): string {
  const params = new URLSearchParams(search)
  for (const name of OUTCOME_PARAMS) params.delete(name)
  const rest = params.toString()
  return rest ? `?${rest}` : ''
}

export type OAuthOutcomeMessages = Partial<Record<OAuthOutcome, string>> & Record<ForgeOAuthOutcome, string>
export type JiraOAuthOutcomeMessages = Record<JiraOAuthOutcome, string>

/** What the profile says about a callback's outcome, the sites named when none was covered. */
export function oauthOutcomeMessage(outcome: OAuthOutcome, messages: OAuthOutcomeMessages, sites: string[]): string {
  return (messages[outcome] ?? '').replace('{sites}', sites.length ? sites.join(', ') : '—')
}

export const jiraOAuthOutcomeMessage: (outcome: JiraOAuthOutcome, messages: JiraOAuthOutcomeMessages, sites: string[]) => string = oauthOutcomeMessage

/** How the outcome is shown: a success, a choice the person made, or a failure to act on. */
export function oauthOutcomeTone(outcome: OAuthOutcome): 'success' | 'info' | 'error' {
  if (outcome === 'connected') return 'success'
  if (outcome === 'cancelled') return 'info'
  return 'error'
}

export const jiraOAuthOutcomeTone = oauthOutcomeTone

/** The error code the server answers a Connect with when the tracker's OAuth app is not configured. */
export function notConfiguredCode(tracker: OAuthTracker): string {
  return `${tracker}_oauth_not_configured`
}

/** The path that starts a tracker's consent. */
export function connectPath(tracker: OAuthTracker): string {
  return `/me/tracker-credentials/${tracker}/connect`
}

/**
 * A tracker's entry wording. Jira keeps its own block, which says more (the
 * sites a grant covers, the Atlassian note, the `no_site` outcome).
 */
export function oauthStrings(t: TranslationSchema, tracker: OAuthTracker): OAuthEntryStrings {
  return tracker === 'jira' ? t.trackerCredentials.oauth : t.trackerCredentials[`${tracker}OAuth`]
}

/** A forge's entry wording, with the note on the instance a grant covers. */
export function forgeOAuthStrings(t: TranslationSchema, tracker: OAuthTracker): ForgeOAuthStrings | undefined {
  return tracker === 'jira' ? undefined : t.trackerCredentials[`${tracker}OAuth`]
}
