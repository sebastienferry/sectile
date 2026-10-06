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

const detailKeys = ['project', 'tracker', 'trackerNotConfigured', 'trackerDefaultProject', 'issueType', 'issueTypeDefault', 'sprint', 'sprintPlaceholder']
const pick = labels => Object.fromEntries(detailKeys.map(key => [key, labels[key]]))

test('the quick-add details labels keep their French text and have an English one', () => {
  assert.deepEqual(
    { ...pick(translations.fr.quickAdd), sprintClear: translations.fr.taskDetail.lookups.sprintClear },
    {
      project: 'Projet *',
      tracker: 'Tracker :',
      trackerNotConfigured: 'projet non configuré',
      trackerDefaultProject: 'projet par défaut',
      issueType: 'Type de ticket',
      issueTypeDefault: 'Défaut',
      sprint: 'Sprint',
      sprintPlaceholder: 'Affecter un sprint (optionnel)…',
      sprintClear: 'Backlog (aucun sprint)',
    })
  assert.equal(translations.en.quickAdd.project, 'Project *')
  assert.equal(translations.en.quickAdd.sprintPlaceholder, 'Assign a sprint (optional)…')
})

test('the quick-add dialog no longer hard-codes French labels', () => {
  const frenchLabels = [
    'Projet *',
    'Tracker :',
    'non configuré',
    'par défaut',
    'Favoris',
    'Autres projets',
    'Type de ticket',
    'Défaut',
    'Tâche',
    'Affecter un sprint',
    'Backlog (aucun',
    'Aucun sprint',
    'Nouveau sprint',
  ]
  for (const french of frenchLabels) {
    assert.ok(!modal.includes(french), `QuickAddModal.tsx still hard-codes "${french}"`)
  }
  assert.match(modal, /t\.taskDetail\.fields\.favorites/)
  assert.match(modal, /t\.taskDetail\.lookups\.newSprint/)
})
