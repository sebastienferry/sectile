import assert from 'node:assert/strict'
import { test } from 'node:test'

const { STAGES, skillLabel } = await import('../src/workflow.mjs')

test('every workflow skill reads as the workflow names it', () => {
  assert.equal(skillLabel('clarify'), 'Clarify')
  assert.equal(skillLabel('specify'), 'Specify')
  assert.equal(skillLabel('implement'), 'Implement')
  assert.equal(skillLabel('adjust'), 'Adjust')
  assert.equal(skillLabel('handoff'), 'Handoff')
  assert.equal(skillLabel('create_pr'), 'Create PR')
})

test('any other skill is capitalized', () => {
  assert.equal(skillLabel('pickup'), 'Pickup')
  assert.equal(skillLabel('discuss'), 'Discuss')
  assert.equal(skillLabel(' custom '), 'Custom')
})

test('a missing skill has no label', () => {
  assert.equal(skillLabel(''), '')
  assert.equal(skillLabel('   '), '')
  assert.equal(skillLabel(undefined), '')
  assert.equal(skillLabel(null), '')
})

test('the stages are listed in workflow order', () => {
  assert.deepEqual(STAGES, ['new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished'])
})

const { taskStage } = await import('../src/workflow.mjs')

const board = {
  trackerColumns: [{ name: 'In Review', statuses: ['Code Review'] }, { name: 'Todo', statuses: ['Open'] }],
  stageColumns: { implemented: ['In Review'], new: ['Todo'] },
  trackers: [{ trackerId: 't2', trackerColumns: [{ name: 'Doing', statuses: ['WIP'] }], stageColumns: { specified: ['Doing'] } }],
}

test('without a board the stage reads as before', () => {
  assert.equal(taskStage({ labels: [], status: 'to_test', trackerStatus: 'Code Review' }), 'implemented')
  assert.equal(taskStage({ labels: [], status: 'to_clarify', trackerStatus: 'Code Review' }), 'new')
  assert.equal(taskStage({ labels: ['#Reviewed'], status: 'to_clarify' }), 'reviewed')
  assert.equal(taskStage({ labels: [], status: 'done' }), 'finished')
})

test('an explicit label beats the column', () => {
  assert.equal(taskStage({ labels: [' #clarified '], status: 'to_clarify', trackerStatus: 'Code Review' }, board), 'clarified')
  assert.equal(taskStage({ labels: ['#closed'], status: 'to_clarify', trackerStatus: 'Open' }, board), 'finished')
})

test('the column beats the status', () => {
  assert.equal(taskStage({ labels: ['bug'], status: 'to_clarify', trackerStatus: 'code review' }, board), 'implemented')
})

test('a finished status stays finished whatever the column', () => {
  assert.equal(taskStage({ labels: [], status: 'finished', trackerStatus: 'Open' }, board), 'finished')
})

test('an unmapped column falls back on the status', () => {
  assert.equal(taskStage({ labels: [], status: 'to_close', trackerStatus: 'Elsewhere' }, board), 'reviewed')
})

test('a ticket of a second tracker reads that tracker mapping', () => {
  assert.equal(taskStage({ labels: [], status: 'to_clarify', trackerStatus: 'WIP', trackerId: 't2' }, board), 'specified')
  assert.equal(taskStage({ labels: [], status: 'to_clarify', trackerStatus: 'Code Review', trackerId: 't2' }, board), 'new')
})
