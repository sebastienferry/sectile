import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EPIC_PRIORITY_LEVEL,
  EPIC_READINESS,
  epicPriorityLabel,
  matchesPriority,
  normalizeQuarter,
  seedProposals,
  sortByPriority,
  suggestReadiness,
  titleAxes,
} from '../src/lib/epicAxes.ts'
import { buildEpicRows, freeEpicLabels, isEpicAxisLabel } from '../src/lib/roadmap.ts'

const row = (key, priority = '', quarter = '', title = key) => ({ key, title, priority, quarter })

test('a typed quarter is normalized, an empty one clears, anything else is refused', () => {
  for (const [input, want] of [
    ['2026-Q4', '2026-Q4'],
    ['2026.q4', '2026-Q4'],
    ['2026 Q4', '2026-Q4'],
    ['  2026-q1 ', '2026-Q1'],
    ['', ''],
    ['   ', ''],
  ]) {
    assert.equal(normalizeQuarter(input), want, input)
  }
  for (const input of ['Q4', '2026-Q5', '2026-Q0', '2026Q4', '26-Q4', 'bientôt', 'quarter:2026-q4']) {
    assert.equal(normalizeQuarter(input), null, input)
  }
})

test('the title markers are read on word boundaries only', () => {
  assert.deepEqual(titleAxes('2026.Q4 [P2] - Platform AI'), { priority: 'p2', quarter: '2026-Q4' })
  assert.deepEqual(titleAxes('2026.Q4 P0  - Grafana Structure Decision'), { priority: 'p0', quarter: '2026-Q4' })
  assert.deepEqual(titleAxes('2026 q1 - [p3] cleanup'), { priority: 'p3', quarter: '2026-Q1' })
  assert.deepEqual(titleAxes('PEP1 rollout for X2026-Q4 and 1.2026-Q4'), { priority: '', quarter: '' })
  assert.deepEqual(titleAxes('P4 is not a priority, 2026-Q5 not a quarter'), { priority: '', quarter: '' })
  assert.deepEqual(titleAxes('No marker at all'), { priority: '', quarter: '' })
})

test('the first candidate of each axis wins', () => {
  assert.deepEqual(titleAxes('2026.Q4 [P2] - P0 migration, then 2027-Q1'), { priority: 'p2', quarter: '2026-Q4' })
})

test('the seeding proposes only the axes an epic has no value on', () => {
  const lines = seedProposals([
    row('PE-1', '', '', '2026.Q4 [P2] - Platform AI'),
    row('PE-2', 'p1', '', '2026.Q3 [P0] - Already prioritized'),
    row('PE-3', 'p1', '2026-Q1', '2026.Q3 [P0] - Both set'),
    row('PE-4', '', '', 'Nothing in the title'),
  ])
  assert.deepEqual(lines, [
    { key: 'PE-1', title: '2026.Q4 [P2] - Platform AI', priority: 'p2', quarter: '2026-Q4' },
    { key: 'PE-2', title: '2026.Q3 [P0] - Already prioritized', quarter: '2026-Q3' },
  ])
})

test('the priority sort keeps the backlog order and puts unprioritized epics last', () => {
  const rows = [row('A'), row('B', 'p2'), row('C', 'p0'), row('D', 'p2'), row('E'), row('F', 'p3')]
  const keys = list => list.map(r => r.key)
  assert.deepEqual(keys(sortByPriority(rows, 'backlog')), ['A', 'B', 'C', 'D', 'E', 'F'])
  assert.deepEqual(keys(sortByPriority(rows, 'priority-desc')), ['C', 'B', 'D', 'F', 'A', 'E'])
  assert.deepEqual(keys(sortByPriority(rows, 'priority-asc')), ['F', 'B', 'D', 'C', 'A', 'E'])
  assert.deepEqual(keys(rows), ['A', 'B', 'C', 'D', 'E', 'F'], 'the input is not reordered in place')
})

test('the priority filter includes "no priority"', () => {
  const rows = [row('A'), row('B', 'p1'), row('C', 'p2')]
  assert.deepEqual(rows.filter(r => matchesPriority(r, 'p1')).map(r => r.key), ['B'])
  assert.deepEqual(rows.filter(r => matchesPriority(r, 'none')).map(r => r.key), ['A'])
  assert.equal(rows.filter(r => matchesPriority(r, null)).length, 3)
})

