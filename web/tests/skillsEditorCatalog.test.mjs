import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'
import { commandPreview } from '../src/lib/commandTemplate.ts'
import { getCommandPresets } from '../src/lib/commandPresets.ts'

const fr = translations.fr.skillsEditor
const en = translations.en.skillsEditor

test('the execution modes keep their French wording and read naturally in English', () => {
  assert.equal(fr.modes.projectDefault, 'Défaut du projet')
  assert.equal(fr.modes.interactive, 'Interactif')
  assert.equal(fr.modes.autonomous, 'Autonome')
  assert.equal(fr.modes.projectDefaultHelp, 'La skill ne fixe rien : le défaut du projet décide')

  assert.equal(en.modes.projectDefault, 'Project default')
  assert.equal(en.modes.interactive, 'Interactive')
  assert.equal(en.modes.autonomous, 'Headless')
  assert.equal(en.modes.projectDefaultHelp, 'The skill sets nothing: the project default decides')
})

test('the divergence indicator and its tooltip exist in both languages', () => {
  assert.equal(fr.indicators.diverged, 'DIVERGENTE')
  assert.equal(fr.indicators.divergedTitle, 'Le fichier du dépôt diffère')
  assert.equal(en.indicators.diverged, 'DIVERGED')
  assert.equal(en.indicators.divergedTitle, 'The repository file differs')
})

test('the French editor chrome is unchanged', () => {
  assert.equal(fr.list.noProject, 'Sélectionne un projet : les skills sont éditées par projet, et régénérées dans le dépôt de ce projet.')
  assert.equal(fr.indicators.custom, 'PERSO')
  assert.equal(fr.indicators.notInstalled, 'NON INSTALLÉE')
  assert.equal(fr.editor.reset, 'Réinitialiser')
  assert.equal(fr.editor.updatedAt, 'modifiée le {date}')
  assert.equal(fr.list.macroRealignment, 'Réalignement Macro')
})

test('command preview errors follow the catalog, the command lines never do', () => {
  // English stays the default, so existing callers read what they read before.
  assert.match(commandPreview('gemini', '', '', true).error, /no attested headless mode/)
  const french = commandPreview('gemini', '', '', true, '', fr.feedback)
  assert.match(french.error, /^gemini n'a pas de mode headless attesté/)
  assert.match(french.error, /\{mode:AUTONOMOUS\|INTERACTIVE\}/)
  assert.equal(commandPreview('claude', '', '', false, '', fr.feedback).command, "claude '{prompt}'")
})

test('the empty command preset is named in the UI language', () => {
  assert.equal(getCommandPresets()[0].label, 'Défaut du fournisseur')
  assert.equal(getCommandPresets(en.modes.providerDefault)[0].label, 'Provider default')
})
