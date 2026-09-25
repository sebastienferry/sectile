import test from 'node:test'
import assert from 'node:assert/strict'
import { fill, relativeTime, windowMinutes } from '../src/lib/adminStats.ts'

test('fill replaces the named placeholders and leaves unknown ones', () => {
  assert.equal(fill('Seen within {minutes} min', { minutes: 5 }), 'Seen within 5 min')
  assert.equal(fill('{a} and {b}', { a: 1 }), '1 and {b}')
})

test('windowMinutes rounds the window and never answers zero', () => {
  assert.equal(windowMinutes({ activeWindowSeconds: 300 }), 5)
  assert.equal(windowMinutes({ activeWindowSeconds: 10 }), 1)
  assert.equal(windowMinutes(null), 1)
})

test('relativeTime reads an instant against now, in the given language', () => {
  const now = Date.parse('2026-09-25T12:00:00Z')
  assert.equal(relativeTime('2026-09-25T11:57:00Z', now, 'en'), '3 minutes ago')
  assert.equal(relativeTime('2026-09-25T10:00:00Z', now, 'en'), '2 hours ago')
  assert.equal(relativeTime('2026-09-23T12:00:00Z', now, 'en'), '2 days ago')
  assert.match(relativeTime('2026-09-25T11:57:00Z', now, 'fr'), /il y a 3 minutes/)
})

test('relativeTime answers null for a missing or unreadable instant', () => {
  assert.equal(relativeTime(undefined, Date.now(), 'en'), null)
  assert.equal(relativeTime('not a date', Date.now(), 'en'), null)
})
