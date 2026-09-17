import assert from 'node:assert/strict'
import { test } from 'node:test'
import { describeExpiry, expiryState, EXPIRY_WARNING_DAYS } from '../src/lib/apiKeys.ts'

const now = new Date('2026-09-17T12:00:00Z')
const days = n => new Date(now.getTime() + n * 24 * 60 * 60 * 1000).toISOString()

test('a key without expiry says so and is never flagged', () => {
  assert.equal(expiryState(null, now), 'none')
  assert.equal(describeExpiry(null, now), 'No expiry')
  assert.equal(expiryState('not a date', now), 'none')
})

test('a key inside the warning window is flagged with the days left', () => {
  assert.equal(expiryState(days(EXPIRY_WARNING_DAYS + 5), now), 'valid')
  assert.equal(expiryState(days(3), now), 'soon')
  assert.match(describeExpiry(days(3), now), /Expires in 3 days, renew it/)
  assert.match(describeExpiry(days(0.5), now), /Expires in 1 day/)
})

test('an expired key is reported as expired, not as soon', () => {
  assert.equal(expiryState(days(-1), now), 'expired')
  assert.match(describeExpiry(days(-1), now), /^Expired /)
})
