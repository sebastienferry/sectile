import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  cleanEpicAxisPrefix,
  epicAxisLabelPrefixes,
  epicAxisPrefixProblem,
  epicLabelInventory,
  freeEpicLabels,
  isEpicAxisLabel,
  matchesEpicLabels,
} from '../src/lib/roadmap.ts'

// The prefixes a project names for its epic axes (#635).
const project = { epicAxisPrefixes: { priority: 'prio-', quarter: 'target/', readiness: 'stage-' } }

test('a project naming no prefix keeps the defaults', () => {
  assert.deepEqual(epicAxisLabelPrefixes(undefined), ['roadmap:', 'priority:', 'quarter:', 'readiness:'])
  assert.deepEqual(epicAxisLabelPrefixes({ epicAxisPrefixes: { quarter: 'q-' } }), ['roadmap:', 'priority:', 'q-', 'readiness:'])
})

test("the project's own prefixes are protected, the former ones are free", () => {
  assert.equal(isEpicAxisLabel('prio-p1', project), true)
  assert.equal(isEpicAxisLabel('#Prio-P0', project), true)
  assert.equal(isEpicAxisLabel('target/2026-q4', project), true)
  assert.equal(isEpicAxisLabel('stage-ready', project), true)
  assert.equal(isEpicAxisLabel('priority:p1', project), false)
  assert.equal(isEpicAxisLabel('quarter:2026-q4', project), false)
  assert.equal(isEpicAxisLabel('readiness:ready', project), false)
  // The horizon and the bare quarter stay axis labels whatever the prefixes.
  assert.equal(isEpicAxisLabel('roadmap:now', project), true)
  assert.equal(isEpicAxisLabel('2026-Q3', project), true)
})

test("an epic's free labels hide the current axes and show the former ones", () => {
  const meta = { labels: ['prio-p1', 'priority:p2', 'target/2026-q4', '2026-Q3', 'roadmap:now', 'team-a'] }
  assert.deepEqual(freeEpicLabels(meta, project), ['priority:p2', 'team-a'])
  assert.deepEqual(freeEpicLabels(meta), ['prio-p1', 'target/2026-q4', 'team-a'])
  const rows = [{ key: 'PE-1', meta }]
  assert.deepEqual(epicLabelInventory(rows, project).map(entry => entry.label), ['priority:p2', 'team-a'])
  assert.equal(matchesEpicLabels(rows[0], ['priority:p2'], project), true)
  assert.equal(matchesEpicLabels(rows[0], ['prio-p1'], project), false)
})

test('a typed prefix is cleaned as the server stores it', () => {
  assert.equal(cleanEpicAxisPrefix('  #Prio- '), 'prio-')
  assert.equal(cleanEpicAxisPrefix(''), '')
})

test('the typed prefixes are refused as the server refuses them', () => {
  assert.equal(epicAxisPrefixProblem({}), null)
  assert.equal(epicAxisPrefixProblem({ priority: 'Prio-', quarter: 'target/', readiness: 'stage-' }), null)
  assert.equal(epicAxisPrefixProblem({ priority: 'priority:' }), null)
  assert.deepEqual(epicAxisPrefixProblem({ priority: 'prio p' }), { kind: 'space', axis: 'priority' })
  assert.deepEqual(epicAxisPrefixProblem({ readiness: '  #  ' }), { kind: 'empty', axis: 'readiness' })
  assert.deepEqual(epicAxisPrefixProblem({ priority: 'p', quarter: 'priority:' }), { kind: 'overlap', axis: 'priority', other: 'quarter' })
  assert.deepEqual(epicAxisPrefixProblem({ priority: 'x-', readiness: 'X-' }), { kind: 'overlap', axis: 'priority', other: 'readiness' })
  // Against a default left in effect: "q" starts "quarter:".
  assert.deepEqual(epicAxisPrefixProblem({ priority: 'q' }), { kind: 'overlap', axis: 'priority', other: 'quarter' })
  assert.deepEqual(epicAxisPrefixProblem({ quarter: 'road' }), { kind: 'horizon', axis: 'quarter' })
  assert.deepEqual(epicAxisPrefixProblem({ priority: 'roadmap:prio-' }), { kind: 'horizon', axis: 'priority' })
})
