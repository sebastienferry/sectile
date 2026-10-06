import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { PICKUP_MODE, launchModeFor } from '../src/run-project.mjs'

const require = createRequire(import.meta.url)
const { launchRequest } = require('../electron/run-project.cjs')

// A run works for one project (#741); the desktop names the project of the
// board a launch is made from, and the agent forwards it to the run.

test('launch passes the project of the board to the run', () => {
  const request = launchRequest('da', 'task-1', 'clarify', '', '', false, 'terminal')
  assert.equal(request.route, '/desktop/tasks?projectId=da', 'the agent forwards the project of the address to the run')
  assert.deepEqual(request.body, { taskID: 'task-1', skillID: 'clarify', prompt: '' }, 'no override travels without a choice')
  assert.equal(launchRequest('a b', 't', 's', '', 'interactive', true, 'conversation').route, '/desktop/tasks?projectId=a%20b')
  assert.deepEqual(launchRequest('da', 't', 's', 'p', 'interactive', true, 'conversation').body, {
    taskID: 't', skillID: 's', prompt: 'p', mode: 'interactive', force: true, view: 'conversation',
  })
})

test('a pickup launches in the autonomous mode', () => {
  // A pickup is unattended: it names the autonomous mode explicitly.
  assert.equal(PICKUP_MODE, 'autonomous')
  assert.equal(launchModeFor('pickup', undefined), 'autonomous')
  assert.equal(launchModeFor('pickup', 'interactive'), 'autonomous')
  assert.equal(launchModeFor('next', undefined), undefined, 'the next step keeps no override')
  assert.equal(launchModeFor('', 'interactive'), 'interactive')
})
