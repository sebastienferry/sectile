import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  EMPTY_VALUE,
  format,
  formatDate,
  formatDateTime,
  formatNumber,
  intlLocale,
  parseDateInput,
  plural,
  resolveInitialLocale,
} from '../src/lib/i18n.ts'

test('format replaces every occurrence and keeps unknown placeholders visible', () => {
  assert.equal(format('{key} moved to {project}, {key} kept', { key: '#526', project: 'Sectile' }), '#526 moved to Sectile, #526 kept')
  assert.equal(format('Hello {name}', {}), 'Hello {name}')
  assert.equal(format('{n} items', { n: 0 }), '0 items')
})

test('plural follows each language: French 0 and 1 singular, English only 1', () => {
  const fr = { one: '{count} tâche', other: '{count} tâches' }
  const en = { one: '{count} task', other: '{count} tasks' }
  assert.equal(plural('fr', 0, fr), '0 tâche')
  assert.equal(plural('fr', 1, fr), '1 tâche')
  assert.equal(plural('fr', 5, fr), '5 tâches')
  assert.equal(plural('en', 0, en), '0 tasks')
  assert.equal(plural('en', 1, en), '1 task')
  assert.equal(plural('en', 5, en), '5 tasks')
  assert.equal(plural('en', 2, { one: '{count} in {sprint}', other: '{count} in {sprint}' }, { sprint: 'S1' }), '2 in S1')
})

test('counts are grouped in the UI language', () => {
  assert.equal(plural('en', 1200, { one: '{count} task', other: '{count} tasks' }), '1,200 tasks')
  assert.match(formatNumber('fr', 1200), /^1\s1200?$|^1 200$|^1 200$/)
})

test('dates use the UI language, not the browser', () => {
  assert.equal(intlLocale('fr'), 'fr-FR')
  assert.equal(intlLocale('en'), 'en-US')
  assert.equal(formatDate('en', '2026-09-01'), 'Sep 1, 2026')
  assert.equal(formatDate('fr', '2026-09-01'), '1 sept. 2026')
})

test('missing and invalid dates render a neutral placeholder', () => {
  for (const value of [null, undefined, '', 'not a date', Number.NaN]) {
    assert.equal(formatDate('en', value), EMPTY_VALUE)
    assert.equal(formatDateTime('fr', value), EMPTY_VALUE)
  }
})

test('a date-only value is a calendar day in any time zone, an instant is converted', () => {
  const script = `
    import { formatDate, formatDateTime } from ${JSON.stringify(fileURLToPath(new URL('../src/lib/i18n.ts', import.meta.url)))}
    console.log(JSON.stringify([
      formatDate('en', '2026-09-01'),
      formatDateTime('en', '2026-09-01T02:00:00Z', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit', hour12: false }),
    ]))
  `
  const run = (tz) => JSON.parse(execFileSync(process.execPath, ['--input-type=module', '-e', script], { env: { ...process.env, TZ: tz } }).toString())
  const [losAngelesDay, losAngelesInstant] = run('America/Los_Angeles')
  const [tokyoDay, tokyoInstant] = run('Asia/Tokyo')
  assert.equal(losAngelesDay, 'Sep 1, 2026')
  assert.equal(tokyoDay, 'Sep 1, 2026')
  assert.equal(losAngelesInstant, 'Aug 31, 19:00')
  assert.equal(tokyoInstant, 'Sep 1, 11:00')
})

test('parseDateInput accepts dates, timestamps and ISO strings', () => {
  const date = new Date(2026, 8, 1)
  assert.equal(parseDateInput(date), date)
  assert.equal(parseDateInput(date.getTime())?.getTime(), date.getTime())
  assert.equal(parseDateInput('2026-09-01')?.getDate(), 1)
})

test('the signed-out language: remembered, else browser, French only for fr*', () => {
  assert.equal(resolveInitialLocale('en', ['fr-FR']), 'en')
  assert.equal(resolveInitialLocale('fr', ['en-US']), 'fr')
  assert.equal(resolveInitialLocale(null, ['fr-CA', 'en']), 'fr')
  assert.equal(resolveInitialLocale(null, ['de-DE', 'fr']), 'en')
  assert.equal(resolveInitialLocale('xx', ['FR']), 'fr')
  assert.equal(resolveInitialLocale(null, []), 'en')
})
