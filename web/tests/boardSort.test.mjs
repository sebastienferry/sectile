import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BOARD_SORT_STORAGE_KEY,
  DEFAULT_BOARD_SORT,
  loadBoardSort,
  pickBoardSortField,
  saveBoardSort,
  sortTasks,
} from '../src/lib/boardSort.ts'

function task(key, overrides = {}) {
  return {
    id: overrides.id ?? `id-${key}`,
    key,
    priority: 'medium',
    updatedAt: '',
    ...overrides,
  }
}

const keys = list => list.map(t => t.key)

function createMockStorage(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return map.has(key) ? map.get(key) : null
    },
    setItem(key, val) {
      map.set(key, String(val))
    },
    _map: map,
  }
}

test('priority descending: urgent, high, medium, low; equal priority by numeric key', () => {
  const tasks = [
    task('#42', { priority: 'low' }),
    task('#402', { priority: 'high' }),
    task('#9', { priority: 'high' }),
    task('#1', { priority: 'urgent' }),
    task('#7', { priority: 'medium' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'priority', asc: false })), ['#1', '#9', '#402', '#7', '#42'])
})

test('priority ascending: low first; equal priority still oldest key first', () => {
  const tasks = [
    task('#402', { priority: 'low' }),
    task('#1', { priority: 'urgent' }),
    task('#9', { priority: 'low' }),
    task('#5', { priority: 'high' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'priority', asc: true })), ['#9', '#402', '#5', '#1'])
})

test('key ascending is numeric-aware, descending reverses it', () => {
  const tasks = [task('#402'), task('#9'), task('#42')]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'key', asc: true })), ['#9', '#42', '#402'])
  assert.deepEqual(keys(sortTasks(tasks, { field: 'key', asc: false })), ['#402', '#42', '#9'])

  const jira = [task('PROJ-12'), task('PROJ-9')]
  assert.deepEqual(keys(sortTasks(jira, { field: 'key', asc: true })), ['PROJ-9', 'PROJ-12'])
})

test('last updated descending prefers the tracker date, falls back to the Sectile date, undated last', () => {
  const tasks = [
    task('#1', { trackerUpdatedAt: '2026-01-01T00:00:00Z', updatedAt: '2026-09-01T00:00:00Z' }),
    task('#2', { updatedAt: '2026-05-01T00:00:00Z' }),
    task('#3'),
    task('#4', { trackerUpdatedAt: '2026-08-01T00:00:00Z' }),
    task('#5', { updatedAt: 'not a date' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'updated', asc: false })), ['#4', '#2', '#1', '#3', '#5'])
})

test('last updated: an unreadable tracker date falls back to the Sectile date', () => {
  const tasks = [
    task('#1', { trackerUpdatedAt: 'not a date', updatedAt: '2026-09-01T00:00:00Z' }),
    task('#2', { trackerUpdatedAt: '2026-05-01T00:00:00Z' }),
    task('#3', { trackerUpdatedAt: 'not a date', updatedAt: 'nor this' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'updated', asc: false })), ['#1', '#2', '#3'])
})

test('last updated ascending: oldest first, undated tickets still last', () => {
  const tasks = [
    task('#3', { priority: 'low' }),
    task('#1', { trackerUpdatedAt: '2026-01-01T00:00:00Z' }),
    task('#6', { priority: 'urgent' }),
    task('#4', { trackerUpdatedAt: '2026-08-01T00:00:00Z' }),
    task('#2', { updatedAt: '2026-05-01T00:00:00Z' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'updated', asc: true })), ['#1', '#2', '#4', '#6', '#3'])
})

test('epic descending: the group holding the highest priority first, cards by priority inside', () => {
  const tasks = [
    task('#1', { parentKey: 'A', priority: 'low' }),
    task('#2', { parentKey: 'B', priority: 'urgent' }),
    task('#3', { parentKey: 'A', priority: 'high' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'epic', asc: false })), ['#2', '#3', '#1'])
})

test('epic: groups sharing their top priority are ordered by numeric parent key', () => {
  const tasks = [
    task('#1', { parentKey: 'M-10', priority: 'high' }),
    task('#2', { parentKey: 'M-2', priority: 'high' }),
    task('#3', { parentKey: 'M-10', priority: 'low' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'epic', asc: false })), ['#2', '#1', '#3'])
  // The parent key tie-break never flips.
  assert.deepEqual(keys(sortTasks(tasks, { field: 'epic', asc: true })), ['#2', '#1', '#3'])
})

test('epic: tickets without a parent close the list, by priority; empty and missing parent are the same', () => {
  const tasks = [
    task('#1', { priority: 'urgent', parentKey: '' }),
    task('#2', { parentKey: 'M-1', priority: 'low' }),
    task('#3', { priority: 'high' }),
    task('#4', { priority: 'urgent' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'epic', asc: false })), ['#2', '#1', '#4', '#3'])
})

