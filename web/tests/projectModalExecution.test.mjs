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

// The project selects trackers and a label (#741); the tracker's own settings
// (board, columns, default mapping, issue types, background sync) are an
// admin's, on the tracker (D11), so no save of the project carries them. The
// project's own stage mapping per tracker travels as trackerStageColumns, never
// as the legacy stageColumns.
const LEGACY_TRACKER_KEYS = [
  'issueTracker',
  'trackerUrl',
  'githubRepo',
  'gitlabProject',
  'jiraProject',
  'boardId',
  'trackerColumns',
  'stageColumns',
  'issueTypes',
  'autoSyncEnabled',
  'autoSyncIntervalMin',
]

test('the project payload carries its trackers, label and own stage mappings, never the tracker settings', () => {
  const payload = modal.match(/const payload = \{[\s\S]*?\n {6}\}/)
  assert.ok(payload, 'the project payload is still built in one place')
  assert.match(payload[0], /\.\.\.selection/, 'the payload carries the tracker selection')
  assert.match(payload[0], /trackerStageColumns/, "the payload carries the project's own stage mappings")
  for (const key of LEGACY_TRACKER_KEYS) {
    assert.doesNotMatch(payload[0], new RegExp(`\\b${key}\\b`), `the project payload carries ${key}`)
  }
  assert.doesNotMatch(modal, /BoardColumnsEditor/, 'the column mapping editor left the project settings')
  assert.doesNotMatch(sync, /updateProject\(/, 'the sync view no longer rewrites the project tracker')
})
