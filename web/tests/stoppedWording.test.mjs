import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'
import { shell } from '../src/locales/shell.ts'
import { runStateLabel, runStateOf } from '../../shared/runStates.ts'

test('canceled execution presentation uses stopped wording in each existing language', () => {
  assert.equal(runStateLabel(runStateOf({ status: 'canceled' })), 'Stopped')
  for (const [locale, count, tooltip, feedback] of [
    ['fr', 'Arrêtées', 'Exécution distante arrêtée', 'Exécution arrêtée avec succès'],
    ['en', 'Stopped', 'Remote execution stopped', 'Execution stopped successfully'],
  ]) {
    assert.equal(translations[locale].activities.stats.canceled, count)
    assert.equal(shell[locale].remoteRun.states.canceled, tooltip)
    assert.equal(translations[locale].toasts.activityCanceled, feedback)
  }
})
