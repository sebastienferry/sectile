import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  summarizeSessions,
  sessionLabel,
  connectedFor,
  sortSessions,
} from '../src/lib/mcpSessions.ts'

const session = (id, overrides = {}) => ({
  id,
  client: 'sectile-stdio',
  connectedAt: '2026-09-16T10:00:00Z',
  runs: [],
  ...overrides,
})

test('summarizes clients and the runs they hold', () => {
  const sessions = [session('a', { runs: ['run-1', 'run-2'] }), session('b', { runs: ['run-3'] }), session('c')]
  assert.deepEqual(summarizeSessions(sessions), { clients: 3, runs: 3 })
  assert.deepEqual(summarizeSessions([]), { clients: 0, runs: 0 })
})

test('tolerates a session without a runs array', () => {
  assert.deepEqual(summarizeSessions([{ id: 'a', client: 'x', connectedAt: '', runs: undefined }]), { clients: 1, runs: 0 })
})

test('names a client by what it calls itself', () => {
  assert.equal(sessionLabel(session('a', { title: 'laptop/4821' })), 'sectile-stdio · laptop/4821')
  assert.equal(sessionLabel(session('a')), 'sectile-stdio')
  assert.equal(sessionLabel(session('a', { client: 'claude', title: 'claude' })), 'claude')
  assert.equal(sessionLabel({ id: 'a', client: '', connectedAt: '', runs: [] }), 'unknown client')
})

test('reads connection age in a compact, language-neutral form', () => {
  const now = new Date('2026-09-16T10:00:00Z').getTime()
  const at = seconds => new Date(now - seconds * 1000).toISOString()
  assert.equal(connectedFor(at(5), now), '5s')
  assert.equal(connectedFor(at(59), now), '59s')
  assert.equal(connectedFor(at(60), now), '1m')
  assert.equal(connectedFor(at(3599), now), '59m')
  assert.equal(connectedFor(at(3600), now), '1h')
  assert.equal(connectedFor(at(86_400), now), '1d')
})

test('reports nothing rather than NaN for an unusable timestamp', () => {
  assert.equal(connectedFor('', Date.now()), '')
  assert.equal(connectedFor('not-a-date', Date.now()), '')
})

test('a clock skewed into the future never reads as negative', () => {
  const now = new Date('2026-09-16T10:00:00Z').getTime()
  assert.equal(connectedFor(new Date(now + 60_000).toISOString(), now), '0s')
})

test('orders the most recent connection first', () => {
  const older = session('older', { connectedAt: '2026-09-16T09:00:00Z' })
  const newer = session('newer', { connectedAt: '2026-09-16T09:30:00Z' })
  assert.deepEqual(sortSessions([older, newer]).map(item => item.id), ['newer', 'older'])
})
