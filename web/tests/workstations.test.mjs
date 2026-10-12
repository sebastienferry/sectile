import assert from 'node:assert/strict'
import { test } from 'node:test'
import { commonWorkstations, workstationName, mayCancelExecution } from '../src/lib/workstations.ts'

test('batch eligibility intersects identities and deduplicates devices', () => {
  const a = { deviceId: 'a', label: 'Laptop' }, b = { deviceId: 'b', label: 'Laptop' }
  assert.deepEqual(commonWorkstations([[a, b, a], [b]]), [b])
  assert.deepEqual(commonWorkstations([[a], [b]]), [])
  assert.deepEqual(commonWorkstations([]), [])
})
test('duplicate names remain distinguishable and blank labels fall back to identity', () => {
  const devices = [{ deviceId: 'a', label: 'Mac' }, { deviceId: 'b', label: 'Mac' }]
  assert.equal(workstationName(devices[0], devices), 'Mac (a)')
  assert.equal(workstationName({ deviceId: 'legacy', label: ' ' }), 'legacy')
})
test('cancel controls cover all active states and enforce owner/admin permission', () => {
  for (const status of ['queued', 'pending', 'running']) {
    const run = { status, userId: 'owner' }
    assert.equal(mayCancelExecution(run, { userId: 'owner' }), true)
    assert.equal(mayCancelExecution(run, { userId: 'other' }), false)
    assert.equal(mayCancelExecution(run, { role: 'admin' }), true)
    assert.equal(mayCancelExecution({ status }, { userId: 'owner' }), false)
  }
  assert.equal(mayCancelExecution({ status: 'canceled', userId: 'owner' }, { userId: 'owner' }), false)
})
