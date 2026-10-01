import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'
import {
  JIRA_OAUTH_OUTCOMES,
  NO_JIRA_OAUTH,
  jiraEntryState,
  jiraOAuthFrom,
  oauthOutcomeFromSearch,
  withoutOAuthOutcome,
} from '../src/lib/jiraOAuth.ts'
import { JIRA_OAUTH_SCOPES, jiraOAuthAppPayload, jiraOAuthAppSourceLabel } from '../src/lib/jiraOAuthApp.ts'
import { sealableCredentials } from '../src/lib/trackers.ts'

const configured = { configured: true, sites: ['https://acme.atlassian.net'] }
const token = { tracker: 'jira', kind: 'api_token', sealed: false, unlocked: true }
const grant = { tracker: 'jira', kind: 'oauth', sealed: false, unlocked: true, account: 'Ada', grantedSites: ['https://acme.atlassian.net'] }

test('the Jira entry shows the token form whenever OAuth is not configured', () => {
  // AC10: without OAuth, the profile is today's.
  assert.equal(jiraEntryState(undefined, NO_JIRA_OAUTH), 'form')
  assert.equal(jiraEntryState(token, NO_JIRA_OAUTH), 'form')
  assert.equal(jiraEntryState({ tracker: 'jira', sealed: true, unlocked: false }, NO_JIRA_OAUTH), 'form')
})

test('with OAuth configured, the Jira entry offers Connect Jira first', () => {
  assert.equal(jiraEntryState(undefined, configured), 'connect')
  assert.equal(jiraEntryState(token, configured), 'token-and-connect')
  // A row written before kinds existed carries none and is an API token.
  assert.equal(jiraEntryState({ tracker: 'jira', sealed: false, unlocked: true }, configured), 'token-and-connect')
  assert.equal(jiraEntryState(grant, configured), 'connected')
  assert.equal(jiraEntryState({ ...grant, disconnected: true }, configured), 'disconnected')
})

test('a grant keeps its own state when OAuth is no longer configured', () => {
  assert.equal(jiraEntryState(grant, NO_JIRA_OAUTH), 'connected')
  assert.equal(jiraEntryState({ ...grant, disconnected: true }, NO_JIRA_OAUTH), 'disconnected')
})

test('jiraOAuth is read leniently from the credentials answer', () => {
  assert.deepEqual(jiraOAuthFrom({}), NO_JIRA_OAUTH)
  assert.deepEqual(jiraOAuthFrom({ jiraOAuth: null }), NO_JIRA_OAUTH)
  assert.deepEqual(jiraOAuthFrom({ jiraOAuth: { configured: 'yes', sites: 'x' } }), { configured: false, sites: [] })
  assert.deepEqual(jiraOAuthFrom({ jiraOAuth: { configured: true, sites: ['a', 3, 'b'] } }), { configured: true, sites: ['a', 'b'] })
})

test('every callback outcome is read, and nothing else', () => {
  for (const outcome of JIRA_OAUTH_OUTCOMES) {
    assert.deepEqual(oauthOutcomeFromSearch(`?trackerCredentials=jira&jiraOAuth=${outcome}`), { outcome })
  }
  assert.deepEqual(JIRA_OAUTH_OUTCOMES, ['connected', 'cancelled', 'invalid', 'no_site', 'unreachable'])
  assert.equal(oauthOutcomeFromSearch('?trackerCredentials=jira&jiraOAuth=pwned'), null)
  assert.equal(oauthOutcomeFromSearch('?trackerCredentials=github&jiraOAuth=connected'), null)
  assert.equal(oauthOutcomeFromSearch('?view=board'), null)
  assert.equal(oauthOutcomeFromSearch(''), null)
})

test('the callback parameters are dropped from the address, the rest kept', () => {
  assert.equal(withoutOAuthOutcome('?trackerCredentials=jira&jiraOAuth=connected'), '')
  assert.equal(withoutOAuthOutcome('?view=board&trackerCredentials=jira&jiraOAuth=no_site'), '?view=board')
})

