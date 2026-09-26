import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  TOAST_DURATION_MS,
  TOAST_WITH_LINK_DURATION_MS,
  createDismissTimer,
  toastDuration,
} from '../src/lib/toastTimer.ts'

// A clock the test moves by hand: one pending timeout at most, which is all a
// dismiss timer ever holds.
function manualClock() {
  let now = 0
  let pending = null
  return {
    now: () => now,
    setTimeout(callback, ms) {
      pending = { callback, at: now + ms }
      return pending
    },
    clearTimeout(handle) {
      if (pending === handle) pending = null
    },
    advance(ms) {
      now += ms
      if (pending && pending.at <= now) {
        const { callback } = pending
        pending = null
        callback()
      }
    },
  }
}

test('a toast without a link keeps the short default', () => {
  assert.equal(toastDuration({}), TOAST_DURATION_MS)
})

test('a toast with a link stays longer', () => {
  assert.equal(toastDuration({ link: { label: 'Ouvrir #1' } }), TOAST_WITH_LINK_DURATION_MS)
})

test('an explicit duration wins over both defaults', () => {
  assert.equal(toastDuration({ duration: 1200 }), 1200)
  assert.equal(toastDuration({ duration: 1200, link: {} }), 1200)
})

test('the toast is dismissed once its duration has run', () => {
  const clock = manualClock()
  let dismissed = 0
  createDismissTimer(1000, () => dismissed++, clock)
  clock.advance(999)
  assert.equal(dismissed, 0)
  clock.advance(1)
  assert.equal(dismissed, 1)
})

test('time spent under the pointer does not count', () => {
  const clock = manualClock()
  let dismissed = 0
  const timer = createDismissTimer(1000, () => dismissed++, clock)
  clock.advance(600)
  timer.pause('hover')
  clock.advance(10_000)
  assert.equal(dismissed, 0)
  timer.resume('hover')
  clock.advance(399)
  assert.equal(dismissed, 0, 'only the 400 ms left before the pause should remain')
  clock.advance(1)
  assert.equal(dismissed, 1)
})

test('leaving with the pointer does not resume while the focus is still inside', () => {
  const clock = manualClock()
  let dismissed = 0
  const timer = createDismissTimer(1000, () => dismissed++, clock)
  timer.pause('hover')
  timer.pause('focus')
  timer.resume('hover')
  clock.advance(5000)
  assert.equal(dismissed, 0)
  timer.resume('focus')
  clock.advance(1000)
  assert.equal(dismissed, 1)
})

test('pausing twice for the same reason is released once', () => {
  const clock = manualClock()
  let dismissed = 0
  const timer = createDismissTimer(1000, () => dismissed++, clock)
  timer.pause('focus')
  timer.pause('focus')
  timer.resume('focus')
  clock.advance(1000)
  assert.equal(dismissed, 1)
})

test('a cancelled timer never dismisses, even when resumed', () => {
  const clock = manualClock()
  let dismissed = 0
  const timer = createDismissTimer(1000, () => dismissed++, clock)
  timer.cancel()
  timer.resume('hover')
  clock.advance(5000)
  assert.equal(dismissed, 0)
})
