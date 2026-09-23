import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ACCENT_COLORS } from '../src/lib/accents.ts'
import { epicBadgeStyle, epicColor, epicColorHex, epicColorsEnabled, fnv1a } from '../src/lib/epicColor.ts'

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

test('the epic key badge is tinted with the epic colour', () => {
  assert.equal(epicBadgeStyle(undefined), null)
  assert.equal(epicBadgeStyle('  '), null)
  const def = epicColor('#12')
  assert.deepEqual(epicBadgeStyle(' #12 '), {
    color: def.hex,
    backgroundColor: `rgb(${def.rgb} / 0.15)`,
    borderColor: `rgb(${def.rgb} / 0.35)`,
  })
})

test('epic colours are off unless the task project asks for them', () => {
  const projects = [
    { id: 'on', epicColors: true },
    { id: 'off', epicColors: false },
    { id: 'unset' },
  ]
  assert.equal(epicColorsEnabled(projects, 'on'), true)
  assert.equal(epicColorsEnabled(projects, 'off'), false)
  assert.equal(epicColorsEnabled(projects, 'unset'), false)
  assert.equal(epicColorsEnabled(projects, 'unknown'), false)
  assert.equal(epicColorsEnabled([], undefined), false)
})

test('a task without a project reads the setting of the project on screen', () => {
  const projects = [{ id: 'on', epicColors: true }]
  assert.equal(epicColorsEnabled(projects, undefined, projects[0]), true)
  assert.equal(epicColorsEnabled(projects, '', { id: 'off', epicColors: false }), false)
  assert.equal(epicColorsEnabled(projects, undefined, null), false)
  // A task that names its project is read against it, not against the fallback.
  assert.equal(epicColorsEnabled(projects, 'other', projects[0]), false)
})
