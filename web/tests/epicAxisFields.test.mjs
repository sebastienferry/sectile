import assert from 'node:assert/strict'
import { test } from 'node:test'
import { choiceLabel, normalizeQuarterValue, optionChoices, pickField, quarterRows, setOption } from '../src/lib/epicAxisFields.ts'

// Synthetic fields only: no test names a field or an option of a real site (#680).
const flat = { id: 'cf-epic-rank', name: 'Epic rank', kind: 'select', options: [{ id: 'o0', value: 'P0' }, { id: 'o1', value: 'P1' }], deduced: { priority: { p0: 'o0', p1: 'o1' }, quarter: {} } }
const cascade = {
  id: 'cf-epic-period',
  name: 'Epic period',
  kind: 'cascade',
  options: [{ id: 'y26', value: '2026', children: [{ id: 'q3', value: 'Q3' }, { id: 'q4', value: 'Q4' }] }],
  deduced: { quarter: { '2026-Q3': 'y26/q3', '2026-Q4': 'y26/q4' } },
}

test('a cascade lists its leaves as parent/child paths', () => {
  assert.deepEqual(optionChoices(flat), [{ path: 'o0', label: 'P0' }, { path: 'o1', label: 'P1' }])
  assert.deepEqual(optionChoices(cascade), [
    { path: 'y26/q3', label: '2026 / Q3' },
    { path: 'y26/q4', label: '2026 / Q4' },
  ])
  assert.deepEqual(optionChoices(undefined), [])
})

test('picking a field prefills its map with the deductions of the axis', () => {
  const field = pickField(cascade, 'quarter')
  assert.equal(field.id, 'cf-epic-period')
  assert.equal(field.kind, 'cascade')
  assert.deepEqual(field.options, { '2026-Q3': 'y26/q3', '2026-Q4': 'y26/q4' })
  assert.deepEqual(pickField(cascade, 'priority').options, {})
})

test('a line is set and cleared without touching the others', () => {
  const field = pickField(flat, 'priority')
  const changed = setOption(field, 'p2', 'o1')
  assert.deepEqual(changed.options, { p0: 'o0', p1: 'o1', p2: 'o1' })
  assert.deepEqual(setOption(changed, 'p0', '').options, { p1: 'o1', p2: 'o1' })
  assert.deepEqual(field.options, { p0: 'o0', p1: 'o1' }, 'the original map is not changed in place')
})

test('quarter rows are sorted and include the one being added', () => {
  const field = pickField(cascade, 'quarter')
  assert.deepEqual(quarterRows(field, ['2025-Q1']), ['2025-Q1', '2026-Q3', '2026-Q4'])
  assert.deepEqual(quarterRows(undefined), [])
})

test('a typed quarter reads like the server reads it', () => {
  assert.equal(normalizeQuarterValue('2026 q4'), '2026-Q4')
  assert.equal(normalizeQuarterValue(' 2027-Q1 '), '2027-Q1')
  assert.equal(normalizeQuarterValue('Q4'), null)
  assert.equal(normalizeQuarterValue('2026-Q5'), null)
})

test('a stored path shows its label, or itself when the field was not read', () => {
  const choices = optionChoices(cascade)
  assert.equal(choiceLabel(choices, 'y26/q4'), '2026 / Q4')
  assert.equal(choiceLabel([], 'y26/q4'), 'y26/q4')
  assert.equal(choiceLabel(choices, undefined), '')
})
