const test = require('node:test')
const assert = require('node:assert')
const {APPEARANCES, normalizeAppearance, windowColors} = require('../electron/appearance.cjs')

test('a stored appearance is kept when it is one of the three values', () => {
 for (const value of APPEARANCES) assert.strictEqual(normalizeAppearance(value), value)
})

test('a missing or unknown appearance follows the system', () => {
 for (const value of [undefined, null, '', 'Light', 'auto', 1, {}]) {
  assert.strictEqual(normalizeAppearance(value), 'system', String(value))
 }
})

test('the dark window keeps the colours the app always had', () => {
 assert.deepStrictEqual(windowColors(true), {background: '#11151c', symbol: '#d8e0ec'})
})

test('the light window is painted light, with dark window controls', () => {
 const {background, symbol} = windowColors(false)
 assert.notStrictEqual(background, windowColors(true).background)
 assert.strictEqual(background, '#f5f6f8')
 assert.strictEqual(symbol, '#111315')
})
