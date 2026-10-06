import assert from 'node:assert/strict'
import { test } from 'node:test'
import { activeMacroRun, macroLaunchBlocker, macroSkillId } from '../src/lib/macroRuns.ts'
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

test('a run is matched to its skill through the catalog aliases', () => {
  for (const name of ['refine_macro', 'refine-macro', 'refine', 'sectile:refine-macro', ' Refine-Macro ']) {
    assert.equal(macroSkillId(name), 'refine_macro', name)
  }
  for (const name of ['realign_macro', 'realign-macro', 'realign', 'sectile:realign-macro']) {
    assert.equal(macroSkillId(name), 'realign_macro', name)
  }
  assert.equal(macroSkillId('pickup-issue'), 'pickup_issue')
})

test('another skill\'s run gives its own reason', () => {
  const refine = { ...planning.fr.macro.refine, otherRunning: planning.fr.macro.otherSkillRunning }
  const agents = [{ projectId: 'p1' }]
  const realigning = [run({ status: 'running', skillName: 'realign-macro' })]
  assert.equal(macroLaunchBlocker(agents, 'p1', realigning, refine, '', 'refine_macro'), refine.otherRunning)
  assert.equal(macroLaunchBlocker(agents, 'p1', [run({ status: 'running', skillName: 'refine' })], refine, '', 'refine_macro'), refine.alreadyRunning)
  // Without a skill, as before, any active run is the button's own.
  assert.equal(macroLaunchBlocker(agents, 'p1', realigning, refine), refine.alreadyRunning)
  // The missing agent comes first.
  assert.equal(macroLaunchBlocker([], 'p1', realigning, refine, '', 'refine_macro'), refine.noAgent)
})