test('the passphrase never applies to a Jira grant', () => {
  const sealed = { tracker: 'github', sealed: true, unlocked: false }
  assert.deepEqual(sealableCredentials([grant, token, sealed]), [token, sealed])
})

test('the admin save sends a secret only when one was typed', () => {
  assert.deepEqual(jiraOAuthAppPayload({ clientId: ' c ', clientSecret: '  ', redirectUrl: ' https://x/cb ' }), { clientId: 'c', redirectUrl: 'https://x/cb' })
  assert.deepEqual(jiraOAuthAppPayload({ clientId: 'c', clientSecret: 's', redirectUrl: 'u' }), { clientId: 'c', redirectUrl: 'u', clientSecret: 's' })
  const labels = { sourceStored: 'S', sourceEnvironment: 'E', sourceNone: 'N' }
  assert.equal(jiraOAuthAppSourceLabel('database', labels), 'S')
  assert.equal(jiraOAuthAppSourceLabel('environment', labels), 'E')
  assert.equal(jiraOAuthAppSourceLabel('none', labels), 'N')
})

test('the scopes shown to the admin are the ones the server asks for', () => {
  const server = readFileSync(new URL('../../internal/atlassian/oauth.go', import.meta.url), 'utf8')
  const list = server.slice(server.indexOf('var Scopes = []string{'), server.indexOf('}', server.indexOf('var Scopes = []string{')))
  const asked = [...list.matchAll(/"([^"]+)"/g)].map(m => m[1])
  assert.deepEqual(JIRA_OAUTH_SCOPES, asked)
})

// AC9 for the bundle: no module of the web app reads the OAuth app's
// environment, and the admin state type has no secret field.
test('the web app never holds the client secret', () => {
  const root = new URL('../src/', import.meta.url).pathname
  const files = []
  const walk = dir => {
    for (const name of readdirSync(dir)) {
      const path = join(dir, name)
      if (statSync(path).isDirectory()) walk(path)
      else if (/\.(ts|tsx|mjs|js)$/.test(name)) files.push(path)
    }
  }
  walk(root)
  for (const file of files) {
    assert.doesNotMatch(readFileSync(file, 'utf8'), /SECTILE_JIRA_OAUTH_/, file)
  }
  const app = readFileSync(new URL('../src/lib/jiraOAuthApp.ts', import.meta.url), 'utf8')
  const state = app.slice(app.indexOf('export interface JiraOAuthAppState'), app.indexOf('}', app.indexOf('export interface JiraOAuthAppState')))
  assert.doesNotMatch(state, /secret\s*[?]?:\s*string/i)
})

test('an outcome reads in the language given and names the sites when none was covered', async () => {
  const { translations } = await import('../src/locales/translations.ts')
  const { jiraOAuthOutcomeMessage, jiraOAuthOutcomeTone } = await import('../src/lib/jiraOAuth.ts')
  const en = translations.en.trackerCredentials.oauth.outcomes
  const fr = translations.fr.trackerCredentials.oauth.outcomes
  assert.match(jiraOAuthOutcomeMessage('no_site', en, ['https://equativ.atlassian.net']), /pick one of these sites: https:\/\/equativ\.atlassian\.net\./)
  assert.match(jiraOAuthOutcomeMessage('no_site', fr, ['https://a.atlassian.net', 'https://b.atlassian.net']), /https:\/\/a\.atlassian\.net, https:\/\/b\.atlassian\.net/)
  assert.match(jiraOAuthOutcomeMessage('no_site', en, []), /—/)
  assert.equal(jiraOAuthOutcomeMessage('connected', en, []), en.connected)
  assert.equal(jiraOAuthOutcomeTone('connected'), 'success')
  assert.equal(jiraOAuthOutcomeTone('cancelled'), 'info')
  for (const outcome of ['invalid', 'no_site', 'unreachable']) assert.equal(jiraOAuthOutcomeTone(outcome), 'error')
})
