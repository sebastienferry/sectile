import assert from 'node:assert/strict'
import { test } from 'node:test'
import { deriveRunIndicator, activeTaskIds, CANCELED_VISIBILITY_MS } from '../src/lib/remoteRunIndicator.ts'

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

const WAITED = new Date(NOW - 4 * 60_000).toISOString()

test('a waiting run outranks a running one and still counts both', () => {
  const activities = [run({ id: 'a' }), run({ id: 'b', waitingSince: WAITED })]
  const indicator = deriveRunIndicator(activities, 'task-1', NOW)
  assert.equal(indicator.state, 'waiting')
  // The task has two live runs; the label must not claim it has one.
  assert.equal(indicator.count, 2)
  assert.deepEqual(indicator.cancelableRunIds, ['a', 'b'])
  assert.equal(indicator.waitingSince, WAITED)
})

test('the longest wait is the one reported', () => {
  const later = new Date(NOW - 60_000).toISOString()
  const activities = [run({ id: 'a', waitingSince: later }), run({ id: 'b', waitingSince: WAITED })]
  assert.equal(deriveRunIndicator(activities, 'task-1', NOW).waitingSince, WAITED)
})

test('resuming returns the indicator to running', () => {
  const indicator = deriveRunIndicator([run({ id: 'a', waitingSince: undefined })], 'task-1', NOW)
  assert.equal(indicator.state, 'running')
  assert.equal(indicator.waitingSince, undefined)
})

test('a waiting mark left on a run that is no longer running is ignored', () => {
  // The state is read off the run, not off the timestamp: a stale mark on a
  // finished run must never make it look alive.
  const finished = run({ status: 'completed', waitingSince: WAITED })
  assert.equal(deriveRunIndicator([finished], 'task-1', NOW), null)
  const queued = run({ status: 'queued', waitingSince: WAITED })
  assert.equal(deriveRunIndicator([queued], 'task-1', NOW).state, 'queued')
})

test('a waiting run without a usable timestamp still shows as waiting', () => {
  const indicator = deriveRunIndicator([run({ waitingSince: 'not-a-date' })], 'task-1', NOW)
  assert.equal(indicator.state, 'waiting')
  assert.equal(indicator.waitingSince, undefined)
})

test('the active set holds the tasks a run is working on', () => {
  const activities = [
    run({ id: 'a', taskId: 'task-1', status: 'running' }),
    run({ id: 'b', taskId: 'task-2', status: 'queued' }),
    run({ id: 'c', taskId: 'task-3', status: 'running', waitingSince: '2026-01-01T11:00:00Z' }),
  ]
  assert.deepEqual([...activeTaskIds(activities)].sort(), ['task-1', 'task-2', 'task-3'])
})

test('a finished or just-canceled run leaves its task out of the active set', () => {
  const ended = ['completed', 'failed', 'canceled'].map((status, index) =>
    run({ id: `end-${index}`, taskId: `task-${index}`, status, completedAt: new Date(NOW).toISOString() }))
  assert.equal(activeTaskIds(ended).size, 0)
  // The indicator still shows the cancellation; the filter does not keep the task.
  assert.equal(deriveRunIndicator([ended[2]], 'task-2', NOW).state, 'canceled')
})

test('the active set ignores what is not a remote run, and has no entry without one', () => {
  assert.equal(activeTaskIds([]).size, 0)
  assert.equal(activeTaskIds([run({ skillId: 'tracker_op', status: 'running' })]).size, 0)
})

test('two runs on one task yield a single entry', () => {
  const activities = [
    run({ id: 'a', taskId: 'task-1', status: 'running' }),
    run({ id: 'b', taskId: 'task-1', status: 'queued' }),
  ]
  assert.deepEqual([...activeTaskIds(activities)], ['task-1'])
})

test('the active set does not depend on the order the activities arrive in', () => {
  const a = run({ id: 'a', taskId: 'task-2', status: 'running' })
  const b = run({ id: 'b', taskId: 'task-1', status: 'queued' })
  // The context keys its memo on the sorted members, so a reordered poll must
  // yield the same signature and leave the board and the list untouched.
  const one = [...activeTaskIds([a, b])].sort().join(',')
  const other = [...activeTaskIds([b, a])].sort().join(',')
  assert.equal(one, other)
  assert.equal(one, 'task-1,task-2')
})

const CLIENT = 'Remote skill execution'
const SILENCE = 'Execution reported - No MCP call for 4h0m0s: the client may be busy or waiting for input, and this run stays open'

test('a silent run outranks a working one, and a waiting run outranks both (#319)', () => {
  const silent = run({ id: 'a', summary: SILENCE })
  assert.equal(deriveRunIndicator([silent, run({ id: 'b' })], 'task-1', NOW).state, 'silent')
  assert.equal(deriveRunIndicator([silent, run({ id: 'b', waitingSince: WAITED })], 'task-1', NOW).state, 'waiting')
  // A silence that has since become a wait is shown as the wait.
  assert.equal(deriveRunIndicator([run({ summary: SILENCE, waitingSince: WAITED })], 'task-1', NOW).state, 'waiting')
})

test('a client run is closable by its owner and by an admin only', () => {
  const clientRun = run({ id: 'c', action: CLIENT, userId: 'carol' })
  assert.deepEqual(deriveRunIndicator([clientRun], 'task-1', NOW, { userId: 'carol', role: 'member' }).closableRunIds, ['c'])
  assert.deepEqual(deriveRunIndicator([clientRun], 'task-1', NOW, { userId: 'bob', role: 'member' }).closableRunIds, [])
  assert.deepEqual(deriveRunIndicator([clientRun], 'task-1', NOW, { userId: 'alice', role: 'admin' }).closableRunIds, ['c'])
  // Nobody known yet, nothing offered.
  assert.deepEqual(deriveRunIndicator([clientRun], 'task-1', NOW).closableRunIds, [])
  // An ownerless run is an admin's.
  const legacy = run({ id: 'l', action: CLIENT })
  assert.deepEqual(deriveRunIndicator([legacy], 'task-1', NOW, { userId: 'bob', role: 'member' }).closableRunIds, [])
  assert.deepEqual(deriveRunIndicator([legacy], 'task-1', NOW, { userId: 'alice', role: 'admin' }).closableRunIds, ['l'])
})

test('an agent run is stoppable, never closable, and a cancelled run offers neither', () => {
  const indicator = deriveRunIndicator([run({ userId: 'carol' })], 'task-1', NOW, { userId: 'carol', role: 'admin' })
  assert.deepEqual(indicator.closableRunIds, [])
  assert.deepEqual(indicator.cancelableRunIds, ['run-1'])
  const canceled = run({ action: CLIENT, userId: 'carol', status: 'canceled', completedAt: new Date(NOW).toISOString() })
  assert.deepEqual(deriveRunIndicator([canceled], 'task-1', NOW, { userId: 'carol' }).closableRunIds, [])
})

test('the longest wait carries its reason onto the indicator', () => {
  const parked = run({ id: 'a', waitingSince: '2026-01-01T11:00:00Z', waitingReason: 'repository' })
  const asking = run({ id: 'b', waitingSince: '2026-01-01T11:30:00Z' })
  const indicator = deriveRunIndicator([asking, parked], 'task-1', NOW)
  assert.equal(indicator.state, 'waiting')
  assert.equal(indicator.waitingSince, '2026-01-01T11:00:00Z')
  assert.equal(indicator.waitingReason, 'repository')
  assert.equal(deriveRunIndicator([asking], 'task-1', NOW).waitingReason, undefined)
})
