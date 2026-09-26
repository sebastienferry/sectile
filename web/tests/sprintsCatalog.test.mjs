import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sprints } from '../src/locales/sprints.ts'
import { format, formatDate, plural } from '../src/lib/i18n.ts'
import { getSprintRelativeInfo } from '../src/lib/sprints.ts'

const { fr, en } = sprints

test('the close dialog keeps its French wording and reads in English', () => {
  assert.equal(format(fr.close.heading, { name: 'Sprint 4' }), 'Clôturer Sprint 4')
  assert.equal(fr.close.subtitle, 'Bilan du cycle et réaffectation des tâches restantes')
  assert.equal(fr.close.confirm, 'Confirmer la clôture')
  assert.equal(format(fr.close.toNext, { name: 'Sprint 5' }), 'Transférer vers Sprint 5 (Recommandé)')
  assert.equal(fr.close.toBacklog, 'Renvoyer vers le Backlog général')
  assert.equal(fr.close.keep, 'Conserver dans ce sprint clôturé')

  assert.equal(format(en.close.heading, { name: 'Sprint 4' }), 'Close Sprint 4')
  assert.equal(en.close.confirm, 'Confirm closing')
  assert.equal(format(en.close.toNext, { name: 'Sprint 5' }), 'Move to Sprint 5 (recommended)')
  assert.equal(en.close.toBacklog, 'Send back to the general backlog')
})

test('unfinished-task counts take the singular and plural forms of each language', () => {
  assert.equal(plural('en', 1, en.close.question), 'What should happen to the 1 unfinished task?')
  assert.equal(plural('en', 3, en.close.question), 'What should happen to the 3 unfinished tasks?')
  assert.equal(plural('fr', 1, fr.close.question), 'Que faire de 1 tâche non terminée ?')
  assert.equal(plural('fr', 3, fr.close.question), 'Que faire des 3 tâches non terminées ?')

  assert.equal(plural('en', 1, en.close.sentToBacklog), '1 unfinished task sent back to the backlog.')
  assert.equal(plural('en', 3, en.close.sentToBacklog), '3 unfinished tasks sent back to the backlog.')
  assert.equal(plural('fr', 0, fr.close.keptInSprint), '0 tâche non terminée conservée dans le sprint.')
  assert.equal(
    plural('fr', 2, fr.close.movedToNext, { name: 'Sprint 5' }),
    '2 tâches non terminées déplacées vers Sprint 5.',
  )
  assert.equal(plural('en', 0, en.close.movedToNext, { name: 'Sprint 5' }), '0 unfinished tasks moved to Sprint 5.')
})

test('the timeline and feedback labels keep today\'s French and have English', () => {
  assert.equal(fr.feedback.saved, 'Sprints mis à jour')
  assert.equal(en.feedback.saved, 'Sprints updated')
  assert.equal(fr.timeline.subtitle, 'Découpage temporel & calcul automatique des cycles')
  assert.equal(en.timeline.subtitle, 'Time slicing & automatic cycle computation')
  assert.equal(fr.timeline.startS1, 'Début S1 :')
  assert.equal(en.timeline.startS1, 'Start S1:')
  assert.equal(fr.timeline.showClosed, 'Afficher les sprints clôturés')
  assert.equal(en.timeline.showClosed, 'Show closed sprints')
  assert.equal(fr.backlog.assignPlaceholder, 'Affecter à un sprint...')
  assert.equal(en.backlog.assignPlaceholder, 'Assign to a sprint...')
  assert.equal(fr.backlog.title, 'Backlog non planifié')
  assert.equal(en.backlog.title, 'Unplanned backlog')
})

test('a sprint starting 2026-09-01 is shown on 1 September in both languages', () => {
  assert.equal(formatDate('en', '2026-09-01'), 'Sep 1, 2026')
  assert.equal(formatDate('fr', '2026-09-01'), '1 sept. 2026')
})

test('the relative badge of a sprint follows the UI language', () => {
  const closed = { id: '1', name: 'Sprint 1', state: 'closed' }
  assert.equal(getSprintRelativeInfo(closed, fr.timeline.relative, 'fr').label, 'Sprint clôturé')
  assert.equal(getSprintRelativeInfo(closed, en.timeline.relative, 'en').label, 'Closed sprint')

  const future = { id: '2', name: 'Sprint 2', state: 'future', startDate: '2999-01-10', endDate: '2999-01-23' }
  assert.match(getSprintRelativeInfo(future, en.timeline.relative, 'en').label, /^Starts in [\d,]+ days$/)
  assert.match(getSprintRelativeInfo(future, fr.timeline.relative, 'fr').label, /^Débute dans [\d\s ]+ jours$/)
})

// Every leaf of a catalog, as [dotted path, value].
function leaves(node, path = '') {
  if (typeof node !== 'object' || node === null) return [[path, node]]
  return Object.entries(node).flatMap(([key, value]) => leaves(value, path ? `${path}.${key}` : key))
}

test('English holds every French key with the same placeholders', () => {
  const placeholders = (text) => [...String(text).matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort()
  const english = new Map(leaves(en))
  for (const [key, value] of leaves(fr)) {
    assert.ok(english.has(key), `missing in English: ${key}`)
    assert.deepEqual(placeholders(english.get(key)), placeholders(value), `placeholders differ: ${key}`)
  }
})
