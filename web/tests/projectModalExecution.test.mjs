import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

// The project dialog edits no execution setting (#305): those are the
// workstation's, set in the desktop app. Asserting on the source is what keeps
// a later edit from bringing an editor or a payload field back.
const modal = await readFile(new URL('../src/components/ProjectModal.tsx', import.meta.url), 'utf8')
const sync = await readFile(new URL('../src/components/SyncView.tsx', import.meta.url), 'utf8')
const EXECUTION_KEYS = ['repoPath', 'useWorktrees', 'setupProviders', 'skillOverrides', 'aiProvider', 'aiModel', 'aiSkillModels', 'aiCommandTemplate', 'ttyMode', 'externalTerminalCommand']

test('the project dialog has no agent settings category', () => {
  assert.doesNotMatch(modal, /id: 'agent'/)
  for (const id of ['general', 'tracker', 'workflow', 'skills']) {
    assert.match(modal, new RegExp(`id: '${id}'`))
  }
})

test('the PR creation stage is edited in the agentic workflow category', () => {
  const workflow = modal.indexOf("=== 'workflow'")
  const field = modal.indexOf('id="prCreationStage"')
  assert.ok(workflow > 0 && field > workflow, 'the PR creation stage sits under the workflow category')
})

test('no save carries an execution setting', () => {
  const payload = modal.match(/const payload = \{[\s\S]*?\n {6}\}/)
  assert.ok(payload, 'the project payload is still built in one place')
  for (const key of EXECUTION_KEYS) {
    assert.doesNotMatch(payload[0], new RegExp(`\\b${key}\\b`), `the project payload carries ${key}`)
    assert.doesNotMatch(sync, new RegExp(`\\b${key}\\b`), `the sync view still handles ${key}`)
  }
})
