import assert from 'node:assert/strict'
import { test } from 'node:test'
import { describeExpiry, expiryState, expiryStatus, EXPIRY_WARNING_DAYS } from '../src/lib/apiKeys.ts'
import { translations } from '../src/locales/translations.ts'

const now = new Date('2026-09-17T12:00:00Z')
const days = n => new Date(now.getTime() + n * 24 * 60 * 60 * 1000).toISOString()
const en = translations.en.signIn.apiKeys

test('a key without expiry says so and is never flagged', () => {
  assert.equal(expiryState(null, now), 'none')
  assert.deepEqual(expiryStatus(null, now), { state: 'none' })
  assert.equal(describeExpiry(null, en, 'en', now), 'No expiry')
  assert.equal(expiryState('not a date', now), 'none')
})

test('a key inside the warning window is flagged with the days left', () => {
  assert.equal(expiryState(days(EXPIRY_WARNING_DAYS + 5), now), 'valid')
  assert.equal(expiryState(days(3), now), 'soon')
  const soon = expiryStatus(days(3), now)
  assert.equal(soon.state, 'soon')
  assert.equal(soon.days, 3)
  assert.match(describeExpiry(days(3), en, 'en', now), /Expires in 3 days, renew it/)
  assert.match(describeExpiry(days(0.5), en, 'en', now), /Expires in 1 day,/)
})

test('a valid key names its date and the days left', () => {
  const status = expiryStatus(days(30), now)
  assert.equal(status.state, 'valid')
  assert.equal(status.days, 30)
  assert.match(describeExpiry(days(30), en, 'en', now), /^Expires .+ \(30 days\)$/)
})

test('an expired key is reported as expired, not as soon', () => {
  assert.equal(expiryState(days(-1), now), 'expired')
  assert.equal(expiryStatus(days(-1), now).state, 'expired')
  assert.match(describeExpiry(days(-1), en, 'en', now), /^Expired /)
})
