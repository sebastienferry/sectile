import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ACCENT_COLORS } from '../src/lib/accents.ts'
import { epicColor, epicColorHex, fnv1a } from '../src/lib/epicColor.ts'

test('a task without a parent gets no colour', () => {
  assert.equal(epicColor(undefined), null)
  assert.equal(epicColor(null), null)
  assert.equal(epicColor(''), null)
  assert.equal(epicColor('   '), null)
  assert.equal(epicColorHex(undefined), null)
})

test('the colour is a palette entry', () => {
  for (const key of ['#1', '#12', 'PROJ-7', 'ÉPIC-42', 'x'.repeat(500)]) {
    assert.ok(ACCENT_COLORS.includes(epicColor(key)), key)
  }
})

test('the colour depends on the trimmed key only', () => {
  assert.equal(epicColor(' #12 '), epicColor('#12'))
  assert.equal(epicColor('#12'), epicColor('#12'))
  assert.equal(epicColorHex('#12'), epicColor('#12').hex)
})

test('the hash is pinned, so a change does not silently repaint every board', () => {
  assert.equal(fnv1a(''), 0x811c9dc5)
  assert.equal(fnv1a('#12'), 4271395249)
  assert.equal(epicColor('#12').name, 'violet')
  assert.equal(epicColor('PROJ-1').name, 'indigo')
})

test('neighbouring keys spread over the palette', () => {
  const names = new Set()
  for (let i = 1; i <= 24; i++) names.add(epicColor(`#${i}`).name)
  assert.ok(names.size >= 6, `only ${names.size} colours for 24 epics`)
})
