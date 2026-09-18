import assert from 'node:assert/strict'
import { test } from 'node:test'
import { PROJECT_TRACKERS, TRACKERS, canCheck, initialTracker, storedFor, trackerFields } from '../src/lib/trackers.ts'

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
  assert.equal(canCheck('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }), true)
  assert.equal(canCheck('jira', { siteUrl: 'acme.atlassian.net' }), false)
  assert.equal(canCheck('github', { siteUrl: 'https://api.github.com' }), true)
  assert.equal(canCheck('github', { siteUrl: '  ' }), false)
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
