import assert from 'node:assert/strict'
import { test } from 'node:test'
import { foldForSearch, matchesSearch } from '../src/lib/searchFold.ts'
import { macroLookup, valueLookup } from '../src/lib/lookups.ts'
import { matchesEpicSearch } from '../src/lib/roadmap.ts'

test('case and accents fold away', () => {
  for (const text of ['ÉQUIPE', 'Équipe', 'équipe', 'Equipe', 'equipe']) {
    assert.equal(foldForSearch(text), 'equipe')
  }
  assert.equal(foldForSearch('Hélène Noël'), 'helene noel')
})

test('nothing folds to the empty string', () => {
  assert.equal(foldForSearch(''), '')
  assert.equal(foldForSearch(null), '')
  assert.equal(foldForSearch(undefined), '')
})

test('a blank query matches everything', () => {
  assert.equal(matchesSearch('', 'anything'), true)
  assert.equal(matchesSearch('   '), true)
})

test('the query is found in any field, whatever its case and accents', () => {
  assert.equal(matchesSearch('helene', 'Hélène'), true)
  assert.equal(matchesSearch('  TERMINEE ', undefined, 'Clarification terminée'), true)
  assert.equal(matchesSearch('Équipe', 'equipe mobile'), true)
  assert.equal(matchesSearch('equipe', undefined, null, 'Réseau'), false)
})

test('the pickers fold both the values and the query', async () => {
  const people = await valueLookup(['Hélène', 'Bob'])('helene')
  assert.deepEqual(
    people.map(option => option.id),
    ['Hélène']
  )
  const macros = await macroLookup([{ key: 'M-1', title: "Écran d'accueil" }, { key: 'M-2', title: 'Other' }])('ECRAN')
  assert.deepEqual(
    macros.map(option => option.id),
    ['M-1']
  )
})

test('a roadmap row is found by an unaccented, upper-case term', () => {
  const row = { key: 'M-1', title: "Écran d'accueil", squad: 'Équipe Web', tasks: [{ key: 'T-1', title: 'Bouton' }] }
  assert.equal(matchesEpicSearch(row, 'ECRAN'), true)
  // Every term still has to match somewhere, in any order.
  assert.equal(matchesEpicSearch(row, 'equipe ecran'), true)
  assert.equal(matchesEpicSearch(row, 'ecran absent'), false)
})
