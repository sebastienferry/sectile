import assert from 'node:assert/strict'
import { test } from 'node:test'
import { consoleNotice, headlessBanner, needsConsoleNotice, showsHeadlessOutput } from '../src/run-console.mjs'

// A headless run still gets no PTY: the pane must never attach to one.
test('a run with no console of its own is never attached to', () => {
  assert.equal(needsConsoleNotice({ status: 'queued', sessionId: '' }), true)
  assert.equal(needsConsoleNotice({ status: 'preparing', sessionId: '' }), true)
  assert.equal(needsConsoleNotice({ status: 'running', headless: true, sessionId: '' }), true)
  assert.equal(needsConsoleNotice({ status: 'failed', sessionId: '' }), true)
  assert.equal(needsConsoleNotice({ status: 'running', sessionId: 'run-1' }), false)
})

// The branches this change did not touch must keep saying what they said.
test('a run without a console keeps its own wording', () => {
  assert.match(consoleNotice({ status: 'queued' }), /Waiting for a console/)
  assert.match(consoleNotice({ status: 'preparing' }), /Waiting for a console/)
  assert.match(consoleNotice({ status: 'canceled' }), /canceled before a console was created/)
  assert.match(consoleNotice({ status: 'failed' }), /task activity and local agent\.log/)
})

test('the headless notice states that the pane takes no input', () => {
  const notice = consoleNotice({ status: 'running', headless: true })
  assert.equal(notice, headlessBanner())
  assert.match(notice, /read-only/)
  assert.match(notice, /no terminal to answer/)
  assert.match(headlessBanner(true), /Nothing printed yet/)
})

// A queued run has printed nothing yet, autonomous or not: it waits like any
// other, and the transcript poll would only report an empty run.
test('only a headless run past the queue reads its transcript', () => {
  assert.equal(showsHeadlessOutput({ status: 'running', headless: true }), true)
  assert.equal(showsHeadlessOutput({ status: 'completed', headless: true }), true)
  assert.equal(showsHeadlessOutput({ status: 'queued', headless: true }), false)
  assert.equal(showsHeadlessOutput({ status: 'preparing', headless: true }), false)
  assert.equal(showsHeadlessOutput({ status: 'running', sessionId: '' }), false)
})
