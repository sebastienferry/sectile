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
const launchModel = await read('lib/launchModel.ts')

test('the models offered come from the settings, with the shipped list as fallback', () => {
  assert.match(models, /export function providerModels\(/)
  assert.match(models, /settings\?\.aiProviderModels\?\.\[provider\]/)
  // An emptied list is a decision, not an absence: only a missing key falls back.
  assert.match(models, /const configured = settings\?\.aiProviderModels\?\.\[provider\]\s*\n\s*if \(configured\) return configured/)
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
  assert.match(context, /\{mode:auto \? 'autonomous' : mode, model\}/)
})

test('the detail view offers no model selector', () => {
  // The model is chosen from the card submenu or the settings; the detail view
  // launches with whatever the precedence resolves.
  assert.doesNotMatch(modal, /launchModel/)
  assert.doesNotMatch(modal, /providerModels/)
})

test('the card submenu selects a model and launches nothing', () => {
  assert.match(card, /const cardModels = providerModels\(settings, cardProvider\)/)
  // The model is resolved for the skill the card actually launches, not for the
  // one the next-step label names: at stage reviewed they differ.
  assert.match(card, /const cardSkillId = skillForStage\(resolveTaskStage\(task, taskProject\)\) \|\| undefined/)
  assert.match(card, /const configuredCardModel = resolveConfiguredModel\(taskProject \|\| undefined, settings, cardSkillId\)/)
  assert.match(card, /const offeredModels = cardModels\.filter\(model => model !== configuredCardModel\)/)
  // Picking a row only changes the selection: no row launches anything.
  assert.match(card, /onClick=\{\(\) => chooseModel\(''\)\}/)
  assert.match(card, /onClick=\{\(\) => chooseModel\(model\)\}/)
  assert.doesNotMatch(card, /chooseModel[\s\S]{0,120}handleAdvance/)
  // The retained one is ticked, and the rows are a single choice.
  assert.match(card, /role="menuitemradio"/)
  assert.match(card, /aria-checked=\{effectiveLaunchModel === ''\}/)
  assert.match(card, /aria-checked=\{effectiveLaunchModel === model\}/)
  // A submenu, not a form: no input element anywhere in the card menu.
  assert.doesNotMatch(card, /<input/)
  assert.match(card, /aria-haspopup="menu"/)
  // Keyboard: right opens the submenu, left closes it and returns focus.
  assert.match(card, /e\.key === 'ArrowRight'/)
  assert.match(card, /if \(e\.key === 'ArrowLeft'\)[\s\S]{0,160}modelEntryRef\.current\?\.focus/)
  // Escape is decided in the document-level handler, which closes the submenu
  // alone when it is open. A React handler inside the portal cannot guarantee
  // that, which is why the outer handler owns the decision.
  assert.match(card, /if \(isModelMenuOpen\) \{\s*\n\s*setIsModelMenuOpen\(false\)/)
  assert.match(card, /\}, \[isMenuOpen, isModelMenuOpen\]\)/)
  // Hidden when the provider has nothing to offer.
  assert.match(card, /\{cardModels\.length > 0 && \(/)
})

test('every launch from the card uses the retained model', () => {
  // One handler, no per-call model: the chevrons, the chain and both modes all
  // go through it, which is what the indicator in front of them promises.
  assert.match(card, /const handleAdvance = async \(auto: boolean, mode\?: SkillMode\) => \{/)
  assert.match(card, /await advanceTask\(task\.id, auto, mode, effectiveLaunchModel\)/)
  // The full chain carries it too: from a card it is a single pickup run.
  assert.match(context, /model\}\)$/m)
  assert.doesNotMatch(context, /model:auto \? undefined : model/)
})

test('the card shows the model its buttons will use', () => {
  // Four characters at most, the full name in the tooltip.
  assert.match(card, /const launchedModel = effectiveLaunchModel \|\| configuredCardModel/)
  assert.match(card, /const modelIndicator = launchedModel \? \(/)
  // Discreet: coloured text only, no badge chrome competing with the buttons.
  assert.doesNotMatch(card, /modelIndicator[\s\S]{0,400}rounded|modelIndicator[\s\S]{0,400}border/)
  assert.match(card, /\{shortModelLabel\(launchedModel\)\}/)
  assert.match(card, /title=\{\s*effectiveLaunchModel\s*\? `Modèle retenu pour cette tâche : \$\{launchedModel\}`/)

  // One definition, rendered by both shapes: a condensed card keeps its actions
  // behind the menu, so the indicator precedes that menu there, and precedes the
  // chevrons on an expanded card. Neither shape can lose it on its own.
  const definitions = card.match(/const modelIndicator = /g) ?? []
  assert.equal(definitions.length, 1)
  const condensedRow = card.match(/<RemoteRunBadge taskId=\{task\.id\} \/>\s*\n\s*\{modelIndicator\}\s*\n\s*\{actionsMenu\}/)
  assert.ok(condensedRow, 'the condensed card shows it before its actions menu')
  const expanded = card.indexOf('{modelIndicator}', card.indexOf('Ligne 4'))
  const firstAction = card.indexOf('handleAdvance(false)', card.indexOf('Ligne 4'))
  assert.ok(expanded > 0 && expanded < firstAction, 'the expanded card shows it before the chevrons')
})

test('the selection is remembered per task', () => {
  assert.match(card, /useState\(\(\) => loadLaunchModel\(task\.id\)\)/)
  assert.match(card, /saveLaunchModel\(task\.id, model\)/)
  // A selection the project's engine no longer offers cannot be launched.
  assert.match(card, /const effectiveLaunchModel = cardModels\.includes\(launchModel\) \? launchModel : ''/)
  // Storage failures must not take a card down with them.
  assert.match(launchModel, /} catch \{/)
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
  assert.match(profile, /<ProviderModelsField[\s\S]{0,160}providers=\{AI_PROVIDERS\.map\(p => p\.id\)\}/)
  // Every engine is reachable, not only the one the profile itself runs: a
  // project may run another, and its list has to be editable.
  assert.match(providerField, /providers\.map\(id => \(/)
  assert.match(providerField, /const \[edited, setEdited\] = useState<AIProvider \| ''>\(provider\)/)
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
