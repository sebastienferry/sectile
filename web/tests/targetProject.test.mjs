import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sameTrackerInstance, targetProjectOptions, trackerAddress } from '../src/lib/lookups.ts'

// The same fixtures as internal/db/trackerinstance_test.go: the picker and the
// server must agree on what they offer and accept.
const jira = (id, trackerUrl) => ({ id, name: id, issueTracker: 'jira', trackerUrl })
const github = (id, githubRepo, githubApiUrl) => ({ id, name: id, issueTracker: 'github', githubRepo, githubApiUrl })
const local = id => ({ id, name: id, issueTracker: 'local' })

test('tracker addresses compare without scheme, case or trailing slash', () => {
  assert.equal(trackerAddress('https://Equativ.Atlassian.net/'), 'equativ.atlassian.net')
  assert.equal(trackerAddress('https://git.example.com/api/v4'), 'git.example.com/api/v4')
})

test('Jira projects of one site share an instance', () => {
  assert.equal(sameTrackerInstance(jira('a', 'https://equativ.atlassian.net'), jira('b', 'https://EQUATIV.atlassian.net/')), true)
  assert.equal(sameTrackerInstance(jira('a', 'https://equativ.atlassian.net'), jira('b', 'https://other.atlassian.net')), false)
  // An empty site falls back to the settings' one, as the server's credentials do.
  assert.equal(sameTrackerInstance(jira('a', ''), jira('b', 'https://equativ.atlassian.net'), { jiraUrl: 'https://equativ.atlassian.net' }), true)
  assert.equal(sameTrackerInstance(jira('a', ''), jira('b', '')), false)
})

test('GitHub projects share an instance only on one repository', () => {
  assert.equal(sameTrackerInstance(github('a', 'org/repo'), github('b', 'ORG/repo')), true)
  assert.equal(sameTrackerInstance(github('a', 'org/repo'), github('b', 'org/other')), false)
  assert.equal(sameTrackerInstance(github('a', 'org/repo'), github('b', 'org/repo', 'https://ghe.example.com/api/v3')), false)
})

test('mixed tracker kinds never share an instance, local boards always do', () => {
  assert.equal(sameTrackerInstance(local('a'), local('b')), true)
  assert.equal(sameTrackerInstance(local('a'), jira('b', 'https://x.atlassian.net')), false)
  assert.equal(sameTrackerInstance(github('a', 'org/repo'), jira('b', 'https://x.atlassian.net')), false)
})

test('the picker offers the other projects of the instance only', () => {
  const macro = jira('a', 'https://equativ.atlassian.net')
  const offered = targetProjectOptions(macro, [macro, jira('b', 'https://equativ.atlassian.net'), jira('c', 'https://other.atlassian.net'), local('d')])
  assert.deepEqual(offered.map(p => p.id), ['b'])
})
