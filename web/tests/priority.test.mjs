import assert from 'node:assert/strict'
import { test } from 'node:test'
import { PRIORITY_COLORS, PRIORITY_LEVELS, priorityColor } from '../src/lib/priority.ts'

test('the levels go from the most to the least urgent', () => {
  assert.deepEqual([...PRIORITY_LEVELS], ['urgent', 'high', 'medium', 'low'])
  assert.deepEqual(Object.keys(PRIORITY_COLORS).sort(), [...PRIORITY_LEVELS].sort())
})

test('the colours are pinned, so the forms and the cards keep one code', () => {
  assert.equal(priorityColor('urgent'), 'var(--status-danger)')
  assert.equal(priorityColor('high'), 'var(--status-warn)')
  assert.equal(priorityColor('medium'), 'var(--status-info)')
  assert.equal(priorityColor('low'), 'var(--text-muted)')
})

test('a value outside the four levels falls back to the muted colour', () => {
  for (const value of [undefined, null, '', 'Highest', 'toString', '__proto__']) {
    assert.equal(priorityColor(value), 'var(--text-muted)', String(value))
  }
})
