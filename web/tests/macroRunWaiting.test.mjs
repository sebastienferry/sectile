import assert from 'node:assert/strict'
import { test } from 'node:test'
import { macroRunLabel } from '../src/lib/macroRuns.ts'
import { planning } from '../src/locales/planning.ts'

const run = overrides => ({ id: 'r1', skillName: 'refine_macro', status: 'running', summary: '', createdAt: '', ...overrides })

// A macro run whose skill reported a wait says so, as a task card does (#648).
test('a macro run waiting on its user is labelled as waiting', () => {
  for (const locale of ['fr', 'en']) {
    const strings = planning[locale].macro.realign
    assert.equal(macroRunLabel(run({}), strings), strings.running)
    assert.equal(macroRunLabel(run({ waitingSince: '2026-10-01T08:00:00Z' }), strings), strings.waiting)
    assert.notEqual(strings.waiting, strings.running)
  }
})
