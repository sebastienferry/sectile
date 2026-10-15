import assert from 'node:assert/strict'
import { test } from 'node:test'
import { macroProjectIds, macroUrlIndex, macroUrlKey } from '../src/lib/macroLinks.ts'

test('macroProjectIds keeps the projects of tickets with a macro, sorted and once', () => {
  const tasks = [
    { projectId: 'p2', parentKey: 'M-1' },
    { projectId: 'p1', parentKey: 'PE-4' },
    { projectId: 'p2', parentKey: 'M-2' },
    { projectId: 'p3', parentKey: '' },
    { projectId: 'p4', parentKey: '  ' },
    { projectId: 'p5' },
  ]
  assert.deepEqual(macroProjectIds(tasks), ['p1', 'p2'])
})

test('macroProjectIds uses the fallback for a ticket without a project', () => {
  assert.deepEqual(macroProjectIds([{ parentKey: 'M-1' }], 'cur'), ['cur'])
  assert.deepEqual(macroProjectIds([{ parentKey: 'M-1' }]), [], 'no project, no fallback: nothing to fetch')
  assert.deepEqual(macroProjectIds([]), [])
})

test('macroUrlIndex skips macros without an address and keeps projects apart', () => {
  const index = macroUrlIndex({
    p1: [
      { key: 'M-1', externalUrl: 'https://github.com/acme/a/milestone/1' },
      { key: 'M-2', externalUrl: '' },
      { key: 'M-3' },
    ],
    p2: [{ key: 'M-1', externalUrl: 'https://github.com/acme/b/milestone/1' }],
  })
  assert.equal(index.get(macroUrlKey('p1', 'M-1')), 'https://github.com/acme/a/milestone/1')
  assert.equal(index.get(macroUrlKey('p2', 'M-1')), 'https://github.com/acme/b/milestone/1')
  assert.equal(index.has(macroUrlKey('p1', 'M-2')), false)
  assert.equal(index.has(macroUrlKey('p1', 'M-3')), false)
  assert.equal(index.size, 2)
})
