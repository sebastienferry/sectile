import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createLatestRequest } from '../src/lib/latestRequest.ts'

test('the only request started is the latest', () => {
  const gate = createLatestRequest()
  const ticket = gate.begin()
  assert.equal(gate.isLatest(ticket), true)
})

test('a newer request makes the older answer stale', () => {
  const gate = createLatestRequest()
  const older = gate.begin()
  const newer = gate.begin()
  assert.equal(gate.isLatest(older), false)
  assert.equal(gate.isLatest(newer), true)
})

test('two gates do not share their tickets', () => {
  const tasks = createLatestRequest()
  const facets = createLatestRequest()
  const taskTicket = tasks.begin()
  facets.begin()
  facets.begin()
  assert.equal(tasks.isLatest(taskTicket), true)
})

test('a read that joins the newest request keeps it latest', () => {
  const gate = createLatestRequest()
  const load = gate.begin()
  const joined = gate.current()
  assert.equal(gate.isLatest(load), true)
  assert.equal(gate.isLatest(joined), true)
})

test('a joined read goes stale once a new request starts', () => {
  const gate = createLatestRequest()
  const joined = gate.current()
  gate.begin()
  assert.equal(gate.isLatest(joined), false)
})
