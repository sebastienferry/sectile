import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ENDED_REFRESH_MS, LIVE_REFRESH_MS, skillResultDue, skillResultStamp } from '../src/skill-result-refresh.mjs'

const run = (status, waitingSince = '') => ({ id: 'r', status, waitingSince })

test('a result never read is read', () => {
  assert.equal(skillResultDue(run('running'), undefined, 0), true)
})

test('a change of status or of wait is read at once', () => {
  const last = skillResultStamp(run('running'), 0)
  assert.equal(skillResultDue(run('running'), last, 1000), false)
  assert.equal(skillResultDue(run('completed'), last, 1000), true)
  assert.equal(skillResultDue(run('running', '2026-10-01T12:00:00Z'), last, 1000), true)
})

test('a live run is read again at the live pace, an ended one more slowly', () => {
  const live = skillResultStamp(run('running'), 0)
  assert.equal(skillResultDue(run('running'), live, LIVE_REFRESH_MS - 1), false)
  assert.equal(skillResultDue(run('running'), live, LIVE_REFRESH_MS), true)
  const ended = skillResultStamp(run('completed'), 0)
  assert.equal(skillResultDue(run('completed'), ended, LIVE_REFRESH_MS), false)
  assert.equal(skillResultDue(run('completed'), ended, ENDED_REFRESH_MS), true)
})
