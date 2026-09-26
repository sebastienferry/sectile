import assert from 'node:assert/strict'
import { test } from 'node:test'
import { planning } from '../src/locales/planning.ts'
import { translations } from '../src/locales/translations.ts'
import { format, plural } from '../src/lib/i18n.ts'

// The planning views (#529): roadmap, macros, triage, curation and team.

test('the planning namespace is the one the catalog exposes', () => {
  assert.equal(translations.fr.planning, planning.fr)
  assert.equal(translations.en.planning, planning.en)
})

test('the roadmap tabs follow the language, the horizons do not', () => {
  assert.deepEqual(planning.fr.roadmap.tabs, { unclassified: 'Non classés', hidden: 'Masqués' })
  assert.deepEqual(planning.en.roadmap.tabs, { unclassified: 'Unclassified', hidden: 'Hidden' })
  // NOW, NEXT and LATER are product vocabulary: the options name them in both languages.
  for (const strings of [planning.fr, planning.en]) {
    assert.match(strings.roadmap.createModal.horizonNow, /^NOW /)
    assert.match(strings.roadmap.createModal.horizonNext, /^NEXT /)
    assert.match(strings.roadmap.createModal.horizonLater, /^LATER /)
  }
  assert.equal(format(planning.en.roadmap.classifyAs, { horizon: 'NOW' }), 'Classify as NOW')
  assert.equal(format(planning.fr.roadmap.classifyAs, { horizon: 'NOW' }), 'Classer en NOW')
})

test('the roadmap feedback keeps its French wording', () => {
  assert.equal(planning.fr.roadmap.framingRequired, 'Cadrage requis')
  assert.equal(planning.en.roadmap.framingRequired, 'Framing required')
  assert.equal(planning.fr.roadmap.filters.unassigned, 'non assigné')
  assert.equal(planning.en.roadmap.filters.pinnedOnly, 'pinned only')
})

test('the triage empty state is translated', () => {
  assert.equal(planning.fr.triage.emptyTitle, 'Tout est trié !')
  assert.equal(planning.en.triage.emptyTitle, 'All sorted!')
  assert.equal(planning.fr.triage.searchPlaceholder, 'Filtrer clé, titre, membre…')
  assert.equal(planning.en.triage.missing.team, 'no team')
  assert.equal(planning.fr.triage.missing.assignee, 'sans assigné')
})

test('the selection count has a singular and a plural in both languages', () => {
  const fr = planning.fr.triage.selectedCount
  const en = planning.en.triage.selectedCount
  // French treats 0 and 1 as singular, English only 1.
  assert.equal(plural('fr', 0, fr), '0 sélectionné')
  assert.equal(plural('fr', 1, fr), '1 sélectionné')
  assert.equal(plural('fr', 3, fr), '3 sélectionnés')
  assert.equal(plural('en', 0, en), '0 selected')
  assert.equal(plural('en', 1, en), '1 selected')

  const people = planning.en.team.people
  assert.equal(
    plural('en', 1, planning.en.team.assignedSummary, { people: plural('en', 2, people) }),
    '1 ticket assigned to 2 people',
  )
  assert.equal(
    plural('fr', 4, planning.fr.team.assignedSummary, { people: plural('fr', 1, planning.fr.team.people) }),
    '4 tickets assignés à 1 personne',
  )
})

test('the team view names what Sectile invents, in the UI language', () => {
  assert.equal(planning.fr.team.unassigned, 'Non assigné')
  assert.equal(planning.en.team.unassigned, 'Unassigned')
  assert.equal(planning.fr.team.outsideBadge, 'hors équipe')
  assert.equal(planning.en.team.outsideBadge, 'outside the team')
})
