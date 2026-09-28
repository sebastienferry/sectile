import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EDITORS, editorChoice, editorLabel } from '../src/editors.mjs'

test('the picker offers None, the known editors and a custom command, in that order', () => {
  assert.deepEqual(EDITORS.map(editor => editor.label), ['None', 'VS Code', 'Cursor', 'Zed', 'Sublime Text', 'Custom command…'])
})

test('a stored command loads as its preset, as None, or as a custom command', () => {
  assert.deepEqual(editorChoice(''), { select: '', custom: '' })
  assert.deepEqual(editorChoice(undefined), { select: '', custom: '' })
  assert.deepEqual(editorChoice('   '), { select: '', custom: '' })
  for (const id of ['code', 'cursor', 'zed', 'subl']) assert.deepEqual(editorChoice(id), { select: id, custom: '' })
  assert.deepEqual(editorChoice(' zed '), { select: 'zed', custom: '' })
  // A preset with arguments is no longer the preset: it stays as written.
  assert.deepEqual(editorChoice('cursor -n'), { select: 'custom', custom: 'cursor -n' })
  // Preset matching is exact, like the command the agent runs.
  assert.deepEqual(editorChoice('Code'), { select: 'custom', custom: 'Code' })
  // The picker's own sentinel is not a command anyone stored.
  assert.deepEqual(editorChoice('custom'), { select: 'custom', custom: 'custom' })
})

test('the button names the preset, else the program a custom command starts', () => {
  assert.equal(editorLabel('code'), 'VS Code')
  assert.equal(editorLabel('cursor'), 'Cursor')
  assert.equal(editorLabel('subl'), 'Sublime Text')
  assert.equal(editorLabel('nvim-qt --maximized'), 'nvim-qt')
  assert.equal(editorLabel('/usr/local/bin/idea .'), 'idea')
  assert.equal(editorLabel('cursor -n'), 'cursor')
  assert.equal(editorLabel(''), '')
})
