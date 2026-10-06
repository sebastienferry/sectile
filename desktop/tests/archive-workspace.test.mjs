import test from 'node:test'
import assert from 'node:assert/strict'
import { archiveLabel, archiveRefusal, archiveFailure } from '../src/archive-workspace.mjs'

test('the archive label says a ticket task loses its worktree', () => {
 assert.equal(archiveLabel('#47'), 'Archive #47 and remove its worktree')
 assert.equal(archiveLabel('#47', { active: true }), 'Stop, archive #47 and remove its worktree')
 assert.equal(archiveLabel('Console', { ticket: false }), 'Archive Console')
 assert.equal(archiveLabel('Console', { active: true, ticket: false }), 'Stop and archive Console')
})

test('an archivable answer refuses nothing', () => {
 assert.equal(archiveRefusal({ archivable: true, repositories: [{ outcome: 'removed' }] }), '')
 assert.equal(archiveRefusal({ archivable: true, repositories: [{ outcome: 'shared' }, { outcome: 'absent' }] }), '')
})

test('a refusal names each worktree left behind and the way out', () => {
 const message = archiveRefusal({ archivable: false, repositories: [
  { repository: 'github.com/o/a', role: 'code', outcome: 'removed' },
  { repository: 'github.com/o/b', role: 'code', outcome: 'failed', error: 'contains modified or untracked files' },
  { repository: '', role: 'specifications', outcome: 'failed', error: 'locked' },
 ] })
 assert.equal(message, [
  'Not archived: a worktree of this task could not be removed.',
  'github.com/o/b: contains modified or untracked files',
  'Project checkout (specifications): locked',
  'Commit or discard the changes there, then archive again.',
 ].join('\n'))
 assert.doesNotMatch(message, /github\.com\/o\/a/)
})

test('an empty answer is an agent to update', () => {
 assert.match(archiveRefusal({ archivable: false, repositories: [] }), /Update and restart the agent/)
 assert.match(archiveRefusal(null), /^Not archived/)
})

test('a transport failure says to start or update the agent', () => {
 assert.equal(archiveFailure(new Error("Error invoking remote method 'archive-workspace': Error: connect ECONNREFUSED")),
  'Not archived: connect ECONNREFUSED\nStart or update the local agent, then archive again.')
})
