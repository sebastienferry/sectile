import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { CLAUDE_MARK_PATH, claudeMark } from '../src/claude-mark.mjs'
import { CONVERSATION_MODE_CHOICES } from '../src/appearance.mjs'

// The desktop reuses the Claude mark the web app vendored; it never draws one of its own.
test('the Claude mark is the one the web app draws', () => {
 const web = readFileSync(new URL('../../web/src/components/icons/Claude.tsx', import.meta.url), 'utf8')
 assert.equal(web.match(/<path d="([^"]+)"/)[1], CLAUDE_MARK_PATH)
})

test('the Claude mark is decorative and follows the text colour', () => {
 const made = []
 const element = name => {
  const node = {name, attributes: {}, children: [], setAttribute(key, value) { this.attributes[key] = value }, append(...nodes) { this.children.push(...nodes) }}
  made.push(node);return node
 }
 const svg = claudeMark({createElementNS: (ns, name) => { assert.equal(ns, 'http://www.w3.org/2000/svg');return element(name) }})
 assert.equal(svg.name, 'svg')
 assert.equal(svg.attributes['aria-hidden'], 'true')
 assert.equal(svg.attributes.fill, 'currentColor')
 assert.equal(svg.attributes.viewBox, '0 0 24 24')
 assert.equal(svg.children.length, 1)
 assert.equal(svg.children[0].attributes.d, CLAUDE_MARK_PATH)
})

// The setting names the modes as the composer's Permission mode select does.
test('the conversation mode choices are the composer modes, in its order and words', () => {
 const composer = readFileSync(new URL('../src/conversation.js', import.meta.url), 'utf8')
 const select = composer.match(/<select aria-label="Permission mode">(.*?)<\/select>/)[1]
 const options = [...select.matchAll(/<option value="([^"]+)">([^<]+)<\/option>/g)].map(([, value, label]) => ({value, label}))
 assert.deepEqual(CONVERSATION_MODE_CHOICES, options)
})
