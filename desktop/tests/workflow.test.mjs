import assert from 'node:assert/strict'
import { test } from 'node:test'

const { skillLabel } = await import('../src/workflow.mjs')

test('every workflow skill reads as the workflow names it', () => {
  assert.equal(skillLabel('clarify'), 'Clarify')
  assert.equal(skillLabel('specify'), 'Specify')
  assert.equal(skillLabel('implement'), 'Implement')
  assert.equal(skillLabel('adjust'), 'Adjust')
  assert.equal(skillLabel('handoff'), 'Handoff')
  assert.equal(skillLabel('create_pr'), 'Create PR')
})

test('any other skill is capitalized', () => {
  assert.equal(skillLabel('pickup'), 'Pickup')
  assert.equal(skillLabel('discuss'), 'Discuss')
  assert.equal(skillLabel(' custom '), 'Custom')
})

test('a missing skill has no label', () => {
  assert.equal(skillLabel(''), '')
  assert.equal(skillLabel('   '), '')
  assert.equal(skillLabel(undefined), '')
  assert.equal(skillLabel(null), '')
})
