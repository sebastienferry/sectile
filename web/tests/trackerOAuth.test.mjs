import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'
import {
  FORGE_OAUTH_OUTCOMES,
  JIRA_OAUTH_OUTCOMES,
  NO_OAUTH,
  OAUTH_TRACKERS,
  connectPath,
  entryState,
  notConfiguredCode,
  oauthFor,
  oauthFrom,
  oauthOutcomeFromSearch,
  oauthOutcomeMessage,
  oauthStrings,
  trackerOAuthFrom,
  withoutOAuthOutcome,
} from '../src/lib/trackerOAuth.ts'
import { OAUTH_APP_SCOPES, oauthAppPath, oauthAppPayload, oauthAppRedirectPlaceholder } from '../src/lib/oauthApp.ts'
import { sealableCredentials } from '../src/lib/trackers.ts'

const configured = { configured: true, sites: [] }
const notConfigured = { configured: false, sites: [] }

test('every OAuth tracker gets the same entry states', () => {
  assert.deepEqual(OAUTH_TRACKERS, ['jira', 'github', 'gitlab'])
  for (const tracker of OAUTH_TRACKERS) {
    const token = { tracker, kind: 'api_token', sealed: false, unlocked: true }
    const grant = { tracker, kind: 'oauth', sealed: false, unlocked: true, account: 'ada' }
    assert.equal(entryState(undefined, notConfigured), 'form', tracker)
    assert.equal(entryState(token, notConfigured), 'form', tracker)
    assert.equal(entryState(undefined, configured), 'connect', tracker)
    assert.equal(entryState(token, configured), 'token-and-connect', tracker)
    assert.equal(entryState(grant, configured), 'connected', tracker)
    assert.equal(entryState({ ...grant, disconnected: true }, configured), 'disconnected', tracker)
    // A grant keeps its state once the app is no longer configured.
    assert.equal(entryState(grant, notConfigured), 'connected', tracker)
    assert.equal(entryState({ ...grant, disconnected: true }, undefined), 'disconnected', tracker)
  }
})

test('Connect stays hidden while a forge app is not configured', () => {
  // Nothing in the answer: no tracker offers Connect.
  const none = oauthFrom({})
  assert.deepEqual(none, NO_OAUTH)
  for (const tracker of OAUTH_TRACKERS) {
    assert.equal(oauthFor(none, tracker)?.configured, false, tracker)
    assert.equal(entryState(undefined, oauthFor(none, tracker)), 'form', tracker)
  }
  // One forge configured offers Connect on that forge alone.
  const github = oauthFrom({ githubOAuth: { configured: true }, gitlabOAuth: { configured: false } })
  assert.equal(entryState(undefined, oauthFor(github, 'github')), 'connect')
  assert.equal(entryState(undefined, oauthFor(github, 'gitlab')), 'form')
  assert.equal(entryState(undefined, oauthFor(github, 'jira')), 'form')
  // A tracker that never connects through OAuth has no availability at all.
  assert.equal(oauthFor(github, 'local'), undefined)
})

test('a forge availability is read leniently and never names sites', () => {
  assert.deepEqual(trackerOAuthFrom({ gitlabOAuth: null }, 'gitlab'), { configured: false, sites: [] })
  assert.deepEqual(trackerOAuthFrom({ githubOAuth: { configured: 'yes' } }, 'github'), { configured: false, sites: [] })
  assert.deepEqual(trackerOAuthFrom({ githubOAuth: { configured: true, sites: ['x'] } }, 'github'), { configured: true, sites: [] })
  assert.deepEqual(trackerOAuthFrom({ jiraOAuth: { configured: true, sites: ['a', 3] } }, 'jira'), { configured: true, sites: ['a'] })
})

test('a forge grant is never sealed', () => {
  const grant = { tracker: 'gitlab', kind: 'oauth', sealed: false, unlocked: true, account: 'ada' }
  const token = { tracker: 'github', sealed: true, unlocked: false }
  assert.deepEqual(sealableCredentials([grant, token]), [token])
})

test('a forge outcome is read from oauth=, in any order, and Jira keeps jiraOAuth=', () => {
  assert.deepEqual(FORGE_OAUTH_OUTCOMES, ['connected', 'cancelled', 'invalid', 'unreachable'])
  for (const tracker of ['github', 'gitlab']) {
    for (const outcome of FORGE_OAUTH_OUTCOMES) {
      assert.deepEqual(oauthOutcomeFromSearch(`?oauth=${outcome}&trackerCredentials=${tracker}`), { tracker, outcome })
      assert.deepEqual(oauthOutcomeFromSearch(`?trackerCredentials=${tracker}&oauth=${outcome}`), { tracker, outcome })
    }
    // A forge has no site to miss, and does not read Jira's parameter.
    assert.equal(oauthOutcomeFromSearch(`?trackerCredentials=${tracker}&oauth=no_site`), null)
    assert.equal(oauthOutcomeFromSearch(`?trackerCredentials=${tracker}&jiraOAuth=connected`), null)
    assert.equal(oauthOutcomeFromSearch(`?trackerCredentials=${tracker}&oauth=pwned`), null)
  }
  for (const outcome of JIRA_OAUTH_OUTCOMES) {
    assert.deepEqual(oauthOutcomeFromSearch(`?trackerCredentials=jira&jiraOAuth=${outcome}`), { tracker: 'jira', outcome })
  }
  assert.equal(oauthOutcomeFromSearch('?trackerCredentials=jira&oauth=connected'), null)
  assert.equal(oauthOutcomeFromSearch('?trackerCredentials=local&oauth=connected'), null)
  assert.equal(oauthOutcomeFromSearch('?oauth=connected'), null)
  assert.equal(oauthOutcomeFromSearch(''), null)
})

