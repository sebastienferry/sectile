import assert from 'node:assert/strict'
import { test } from 'node:test'
import { projectFields, ownEntries, compact, parseModelList, sourceHint, workstationPayload, validSkillCommand } from '../src/execution-fields.mjs'

test('project fields come from the agent, with their source', () => {
  const fields = projectFields({ fields: { defaultEngine: { value: 'e-codex', inherited: 'e-opus', source: 'project' } }, terminal: 'iterm', terminalOverride: true })
  assert.deepEqual(fields.defaultEngine, { value: 'e-codex', inherited: 'e-opus', source: 'project' })
  // An agent that predates #305 answers flat keys: they read as inherited
  // from the workstation unless their override flag says otherwise.
  assert.equal(fields.terminal.value, 'iterm')
  assert.equal(fields.terminal.source, 'project')
  assert.equal(fields.skillCommands.source, 'default')
  // The engine fields left the project settings with the catalogue (#510).
  assert.equal(fields.aiProvider, undefined)
  assert.equal(projectFields({}).defaultEngine.source, 'default')
})

test('a map field states only what differs from the inherited one', () => {
  assert.deepEqual(ownEntries({ source: 'project', value: { implement: 'a', clarify: 'b' }, inherited: { clarify: 'b' } }), { implement: 'a' })
  assert.deepEqual(ownEntries({ source: 'workstation', value: { implement: 'a' } }), {})
  assert.deepEqual(compact({ ' implement ': ' a ', clarify: ' ', '': 'x' }), { implement: 'a' })
  assert.deepEqual(parseModelList('opus, sonnet\nopus,,haiku'), ['opus', 'sonnet', 'haiku'])
})

test('hints say where a value comes from', () => {
  assert.match(sourceHint('project', 'claude'), /Set for this project · Inherited: claude/)
  assert.match(sourceHint('workstation', 'claude'), /Inherited from workstation: claude/)
  assert.match(sourceHint('default', true), /Inherited default: Yes/)
})

test('the workstation payload omits what inherits and keeps an emptied list as a choice', () => {
  const payload = workstationPayload({
    aiProvider: 'claude', aiModel: 'opus', aiSkillModels: { implement: 'sonnet' }, editorCommand: ' zed ',
    useWorktrees: false, parallelism: 0, setupProviders: [], aiProviderModels: { claude: [], codex: ['gpt-5'] },
  })
  // The engine fields are the catalogue's (#510): the agent refuses them here.
  assert.deepEqual(payload, {
    editorCommand: 'zed', useWorktrees: false,
    setupProviders: [], aiProviderModels: { claude: [], codex: ['gpt-5'] },
  })
  assert.equal(workstationPayload({ setupProviders: null }).setupProviders, null)
})

test('the workstation payload hands an older agent its own engine fields back', () => {
  // An agent that predates #510 replaces the defaults whole: dropping what it
  // served would erase its provider, model and templates.
  const served = { aiProvider: 'codex', aiModel: 'gpt-5', aiSkillModels: { implement: 'o3' }, aiCommandTemplate: '', editorCommand: 'code' }
  assert.deepEqual(workstationPayload({ editorCommand: 'zed', setupProviders: null }, served), {
    aiProvider: 'codex', aiModel: 'gpt-5', aiSkillModels: { implement: 'o3' }, editorCommand: 'zed', setupProviders: null,
  })
  // A current agent serves none, so nothing is sent.
  assert.deepEqual(workstationPayload({ setupProviders: null }, { editorCommand: 'code' }), { setupProviders: null })
})

test('a skill command name is a single word', () => {
  assert.ok(validSkillCommand('/code-issue'))
  assert.ok(validSkillCommand(''))
  assert.ok(!validSkillCommand('two words'))
})
