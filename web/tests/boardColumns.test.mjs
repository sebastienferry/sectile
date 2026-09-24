import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  mergeDetectedColumns,
  pickBoardId,
  pruneStageColumns,
  recordedBoardId,
  shouldImportBoard,
  suggestedBoardId,
} from '../src/lib/boardColumns.ts'

test('detection produces one column per board column, with its grouped statuses', () => {
  const merged = mergeDetectedColumns([], [
    { name: 'To Do', statuses: ['To Do', 'Backlog'] },
    { name: 'In Progress', statuses: ['In Progress'] },
    { name: 'Done', statuses: ['Done'] },
  ])
  assert.deepEqual(merged.map(c => c.name), ['To Do', 'In Progress', 'Done'])
  assert.deepEqual(merged[0].statuses, ['To Do', 'Backlog'])
})

test('a status assigned by hand survives when the board claims it nowhere', () => {
  const merged = mergeDetectedColumns(
    [{ name: 'To Do', statuses: ['To Do', 'In Review'], hidden: true }],
    [{ name: 'To Do', statuses: ['To Do'] }],
  )
  assert.deepEqual(merged[0].statuses, ['To Do', 'In Review'])
  assert.equal(merged[0].hidden, true)
})

test('a status the board moved elsewhere is not kept twice', () => {
  const merged = mergeDetectedColumns(
    [{ name: 'To Do', statuses: ['To Do', 'Done'] }],
    [{ name: 'To Do', statuses: ['To Do'] }, { name: 'Done', statuses: ['Done'] }],
  )
  assert.deepEqual(merged.map(c => c.statuses), [['To Do'], ['Done']])
})

test('a column the board does not know is kept at the end, then dropped once emptied', () => {
  const kept = mergeDetectedColumns(
    [{ name: 'Peer review', statuses: ['In Review'] }],
    [{ name: 'To Do', statuses: ['To Do'] }],
  )
  assert.deepEqual(kept.map(c => c.name), ['To Do', 'Peer review'])

  const dropped = mergeDetectedColumns(
    [{ name: 'Peer review', statuses: ['In Review'] }],
    [{ name: 'To Do', statuses: ['In Review'] }],
  )
  assert.deepEqual(dropped.map(c => c.name), ['To Do'])
})

test('a stage mapped to a vanished column is dropped, the others are untouched', () => {
  const pruned = pruneStageColumns(
    { specified: ['To Do'], reviewed: ['Peer review'], implemented: ['To Do', 'Peer review'] },
    [{ name: 'To Do', statuses: [] }],
  )
  assert.deepEqual(pruned, { specified: ['To Do'], implemented: ['To Do'] })
})

test('the board picker keeps the project board, else the first scrum one, else the first', () => {
  const boards = [
    { id: '1', name: 'Kanban', type: 'kanban' },
    { id: '2', name: 'Scrum', type: 'scrum' },
  ]
  assert.equal(pickBoardId(boards, '1'), '1')
  assert.equal(pickBoardId(boards, ''), '2')
  assert.equal(pickBoardId(boards, 'gone'), '2')
  assert.equal(pickBoardId([{ id: '9', name: 'Only', type: 'kanban' }]), '9')
  assert.equal(pickBoardId([]), '')
})

const boards = [
  { id: '1', name: 'Kanban', type: 'kanban' },
  { id: '2', name: 'Scrum', type: 'scrum' },
]

test('the recorded board is the project board only while the tracker still lists it', () => {
  assert.equal(recordedBoardId(boards, '1'), '1')
  assert.equal(recordedBoardId(boards, ''), '')
  assert.equal(recordedBoardId(boards, undefined), '')
  assert.equal(recordedBoardId(boards, 'gone'), '')
})

test('the default board is suggested only while no board is recorded', () => {
  assert.equal(suggestedBoardId(boards, ''), '2')
  assert.equal(suggestedBoardId(boards, '1'), '')
  assert.equal(suggestedBoardId(boards, '2'), '')
  assert.equal(suggestedBoardId([], ''), '')
})

test('picking the suggested board imports it when no board is recorded', () => {
  assert.equal(shouldImportBoard(suggestedBoardId(boards, ''), ''), true)
  assert.equal(shouldImportBoard('1', ''), true)
})

test('picking another board imports it, the recorded board or the placeholder does not', () => {
  assert.equal(shouldImportBoard('2', '1'), true)
  assert.equal(shouldImportBoard('1', '1'), false)
  assert.equal(shouldImportBoard('', ''), false)
  assert.equal(shouldImportBoard('', '1'), false)
})
