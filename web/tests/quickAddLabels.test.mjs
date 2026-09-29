import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'

const modal = readFileSync(new URL('../src/components/QuickAddModal.tsx', import.meta.url), 'utf8')

test('the in-progress label of the quick-add button is translated', () => {
  assert.equal(translations.fr.quickAdd.creating, 'Création…')
  assert.equal(translations.en.quickAdd.creating, 'Creating…')
})

test('the quick-add dialog reads its in-progress label from the catalog', () => {
  assert.match(modal, /\{t\.quickAdd\.creating\}/)
  // No CLI runs when a ticket is created: the label must not claim one does.
  assert.doesNotMatch(modal, /CLI/)
})
