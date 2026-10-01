import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  axisValueOf,
  comingQuarters,
  groupEpics,
  isGroupableTab,
  parseDraggedEpicKeys,
  planAxisDrop,
  quarterOf,
} from '../src/lib/epicGrouping.ts'

// Local dates: the current quarter is read in the browser's time zone.
const SEPT_30 = new Date(2026, 8, 30)
const DEC_31 = new Date(2026, 11, 31)

const summary = sections => sections.map(s => `${s.id}=${s.rows.map(r => r.key).join(',')}`)

test('the quarter of a date and the coming ones cross the year', () => {
  assert.equal(quarterOf(SEPT_30), '2026-Q3')
  assert.equal(quarterOf(new Date(2026, 0, 1)), '2026-Q1')
  assert.deepEqual(comingQuarters(SEPT_30), ['2026-Q3', '2026-Q4', '2027-Q1', '2027-Q2'])
  assert.deepEqual(comingQuarters(DEC_31, 2), ['2026-Q4', '2027-Q1'])
})

test('the priority axis shows P0 to P3 and no priority, empty ones included', () => {
  const rows = [
    { key: 'A', priority: 'p2' },
    { key: 'B', priority: '' },
    { key: 'C', priority: 'p0' },
    { key: 'D', priority: 'p2' },
  ]
  assert.deepEqual(summary(groupEpics(rows, 'priority', SEPT_30)), [
    'priority:p0=C',
    'priority:p1=',
    'priority:p2=A,D',
    'priority:p3=',
    'priority:none=B',
  ])
})

test('the order of the rows is kept inside a section', () => {
  const rows = [{ key: 'Z', priority: 'p1' }, { key: 'A', priority: 'p1' }]
  assert.deepEqual(groupEpics(rows, 'priority', SEPT_30)[1].rows.map(r => r.key), ['Z', 'A'])
})

test('the quarter axis adds the used quarters to the current and next three, in order', () => {
  const rows = [
    { key: 'A', quarter: '2027-Q3' },
    { key: 'B', quarter: '2026-Q1' },
    { key: 'C' },
    { key: 'D', quarter: '2026-Q4' },
  ]
  assert.deepEqual(summary(groupEpics(rows, 'quarter', SEPT_30)), [
    'quarter:2026-Q1=B',
    'quarter:2026-Q3=',
    'quarter:2026-Q4=D',
    'quarter:2027-Q1=',
    'quarter:2027-Q2=',
    'quarter:2027-Q3=A',
    'quarter:none=C',
  ])
})

test('a past quarter nobody carries does not show', () => {
  const ids = groupEpics([{ key: 'A', quarter: '' }], 'quarter', SEPT_30).map(s => s.id)
  assert.deepEqual(ids, ['quarter:2026-Q3', 'quarter:2026-Q4', 'quarter:2027-Q1', 'quarter:2027-Q2', 'quarter:none'])
})

test('an unreadable quarter counts as none, a loose one is normalized', () => {
  assert.equal(axisValueOf({ key: 'A', quarter: 'soon' }, 'quarter'), '')
  assert.equal(axisValueOf({ key: 'A', quarter: '2026 q4' }, 'quarter'), '2026-Q4')
  const sections = groupEpics([{ key: 'A', quarter: 'soon' }], 'quarter', SEPT_30)
  assert.deepEqual(sections.at(-1).rows.map(r => r.key), ['A'])
})

test('sections carry the value a drop sets, empty for none', () => {
  const sections = groupEpics([], 'priority', SEPT_30)
  assert.deepEqual(sections.map(s => s.value), ['p0', 'p1', 'p2', 'p3', ''])
  assert.ok(sections.every(s => s.axis === 'priority'))
})

test('a drop skips the epics already at the target, keeps the given order', () => {
  const rows = [
    { key: 'A', priority: 'p1' },
    { key: 'B', priority: 'p2' },
    { key: 'C', priority: '' },
  ]
  assert.deepEqual(planAxisDrop(rows, ['C', 'A', 'B'], 'priority', 'p1'), { toSave: ['C', 'B'], skipped: ['A'] })
  assert.deepEqual(planAxisDrop(rows, ['A', 'C', 'X'], 'priority', ''), { toSave: ['A'], skipped: ['C'] })
})

test('a quarter drop compares normalized values', () => {
  const rows = [{ key: 'A', quarter: '2026.q4' }]
  assert.deepEqual(planAxisDrop(rows, ['A'], 'quarter', '2026-Q4'), { toSave: [], skipped: ['A'] })
})

test('only the Hidden tab stays flat', () => {
  assert.deepEqual(['now', 'next', 'later', 'unclassified', 'hidden'].map(isGroupableTab), [true, true, true, true, false])
})

test('a drag payload carries keys only', () => {
  assert.deepEqual(parseDraggedEpicKeys('["M-1","M-2"]'), ['M-1', 'M-2'])
  assert.deepEqual(parseDraggedEpicKeys('["M-1",3,""]'), ['M-1'])
  assert.deepEqual(parseDraggedEpicKeys('{"key":"M-1"}'), [])
  assert.deepEqual(parseDraggedEpicKeys('not json'), [])
})
