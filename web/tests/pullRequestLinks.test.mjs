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

test('a pull request URL names its repository the way the server does', async () => {
  const { pullRequestRepository } = await import('../src/lib/pullRequests.ts')
  assert.equal(pullRequestRepository('https://github.com/O/App/pull/12'), 'github.com/o/app')
  assert.equal(pullRequestRepository('https://gitlab.example.org/g/sub/deploy/-/merge_requests/3'), 'gitlab.example.org/g/sub/deploy')
  assert.equal(pullRequestRepository('https://example.org/o/app/pull/1'), '')
  assert.equal(pullRequestRepository('https://github.com/o/app/pull/0'), '')
  assert.equal(pullRequestRepository('not a url'), '')
})

test('links are grouped per repository, the primary one first', async () => {
  const { repositoryPullRequests } = await import('../src/lib/pullRequests.ts')
  const links = [
    { url: 'https://github.com/o/app/pull/1', branch: 'feat/1' },
    { url: 'https://gitlab.com/g/deploy/-/merge_requests/7', branch: 'feat/1' },
    { url: 'https://example.org/legacy', branch: 'feat/1' },
    { url: 'https://github.com/o/app/pull/2', branch: 'feat/1' },
  ]
  const groups = repositoryPullRequests(links)
  assert.deepEqual(groups.map(g => [g.repository, g.indices]), [
    ['github.com/o/app', [0, 2, 3]],
    ['gitlab.com/g/deploy', [1]],
  ])
  assert.equal(groups[0].current.url, 'https://github.com/o/app/pull/2')
  assert.equal(groups[1].current.url, 'https://gitlab.com/g/deploy/-/merge_requests/7')
  assert.deepEqual(repositoryPullRequests([]), [])
})

test('a link of another repository is added before the current one', () => {
  const links = [{ url: 'https://github.com/o/app/pull/1', branch: 'feat/1' }]
  const next = addPullRequestLink(links, 'https://gitlab.com/g/deploy/-/merge_requests/7', 'feat/1')
  assert.deepEqual(next.map(l => l.url), ['https://gitlab.com/g/deploy/-/merge_requests/7', 'https://github.com/o/app/pull/1'])
  const followUp = addPullRequestLink(next, 'https://github.com/o/app/pull/2', 'feat/1')
  assert.equal(currentPullRequest(followUp), 'https://github.com/o/app/pull/2')
})

test('an unknown state says which token is missing', async () => {
  const { pullRequestStateLabel } = await import('../src/lib/pullRequests.ts')
  const states = { open: 'open', conflicting: 'conflicting', merged: 'merged', closed: 'closed', unknown: 'unknown', missingToken: 'no {forge} token' }
  assert.equal(pullRequestStateLabel({ url: 'u', missingToken: 'gitlab' }, states), 'no GitLab token')
  assert.equal(pullRequestStateLabel({ url: 'u', state: 'open', missingToken: 'gitlab' }, states), 'open')
  assert.equal(pullRequestStateLabel({ url: 'u' }, states), 'unknown')
  assert.equal(pullRequestStateLabel(undefined, states), 'unknown')
})
