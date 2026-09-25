import test from 'node:test'
import assert from 'node:assert/strict'
import { myTasksTooltip, trackerLabel } from '../src/lib/myTasks.ts'

const strings = {
  myTasks: 'My Tasks',
  myTasksFallback: 'On {trackers}, your name and e-mail are used: save or verify a personal credential in your profile.',
  myTasksSignedOut: 'Signed out: the name and e-mail of the local profile are used.',
}

test('myTasksTooltip is the label alone when every tracker knows me', () => {
  const identities = {
    signedIn: true,
    fallback: ['Ada', 'ada@example.com'],
    trackers: [{ tracker: 'github', identity: 'ada', known: true }],
  }
  assert.equal(myTasksTooltip(identities, strings), 'My Tasks')
  // A scope with local tickets only has no tracker to name.
  assert.equal(myTasksTooltip({ ...identities, trackers: [] }, strings), 'My Tasks')
  // Nothing fetched yet says nothing it does not know.
  assert.equal(myTasksTooltip(null, strings), 'My Tasks')
})

test('myTasksTooltip names the trackers that fall back on the name and e-mail', () => {
  const identities = {
    signedIn: true,
    fallback: ['Ada'],
    trackers: [
      { tracker: 'github', identity: 'ada', known: true },
      { tracker: 'jira', known: false },
      { tracker: 'gitlab', known: false },
    ],
  }
  assert.equal(
    myTasksTooltip(identities, strings),
    'My Tasks\nOn Jira, GitLab, your name and e-mail are used: save or verify a personal credential in your profile.'
  )
})

test('myTasksTooltip says a signed-out board uses the local profile', () => {
  const identities = { signedIn: false, fallback: ['Ada'], trackers: [{ tracker: 'jira', known: false }] }
  assert.equal(myTasksTooltip(identities, strings), 'My Tasks\nSigned out: the name and e-mail of the local profile are used.')
})

test('trackerLabel spells the known trackers and keeps the others', () => {
  assert.equal(trackerLabel('github'), 'GitHub')
  assert.equal(trackerLabel('jira'), 'Jira')
  assert.equal(trackerLabel('azure'), 'azure')
})
