import assert from 'node:assert/strict'
import { test } from 'node:test'

const { EMPTY_BOARD, boardOptions, saveBoardOption, boardColumns, boardCardLabels, cardEpicColor } = await import('../src/board.mjs')

function memoryStorage(){
  const values = new Map()
  return { getItem: key => values.has(key) ? values.get(key) : null, setItem: (key, value) => values.set(key, String(value)), values }
}
const throwing = { getItem(){ throw new Error('denied') }, setItem(){ throw new Error('denied') } }

test('the finished column starts hidden and cards condensed', () => {
  assert.deepEqual(boardOptions(memoryStorage()), { hideFinished: true, cardDisplay: 'condensed' })
  assert.deepEqual(boardOptions(undefined), { hideFinished: true, cardDisplay: 'condensed' })
})

test('saved options are read back', () => {
  const storage = memoryStorage()
  saveBoardOption(storage, 'hideFinished', false)
  saveBoardOption(storage, 'cardDisplay', 'full')
  assert.deepEqual(boardOptions(storage), { hideFinished: false, cardDisplay: 'full' })
  saveBoardOption(storage, 'hideFinished', true)
  assert.equal(boardOptions(storage).hideFinished, true)
})

test('an unknown card display is ignored', () => {
  const storage = memoryStorage()
  saveBoardOption(storage, 'cardDisplay', 'huge')
  assert.equal(storage.values.size, 0)
  storage.setItem('boardCardDisplay', 'huge')
  assert.equal(boardOptions(storage).cardDisplay, 'condensed')
})

test('a throwing storage reads the defaults and refuses writes quietly', () => {
  assert.deepEqual(boardOptions(throwing), { hideFinished: true, cardDisplay: 'condensed' })
  assert.doesNotThrow(() => saveBoardOption(throwing, 'cardDisplay', 'full'))
})

test('the six columns come in workflow order, empty ones included', () => {
  const columns = boardColumns([{ id: 'a', key: '#1', status: 'to_test' }], EMPTY_BOARD)
  assert.deepEqual(columns.map(column => column.stage), ['new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished'])
  assert.deepEqual(columns.map(column => column.tasks.length), [0, 0, 0, 1, 0, 0])
})

test('every task lands in exactly one column, finished and unknown ones included', () => {
  const tasks = [
    { id: 'a', key: '#1', status: 'finished' },
    { id: 'b', key: '#2', labels: ['#closed'] },
    { id: 'c', key: '#3', status: 'weird' },
    { id: 'd', key: '#4', status: 'to_close' },
  ]
  const columns = boardColumns(tasks, undefined)
  const placed = Object.fromEntries(columns.flatMap(column => column.tasks.map(task => [task.id, column.stage])))
  assert.deepEqual(placed, { a: 'finished', b: 'finished', c: 'new', d: 'reviewed' })
})

test('the stage mapping places a task by its tracker column', () => {
  const board = { epicColors: false, trackerColumns: [{ name: 'Review', statuses: ['In Review'] }], stageColumns: { reviewed: ['Review'] }, trackers: [] }
  const columns = boardColumns([{ id: 'a', key: '#1', status: 'to_clarify', trackerStatus: 'In Review' }], board)
  assert.equal(columns.find(column => column.stage === 'reviewed').tasks.length, 1)
})

test('a column lists priority first, then key', () => {
  const tasks = [
    { id: 'a', key: '#10', status: 'to_clarify', priority: 'low' },
    { id: 'b', key: '#9', status: 'to_clarify', priority: 'low' },
    { id: 'c', key: '#20', status: 'to_clarify', priority: 'urgent' },
    { id: 'd', key: '#1', status: 'to_clarify' },
  ]
  assert.deepEqual(boardColumns(tasks, EMPTY_BOARD)[0].tasks.map(task => task.key), ['#20', '#9', '#10', '#1'])
})

test('a full card leaves out the workflow labels', () => {
  assert.deepEqual(boardCardLabels({ labels: ['bug', '#Implemented', 'untouched', 'done', ' ', 'ui', '#closed'] }), ['bug', 'ui'])
  assert.deepEqual(boardCardLabels({}), [])
})

test('the colour bar needs the project setting and a parent', () => {
  assert.equal(cardEpicColor({ parentKey: '#12' }, { epicColors: true }), '#8b5cf6')
  assert.equal(cardEpicColor({ parentKey: '#12' }, { epicColors: false }), null)
  assert.equal(cardEpicColor({ parentKey: '#12' }, EMPTY_BOARD), null)
  assert.equal(cardEpicColor({ parentKey: '#12' }, undefined), null)
  assert.equal(cardEpicColor({}, { epicColors: true }), null)
})

test('a macro gets the colour the web board pins for its key', () => {
  // web/tests/epicColor.test.mjs pins '#12' to violet and 'PROJ-1' to indigo.
  assert.equal(cardEpicColor({ parentKey: '#12' }, { epicColors: true }), '#8b5cf6')
  assert.equal(cardEpicColor({ parentKey: 'PROJ-1' }, { epicColors: true }), '#6366f1')
})
