import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

const context = await readFile(new URL('../src/context/AppContext.tsx', import.meta.url), 'utf8')
const card = await readFile(new URL('../src/components/TaskCard.tsx', import.meta.url), 'utf8')
const modal = await readFile(new URL('../src/components/ProjectModal.tsx', import.meta.url), 'utf8')
const skills = await readFile(new URL('../src/components/SkillsView.tsx', import.meta.url), 'utf8')

test('the launch request carries the one-off mode override', () => {
  assert.match(context, /body: JSON\.stringify\(\{ skillId, prompt, withComments: opts\?\.withComments, mode: opts\?\.mode \}\)/)
  assert.match(context, /advanceTask = async \(taskId: string, auto\?: boolean, modeOverride\?: SkillMode\)/)
})

test('the card ... menu offers both modes for a single launch', () => {
  assert.match(card, /handleAdvance\(false, 'interactive'\)/)
  assert.match(card, /handleAdvance\(false, 'non_interactive'\)/)
  // The autonomous run is forced headless server-side and takes no override.
  assert.match(card, /handleAdvance\(true\)/)
})

test('the project modal edits both new settings', () => {
  assert.match(modal, /defaultSkillMode/)
  assert.match(modal, /autonomousStopStage/)
  assert.match(modal, /value="non_interactive"/)
  assert.match(modal, /value="implemented"/)
})

test('the skill editor writes a ternary mode', () => {
  assert.match(skills, /saveSkillMode\(selected\.id, e\.target\.value\)/)
  // The empty option is what hands the decision back to the project default.
  assert.match(skills, /<option value="">/)
  assert.match(context, /skill-editor.*\n.*mode|\/mode`/)
})
