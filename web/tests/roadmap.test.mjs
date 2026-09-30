import assert from 'node:assert/strict'
import { test } from 'node:test'
import { batchSummary, buildMacroRows, macroCopyPayload, pruneTodoSelection, selectableTodoIds, todoOrigin } from '../src/lib/roadmap.ts'

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
