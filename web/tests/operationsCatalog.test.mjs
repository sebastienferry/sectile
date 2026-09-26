import assert from 'node:assert/strict'
import { test } from 'node:test'
import { format, plural } from '../src/lib/i18n.ts'
import { translations } from '../src/locales/translations.ts'

const fr = translations.fr.operations
const en = translations.en.operations

test('connection and tracker read notifications exist in both languages', () => {
  assert.equal(fr.connection.title, 'Erreur de connexion')
  assert.equal(en.connection.title, 'Connection error')
  assert.equal(fr.notifications.boards.statusesTitle, 'Statuts du tracker')
  assert.equal(en.notifications.boards.statusesTitle, 'Tracker statuses')
  assert.equal(fr.notifications.boards.title, 'Boards du tracker')
  assert.equal(en.notifications.boards.title, 'Tracker boards')
})

test('French notifications keep the wording the interface already had', () => {
  assert.equal(fr.sync.globalStarted, 'Synchronisation globale lancée')
  assert.equal(format(fr.sync.globalStartedProject, { name: 'Sectile' }), 'Projet Sectile - Suivi dans Activités.')
  assert.equal(format(fr.projects.createdDescription, { name: 'Sectile' }), 'Le projet Sectile a été créé avec succès.')
  assert.equal(
    format(fr.notifications.tasks.transitionQueuedDescription, { key: 'SFE-12', status: 'Done' }),
    'SFE-12 ➔ « Done ». Suivi dans les activités.',
  )
  assert.equal(format(fr.notifications.skillFailed, { skill: 'Clarify', task: '#526' }), 'Échec de Clarify (#526)')
  assert.equal(fr.activities.createdAt, 'Créée à')
  assert.equal(fr.activities.selectTitle, 'Sélectionnez une activité')
})

test('English notifications interpolate names and keys unchanged', () => {
  assert.equal(
    format(en.sync.githubStartedRepo, { repo: 'sebastienferry/sectile', project: 'Sectile' }),
    'Repository sebastienferry/sectile (Sectile) - Tracked in Activities.',
  )
  assert.equal(format(en.projects.updatedDescription, { name: 'Équipe Rouge' }), 'Project Équipe Rouge was updated.')
  assert.equal(format(en.notifications.tasks.stageAppliedDescription, { key: '#526', stage: 'clarified' }), 'Task #526 moved to stage clarified.')
  assert.equal(format(en.notifications.skillSucceeded, { skill: 'Clarify', task: '#526' }), 'Clarify (#526) completed successfully!')
})

test('a notification title and body come from the same language', () => {
  // Title and description pairs as AppContext composes them: neither English
  // pair may carry the French wording.
  const pairs = [
    [en.sync.githubStarted, en.sync.githubRunning],
    [en.projects.deleted, en.projects.deletedDescription],
    [en.notifications.macros.migrated, en.notifications.macros.migratedDescription],
    [en.notifications.tasks.transitionQueued, en.notifications.tasks.transitionQueuedDescription],
  ]
  for (const [title, body] of pairs) {
    for (const text of [title, body]) {
      assert.doesNotMatch(text, /[éèàùç]|Suivi|Projet|lancée/, text)
    }
  }
})

test('counts follow each language\'s plural rules', () => {
  assert.equal(plural('fr', 1, fr.sync.activeJobs), '1 job actif')
  assert.equal(plural('fr', 3, fr.sync.activeJobs), '3 jobs actifs')
  assert.equal(plural('en', 1, en.sync.activeJobs), '1 active job')
  assert.equal(plural('en', 0, en.sync.taskCount), '0 tasks')
  assert.equal(plural('fr', 0, fr.sync.taskCount), '0 tâche')
})
