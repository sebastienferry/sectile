import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'
import { format, plural } from '../src/lib/i18n.ts'

const fr = translations.fr.shell
const en = translations.en.shell

/** Every leaf of the catalog, as `path -> value`, arrays and plural forms included. */
function leaves(node, prefix = '') {
  if (typeof node === 'string') return [[prefix, node]]
  return Object.entries(node).flatMap(([key, value]) => leaves(value, prefix ? `${prefix}.${key}` : key))
}

test('the shell catalog has the same keys in French and English, none empty', () => {
  const frLeaves = leaves(fr)
  const enLeaves = leaves(en)
  assert.deepEqual(enLeaves.map(([path]) => path), frLeaves.map(([path]) => path))
  for (const [path, value] of [...frLeaves, ...enLeaves]) {
    assert.notEqual(value.trim(), '', `empty value at ${path}`)
  }
})

test('the project picker and navigation tooltips are English in English', () => {
  assert.equal(en.projectPicker.title, 'Project spaces')
  assert.equal(en.projectPicker.removeFavorite, 'Remove from favorites')
  assert.equal(en.projectPicker.configure, 'Configure this project')
  assert.equal(en.projectPicker.newProject, 'New project…')
  assert.equal(en.sidebar.skillsTooltip, 'Agentic workflow skills: one per stage, editable here')
  assert.equal(en.sidebar.teamTooltip, 'Team workload, person by person')
})

test('card actions, board controls and the fallback heading are English in English', () => {
  assert.equal(en.card.selectForBatch, 'Select for a batch (Ctrl/Cmd+click)')
  assert.equal(en.card.delete, 'Delete the task')
  assert.equal(en.card.openTracker, 'Open in the tracker')
  assert.equal(en.pinned.unpin, 'Unpin {key}')
  assert.equal(en.board.hideDone, 'Hide Done')
  assert.equal(en.board.unclassified, 'Unclassified')
  assert.equal(en.filters.unassigned, 'Unassigned')
  assert.equal(en.nextStep.new.stepTooltip, 'Advance one step: Clarify the requirements (clarify-issue)')
})

test('the French values keep the wording the interface already had', () => {
  assert.equal(fr.projectPicker.title, 'Espaces Projets')
  assert.equal(fr.projectPicker.removeFavorite, 'Retirer des favoris')
  assert.equal(fr.projectPicker.configure, 'Configurer ce projet')
  assert.equal(fr.projectPicker.newProject, 'Nouveau projet...')
  assert.equal(fr.sidebar.skillsTooltip, 'Skills du workflow agentique : une par étape, éditables ici')
  assert.equal(fr.card.delete, 'Supprimer la tâche')
  assert.equal(fr.card.deleteConfirm, 'Supprimer la tâche {key} ?')
  assert.equal(fr.card.pinTitle, 'Épingler pour basculer vite dessus')
  assert.equal(fr.list.pin, 'Épingler')
  assert.equal(fr.board.unclassified, 'Non classé')
  assert.equal(fr.board.hideDone, 'Masquer Terminé')
  assert.equal(fr.nextStep.new.stepTooltip, "Avancer d'un pas : Clarifier les exigences (clarify-issue)")
})

test('interpolated strings fill their placeholders in both languages', () => {
  assert.equal(format(fr.pinned.unpin, { key: '#12' }), 'Désépingler #12')
  assert.equal(format(en.pinned.unpin, { key: '#12' }), 'Unpin #12')
  assert.equal(format(fr.card.deleteConfirm, { key: 'PE-1' }), 'Supprimer la tâche PE-1 ?')
  assert.equal(format(en.card.deleteConfirm, { key: 'PE-1' }), 'Delete task PE-1?')
})

test('counts take the singular and plural forms of each language', () => {
  assert.equal(plural('fr', 0, fr.list.backlogCount), '0 tâche dans le backlog')
  assert.equal(plural('fr', 1, fr.list.backlogCount), '1 tâche dans le backlog')
  assert.equal(plural('fr', 3, fr.list.backlogCount), '3 tâches dans le backlog')
  assert.equal(plural('en', 0, en.list.backlogCount), '0 tasks in the backlog')
  assert.equal(plural('en', 1, en.list.backlogCount), '1 task in the backlog')
  assert.equal(plural('en', 3, en.list.backlogCount), '3 tasks in the backlog')

  assert.equal(plural('fr', 1, fr.board.selected), 'sélectionnée')
  assert.equal(plural('fr', 3, fr.board.selected), 'sélectionnées')
  assert.equal(plural('en', 3, en.board.selected), 'selected')

  assert.equal(plural('fr', 2, fr.board.hiddenColumns), '2 colonnes masquées')
  assert.equal(plural('en', 1, en.board.hiddenColumns), '1 hidden column')
  assert.equal(plural('fr', 1, fr.projectPicker.taskCount), '1 tâche')
  assert.equal(plural('en', 4, en.projectPicker.projectCount), '4 projects')
  assert.equal(plural('en', 2, en.list.removeLabel, { label: 'ui' }), 'Remove #ui from 2 tasks')
})
