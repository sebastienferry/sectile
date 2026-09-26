import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

// The execution mode is wired through JSX handlers, which these files are the
// only record of: there is no exported function to call. Asserting on the source
// is weaker than a behavioural test, but it is what stops the wiring from being
// dropped by an unrelated refactor, which is how the choice went missing from
// the skill cards in the first place.
const modal = await readFile(new URL('../src/components/TaskDetailModal.tsx', import.meta.url), 'utf8')
const context = await readFile(new URL('../src/context/AppContext.tsx', import.meta.url), 'utf8')
const card = await readFile(new URL('../src/components/TaskCard.tsx', import.meta.url), 'utf8')
const skills = await readFile(new URL('../src/components/SkillsView.tsx', import.meta.url), 'utf8')
const project = await readFile(new URL('../src/components/ProjectModal.tsx', import.meta.url), 'utf8')

test('the launch request carries the one-off mode override', () => {
  assert.match(context, /mode: opts\?\.mode/)
  // The launch model rides beside the mode on the same options object (#203).
  assert.match(context, /runSkill: \(taskId: string, skillId: string, prompt\?: string, opts\?: \{ withComments\?: boolean; mode\?: SkillMode; model\?: string \}\)/)
})

test('a full chain run forces the autonomous mode instead of resolving it', () => {
  assert.match(context, /auto \? 'autonomous' : mode/)
})

test('each skill card can be started in either mode', () => {
  // The card stopped being a single button so it can hold the two mode buttons:
  // a button nested in a button is invalid HTML.
  assert.match(modal, /handleTriggerSkill\(s\.id, undefined, 'interactive'\)/)
  assert.match(modal, /handleTriggerSkill\(s\.id, undefined, 'autonomous'\)/)
  // The card body still launches in whatever the precedence resolves.
  assert.match(modal, /onClick=\{\(\) => handleTriggerSkill\(s\.id\)\}/)
  // Every control is named, since three buttons per card are otherwise
  // indistinguishable to a screen reader. The names come from the catalog
  // (#527, see taskDetailCatalog.test.mjs for their wording).
  assert.match(modal, /aria-label=\{format\(td\.workflow\.launchInteractive, \{ skill: s\.name \}\)\}/)
  assert.match(modal, /aria-label=\{format\(td\.workflow\.launchAutonomous, \{ skill: s\.name \}\)\}/)
  assert.match(modal, /aria-label=\{format\(td\.workflow\.launchConfigured, \{ skill: s\.name \}\)\}/)
})

test('a card choice is the only mode override of the detail view', () => {
  assert.match(modal, /const handleTriggerSkill = async \(skillId: string, overridePrompt\?: string, modeOverride\?: SkillMode\)/)
  assert.match(modal, /\{ mode: modeOverride \}/)
  // The panel-wide mode selector was removed: the card buttons carry the choice.
  assert.doesNotMatch(modal, /launchMode/)
})

test('the card ... menu offers both modes for a single launch', () => {
  assert.match(card, /handleAdvance\(false, 'interactive'\)/)
  assert.match(card, /handleAdvance\(false, 'autonomous'\)/)
  // The full chain takes no override: it is autonomous by construction.
  assert.match(card, /handleAdvance\(true\)/)
})

test('both card shapes share one definition of the mode entries', () => {
  // The two entries used to sit inside the condensed-only block, which left the
  // override unreachable on an expanded card: its inline chevrons carry no mode.
  // They are now one fragment rendered from both branches. The assertions above
  // match whether that fragment is rendered once or twice, so they cannot catch
  // the expanded branch going missing again; these can.
  const definitions = card.match(/const modeActions = \(/g) ?? []
  assert.equal(definitions.length, 1, 'the mode entries are defined once, not duplicated per shape')

  const condensed = card.match(/\{isCondensed && \([\s\S]*?\n {10}\)\}/)
  assert.ok(condensed, 'the condensed branch is still there')
  assert.match(condensed[0], /\{modeActions\}/)
  // The condensed order is unchanged: advance, then the modes, then the chain.
  assert.match(
    condensed[0],
    /handleAdvance\(false\)[\s\S]*\{modeActions\}[\s\S]*handleAdvance\(true\)/,
  )

  const expanded = card.match(/\{!isCondensed && \([\s\S]*?\n {10}\)\}/)
  assert.ok(expanded, 'the expanded card has its own menu branch')
  assert.match(expanded[0], /\{modeActions\}/)
  // It gains the modes and a separator only: pin, parent filter and pull request
  // are already inline on an expanded card.
  assert.doesNotMatch(expanded[0], /togglePin|setParentFilter|task\.prUrl/)
})

test('the project modal edits both execution settings', () => {
  assert.match(project, /defaultSkillMode/)
  assert.match(project, /fullChainStopStage/)
  assert.match(project, /<option value="autonomous">/)
  assert.match(project, /<option value="implemented">/)
})

test('the skill editor writes a ternary mode', () => {
  assert.match(skills, /saveSkillMode\(selected\.id, e\.target\.value as SkillMode\)/)
  // The empty option is what hands the decision back to the project default.
  // Its label comes from the catalog (#531, see skillsEditorCatalog.test.mjs).
  assert.match(skills, /\{ value: '', label: modes\.projectDefault/)
  assert.match(context, /\/mode`/)
})