test('both callback forms are dropped from the address, the rest kept', () => {
  assert.equal(withoutOAuthOutcome('?trackerCredentials=github&oauth=connected'), '')
  assert.equal(withoutOAuthOutcome('?oauth=invalid&view=board&trackerCredentials=gitlab'), '?view=board')
  assert.equal(withoutOAuthOutcome('?view=board&trackerCredentials=jira&jiraOAuth=no_site'), '?view=board')
})

test('Connect posts to its tracker and recognises its own not-configured refusal', () => {
  assert.equal(connectPath('github'), '/me/tracker-credentials/github/connect')
  assert.equal(connectPath('gitlab'), '/me/tracker-credentials/gitlab/connect')
  assert.equal(connectPath('jira'), '/me/tracker-credentials/jira/connect')
  assert.equal(notConfiguredCode('github'), 'github_oauth_not_configured')
  assert.equal(notConfiguredCode('gitlab'), 'gitlab_oauth_not_configured')
})

test('every forge outcome reads in both languages', async () => {
  const { translations } = await import('../src/locales/translations.ts')
  for (const lang of ['fr', 'en']) {
    const t = translations[lang]
    for (const tracker of ['github', 'gitlab']) {
      const strings = oauthStrings(t, tracker)
      assert.deepEqual(Object.keys(strings.outcomes).sort(), [...FORGE_OAUTH_OUTCOMES].sort(), `${lang} ${tracker}`)
      for (const outcome of FORGE_OAUTH_OUTCOMES) {
        assert.equal(oauthOutcomeMessage(outcome, strings.outcomes, []), strings.outcomes[outcome])
      }
    }
    assert.equal(oauthStrings(t, 'jira'), t.trackerCredentials.oauth)
  }
})

test('the admin panel talks to its tracker and sends a secret only when one was typed', () => {
  assert.equal(oauthAppPath('jira'), '/api/admin/jira-oauth')
  assert.equal(oauthAppPath('github'), '/api/admin/github-oauth')
  assert.equal(oauthAppPath('gitlab'), '/api/admin/gitlab-oauth')
  assert.equal(oauthAppRedirectPlaceholder('github'), 'https://sectile.example.com/auth/github/callback')
  assert.equal(oauthAppRedirectPlaceholder('gitlab'), 'https://sectile.example.com/auth/gitlab/callback')
  assert.deepEqual(oauthAppPayload({ clientId: ' c ', clientSecret: '  ', redirectUrl: ' https://x/cb ' }), { clientId: 'c', redirectUrl: 'https://x/cb' })
  assert.deepEqual(oauthAppPayload({ clientId: 'c', clientSecret: ' s ', redirectUrl: 'u' }), { clientId: 'c', redirectUrl: 'u', clientSecret: 's' })
})

test('the forge scopes shown to the admin are the ones the server asks for', () => {
  assert.deepEqual(OAUTH_APP_SCOPES.github, ['repo', 'read:project'])
  assert.deepEqual(OAUTH_APP_SCOPES.gitlab, ['api'])
  const server = readFileSync(new URL('../../internal/forgeoauth/oauth.go', import.meta.url), 'utf8')
  for (const [name, tracker] of [['GitHub', 'github'], ['GitLab', 'gitlab']]) {
    const block = server.slice(server.indexOf(`var ${name} = Provider{`))
    const line = block.slice(block.indexOf('Scopes:'), block.indexOf('\n', block.indexOf('Scopes:')))
    assert.deepEqual([...OAUTH_APP_SCOPES[tracker]], [...line.matchAll(/"([^"]+)"/g)].map(m => m[1]), tracker)
  }
})

// The bundle never reads an OAuth app's environment, and the admin state type
// has no secret field.
test('the web app never holds a client secret', () => {
  const root = fileURLToPath(new URL('../src/', import.meta.url))
  const files = []
  const walk = dir => {
    for (const name of readdirSync(dir)) {
      const path = join(dir, name)
      if (statSync(path).isDirectory()) walk(path)
      else if (/\.(ts|tsx|mjs|js)$/.test(name)) files.push(path)
    }
  }
  walk(root)
  assert.ok(files.length > 0)
  for (const file of files) {
    assert.doesNotMatch(readFileSync(file, 'utf8'), /SECTILE_(JIRA|GITHUB|GITLAB)_OAUTH_/, file)
  }
  const app = readFileSync(new URL('../src/lib/oauthApp.ts', import.meta.url), 'utf8')
  const start = app.indexOf('export interface OAuthAppState')
  assert.notEqual(start, -1)
  const state = app.slice(start, app.indexOf('}', start))
  assert.match(state, /secretSet: boolean/)
  assert.doesNotMatch(state, /secret\s*[?]?:\s*string/i)
})
