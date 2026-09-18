import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

// Like the execution mode before it, the launch model is wired through JSX
// handlers that no exported function reaches, so these files are the only
// record of the wiring. Asserting on the source is weaker than a behavioural
// test and is what stops an unrelated refactor from dropping a field.
const read = name => readFile(new URL(`../src/${name}`, import.meta.url), 'utf8')
const models = await read('lib/aiModels.ts')
const context = await read('context/AppContext.tsx')
const card = await read('components/TaskCard.tsx')
const modal = await read('components/TaskDetailModal.tsx')
const profile = await read('components/ProfileModal.tsx')
const providerField = await read('components/ProviderModelsField.tsx')
const modelField = await read('components/AIModelField.tsx')
const activities = await read('components/ActivitiesView.tsx')

test('the models offered come from the settings, with the shipped list as fallback', () => {
  assert.match(models, /export function providerModels\(/)
  assert.match(models, /settings\?\.aiProviderModels\?\.\[provider\]/)
  assert.match(models, /if \(configured && configured\.length > 0\) return configured/)
  assert.match(models, /return DEFAULT_PROVIDER_MODELS\[provider as AIProvider\] \|\| \[\]/)
  // The hardcoded suggestion map is gone: one list now feeds every surface.
  assert.doesNotMatch(models, /AI_MODEL_SUGGESTIONS/)
  assert.doesNotMatch(modelField, /AI_MODEL_SUGGESTIONS/)
  assert.match(modelField, /providerModels\(settings, provider\)/)
})

test('the configured model follows the most specific statement', () => {
  // Per-skill entries first, project before global, exactly as the server.
  assert.match(models, /const fromProject = \(project\?\.aiSkillModels\?\.\[skill\] \|\| ''\)\.trim\(\)/)
  assert.match(models, /const fromSettings = \(settings\?\.aiSkillModels\?\.\[skill\] \|\| ''\)\.trim\(\)/)
  assert.match(models, /return \(project\?\.aiModel \|\| ''\)\.trim\(\) \|\| \(settings\?\.aiModel \|\| ''\)\.trim\(\)/)
})

test('the launch request carries the model, and an untouched choice sends none', () => {
  assert.match(context, /opts\?: \{ withComments\?: boolean; mode\?: SkillMode; model\?: string \}/)
  // An empty value must not be sent: it would outrank the workstation override.
  assert.match(context, /model: opts\?\.model\?\.trim\(\) \|\| undefined/)
  // The card path goes through advanceTask, which now forwards the model.
  assert.match(context, /const advanceTask = async \(taskId: string, auto\?: boolean, mode\?: SkillMode, model\?: string\)/)
  assert.match(context, /model:auto \? undefined : model/)
})

test('the detail launcher offers a list, never a free-text model', () => {
  assert.match(modal, /const \[launchModel, setLaunchModel\] = useState\(''\)/)
  assert.match(modal, /const launchModels = providerModels\(settings, activeProvider\)/)
  // A select, beside the existing mode select; no text input for the model.
  assert.match(modal, /value=\{launchModel\}[\s\S]{0,200}onChange=\{e => setLaunchModel\(e\.target\.value\)\}/)
  assert.match(modal, /<option value="">\s*\{configuredLaunchModel \? `Modèle configuré/)
  // Every launch control of the view carries it.
  assert.match(modal, /runSkill\(selectedTask\.id, skillId, promptToUse, \{ mode: modeOverride \?\? launchMode, model: launchModel \}\)/)
  // Nothing is offered when the provider has no configured model.
  assert.match(modal, /\{launchModels\.length > 0 && \(/)
})

test('the card menu offers the next step under a chosen model', () => {
  assert.match(card, /const cardModels = providerModels\(settings, cardProvider\)/)
  assert.match(card, /const configuredCardModel = resolveConfiguredModel\(/)
  // The configured model is first and sends no override.
  assert.match(card, /onClick=\{\(\) => \{ closeMenu\(\); handleAdvance\(false\) \}\}/)
  assert.match(card, /const offeredModels = cardModels\.filter\(model => model !== configuredCardModel\)/)
  assert.match(card, /onClick=\{\(\) => \{ closeMenu\(\); handleAdvance\(false, undefined, model\) \}\}/)
  // A submenu, not a form: no input element anywhere in the card menu.
  assert.doesNotMatch(card, /<input/)
  assert.match(card, /aria-haspopup="menu"/)
  assert.match(card, /role="menuitem"/)
  // Keyboard: right opens, left and escape close.
  assert.match(card, /e\.key === 'ArrowRight'/)
  assert.match(card, /e\.key === 'ArrowLeft' \|\| e\.key === 'Escape'/)
  // Hidden when the provider has nothing to offer.
  assert.match(card, /\{cardModels\.length > 0 && \(/)
})

test('both card shapes share the model entry', () => {
  // The entry lives in the fragment both branches render, which is what stops
  // one shape from losing it on its own, exactly as for the mode entries.
  const definitions = card.match(/const modeActions = \(/g) ?? []
  assert.equal(definitions.length, 1, 'the launch entries are defined once, not duplicated per shape')
  const fragment = card.match(/const modeActions = \([\s\S]*?\n {2}\)\n/)
  assert.ok(fragment, 'the shared fragment is still there')
  assert.match(fragment[0], /advanceWithModel/)
  const condensed = card.match(/\{isCondensed && \([\s\S]*?\n {10}\)\}/)
  const expanded = card.match(/\{!isCondensed && \([\s\S]*?\n {10}\)\}/)
  assert.match(condensed[0], /\{modeActions\}/)
  assert.match(expanded[0], /\{modeActions\}/)
})

test('the profile edits the per-provider model list', () => {
  assert.match(profile, /<ProviderModelsField provider=\{aiProvider\} value=\{aiProviderModels\} onChange=\{setAiProviderModels\} \/>/)
  assert.match(profile, /aiProviderModels,/)
  // A malformed entry blocks the save, like the model field already does.
  assert.match(profile, /Object\.values\(aiProviderModels\)\.every\(list => list\.every\(model => isValidModel\(model\)\)\)/)
  assert.match(providerField, /isValidModel\(draft\)/)
})

test('a run says which engine it ran against', () => {
  assert.match(activities, /runEngineLabel\(act\)/)
  assert.match(activities, /runEngineLabel\(selectedActivity\)/)
  assert.match(modal, /runEngineLabel\(act\)/)
})
