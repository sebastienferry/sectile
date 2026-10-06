import assert from 'node:assert/strict'
import { test } from 'node:test'
import { paletteMatches } from '../src/command-palette.mjs'
import { newTaskShortcutAction, newTaskShortcutLabel } from '../src/new-task-shortcut.mjs'

const entries = [
  { group: 'execution', label: '#693 · Fix the PG test', detail: 'Sectile · running' },
  { group: 'action', label: 'New task', detail: 'Create a task in a project' },
  { group: 'project', label: 'Sectile', detail: 'Open its tasks' },
  { group: 'action', label: 'Toggle sidebar' },
  { group: 'project', label: 'PE-PORTAL', detail: 'Open its tasks' },
]
const labels = list => list.map(entry => entry.label)

test('an empty query lists actions, then projects, then executions', () => {
  assert.deepEqual(labels(paletteMatches(entries, '')), ['New task', 'Toggle sidebar', 'Sectile', 'PE-PORTAL', '#693 · Fix the PG test'])
})

test('every word must match, in the label or the detail', () => {
  assert.deepEqual(labels(paletteMatches(entries, 'sectile')), ['Sectile', '#693 · Fix the PG test'])
  assert.deepEqual(labels(paletteMatches(entries, 'sect run')), ['#693 · Fix the PG test'])
  assert.deepEqual(labels(paletteMatches(entries, 'nothing like it')), [])
})

test('within a group, a label starting with the query ranks first', () => {
  const tasks = [{ group: 'action', label: 'Open the web interface' }, { group: 'action', label: 'Tasks list' }, { group: 'action', label: 'New task' }]
  assert.deepEqual(labels(paletteMatches(tasks, 'ta')), ['Tasks list', 'New task'])
  assert.deepEqual(labels(paletteMatches(entries, '', 2)), ['New task', 'Toggle sidebar'])
})

test('Cmd+N on macOS and Ctrl+N elsewhere open the new task dialog', () => {
  const key = (extra = {}) => ({ key: 'n', metaKey: false, ctrlKey: false, shiftKey: false, altKey: false, repeat: false, defaultPrevented: false, modalOpen: false, inTerminal: false, ...extra })
  assert.equal(newTaskShortcutAction(key({ mac: true, metaKey: true })), 'open')
  assert.equal(newTaskShortcutAction(key({ mac: true, metaKey: true, inTerminal: true })), 'open')
  assert.equal(newTaskShortcutAction(key({ mac: false, ctrlKey: true })), 'open')
  assert.equal(newTaskShortcutAction(key({ mac: false, ctrlKey: true, inTerminal: true })), 'ignore')
  assert.equal(newTaskShortcutAction(key({ mac: true, ctrlKey: true })), 'ignore')
  assert.equal(newTaskShortcutAction(key({ mac: true, metaKey: true, shiftKey: true })), 'ignore')
  assert.equal(newTaskShortcutAction(key({ mac: true, metaKey: true, modalOpen: true })), 'ignore')
  assert.equal(newTaskShortcutAction(key({ mac: true, metaKey: true, key: 'N' })), 'open')
  assert.equal(newTaskShortcutLabel(true), '⌘N')
  assert.equal(newTaskShortcutLabel(false), 'Ctrl+N')
})

test('Cmd+Enter on macOS and Ctrl+Enter elsewhere are the default action; Shift+Enter is not', async () => {
  const { defaultActionShortcut, defaultActionLabel } = await import('../src/dialog-default.mjs')
  const key = (extra = {}) => ({ key: 'Enter', metaKey: false, ctrlKey: false, shiftKey: false, altKey: false, repeat: false, isComposing: false, ...extra })
  assert.equal(defaultActionShortcut(key({ mac: true, metaKey: true })), true)
  assert.equal(defaultActionShortcut(key({ mac: false, ctrlKey: true })), true)
  assert.equal(defaultActionShortcut(key({ mac: true, ctrlKey: true })), false)
  assert.equal(defaultActionShortcut(key({ mac: true, shiftKey: true })), false)
  assert.equal(defaultActionShortcut(key({ mac: true, metaKey: true, shiftKey: true })), false)
  assert.equal(defaultActionShortcut(key({ mac: true, metaKey: true, isComposing: true })), false)
  assert.equal(defaultActionShortcut(key({ mac: true })), false)
  assert.equal(defaultActionLabel(true), '⌘↵')
  assert.equal(defaultActionLabel(false), 'Ctrl+Enter')
})
