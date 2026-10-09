import assert from 'node:assert/strict'
import { test } from 'node:test'
import { explicitStage, stageFromColumn, stageMove } from '../../shared/workflowStage.mjs'

const project = {
  trackerColumns: [
    { name: 'Todo', statuses: ['Open'] },
    { name: 'In Review', statuses: ['Code Review', 'QA'] },
    { name: 'Spec' },
  ],
  stageColumns: { implemented: ['In Review'], clarified: ['Spec'] },
}

test('a move replaces the workflow label and keeps the others', () => {
  const task = { labels: ['bug', '#Clarified', 'untouched', 'ui'], status: 'clarified', trackerStatus: 'Open' }
  assert.deepEqual(stageMove(task, 'specified', null).labels, ['bug', 'ui', '#specified'])
})

test('a move sets the internal status of the target stage', () => {
  const task = { labels: [], status: 'to_clarify' }
  const expected = { new: 'to_clarify', clarified: 'clarified', specified: 'to_implement', implemented: 'to_test', reviewed: 'to_close', finished: 'finished' }
  for (const [stage, status] of Object.entries(expected)) assert.equal(stageMove(task, stage, null).status, status, stage)
})

test('the tracker status is the first status of the first mapped column', () => {
  assert.equal(stageMove({ labels: [], trackerStatus: 'Open' }, 'implemented', project).trackerStatus, 'Code Review')
})

test('a mapped column without statuses gives its name', () => {
  assert.equal(stageMove({ labels: [], trackerStatus: 'Open' }, 'clarified', project).trackerStatus, 'Spec')
})

test('an unmapped stage keeps the task tracker status', () => {
  assert.equal(stageMove({ labels: [], trackerStatus: 'Open' }, 'reviewed', project).trackerStatus, 'Open')
  assert.equal(stageMove({ labels: [] }, 'reviewed', undefined).trackerStatus, undefined)
})

test('a ticket of a second tracker reads that tracker mapping', () => {
  const multi = {
    ...project,
    trackers: [{ trackerId: 't2', trackerColumns: [{ name: 'Doing', statuses: ['WIP'] }], stageColumns: { specified: ['Doing'] } }],
  }
  const task = { labels: [], trackerStatus: 'WIP', trackerId: 't2' }
  assert.equal(stageFromColumn(task, multi), 'specified')
  assert.equal(stageFromColumn({ ...task, trackerId: 'other' }, multi), null)
})

test('an explicit label names the stage, finished family first', () => {
  assert.equal(explicitStage({ labels: ['#done', '#new'] }), 'finished')
  assert.equal(explicitStage({ labels: ['Reviewed'] }), 'reviewed')
  assert.equal(explicitStage({ labels: ['untouched'] }), 'new')
  assert.equal(explicitStage({ labels: ['bug'] }), null)
  assert.equal(explicitStage({}), null)
})
