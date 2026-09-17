import assert from 'node:assert/strict'
import { test } from 'node:test'

const { transitions, notification, stateOf, nameOf } =
  await import('../src/notifications.mjs')

const run = (overrides = {}) => ({ id: 'r1', status: 'running', taskKey: '#174', skill: 'implement', ...overrides })

test('a run that starts waiting is announced once', () => {
  const before = [run()]
  const after = [run({ waitingSince: '2026-09-17T10:00:00Z' })]
  const raised = transitions(before, after)
  assert.equal(raised.length, 1)
  assert.equal(raised[0].state, 'waiting')

  // Polling again with the same state must not raise it a second time.
  assert.deepEqual(transitions(after, after), [])
})

test('a run that reaches a terminal status is announced once', () => {
  assert.deepEqual(transitions([run()], [run({ status: 'completed' })]).map(t => t.state), ['completed'])
  assert.deepEqual(transitions([run({ status: 'completed' })], [run({ status: 'completed' })]), [])
})

test('resuming is not announced', () => {
  const waiting = [run({ waitingSince: '2026-09-17T10:00:00Z' })]
  // Back to running is a state change, but nobody needs a banner for it.
  assert.deepEqual(transitions(waiting, [run()]), [])
})

test('a run seen for the first time is not announced', () => {
  // Otherwise opening the desktop replays every session at once.
  assert.deepEqual(transitions([], [run({ waitingSince: '2026-09-17T10:00:00Z' })]), [])
  assert.deepEqual(transitions([], [run({ status: 'completed' })]), [])
})

test('the state vocabulary is the shared one', () => {
  assert.equal(stateOf(run({ waitingSince: 'x' })), 'waiting')
  assert.equal(stateOf(run({ status: 'failed' })), 'failed')
  assert.equal(stateOf(run({ status: 'queued' })), 'queued')
  // The desktop reads the shared mapping now, so the statuses it never saw are
  // answered too: 'pending' is the activity spelling of 'queued'.
  assert.equal(stateOf(run({ status: 'preparing' })), 'queued')
  assert.equal(stateOf(run({ status: 'pending' })), 'queued')
  assert.equal(stateOf(run({ status: 'completed' })), 'completed')
  assert.equal(stateOf(run({ status: 'canceled' })), 'canceled')
  // An outcome is final: a start of wait left on an ended run changes nothing.
  assert.equal(stateOf(run({ status: 'completed', waitingSince: 'x' })), 'completed')
  assert.equal(stateOf(run({ status: 'running' })), 'running')
})

test('a session is named by its task, or by its directory', () => {
  assert.equal(nameOf(run()), '#174 · implement')
  assert.equal(nameOf({ directory: '/Users/x/w/#174' }), '#174')
  assert.equal(nameOf({}), 'Session')
})

test('a notification carries the shared glyph and no unknown state', () => {
  const payload = notification({ state: 'waiting', name: '#174' })
  assert.match(payload.icon, /^data:image\/svg\+xml;base64,/)
  assert.match(payload.body, /waiting for you/)
  assert.equal(notification({ state: 'nonsense', name: 'x' }), null)
})

const { announce, resetNotificationAvailability } = await import('../src/notifications.mjs')

function recorder(permission = 'granted') {
  const raised = []
  class Fake {
    static permission = permission
    static requestPermission() {}
    constructor(title, options) { raised.push({ title, ...options }) }
  }
  return { Fake, raised }
}

test('each transition raises one native notification carrying its glyph', () => {
  resetNotificationAvailability()
  const { Fake, raised } = recorder()
  const count = announce([{ id: 'r1', state: 'waiting', name: '#174' }], Fake)
  assert.equal(count, 1)
  assert.equal(raised.length, 1)
  assert.match(raised[0].icon, /^data:image\/svg\+xml;base64,/)
  assert.equal(raised[0].tag, 'r1')
})

test('a workstation that denies notifications raises nothing and throws nothing', () => {
  resetNotificationAvailability()
  const { Fake, raised } = recorder('denied')
  assert.equal(announce([{ id: 'r1', state: 'waiting', name: '#174' }], Fake), 0)
  assert.equal(raised.length, 0)
})

test('no notification channel at all is not an error', () => {
  resetNotificationAvailability()
  assert.equal(announce([{ id: 'r1', state: 'waiting', name: 'x' }], undefined), 0)
})

test('a notifier that throws disables the channel instead of failing the poll', () => {
  resetNotificationAvailability()
  class Broken {
    static permission = 'granted'
    constructor() { throw Error('no notification service') }
  }
  assert.doesNotThrow(() => announce([{ id: 'r1', state: 'waiting', name: 'x' }], Broken))
})
