import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildMacroRows, macroCopyPayload } from '../src/lib/roadmap.ts'

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
