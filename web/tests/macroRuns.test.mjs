import assert from 'node:assert/strict'
import { test } from 'node:test'
import { activeMacroRun, macroLaunchBlocker } from '../src/lib/macroRuns.ts'

const run = overrides => ({ id: 'r1', skillName: 'realign_macro', status: 'completed', summary: '', createdAt: '', ...overrides })

test('the most recent running run makes the macro busy', () => {
  assert.equal(activeMacroRun([]), null)
  assert.equal(activeMacroRun([run({})]), null)
  assert.equal(activeMacroRun([run({ id: 'a', status: 'running' }), run({ id: 'b' })]).id, 'a')
})

test('the launch needs an agent serving the project', () => {
  assert.match(macroLaunchBlocker([], 'p1', []), /agent local/)
  assert.match(macroLaunchBlocker([{ projectId: 'p2' }], 'p1', []), /agent local/)
  assert.equal(macroLaunchBlocker([{ projectId: 'p1' }], 'p1', []), null)
  // An agent registered without a project serves them all.
  assert.equal(macroLaunchBlocker([{ projectId: '' }], 'p1', []), null)
})

test('a running realignment blocks a second launch', () => {
  assert.match(macroLaunchBlocker([{ projectId: 'p1' }], 'p1', [run({ status: 'running' })]), /déjà en cours/)
})
