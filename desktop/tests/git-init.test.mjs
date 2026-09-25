import assert from 'node:assert/strict'
import { test } from 'node:test'

const { offerFor, initializedNotice } = await import('../src/git-init.mjs')

test('only a plain folder and an unborn repository are offered an initialization', () => {
  for (const state of ['ready', 'missing', 'unknown', undefined, ''])
    assert.equal(offerFor(state), null, String(state))
  assert.equal(offerFor('folder').title, 'This folder is not a Git repository.')
  assert.equal(offerFor('unborn').title, 'This Git repository has no commit yet.')
  for (const state of ['folder', 'unborn']) {
    assert.equal(offerFor(state).action, 'Initialize a Git repository')
    assert.equal(offerFor(state).dismiss, 'Not now')
  }
})

test('the offer says the repository stays local and worktrees start empty', () => {
  const { detail } = offerFor('folder')
  for (const point of ['stays on this workstation', 'never pushed', 'no remote', 'worktrees', 'existing files'])
    assert.ok(detail.includes(point), point)
  assert.equal(offerFor('unborn').detail, detail)
})

test('the success notice names the folder and asks for a save', () => {
  assert.equal(initializedNotice('/work/notes'), 'Git repository initialized in /work/notes. Save the local configuration to use it.')
})
