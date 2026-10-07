import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextEngine, taskEngine, launchEngineChange, engineMark, engineTooltip, moveEngine, removalImpact, removalMessage } from '../src/engines.mjs'

const catalogue = [
  { id: 'e-opus', name: 'Claude Opus', provider: 'claude', model: 'claude-opus-5' },
  { id: 'e-codex', name: 'Codex', provider: 'codex', model: '' },
  { id: 'e-agy', name: 'Antigravity', provider: 'agy' },
]

test('a click moves a task to the next engine, wrapping around', () => {
  assert.equal(nextEngine(catalogue, 'e-opus', 'e-agy').id, 'e-codex')
  assert.equal(nextEngine(catalogue, 'e-agy', 'e-agy').id, 'e-opus')
  // An unknown current engine reads as the project default one.
  assert.equal(nextEngine(catalogue, 'e-gone', 'e-codex').id, 'e-agy')
  assert.equal(nextEngine(catalogue, undefined, 'e-gone').id, 'e-opus')
  // A single engine stays where it is.
  assert.equal(nextEngine([catalogue[0]], 'e-opus', 'e-opus').id, 'e-opus')
  assert.equal(nextEngine([], 'e-opus', 'e-opus'), null)
})

test('a task runs its stored engine, else its project default one', () => {
  const view = { catalogue, projectDefault: 'e-codex', tasks: { t1: 'e-opus', t2: 'e-gone' } }
  assert.equal(taskEngine(view, 't1').id, 'e-opus')
  assert.equal(taskEngine(view, 't2').id, 'e-codex')
  assert.equal(taskEngine(view, 't3').id, 'e-codex')
  assert.equal(taskEngine({ catalogue, projectDefault: 'e-gone' }, 't3').id, 'e-opus')
  assert.equal(taskEngine(null, 't3'), null)
})

test('every provider has a mark and the tooltip names engine, provider and model', () => {
  for (const [provider, mark] of Object.entries({ claude: 'Cl', codex: 'Cx', agy: 'Ag', custom: '{}' })) {
    assert.equal(engineMark(provider), mark)
  }
  assert.equal(engineMark('Other'), 'Ot')
  // A retired provider (#614) has no mark of its own any more.
  assert.equal(engineMark('gemini'), 'Ge')
  assert.equal(engineMark(''), '?')
  assert.equal(engineTooltip(catalogue[0], false), 'Claude Opus - claude · claude-opus-5')
  assert.equal(engineTooltip(catalogue[1], true), 'Codex - codex · provider default (project default)')
})

test('engines move within the catalogue and a removal says what it affects', () => {
  assert.deepEqual(moveEngine(catalogue, 'e-agy', -1).map(e => e.id), ['e-opus', 'e-agy', 'e-codex'])
  assert.deepEqual(moveEngine(catalogue, 'e-opus', -1).map(e => e.id), ['e-opus', 'e-codex', 'e-agy'])
  const impact = removalImpact({ projects: { p1: 'e-codex', p2: 'e-opus' }, taskCounts: { 'e-codex': 3 } }, 'e-codex', { p1: 'Sectile' })
  assert.deepEqual(impact, { projects: ['Sectile'], tasks: 3 })
  assert.equal(removalMessage('Codex', impact), 'Remove Codex? project Sectile will use the workstation default engine; 3 tasks will use their project default engine.')
  assert.equal(removalMessage('Codex', { projects: [], tasks: 0 }), 'Remove Codex? No project or task uses it.')
})

test('the Launch dialog stores an engine only when the task would run another one', () => {
  const view = { catalogue, projectDefault: 'e-opus', tasks: { switched: 'e-codex' } }
  // The engine the task already runs is not stored again.
  assert.equal(launchEngineChange(view, 'plain', 'e-opus'), null)
  assert.equal(launchEngineChange(view, 'switched', 'e-codex'), null)
  // Another engine is stored, the project default one clearing a switch.
  assert.equal(launchEngineChange(view, 'plain', 'e-agy'), 'e-agy')
  assert.equal(launchEngineChange(view, 'switched', 'e-opus'), 'e-opus')
  // A choice outside the catalogue stores nothing.
  assert.equal(launchEngineChange(view, 'plain', 'e-gone'), null)
  assert.equal(launchEngineChange(view, 'plain', ''), null)
  assert.equal(launchEngineChange(null, 'plain', 'e-opus'), null)
})
