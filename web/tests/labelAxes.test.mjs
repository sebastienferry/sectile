import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  LABEL_AXES,
  axisLabelOf,
  axisNameOf,
  axisWords,
  isAxisLabel,
} from '../src/lib/labelAxes.ts'
import { planning } from '../src/locales/planning.ts'

test('the two axes carry their own prefix', () => {
  assert.equal(LABEL_AXES.phase.prefix, 'phase:')
  assert.equal(LABEL_AXES.goal.prefix, 'goal:')
  assert.notEqual(LABEL_AXES.phase.prefix, LABEL_AXES.goal.prefix)
})

test('a label belongs to one axis and not the other', () => {
  assert.equal(isAxisLabel('phase', 'phase:migration-a'), true)
  assert.equal(isAxisLabel('goal', 'phase:migration-a'), false)
  assert.equal(isAxisLabel('goal', 'goal:latence-200ms'), true)
})

// Le préfixe est ce qui rend la vue possible : sans lui, tout label non
// reconnu se présenterait comme une phase.
test('an ordinary label is not an axis', () => {
  for (const label of ['force-close', 'theodo', 'clarified', 'phase', 'goal']) {
    assert.equal(isAxisLabel('phase', label), false, label)
    assert.equal(isAxisLabel('goal', label), false, label)
  }
})

test('the tracker guarantees no casing, so recognition ignores it', () => {
  assert.equal(isAxisLabel('phase', 'Phase:Migration-A'), true)
  assert.equal(isAxisLabel('phase', '  PHASE:migration-a  '), true)
})

test('the readable name is the label without its prefix', () => {
  assert.equal(axisNameOf('phase', 'phase:migration-a'), 'migration-a')
  assert.equal(axisNameOf('goal', 'goal:latence-200ms'), 'latence-200ms')
})

// Jira refuse les espaces dans un label : « migration A » serait rejeté.
test('spaces become hyphens, and the case is imposed', () => {
  assert.equal(axisLabelOf('phase', 'Migration A'), 'phase:migration-a')
  assert.equal(axisLabelOf('phase', '  migration   A  '), 'phase:migration-a')
  assert.equal(axisLabelOf('goal', 'Latence 200ms'), 'goal:latence-200ms')
})

// Un axe n'a qu'une orthographe : « phase:Migration-A » saisi ici et
// « phase:migration-a » posé depuis le tracker doivent être le même groupe.
test('one spelling per axis, so two sources land in one group', () => {
  assert.equal(axisLabelOf('phase', 'Migration-A'), axisLabelOf('phase', 'migration-a'))
})

test('a name already prefixed is not prefixed twice', () => {
  assert.equal(axisLabelOf('phase', 'phase:migration-a'), 'phase:migration-a')
  assert.equal(axisLabelOf('phase', 'PHASE:Migration A'), 'phase:migration-a')
  assert.equal(axisLabelOf('goal', 'goal:latence'), 'goal:latence')
})

// La chaîne vide est le signal qu'attend le bouton pour rester désactivé.
test('a name with nothing left to name yields no label', () => {
  assert.equal(axisLabelOf('phase', ''), '')
  assert.equal(axisLabelOf('phase', '   '), '')
  assert.equal(axisLabelOf('phase', 'phase:'), '')
  assert.equal(axisLabelOf('goal', '  goal:  '), '')
})

test('each axis names itself in its own words', () => {
  const phase = axisWords('phase', planning.fr.macro.axes)
  const goal = axisWords('goal', planning.fr.macro.axes)
  assert.equal(phase.plural, 'Phases')
  assert.equal(goal.plural, 'Objectifs')
  assert.notEqual(phase.none, goal.none)
  for (const words of [phase, goal]) {
    for (const key of ['singular', 'plural', 'none', 'placeholder']) {
      assert.ok(words[key], `${key} manquant`)
    }
  }
})

test('the axis words follow the UI language', () => {
  assert.equal(axisWords('goal', planning.en.macro.axes).plural, 'Goals')
  assert.equal(axisWords('phase', planning.en.macro.axes).none, 'No phase')
})