test('P0 to P3 borrow the colours of the four existing levels', () => {
  assert.deepEqual(EPIC_PRIORITY_LEVEL, { p0: 'urgent', p1: 'high', p2: 'medium', p3: 'low' })
  assert.equal(epicPriorityLabel('p1'), 'P1')
})

test("the roadmap priority is the epic's own, never its children's", () => {
  const tasks = [
    { id: 't1', key: '#1', title: 'Urgent child', status: 'new', priority: 'urgent', parentKey: 'PE-1' },
    { id: 't2', key: '#2', title: 'Low child', status: 'new', priority: 'low', parentKey: 'PE-2' },
  ]
  const rows = buildEpicRows(tasks, null, [
    { projectId: 'p', key: 'PE-2', horizon: '', description: '', todos: [], priority: 'p1', quarter: '2026-Q4', updatedAt: '' },
  ])
  const byKey = Object.fromEntries(rows.map(r => [r.key, r]))
  assert.equal(byKey['PE-1'].priority, '', 'an urgent child does not make the epic P0')
  assert.equal(byKey['PE-1'].quarter, '')
  assert.equal(byKey['PE-2'].priority, 'p1')
  assert.equal(byKey['PE-2'].quarter, '2026-Q4')
})

test('the readiness levels read in funnel order', () => {
  assert.deepEqual(EPIC_READINESS, ['idea', 'shaping', 'ready'])
})

test('an epic with a child ticket is suggested ready, closed tickets included', () => {
  assert.equal(suggestReadiness({ childCount: 1 }), 'ready')
  assert.equal(suggestReadiness({ childCount: 3, description: '', todos: [] }), 'ready')
})

test('a framing and a fully covered slicing suggest ready', () => {
  const todos = [
    { done: true },
    // A line with a story counts as covered even when it is not ticked.
    { done: false, storyKey: 'PE-7' },
  ]
  assert.equal(suggestReadiness({ childCount: 0, description: 'Le cadrage', todos }), 'ready')
})

test('a framing or a slicing line alone suggests shaping', () => {
  assert.equal(suggestReadiness({ childCount: 0, description: 'Le cadrage', todos: [] }), 'shaping')
  assert.equal(suggestReadiness({ childCount: 0, description: 'Le cadrage', todos: [{ done: false }] }), 'shaping')
  assert.equal(suggestReadiness({ childCount: 0, description: '', todos: [{ done: true }] }), 'shaping')
  // A slicing fully covered but without framing is not ready.
  assert.equal(suggestReadiness({ childCount: 0, todos: [{ done: false, storyKey: 'PE-1' }] }), 'shaping')
})

test('nothing, or a whitespace framing, suggests idea', () => {
  assert.equal(suggestReadiness({ childCount: 0 }), 'idea')
  assert.equal(suggestReadiness({ childCount: 0, description: '  \n ', todos: [] }), 'idea')
  assert.equal(suggestReadiness({ childCount: 0, description: 'x', todos: [{ done: false, storyKey: '  ' }] }), 'shaping')
})

test('a row carries the decided readiness and the suggestion apart', () => {
  const meta = (key, extra) => ({ projectId: 'p', key, horizon: '', description: '', todos: [], updatedAt: '', ...extra })
  const rows = buildEpicRows([], null, [
    meta('PE-1', { readiness: 'shaping' }),
    meta('PE-2', { description: 'Le cadrage' }),
  ])
  const byKey = Object.fromEntries(rows.map(r => [r.key, r]))
  assert.equal(byKey['PE-1'].readiness, 'shaping')
  assert.equal(byKey['PE-1'].suggestedReadiness, 'idea')
  assert.equal(byKey['PE-2'].readiness, '')
  assert.equal(byKey['PE-2'].suggestedReadiness, 'shaping')
})

test('readiness labels belong to the roadmap, never to the free labels', () => {
  assert.equal(isEpicAxisLabel('readiness:ready'), true)
  assert.equal(isEpicAxisLabel('#Readiness:Idea'), true)
  assert.deepEqual(freeEpicLabels({ labels: ['readiness:ready', '#Readiness:Idea', 'team-a'] }), ['team-a'])
})
