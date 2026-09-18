import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  PROJECT_TRACKERS,
  TRACKERS,
  canCheck,
  credentialState,
  initialTracker,
  saveBlockedReason,
  scopesFor,
  sealingConsequence,
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

test('the screen states what sealing costs, at the moment of the choice', () => {
  // A sealed token cannot be opened by the server alone, so nothing running
  // without its owner can use it. Saying it afterwards would be too late.
  assert.match(sealingConsequence(true), /phrase/)
  assert.match(sealingConsequence(true), /file de fond/)
  assert.match(sealingConsequence(false), /clé du serveur/)
  assert.notEqual(sealingConsequence(true), sealingConsequence(false))
})

test('a personal credential reads as absent, stored, sealed or locked', () => {
  assert.match(credentialState(undefined), /jeton du serveur/)
  assert.match(credentialState({ tracker: 'jira', sealed: false, unlocked: true }), /clé du serveur/)
  assert.match(credentialState({ tracker: 'jira', sealed: true, unlocked: true }), /déverrouillé/)
  assert.match(credentialState({ tracker: 'jira', sealed: true, unlocked: false }), /verrouillé/)
})
