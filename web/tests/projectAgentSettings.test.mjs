import assert from 'node:assert/strict'
import { test } from 'node:test'
import { hasProjectAgentOverride, projectAgentSettings } from '../src/lib/projectAgentSettings.ts'

test('disabling the project agent clears its persisted override before reopening', () => {
  const project = { aiProvider: 'claude', aiModel: 'claude-opus-5' }
  assert.equal(hasProjectAgentOverride(project.aiProvider, project.aiModel), true)

  const saved = { ...project, ...projectAgentSettings(false, project.aiProvider, project.aiModel) }
  assert.deepEqual(saved, { aiProvider: '', aiModel: '' })

  // This is the state returned after the modal saves and reloads the project.
  assert.equal(hasProjectAgentOverride(saved.aiProvider, saved.aiModel), false)
})

test('enabling the project agent keeps an explicit provider and trims its model', () => {
  assert.deepEqual(
    projectAgentSettings(true, 'codex', '  gpt-5.6  '),
    { aiProvider: 'codex', aiModel: 'gpt-5.6' },
  )
})
