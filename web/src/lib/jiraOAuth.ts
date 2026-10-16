import { oauthOutcomeFromSearch as trackerOAuthOutcomeFromSearch, type JiraOAuthOutcome } from './trackerOAuth.ts'

/**
 * Connecting Jira through Atlassian's consent screen (#654, ADR 0044). The
 * logic now lives in trackerOAuth.ts, shared with GitHub and GitLab (#804);
 * this module keeps the Jira names it shipped with.
 */
export {
  JIRA_OAUTH_OUTCOMES,
  NO_JIRA_OAUTH,
  entryState as jiraEntryState,
  jiraOAuthFrom,
  jiraOAuthOutcomeMessage,
  jiraOAuthOutcomeTone,
  withoutOAuthOutcome,
  type JiraEntryState,
  type JiraOAuthInfo,
  type JiraOAuthOutcome,
  type JiraOAuthOutcomeMessages,
} from './trackerOAuth.ts'

/** The outcome Jira's callback redirected with, or null when the address carries none for Jira. */
export function oauthOutcomeFromSearch(search: string): { outcome: JiraOAuthOutcome } | null {
  const found = trackerOAuthOutcomeFromSearch(search)
  return found?.tracker === 'jira' ? { outcome: found.outcome as JiraOAuthOutcome } : null
}
