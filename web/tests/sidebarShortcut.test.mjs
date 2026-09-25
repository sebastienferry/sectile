import {test} from 'node:test'
import assert from 'node:assert/strict'
import {isMacPlatform, sidebarShortcutAction, sidebarShortcutLabel, sidebarShortcutAria} from '../../shared/sidebarShortcut.mjs'

const press = (over = {}) => ({
  key: 'b', metaKey: false, ctrlKey: false, shiftKey: false, altKey: false, repeat: false,
  defaultPrevented: false, mac: false, inTerminal: false, modalOpen: false, ...over,
})

test('Cmd+B toggles on macOS, Ctrl+B elsewhere', () => {
  assert.equal(sidebarShortcutAction(press({mac: true, metaKey: true})), 'toggle')
  assert.equal(sidebarShortcutAction(press({ctrlKey: true})), 'toggle')
  assert.equal(sidebarShortcutAction(press({mac: true, metaKey: true, key: 'B'})), 'toggle')
})

test('the other platform\'s modifier is left alone', () => {
  assert.equal(sidebarShortcutAction(press({mac: true, ctrlKey: true})), 'ignore')
  assert.equal(sidebarShortcutAction(press({metaKey: true})), 'ignore')
  assert.equal(sidebarShortcutAction(press({mac: true, metaKey: true, ctrlKey: true})), 'ignore')
  assert.equal(sidebarShortcutAction(press({metaKey: true, ctrlKey: true})), 'ignore')
})

test('only the exact chord counts', () => {
  for (const mac of [true, false]) {
    const chord = mac ? {mac, metaKey: true} : {mac, ctrlKey: true}
    assert.equal(sidebarShortcutAction(press({...chord, shiftKey: true})), 'ignore')
    assert.equal(sidebarShortcutAction(press({...chord, altKey: true})), 'ignore')
    assert.equal(sidebarShortcutAction(press({...chord, repeat: true})), 'ignore')
    assert.equal(sidebarShortcutAction(press({...chord, key: 'k'})), 'ignore')
    assert.equal(sidebarShortcutAction(press({mac, key: 'b'})), 'ignore', 'bare B belongs to the board view')
  }
})

test('a consumed key or an open modal is ignored', () => {
  for (const mac of [true, false]) {
    const chord = mac ? {mac, metaKey: true} : {mac, ctrlKey: true}
    assert.equal(sidebarShortcutAction(press({...chord, defaultPrevented: true})), 'ignore', 'the Markdown editor keeps bold')
    assert.equal(sidebarShortcutAction(press({...chord, modalOpen: true})), 'ignore')
  }
})

test('a focused terminal keeps Ctrl+B but not Cmd+B', () => {
  assert.equal(sidebarShortcutAction(press({ctrlKey: true, inTerminal: true})), 'ignore')
  assert.equal(sidebarShortcutAction(press({mac: true, metaKey: true, inTerminal: true})), 'toggle')
  assert.equal(sidebarShortcutAction(press({mac: true, ctrlKey: true, inTerminal: true})), 'ignore')
})

test('isMacPlatform reads userAgentData first, then the legacy platform', () => {
  assert.equal(isMacPlatform({userAgentData: {platform: 'macOS'}, platform: 'Win32'}), true)
  assert.equal(isMacPlatform({userAgentData: {platform: 'Windows'}, platform: 'MacIntel'}), false)
  assert.equal(isMacPlatform({platform: 'MacIntel'}), true)
  assert.equal(isMacPlatform({platform: 'Linux x86_64'}), false)
  assert.equal(isMacPlatform({}), false)
  assert.equal(isMacPlatform(undefined), false)
})

test('labels name the platform\'s chord', () => {
  assert.equal(sidebarShortcutLabel(true), '⌘B')
  assert.equal(sidebarShortcutLabel(false), 'Ctrl+B')
  assert.equal(sidebarShortcutAria(true), 'Meta+B')
  assert.equal(sidebarShortcutAria(false), 'Control+B')
})
