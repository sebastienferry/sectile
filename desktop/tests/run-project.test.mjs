import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { NO_RUN_PROJECT, PICKUP_MODE, launchErrorText, launchModeFor, relaunchProject } from '../src/run-project.mjs'

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

test('a run recorded without a project is launched again for the board', () => {
  assert.equal(relaunchProject({ projectId: 'da' }, 'other'), 'da', 'a run keeps its own project')
  assert.equal(relaunchProject({ projectId: '' }, 'board'), 'board', 'a run recorded before #741 falls back to the board')
  assert.equal(relaunchProject({}, 'board'), 'board')
  assert.equal(relaunchProject({ projectId: '' }, ''), '', 'neither leaves the relaunch without a project')
  assert.equal(relaunchProject(undefined, undefined), '')
  assert.match(NO_RUN_PROJECT, /^This execution was recorded without a project\./)
})

test('a refused launch shows the reason, not the raw body', () => {
  const refusal = JSON.stringify({ error: 'This ticket belongs to several projects', candidates: [{ id: 'a', name: 'Alpha' }], unattended: false })
  // The structured body crosses the IPC bridge inside the error text.
  assert.equal(launchErrorText(new Error("Error invoking remote method 'launch-server-task': Error: " + refusal)), 'This ticket belongs to several projects')
  assert.equal(launchErrorText(new Error(refusal)), 'This ticket belongs to several projects')
  // A plain reason is shown as the agent gave it, without Electron's wrapper.
  assert.equal(launchErrorText(new Error("Error invoking remote method 'launch-server-task': Error: Unknown project skill")), 'Unknown project skill')
  // A body without a reason, or text that is not JSON, is shown whole.
  assert.equal(launchErrorText(new Error('{"active":true}')), '{"active":true}')
  assert.equal(launchErrorText(new Error('broken {json')), 'broken {json')
  assert.equal(launchErrorText('plain'), 'plain')
})
