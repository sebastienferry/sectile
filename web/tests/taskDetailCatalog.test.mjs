import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'
import { format, plural } from '../src/lib/i18n.ts'
import { sprintKindLabel, sprintLookup } from '../src/lib/lookups.ts'

const fr = translations.fr.taskDetail
const en = translations.en.taskDetail

test('the task detail namespace is merged into both catalogs', () => {
  assert.ok(fr, 'French task detail strings are exposed as t.taskDetail')
  assert.ok(en, 'English task detail strings are exposed as t.taskDetail')
})

test('the missing type fallback is translated and French is unchanged', () => {
  assert.equal(fr.issueType.defaultOption, 'Type: Défaut')
  assert.equal(en.issueType.defaultOption, 'Type: Default')
})

test('the cross-tracker move confirmation interpolates both project names untouched', () => {
  const params = { target: 'Platform (Jira)', source: 'Sectile' }
  assert.equal(
    format(fr.move.confirm, params),
    'Attention: Le projet "Platform (Jira)" a un tracker différent de "Sectile". Déplacer ce ticket vers ce projet quand même ?',
  )
  const english = format(en.move.confirm, params)
  assert.match(english, /"Platform \(Jira\)"/)
  assert.match(english, /"Sectile"/)
  assert.doesNotMatch(english, /\{\w+\}/, 'every placeholder is filled')
})

test('the key copy feedback names the task key unchanged in both languages', () => {
  assert.equal(fr.toasts.keyCopiedTitle, 'Identifiant copié')
  assert.equal(format(fr.toasts.keyCopiedDescription, { key: 'PE-42' }), 'PE-42 a été copié dans le presse-papiers.')
  assert.equal(format(fr.header.copyKey, { key: '#431' }), 'Copier #431')
  assert.equal(en.toasts.keyCopiedTitle, 'Key copied')
  assert.equal(format(en.toasts.keyCopiedDescription, { key: 'PE-42' }), 'PE-42 was copied to the clipboard.')
  assert.equal(format(en.header.copyKey, { key: '#431' }), 'Copy #431')
})

test('the specification and rewrite views keep their French wording and read English', () => {
  assert.equal(format(fr.spec.title, { framework: 'Spec Kit' }), 'Spécification Technique (Spec Kit)')
  assert.equal(fr.spec.copyFull, 'Copier la spec complète')
  assert.equal(format(fr.workflow.recommendedStep, { skill: 'Clarify' }), 'Étape recommandée : Clarify')
  assert.equal(fr.rewrite.apply, 'Appliquer à la description')
  assert.equal(format(en.spec.title, { framework: 'Spec Kit' }), 'Technical specification (Spec Kit)')
  assert.equal(en.spec.copyFull, 'Copy the full spec')
  assert.equal(format(en.workflow.recommendedStep, { skill: 'Clarify' }), 'Recommended step: Clarify')
  assert.equal(en.rewrite.apply, 'Apply to the description')
})

test('the labels French browser tests assert are unchanged', () => {
  assert.equal(fr.fields.description, 'Description & Contexte Technique')
  assert.equal(fr.fields.creator, 'Créé par')
  assert.equal(fr.fields.team, 'Équipe')
  assert.equal(fr.fields.macro, 'Macro (Milestone)')
  assert.equal(fr.pr.detach, 'Détacher cette pull request du ticket')
  assert.equal(fr.pr.link, 'Lier')
  assert.equal(fr.rewrite.includeComments, 'Inclure les commentaires')
  assert.equal(fr.rewrite.rewrite, 'Reformuler la story')
  assert.deepEqual(fr.pr.states, {
    open: 'PR ouverte',
    conflicting: 'PR en conflits',
    merged: 'PR fusionnée',
    closed: 'PR fermée sans fusion',
    unknown: 'État de la PR inconnu',
  })
  assert.equal(format(fr.workflow.launchInteractive, { skill: 'Clarify' }), 'Lancer Clarify en interactif')
})

test('team ticket counts follow each language plural rules', () => {
  assert.equal(plural('fr', 1, fr.lookups.teamTaskCount), '1 ticket sur ce board')
  assert.equal(plural('fr', 3, fr.lookups.teamTaskCount), '3 tickets sur ce board')
  assert.equal(plural('en', 1, en.lookups.teamTaskCount), '1 ticket on this board')
  assert.equal(plural('en', 3, en.lookups.teamTaskCount), '3 tickets on this board')
})

test('sprint options speak the UI language, French staying the default', async () => {
  assert.equal(sprintKindLabel('milestone:4', en.lookups.sprintKinds), 'Milestone')
  assert.equal(sprintKindLabel('milestone:4', fr.lookups.sprintKinds), sprintKindLabel('milestone:4'))
  const sprints = [
    { id: 'iteration:31', name: 'Iteration 7', state: 'active' },
    { id: '9', name: 'Jira sprint', state: 'future' },
  ]
  const english = await sprintLookup(sprints, en.lookups.sprintKinds)('')
  assert.deepEqual(english.map(o => o.sublabel), ['Iteration · active sprint', undefined])
  const french = await sprintLookup(sprints, fr.lookups.sprintKinds)('')
  assert.deepEqual(french.map(o => o.sublabel), ['Itération · sprint en cours', undefined])
})

test('English and French define the same keys', () => {
  const shape = value =>
    value && typeof value === 'object'
      ? Object.fromEntries(Object.keys(value).sort().map(key => [key, shape(value[key])]))
      : typeof value
  assert.deepEqual(shape(en), shape(fr))
})
