import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'

// Every leaf of the catalog, as [dotted path, value].
function leaves(node, path = '') {
  if (typeof node === 'string') return [[path, node]]
  if (node === null || typeof node !== 'object') return [[path, node]]
  return Object.entries(node).flatMap(([key, value]) => leaves(value, path ? `${path}.${key}` : key))
}

const placeholders = (text) => [...text.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort()

const fr = new Map(leaves(translations.fr))
const en = new Map(leaves(translations.en))

test('French and English hold the same keys', () => {
  const missingInEnglish = [...fr.keys()].filter((key) => !en.has(key))
  const missingInFrench = [...en.keys()].filter((key) => !fr.has(key))
  assert.deepEqual(missingInEnglish, [], 'keys missing in English')
  assert.deepEqual(missingInFrench, [], 'keys missing in French')
})

test('every catalog value is a non-empty string', () => {
  for (const [language, entries] of [['fr', fr], ['en', en]]) {
    const invalid = [...entries].filter(([, value]) => typeof value !== 'string' || value.trim() === '').map(([key]) => key)
    assert.deepEqual(invalid, [], `empty or non-string values in ${language}`)
  }
})

test('both languages interpolate the same placeholders', () => {
  const mismatched = [...fr]
    .filter(([key, value]) => en.has(key) && placeholders(value).join() !== placeholders(en.get(key)).join())
    .map(([key, value]) => `${key}: fr {${placeholders(value)}} en {${placeholders(en.get(key))}}`)
  assert.deepEqual(mismatched, [])
})

test('the document title exists in both languages and differs', () => {
  assert.equal(translations.fr.app.documentTitle, 'Sectile - Gestionnaire de tâches agentique')
  assert.equal(translations.en.app.documentTitle, 'Sectile - Agentic Task Workflow Manager')
})
