import assert from 'node:assert/strict'
import { test } from 'node:test'
import { clearAll, countSetPickers, multiValueFilters } from '../src/lib/activeFilters.ts'

test('clearAll clears each listed filter once, in order', () => {
  const calls = []
  clearAll([
    { id: 'sprint', label: 'S1', clear: () => calls.push('sprint') },
    { id: 'team', label: 'Core', clear: () => calls.push('team') },
  ])
  assert.deepEqual(calls, ['sprint', 'team'])
})

test('clearAll on an empty list does nothing', () => {
  assert.doesNotThrow(() => clearAll([]))
})

test('multiValueFilters gives one entry per value, with distinct ids', () => {
  const removed = []
  const filters = multiValueFilters('label', ['ux', 'perf'], v => `#${v}`, v => removed.push(v))
  assert.deepEqual(filters.map(f => f.id), ['label:ux', 'label:perf'])
  assert.deepEqual(filters.map(f => f.label), ['#ux', '#perf'])
  filters[1].clear()
  assert.deepEqual(removed, ['perf'])
})

test('two dimensions sharing a value keep distinct ids', () => {
  const a = multiValueFilters('sprint', ['Q4'], v => v, () => {})
  const b = multiValueFilters('label', ['Q4'], v => v, () => {})
  assert.notEqual(a[0].id, b[0].id)
})

test('countSetPickers counts set dimensions, not values', () => {
  assert.equal(countSetPickers(['S1', ['a', 'b', 'c'], null]), 2)
})

test('countSetPickers ignores null, undefined, empty strings, empty lists and false', () => {
  assert.equal(countSetPickers([null, undefined, '', [], false]), 0)
})