test('epic ascending reverses the groups only: inside order and orphans unchanged', () => {
  const tasks = [
    task('#1', { parentKey: 'A', priority: 'low' }),
    task('#2', { parentKey: 'B', priority: 'urgent' }),
    task('#3', { parentKey: 'A', priority: 'high' }),
    task('#4', { parentKey: 'B', priority: 'medium' }),
    task('#5', { priority: 'low' }),
    task('#6', { priority: 'urgent' }),
  ]
  assert.deepEqual(keys(sortTasks(tasks, { field: 'epic', asc: false })), ['#2', '#4', '#3', '#1', '#6', '#5'])
  assert.deepEqual(keys(sortTasks(tasks, { field: 'epic', asc: true })), ['#3', '#1', '#2', '#4', '#6', '#5'])
})

test('the same tickets in another order give the same result, for every criterion', () => {
  const tasks = [
    task('#1', { parentKey: 'A', priority: 'low', updatedAt: '2026-02-01T00:00:00Z' }),
    task('#2', { parentKey: 'B', priority: 'low', updatedAt: '2026-02-01T00:00:00Z' }),
    task('#3', { priority: 'low' }),
    task('#4', { parentKey: 'A', priority: 'high' }),
    task('#5', { priority: 'low' }),
  ]
  const reversed = [...tasks].reverse()
  for (const field of ['priority', 'epic', 'key', 'updated']) {
    for (const asc of [true, false]) {
      assert.deepEqual(
        keys(sortTasks(reversed, { field, asc })),
        keys(sortTasks(tasks, { field, asc })),
        `${field} ${asc ? 'asc' : 'desc'}`,
      )
    }
  }
})

test('two tickets sharing key and priority in two projects keep one order', () => {
  const a = task('#42', { id: 'project-a-42' })
  const b = task('#42', { id: 'project-b-42' })
  for (const field of ['priority', 'epic', 'key', 'updated']) {
    const ids = list => sortTasks(list, { field, asc: false }).map(t => t.id)
    assert.deepEqual(ids([a, b]), ['project-a-42', 'project-b-42'], field)
    assert.deepEqual(ids([b, a]), ['project-a-42', 'project-b-42'], field)
  }
})

test('sortTasks returns a new array and leaves its input untouched', () => {
  const tasks = [task('#3', { priority: 'low' }), task('#1', { priority: 'urgent' }), task('#2', { parentKey: 'A' })]
  const before = keys(tasks)
  for (const field of ['priority', 'epic', 'key', 'updated']) {
    const sorted = sortTasks(tasks, { field, asc: true })
    assert.notEqual(sorted, tasks)
    assert.deepEqual(keys(tasks), before, field)
  }
})

test('picking a criterion starts it in its natural direction', () => {
  assert.deepEqual(pickBoardSortField('priority'), { field: 'priority', asc: false })
  assert.deepEqual(pickBoardSortField('epic'), { field: 'epic', asc: false })
  assert.deepEqual(pickBoardSortField('key'), { field: 'key', asc: true })
  assert.deepEqual(pickBoardSortField('updated'), { field: 'updated', asc: false })
})

test('loadBoardSort falls back to priority, highest first, on anything unusable', () => {
  assert.deepEqual(DEFAULT_BOARD_SORT, { field: 'priority', asc: false })
  const invalid = [
    undefined,
    'not json',
    'null',
    '"epic"',
    '{"field":"title","asc":true}',
    '{"field":"epic","asc":"true"}',
    '{"field":"epic"}',
  ]
  for (const raw of invalid) {
    const storage = createMockStorage(raw === undefined ? {} : { [BOARD_SORT_STORAGE_KEY]: raw })
    assert.deepEqual(loadBoardSort(storage), DEFAULT_BOARD_SORT, String(raw))
  }

  const throwing = {
    getItem() {
      throw new Error('denied')
    },
    setItem() {
      throw new Error('denied')
    },
  }
  assert.deepEqual(loadBoardSort(throwing), DEFAULT_BOARD_SORT)
  assert.doesNotThrow(() => saveBoardSort({ field: 'key', asc: true }, throwing))

  // No window under node: no storage at all.
  assert.deepEqual(loadBoardSort(), DEFAULT_BOARD_SORT)
  assert.doesNotThrow(() => saveBoardSort({ field: 'key', asc: true }))
})

test('a saved sort loads back', () => {
  const storage = createMockStorage()
  saveBoardSort({ field: 'epic', asc: true }, storage)
  assert.equal(storage._map.get(BOARD_SORT_STORAGE_KEY), '{"field":"epic","asc":true}')
  assert.deepEqual(loadBoardSort(storage), { field: 'epic', asc: true })
})
