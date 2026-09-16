import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'
import ts from 'typescript'
const source = await readFile(new URL('../src/lib/remoteRunIndicator.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } })
const { deriveRunIndicator, CANCELED_VISIBILITY_MS } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)

const NOW = Date.parse('2026-01-01T12:00:00Z')
const AGENT_OWNED = 'Agent-owned remote execution'

function run(overrides) {
  return { id: 'run-1', taskId: 'task-1', skillId: 'remote_run', skillName: 'pickup', action: AGENT_OWNED, status: 'running', summary: '', output: '', steps: [], createdAt: '', ...overrides }
}

test('no remote run leaves the indicator hidden', () => {
  assert.equal(deriveRunIndicator([], 'task-1', NOW), null)
  const other = run({ skillId: 'tracker_op' })
  assert.equal(deriveRunIndicator([other], 'task-1', NOW), null)
  assert.equal(deriveRunIndicator([run({})], 'task-2', NOW), null)
})

test('a running run outranks a queued one and counts both', () => {
  const indicator = deriveRunIndicator([run({ id: 'a', status: 'queued' }), run({ id: 'b' })], 'task-1', NOW)
  assert.equal(indicator.state, 'running')
  assert.equal(indicator.count, 1)
  assert.deepEqual(indicator.cancelableRunIds, ['b'])
})

test('queued runs are grouped under a single indicator', () => {
  const indicator = deriveRunIndicator([run({ id: 'a', status: 'queued' }), run({ id: 'b', status: 'queued' })], 'task-1', NOW)
  assert.equal(indicator.state, 'queued')
  assert.equal(indicator.count, 2)
  assert.deepEqual(indicator.cancelableRunIds, ['a', 'b'])
})

test('a run that is not agent-owned offers no cancellation', () => {
  const indicator = deriveRunIndicator([run({ action: 'Scheduled execution' })], 'task-1', NOW)
  assert.equal(indicator.state, 'running')
  assert.deepEqual(indicator.cancelableRunIds, [])
})

test('a cancelled run stays visible for the window then disappears', () => {
  const justCanceled = run({ status: 'canceled', completedAt: new Date(NOW - CANCELED_VISIBILITY_MS).toISOString() })
  const visible = deriveRunIndicator([justCanceled], 'task-1', NOW)
  assert.equal(visible.state, 'canceled')
  assert.deepEqual(visible.cancelableRunIds, [])

  const expired = run({ status: 'canceled', completedAt: new Date(NOW - CANCELED_VISIBILITY_MS - 1).toISOString() })
  assert.equal(deriveRunIndicator([expired], 'task-1', NOW), null)
})

test('a cancelled run without a usable end timestamp is ignored', () => {
  assert.equal(deriveRunIndicator([run({ status: 'canceled' })], 'task-1', NOW), null)
  assert.equal(deriveRunIndicator([run({ status: 'canceled', completedAt: 'not-a-date' })], 'task-1', NOW), null)
})

test('an active run hides a recent cancellation', () => {
  const activities = [run({ id: 'a', status: 'canceled', completedAt: new Date(NOW).toISOString() }), run({ id: 'b', status: 'queued' })]
  const indicator = deriveRunIndicator(activities, 'task-1', NOW)
  assert.equal(indicator.state, 'queued')
})
