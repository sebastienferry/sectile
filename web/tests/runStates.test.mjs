import assert from 'node:assert/strict'
import { test } from 'node:test'
import { runStateOf, runStateLabel, runState, runStateSvg, RUN_STATES } from '../../shared/runStates.ts'

test('a running activity that reported itself blocked is waiting', () => {
  // This is the state the activities view filters on and had no badge for.
  assert.equal(runStateOf({ status: 'running', waitingSince: '2026-09-17T10:00:00Z' }), 'waiting')
  assert.equal(runStateOf({ status: 'running' }), 'running')
})

test('an ended run reports its outcome, whatever it was waiting for', () => {
  for (const status of ['completed', 'failed', 'canceled']) {
    assert.equal(runStateOf({ status }), status)
    assert.equal(runStateOf({ status, waitingSince: '2026-09-17T10:00:00Z' }), status)
  }
})

test('the queued spellings of both surfaces are one state', () => {
  assert.equal(runStateOf({ status: 'queued' }), 'queued')
  assert.equal(runStateOf({ status: 'pending' }), 'queued')
  assert.equal(runStateOf({ status: 'preparing' }), 'queued')
})

test('an unknown or absent status still resolves to a defined state', () => {
  // A surface renders what it is given: no status may leave it with nothing.
  for (const input of [{}, { status: '' }, { status: 'nonsense' }]) {
    assert.ok(runState(runStateOf(input)), 'state must be defined for ' + JSON.stringify(input))
  }
})

test('every defined state carries a label, and an unknown id reports itself', () => {
  for (const id of Object.keys(RUN_STATES)) assert.equal(runStateLabel(id), RUN_STATES[id].label)
  assert.equal(runStateLabel('seventh'), 'seventh')
})

test('running is a filled dot whose fill resolves to the state colour on its own', () => {
  // The notification icon is standalone markup: nothing around it sets the
  // colour a `currentColor` fill would otherwise inherit.
  const svg = runStateSvg('running', 14)
  assert.match(svg, /<circle [^>]*fill="currentColor"/)
  assert.match(svg, new RegExp('<svg [^>]*color="' + RUN_STATES.running.color + '"'))
  assert.equal(RUN_STATES.running.pulses, true)
})
