import assert from 'node:assert/strict'
import { test } from 'node:test'
import { normalizeEngineReport, reportedModel, reportedPickerModels } from '../src/lib/aiModels.ts'

// The engine a card announces is what the caller's workstation reported (#305).
const report = normalizeEngineReport({
  state: 'reported', provider: 'claude', model: 'claude-opus-5',
  skillModels: { implement: 'claude-sonnet-5', blank: ' ' },
  models: ['claude-opus-5', 'claude-sonnet-5', 'claude-haiku-4-5', 'claude-haiku-4-5'],
  modelSlot: true, headless: true,
})

test('a reported engine names the model of each skill', () => {
  assert.equal(reportedModel(report, 'implement'), 'claude-sonnet-5')
  assert.equal(reportedModel(report, 'clarify'), 'claude-opus-5')
  assert.equal(reportedModel(report), 'claude-opus-5')
})

test('the picker offers the reported list minus the model that would run', () => {
  assert.deepEqual(reportedPickerModels(report, 'clarify'), ['claude-sonnet-5', 'claude-haiku-4-5'])
  assert.deepEqual(reportedPickerModels(report, 'implement'), ['claude-opus-5', 'claude-haiku-4-5'])
})

test('an unknown engine offers no picker and names no model', () => {
  const unknown = normalizeEngineReport({ state: 'unknown' })
  assert.equal(unknown.state, 'unknown')
  assert.equal(reportedModel(unknown, 'clarify'), '')
  assert.deepEqual(reportedPickerModels(unknown, 'clarify'), [])
  // An error body or an older server reads as unknown too.
  assert.equal(normalizeEngineReport({ error: 'nope' }).state, 'unknown')
  assert.equal(normalizeEngineReport(null).state, 'unknown')
})

test('a command line without a model slot hides the picker and the model', () => {
  const noSlot = normalizeEngineReport({ state: 'reported', provider: 'claude', model: 'opus', models: ['opus', 'sonnet'] })
  assert.equal(noSlot.modelSlot, false)
  assert.equal(reportedModel(noSlot, 'clarify'), '')
  assert.deepEqual(reportedPickerModels(noSlot, 'clarify'), [])
})
