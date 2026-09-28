import assert from 'node:assert/strict'
import { test } from 'node:test'
import { activeMacroRun, macroLaunchBlocker } from '../src/lib/macroRuns.ts'
import { planning } from '../src/locales/planning.ts'

const strings = planning.fr.macro.realign

const run = overrides => ({ id: 'r1', skillName: 'realign_macro', status: 'completed', summary: '', createdAt: '', ...overrides })

test('the most recent running run makes the macro busy', () => {
  assert.equal(activeMacroRun([]), null)
  assert.equal(activeMacroRun([run({})]), null)
  assert.equal(activeMacroRun([run({ id: 'a', status: 'running' }), run({ id: 'b' })]).id, 'a')
})

test('the launch needs an agent serving the project', () => {
  assert.match(macroLaunchBlocker([], 'p1', [], strings), /agent local/)
  assert.match(macroLaunchBlocker([{ projectId: 'p2' }], 'p1', [], strings), /agent local/)
  assert.equal(macroLaunchBlocker([{ projectId: 'p1' }], 'p1', [], strings), null)
  // An agent registered without a project serves them all.
  assert.equal(macroLaunchBlocker([{ projectId: '' }], 'p1', [], strings), null)
})

test('a running realignment blocks a second launch', () => {
  assert.match(macroLaunchBlocker([{ projectId: 'p1' }], 'p1', [run({ status: 'running' })], strings), /déjà en cours/)
})

test('only the signed-in user\'s agents enable the launch', () => {
  assert.match(macroLaunchBlocker([{ userId: 'other', projectId: 'p1' }], 'p1', [], strings, 'me'), /agent local/)
  assert.equal(macroLaunchBlocker([{ userId: 'me', projectId: 'p1' }], 'p1', [], strings, 'me'), null)
})

test('the reason follows the strings it is given', () => {
  const english = planning.en.macro.realign
  assert.equal(macroLaunchBlocker([], 'p1', [], english), english.noAgent)
  assert.equal(macroLaunchBlocker([{ projectId: 'p1' }], 'p1', [run({ status: 'queued' })], english), english.alreadyRunning)
})
