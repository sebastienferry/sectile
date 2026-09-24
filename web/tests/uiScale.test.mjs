import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  DEFAULT_UI_SCALE,
  UI_SCALE_OPTIONS,
  canStepUIScale,
  normalizeUIScale,
  stepUIScale,
  uiScaleLabel,
} from '../src/lib/uiScale.ts'

test('the ladder is sorted, free of duplicates, and contains the default', () => {
  assert.ok(UI_SCALE_OPTIONS.length > 0)
  assert.deepEqual(UI_SCALE_OPTIONS, [...UI_SCALE_OPTIONS].sort((a, b) => a - b))
  assert.equal(new Set(UI_SCALE_OPTIONS).size, UI_SCALE_OPTIONS.length)
  assert.ok(UI_SCALE_OPTIONS.includes(DEFAULT_UI_SCALE))
})

// Les quatre crans historiques ne bougent pas : un réglage déjà choisi par
// quelqu'un ne se déplace pas pour faire une plus jolie suite.
test('the historical levels survive', () => {
  for (const level of [90, 100, 112, 125]) {
    assert.ok(UI_SCALE_OPTIONS.includes(level), `${level} manquant`)
    assert.equal(normalizeUIScale(level), level)
  }
})

test('a missing or absurd value reads as the default', () => {
  assert.equal(normalizeUIScale(undefined), DEFAULT_UI_SCALE)
  assert.equal(normalizeUIScale(null), DEFAULT_UI_SCALE)
  assert.equal(normalizeUIScale(0), DEFAULT_UI_SCALE)
  assert.equal(normalizeUIScale(-50), DEFAULT_UI_SCALE)
})

// Un réglage écrit par une autre version ne doit pas rendre l'interface
// inutilisable : il s'accroche au cran le plus proche.
test('a value off the ladder snaps to the nearest step', () => {
  assert.equal(normalizeUIScale(111), 112)
  assert.equal(normalizeUIScale(1000), UI_SCALE_OPTIONS[UI_SCALE_OPTIONS.length - 1])
  assert.equal(normalizeUIScale(1), UI_SCALE_OPTIONS[0])
})

test('stepping walks the ladder one rung at a time', () => {
  assert.equal(stepUIScale(100, 'up'), 112)
  assert.equal(stepUIScale(100, 'down'), 90)
  assert.equal(stepUIScale(112, 'up'), 125)
})

// Un bouton qui ne peut plus rien faire se désactive, il ne boucle pas :
// repartir du plus petit alors qu'on demandait plus grand serait le contraire
// du geste.
test('the ends hold instead of wrapping around', () => {
  const lowest = UI_SCALE_OPTIONS[0]
  const highest = UI_SCALE_OPTIONS[UI_SCALE_OPTIONS.length - 1]
  assert.equal(stepUIScale(lowest, 'down'), lowest)
  assert.equal(stepUIScale(highest, 'up'), highest)
  assert.equal(canStepUIScale(lowest, 'down'), false)
  assert.equal(canStepUIScale(highest, 'up'), false)
  assert.equal(canStepUIScale(lowest, 'up'), true)
  assert.equal(canStepUIScale(highest, 'down'), true)
})

test('stepping from a value off the ladder starts from its nearest step', () => {
  assert.equal(stepUIScale(111, 'up'), 125)
  assert.equal(stepUIScale(undefined, 'up'), 112)
})

// La demande qui a fait ajouter des crans : s'arrêter à 125 laissait « c'est
// trop petit » sans réponse.
test('the ladder reaches past the old ceiling and below the old floor', () => {
  assert.ok(UI_SCALE_OPTIONS[UI_SCALE_OPTIONS.length - 1] > 125)
  assert.ok(UI_SCALE_OPTIONS[0] < 90)
})

test('a level shows as a percentage', () => {
  assert.equal(uiScaleLabel(100), '100 %')
  assert.equal(uiScaleLabel(175), '175 %')
})
