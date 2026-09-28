import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'
import { translations } from '../src/locales/translations.ts'
import { format, plural } from '../src/lib/i18n.ts'

// The project settings (#528) follow the UI language: every tab, the provider
// descriptions and the board column editor come from `projectSettings`.
const fr = translations.fr.projectSettings
const en = translations.en.projectSettings
const TRACKERS = ['github', 'gitlab', 'jira']

test('the tabs are translated and French keeps its wording', () => {
  assert.deepEqual(fr.tabs, {
    general: 'Général',
    tracker: 'Tracker',
    workflow: 'Agentic workflow',
    skills: 'Compétences IA & SDD',
  })
  assert.deepEqual(en.tabs, {
    general: 'General',
    tracker: 'Tracker',
    workflow: 'Agentic workflow',
    skills: 'Skills & SDD',
  })
})

test('French labels keep the wording the settings already had', () => {
  assert.equal(format(fr.shell.settingsTitle, { name: 'Sectile' }), 'Paramètres : Sectile')
  assert.equal(fr.general.titleLabel, 'Titre du Projet *')
  assert.equal(fr.repositories.sectionTitle, 'Dépôt Git')
  assert.equal(fr.views.sectionTitle, "Vues de l'espace de travail")
  assert.equal(fr.views.epicColors, 'Couleur par épic')
  assert.equal(fr.feedback.delete, 'Supprimer')
  assert.equal(fr.feedback.update, 'Mettre à jour')
  assert.equal(fr.columns.freeStatuses, 'Statuts libres')
  assert.equal(fr.columns.moveUp, 'Monter')
  assert.equal(fr.columns.moveDown, 'Descendre')
  assert.equal(fr.columns.remove, 'Retirer')
})

test('English labels are English', () => {
  assert.equal(format(en.shell.settingsTitle, { name: 'Sectile' }), 'Settings: Sectile')
  assert.equal(en.general.titleLabel, 'Project title *')
  assert.equal(en.repositories.sectionTitle, 'Git repository')
  assert.equal(en.views.sectionTitle, 'Workspace views')
  assert.equal(en.views.epicColors, 'Color by epic')
  assert.equal(en.feedback.delete, 'Delete')
  assert.equal(en.feedback.update, 'Update')
  assert.equal(en.columns.freeStatuses, 'Free statuses')
  assert.equal(en.columns.moveUp, 'Move up')
  assert.equal(en.columns.moveDown, 'Move down')
  assert.equal(en.columns.remove, 'Remove')
})

test('tracker providers are described through their REST API, never a CLI', () => {
  for (const strings of [fr, en]) {
    for (const tracker of TRACKERS) {
      const description = strings.providers.descriptions[tracker]
      assert.doesNotMatch(description, /CLI/, `${tracker} still claims a CLI`)
      assert.match(description, /REST/, `${tracker} does not name its REST API`)
      assert.match(description, /#new, #clarified, #specified/, `${tracker} does not name the stage labels`)
    }
  }
  assert.match(fr.providers.descriptions.github, /API REST de GitHub/)
  assert.match(fr.providers.descriptions.github, /arrière-plan/)
  assert.match(en.providers.descriptions.github, /GitHub REST API/)
  assert.match(en.providers.descriptions.github, /background/)
  assert.notEqual(fr.providers.descriptions.jira, en.providers.descriptions.jira)
})

test('the spec frameworks keep naming their own CLI', () => {
  for (const strings of [fr, en]) {
    assert.match(strings.skills.speckitDescription, /^CLI \{cli\}/)
    assert.match(strings.skills.openspecDescription, /^CLI \{cli\}/)
  }
})

test('column counts use the plural rules of the language', () => {
  const forms = (strings) => strings.columns.toasts.columnsImportedDescription
  assert.equal(plural('fr', 1, forms(fr)), '1 colonne depuis le board distant.')
  assert.equal(plural('fr', 3, forms(fr)), '3 colonnes depuis le board distant.')
  assert.equal(plural('en', 1, forms(en)), '1 column from the remote board.')
  assert.equal(plural('en', 0, forms(en)), '0 columns from the remote board.')
})

test('the project modal and the column editor read their strings from the catalog', async () => {
  const modal = await readFile(new URL('../src/components/ProjectModal.tsx', import.meta.url), 'utf8')
  const editor = await readFile(new URL('../src/components/BoardColumnsEditor.tsx', import.meta.url), 'utf8')
  assert.doesNotMatch(modal, /via la CLI GitHub/)
  assert.doesNotMatch(modal, /'Mettre à jour'|'Supprimer'|Paramètres :/)
  assert.doesNotMatch(editor, />\s*Statuts libres|title="Monter"|title="Retirer"/)
})
