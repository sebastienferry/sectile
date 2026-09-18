import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  PERSONAL_TRACKERS,
  PROJECT_TRACKERS,
  prefillFromCredential,
  TRACKERS,
  canCheck,
  credentialState,
  initialTracker,
  saveBlockedReason,
  scopesFor,
  sealingConsequence,
  SEALING_INVITATION,
  storedFor,
  trackerFields,
} from '../src/lib/trackers.ts'

test('the three trackers are offered, Jira asking for an account e-mail', () => {
  assert.deepEqual(TRACKERS.map(t => t.id), ['jira', 'github', 'gitlab'])
  assert.equal(trackerFields('jira').wantsEmail, true)
  assert.equal(trackerFields('github').wantsEmail, false)
  assert.equal(trackerFields('gitlab').wantsEmail, false)
  // Jira takes a project key, which is what the sync queries on.
  assert.equal(trackerFields('jira').projectPlaceholder, 'PE')
})

test('the screen opens on the tracker the project already uses', () => {
  assert.equal(initialTracker('github'), 'github')
  assert.equal(initialTracker('jira'), 'jira')
  assert.equal(initialTracker('gitlab'), 'gitlab')
  assert.equal(initialTracker('local'), 'jira')
  assert.equal(initialTracker(undefined), 'jira')
})

test('each tracker is prefilled from its own stored values', () => {
  const settings = {
    jiraUrl: 'https://acme.atlassian.net',
    jiraProject: 'PE',
    jiraApiTokenSet: true,
    githubApiUrl: 'https://api.github.com',
    githubRepo: 'acme/app',
    githubTokenFromEnv: true,
    gitlabUrl: 'https://gitlab.com/api/v4',
    gitlabProject: 'group/app',
  }
  assert.deepEqual(storedFor(settings, 'jira'), {
    siteUrl: 'https://acme.atlassian.net',
    project: 'PE',
    tokenIsSet: true,
    tokenFromEnv: false,
  })
  assert.deepEqual(storedFor(settings, 'github'), {
    siteUrl: 'https://api.github.com',
    project: 'acme/app',
    tokenIsSet: false,
    tokenFromEnv: true,
  })
  assert.equal(storedFor(settings, 'gitlab').project, 'group/app')
  // An empty configuration shows empty fields rather than undefined.
  assert.deepEqual(storedFor({}, 'jira'), { siteUrl: '', project: '', tokenIsSet: false, tokenFromEnv: false })
})

test('a token may stay empty: the check revalidates the stored one', () => {
  // An Atlassian account belongs to a site, so the personal card carries both.
  assert.equal(canCheck('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }), true)
  assert.equal(canCheck('jira', { email: 'ada@example.com' }), false)
  assert.equal(canCheck('jira', { siteUrl: 'acme.atlassian.net' }), false)
  // GitHub and GitLab have a public instance the server falls back to, so an
  // empty site must not block their check either.
  assert.equal(canCheck('github', { siteUrl: '' }), true)
  assert.equal(canCheck('gitlab', {}), true)
})

test('a blocked save says why, rather than greying a button in silence', () => {
  assert.match(saveBlockedReason('jira', {}, false), /site/)
  assert.match(saveBlockedReason('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }, false), /Vérifiez les accès/)
  assert.match(saveBlockedReason('github', {}, false), /Vérifiez/)
  // Once the instance accepted them, nothing is in the way any more.
  assert.equal(saveBlockedReason('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }, true), '')
})

test('a tracker that attributes its writes is personal only', () => {
  // The server token stays a configuration fallback, never a box in the screen.
  assert.deepEqual(scopesFor('jira'), ['personal'])
  assert.deepEqual(scopesFor('github'), ['server', 'personal'])
  assert.deepEqual(scopesFor('gitlab'), ['server', 'personal'])
  // The site belongs to the person, not to the server: an Atlassian account is
  // tied to its instance.
  assert.equal(trackerFields('jira').siteIsPersonal, true)
  assert.equal(trackerFields('github').siteIsPersonal, undefined)
})

test('every tracker with a server adapter can be set on a project', () => {
  // The two selectors (project card, sync view) share this list because they
  // drifted once: Jira left the project card and stayed in the sync view, so no
  // project could be put on the tracker the server knew how to drive.
  assert.deepEqual(PROJECT_TRACKERS.map(t => t.id), ['local', 'github', 'jira'])
  // GitLab parameters are storable, but no adapter is registered for it.
  assert.equal(PROJECT_TRACKERS.some(t => t.id === 'gitlab'), false)
  assert.equal(PROJECT_TRACKERS.every(t => t.label.trim().length > 0), true)
})

test('the screen says what sealing asks of you, not how it works', () => {
  // What a reader can act on: they will have to unseal. A key and a cipher
  // teach them nothing, so neither appears.
  assert.match(SEALING_INVITATION, /desceller pour agir sur les tâches/)
  assert.match(sealingConsequence(true), /desceller/)
  assert.notEqual(sealingConsequence(true), sealingConsequence(false))
  for (const text of [SEALING_INVITATION, sealingConsequence(true), sealingConsequence(false)]) {
    assert.doesNotMatch(text, /chiffr|clé du serveur|base de données/i)
  }
})

test('a personal credential reads as absent, stored, sealed or locked', () => {
  assert.match(credentialState(undefined), /ne partiront pas/)
  assert.match(credentialState({ tracker: 'jira', sealed: false, unlocked: true }), /sous votre compte/)
  assert.match(credentialState({ tracker: 'jira', sealed: true, unlocked: true }), /descellé/)
  assert.match(credentialState({ tracker: 'jira', sealed: true, unlocked: false }), /descellez/)
})

test('a stored credential puts its site and e-mail back in the form', () => {
  // Without the e-mail the check button stays disabled, so the save can never
  // unlock: the screen would ask again for what the person already gave.
  const mine = { tracker: 'jira', siteUrl: 'acme.atlassian.net', email: 'ada@example.com', sealed: true, unlocked: false }
  assert.deepEqual(prefillFromCredential(mine, {}), { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' })
  assert.equal(canCheck('jira', prefillFromCredential(mine, {})), true)
  // What is being typed wins over what is stored.
  assert.deepEqual(prefillFromCredential(mine, { siteUrl: 'other.atlassian.net', email: 'bob@example.com' }), {
    siteUrl: 'other.atlassian.net',
    email: 'bob@example.com',
  })
  assert.deepEqual(prefillFromCredential(undefined, {}), { siteUrl: '', email: '' })
})

test('only a tracker the server can drive offers a personal credential', () => {
  // A personal GitLab token was storable and shown as active while no adapter
  // was registered for GitLab at all: nothing could ever have used it.
  assert.deepEqual(PERSONAL_TRACKERS.map(t => t.id), ['jira', 'github'])
  assert.equal(TRACKERS.some(t => t.id === 'gitlab'), true)
  // And every tracker a project can be put on can hold a personal credential.
  for (const t of PROJECT_TRACKERS) {
    if (t.id === 'local') continue
    assert.equal(PERSONAL_TRACKERS.some(p => p.id === t.id), true, `${t.id} has an adapter but no personal credential`)
  }
})
