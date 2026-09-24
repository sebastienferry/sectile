import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextBatchStart, nextSprintAfter, sprintManagementOf, sprintTarget } from '../src/lib/sprints.ts'

test('who owns the sprints follows the tracker', () => {
  assert.equal(sprintManagementOf({ issueTracker: 'jira' }), 'tracker')
  assert.equal(sprintManagementOf({ issueTracker: 'github', githubRepo: 'org/repo' }), 'readonly')
  assert.equal(sprintManagementOf({ issueTracker: '', githubRepo: 'org/repo' }), 'readonly')
  assert.equal(sprintManagementOf({ issueTracker: 'local' }), 'local')
  assert.equal(sprintManagementOf(null), 'local')
})

test('a tracker moves work items by sprint id, the local board by name', () => {
  const sprints = [{ id: '42', name: 'Sprint 1', state: 'active' }]
  assert.deepEqual(sprintTarget(sprints, 'Sprint 1', 'tracker'), { id: '42', name: 'Sprint 1' })
  assert.deepEqual(sprintTarget(sprints, '42', 'tracker'), { id: '42', name: 'Sprint 1' })
  assert.deepEqual(sprintTarget(sprints, 'sprint 1', 'local'), { id: 'Sprint 1', name: 'Sprint 1' })
  assert.deepEqual(sprintTarget(sprints, '', 'tracker'), { id: '', name: '' })
})

test('a new batch starts the day after the last sprint ends', () => {
  const sprints = [{ id: '1', name: 'S1', state: 'closed', endDate: '2026-10-18' }, { id: '2', name: 'S2', state: 'future', endDate: '2026-11-01' }]
  assert.equal(nextBatchStart(sprints, '2026-01-01'), '2026-11-02')
  assert.equal(nextBatchStart([], '2026-01-01'), '2026-01-01')
})

test('a batch continues from a tracker sprint without a gap day', () => {
  // The server ends a sprint one second before the next starts at 09:00.
  const sprints = [{ id: '1', name: 'S1', state: 'future', endDate: '2026-10-19T08:59:59+02:00' }]
  assert.equal(nextBatchStart(sprints, '2026-01-01'), '2026-10-19')
})

test('the next sprint is chosen by start date, not array order', () => {
  const closing = { id: '1', name: 'S1', state: 'active', startDate: '2026-10-05T09:00:00Z' }
  const later = { id: '3', name: 'S3', state: 'future', startDate: '2026-11-02T09:00:00Z' }
  const sooner = { id: '2', name: 'S2', state: 'future', startDate: '2026-10-19T09:00:00Z' }
  const closed = { id: '4', name: 'S0', state: 'closed', startDate: '2026-10-12T09:00:00Z' }
  assert.equal(nextSprintAfter([closing, later, closed, sooner], closing).id, '2')
  assert.equal(nextSprintAfter([closing, closed], closing), null)
})
