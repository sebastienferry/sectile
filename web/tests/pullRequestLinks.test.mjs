import assert from 'node:assert/strict'
import { test } from 'node:test'
import { addPullRequestLink, currentPullRequest, taskPullRequestLinks } from '../src/lib/pullRequests.ts'

test('a task carrying only the legacy single URL reads as one link', () => {
  const links = taskPullRequestLinks({ prUrl: 'https://forge/pull/1', branchName: 'feat/1' })
  assert.deepEqual(links, [{ url: 'https://forge/pull/1', branch: 'feat/1' }])
  // Same value on both sides of the unsaved-changes comparison, so opening the
  // card does not by itself count as an edit.
  assert.equal(
    JSON.stringify(links),
    JSON.stringify(taskPullRequestLinks({ prUrl: 'https://forge/pull/1', branchName: 'feat/1' })),
  )
})

test('the ordered set wins over the legacy single URL', () => {
  const links = taskPullRequestLinks({
    prLinks: [{ url: 'https://forge/pull/1', branch: 'feat/1' }, { url: 'https://forge/pull/2', branch: 'feat/1' }],
    prUrl: 'https://forge/pull/2',
    branchName: 'feat/1',
  })
  assert.equal(links.length, 2)
  assert.equal(currentPullRequest(links), 'https://forge/pull/2')
})

test('a task with no pull request reads as an empty set', () => {
  assert.deepEqual(taskPullRequestLinks({}), [])
  assert.equal(currentPullRequest([]), undefined)
})

test('adding a link appends it and makes it current', () => {
  const links = addPullRequestLink([{ url: 'https://forge/pull/1', branch: 'feat/1' }], '  https://forge/pull/2 ', 'feat/1')
  assert.deepEqual(links, [
    { url: 'https://forge/pull/1', branch: 'feat/1' },
    { url: 'https://forge/pull/2', branch: 'feat/1' },
  ])
  assert.equal(currentPullRequest(links), 'https://forge/pull/2')
})

test('adding an already recorded URL leaves the set untouched', () => {
  const existing = [{ url: 'https://forge/pull/1', branch: 'feat/1' }]
  assert.equal(addPullRequestLink(existing, 'https://forge/pull/1', 'feat/1'), existing)
  assert.equal(addPullRequestLink(existing, '   ', 'feat/1'), existing)
})
