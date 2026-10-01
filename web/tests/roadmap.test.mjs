import assert from 'node:assert/strict'
import { test } from 'node:test'
import { batchSummary, buildMacroRows, macroCopyPayload, moveTodo, pruneTodoSelection, rewordTodo, selectableTodoIds, todoOrigin, todosMirrorState } from '../src/lib/roadmap.ts'

test('copying a macro with a page copies its link', () => {
  const payload = macroCopyPayload({ key: 'M-11', title: 'Roadmap', externalUrl: 'https://github.com/acme/app/milestone/11' })
  assert.deepEqual(payload, { text: 'https://github.com/acme/app/milestone/11', kind: 'link' })
})

test('copying a macro without a page copies its key and title', () => {
  const payload = macroCopyPayload({ key: 'M-12', title: 'Local macro', externalUrl: '' })
  assert.deepEqual(payload, { text: 'M-12: Local macro', kind: 'ref' })
})

test('a macro row carries the page of its macro, never the one of a ticket', () => {
  const child = { id: 't1', key: '#5', title: 'Child', status: 'new', priority: 'medium', parentKey: 'M-11', externalUrl: 'https://github.com/acme/app/issues/5' }
  const meta = [
    { projectId: 'p', key: 'M-11', horizon: 'now', description: '', todos: [], updatedAt: '', externalUrl: 'https://github.com/acme/app/milestone/11' },
    { projectId: 'p', key: 'M-12', horizon: 'now', description: '', todos: [], updatedAt: '' },
  ]
  const byKey = Object.fromEntries(buildMacroRows([child], null, meta).map(r => [r.key, r.externalUrl]))
  assert.equal(byKey['M-11'], 'https://github.com/acme/app/milestone/11')
  assert.equal(byKey['M-12'], '')
})

const slicing = [
  { id: 'a', text: 'A', done: false },
  { id: 'b', text: 'B', done: true, storyKey: '#12' },
  { id: 'c', text: 'C', done: true, storyKey: '  ' },
  { id: 'd', text: 'D', done: false },
]

test('only unattached lines can be selected, in slicing order, whatever their done state', () => {
  assert.deepEqual(selectableTodoIds(slicing), ['a', 'c', 'd'])
})

test('the selection drops lines that became attached or were removed', () => {
  const pruned = pruneTodoSelection(new Set(['a', 'b', 'gone', 'd']), slicing)
  assert.deepEqual([...pruned], ['a', 'd'])
})

const summaryCopy = {
  created: { one: '{count} créée', other: '{count} créées' },
  skipped: { one: '{count} passée', other: '{count} passées' },
  failed: { one: '{count} en échec', other: '{count} en échec' },
}

test('the batch summary reads every count with its plural form', () => {
  assert.equal(batchSummary('fr', { created: 3, skipped: 1, failed: 1 }, summaryCopy), '3 créées, 1 passée, 1 en échec')
  assert.equal(batchSummary('fr', { created: 0, skipped: 2, failed: 0 }, summaryCopy), '0 créée, 2 passées, 0 en échec')
  assert.equal(
    batchSummary('en', { created: 1, skipped: 0, failed: 2 }, { created: { one: '{count} created', other: '{count} created' }, skipped: { one: '{count} skipped', other: '{count} skipped' }, failed: { one: '{count} failed', other: '{count} failed' } }),
    '1 created, 0 skipped, 2 failed',
  )
})

test('a line names its origin, an unknown kind as written', () => {
  assert.deepEqual(todoOrigin({ sourceKind: 'tasks', sourceEntry: '1. Setup' }), { kind: 'tasks', raw: 'tasks', entry: '1. Setup' })
  assert.deepEqual(todoOrigin({ sourceKind: 'spec' }), { kind: 'spec', raw: 'spec', entry: '' })
  assert.deepEqual(todoOrigin({ sourceKind: 'stories', sourceEntry: '#4' }), { kind: 'stories', raw: 'stories', entry: '#4' })
  assert.deepEqual(todoOrigin({}), { kind: 'manual', raw: '', entry: '' })
  assert.deepEqual(todoOrigin({ sourceKind: 'scenarios', sourceEntry: '  ' }), { kind: 'unknown', raw: 'scenarios', entry: '' })
})

const ids = list => list.map(todo => todo.id)

test('a moved todo lands at its new place, the others keeping their order', () => {
  assert.deepEqual(ids(moveTodo(slicing, 0, 2)), ['b', 'c', 'a', 'd'])
  assert.deepEqual(ids(moveTodo(slicing, 3, 0)), ['d', 'a', 'b', 'c'])
  assert.deepEqual(ids(moveTodo(slicing, 1, 2)), ['a', 'c', 'b', 'd'])
  // A move keeps every line whole: its key, its done state.
  assert.deepEqual(moveTodo(slicing, 1, 0)[0], slicing[1])
})

test('a move onto itself or out of range leaves the list as it is', () => {
  for (const [from, to] of [[1, 1], [-1, 0], [0, 4], [4, 0]]) {
    const moved = moveTodo(slicing, from, to)
    assert.deepEqual(ids(moved), ['a', 'b', 'c', 'd'])
    assert.notEqual(moved, slicing, 'a copy, never the list itself')
  }
})

test('a rewording saves the trimmed text and nothing else', () => {
  const next = rewordTodo(slicing, 'b', '  B, reworded ')
  assert.equal(next[1].text, 'B, reworded')
  assert.deepEqual({ ...next[1], text: 'B' }, slicing[1])
  assert.deepEqual(next.filter((_, i) => i !== 1), slicing.filter((_, i) => i !== 1))
})

test('a blank, unchanged or vanished rewording saves nothing', () => {
  assert.equal(rewordTodo(slicing, 'a', '   '), null)
  assert.equal(rewordTodo(slicing, 'a', ' A '), null)
  assert.equal(rewordTodo(slicing, 'gone', 'X'), null)
})

test('the tracker copy status reads as one of five lines', () => {
  assert.equal(todosMirrorState(undefined), 'none')
  assert.equal(todosMirrorState({ kind: '', reason: 'projet local', upToDate: false }), 'local')
  assert.equal(todosMirrorState({ kind: 'jira_comment', upToDate: true, error: 'old' }), 'upToDate')
  assert.equal(todosMirrorState({ kind: 'jira_comment', upToDate: false }), 'pending')
  assert.equal(todosMirrorState({ kind: 'github_description', upToDate: false, error: 'HTTP 500' }), 'failed')
})
