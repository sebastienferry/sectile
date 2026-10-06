import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { PICKUP_MODE, launchModeFor, runProjectRefusal, runProjectRefusalText } from '../src/run-project.mjs'

const require = createRequire(import.meta.url)
const { launchRequest, runProjectDialog, chosenRunProject } = require('../electron/run-project.cjs')

// A run works for one project (#741); the desktop names the project of the
// board a launch is made from, and asks when the server still cannot choose.

// What crosses the IPC when the agent passes the server's refusal on.
const ipcError = body => `Error invoking remote method 'launch-server-task': Error: ${JSON.stringify(body)}`
const candidates = [{ id: 'da', name: 'Delivery admin' }, { id: 'ba', name: 'Bidder admin' }]

test('launch passes the project of the board to the run', () => {
  const request = launchRequest('da', 'task-1', 'clarify', '', '', false, 'terminal')
  assert.equal(request.route, '/desktop/tasks?projectId=da', 'the agent forwards the project of the address to the run')
  assert.deepEqual(request.body, { taskID: 'task-1', skillID: 'clarify', prompt: '' }, 'no override travels without a choice')
  assert.equal(launchRequest('a b', 't', 's', '', 'interactive', true, 'conversation').route, '/desktop/tasks?projectId=a%20b')
  assert.deepEqual(launchRequest('da', 't', 's', 'p', 'interactive', true, 'conversation').body, {
    taskID: 't', skillID: 's', prompt: 'p', mode: 'interactive', force: true, view: 'conversation',
  })
})

test('an interactive launch on a two-project ticket asks which project', () => {
  const refusal = runProjectRefusal(ipcError({ error: 'ce ticket appartient à plusieurs projets', candidates, unattended: false }))
  assert.ok(refusal, 'the 409 is read as a choice to make')
  assert.equal(refusal.unattended, false)
  assert.deepEqual(refusal.candidates, candidates)

  const dialog = runProjectDialog(refusal.candidates, 'GODE-1')
  assert.deepEqual(dialog.buttons, ['Delivery admin', 'Bidder admin', 'Cancel'])
  assert.equal(dialog.cancelId, 2)
  assert.match(dialog.detail, /^GODE-1 belongs to several projects/)
  assert.equal(chosenRunProject(refusal.candidates, 1), 'ba', 'the launch is made again from the project picked')
  assert.equal(chosenRunProject(refusal.candidates, 2), null, 'Cancel gives the launch up')
  assert.equal(chosenRunProject(refusal.candidates, -1), null)

  // A busy ticket is refused too, without candidates: no question.
  assert.equal(runProjectRefusal(ipcError({ error: 'A run is still active.', activeRunId: 'r1' })), null)
  assert.equal(runProjectRefusal('Task does not belong to project'), null)
})

test('a pickup refusal shows the candidate projects', () => {
  // A pickup is unattended: it names the autonomous mode explicitly, so the
  // server refuses it on a ticket of several projects instead of asking.
  assert.equal(PICKUP_MODE, 'autonomous')
  assert.equal(launchModeFor('pickup', undefined), 'autonomous')
  assert.equal(launchModeFor('pickup', 'interactive'), 'autonomous')
  assert.equal(launchModeFor('next', undefined), undefined, 'the next step keeps no override')
  assert.equal(launchModeFor('', 'interactive'), 'interactive')

  const refusal = runProjectRefusal(ipcError({ error: 'lancement automatique refusé', candidates, unattended: true }))
  assert.equal(refusal.unattended, true)
  const text = runProjectRefusalText(refusal)
  assert.match(text, /^lancement automatique refusé/)
  assert.match(text, /Projects: Delivery admin, Bidder admin\.$/)
})
