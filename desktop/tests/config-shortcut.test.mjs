import { test } from 'node:test'
import assert from 'node:assert/strict'
import { configShortcutAction, configShortcutAria, configShortcutLabel } from '../src/config-shortcut.mjs'

const event = (overrides = {}) => ({ key: ',', metaKey: true, ctrlKey: false, shiftKey: false, altKey: false, repeat: false, defaultPrevented: false, mac: true, modalOpen: false, configurationOpen: false, ...overrides })

test('configuration shortcut opens once and consumes an open page without rebuilding it', () => {
 assert.equal(configShortcutAction(event()), 'open')
 assert.equal(configShortcutAction(event({ configurationOpen: true })), 'consume')
 assert.equal(configShortcutAction(event({ repeat: true })), 'ignore')
 assert.equal(configShortcutAction(event({ modalOpen: true })), 'ignore')
 assert.equal(configShortcutAction(event({ defaultPrevented: true })), 'ignore')
})

test('configuration shortcut requires the exact platform chord', () => {
 for (const overrides of [{ key: 'b' }, { ctrlKey: true }, { metaKey: false }, { shiftKey: true }, { altKey: true }]) {
  assert.equal(configShortcutAction(event(overrides)), 'ignore')
 }
 assert.equal(configShortcutAction(event({ mac: false, metaKey: false, ctrlKey: true })), 'open')
 assert.equal(configShortcutAction(event({ mac: false, metaKey: false, ctrlKey: true, configurationOpen: true })), 'consume')
 assert.equal(configShortcutAction(event({ mac: false, metaKey: true, ctrlKey: false })), 'ignore')
})

test('configuration control describes the platform chord', () => {
 assert.equal(configShortcutLabel(true), '⌘,')
 assert.equal(configShortcutAria(true), 'Meta+Comma')
 assert.equal(configShortcutLabel(false), 'Ctrl+,')
 assert.equal(configShortcutAria(false), 'Control+Comma')
})

test('configuration shortcut recognizes comma across keyboard layouts', () => {
 assert.equal(configShortcutAction(event({ key: ';', code: 'Comma' })), 'ignore')
 assert.equal(configShortcutAction(event({ key: ',', code: 'KeyM' })), 'open')
 assert.equal(configShortcutAction(event({ key: ';', code: 'KeyM' })), 'ignore')
 for (const overrides of [{ shiftKey: true }, { altKey: true }, { modalOpen: true }, { repeat: true }]) {
  assert.equal(configShortcutAction(event({ key: ';', code: 'Comma', ...overrides })), 'ignore')
 }
})
