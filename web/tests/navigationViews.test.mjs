import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'

test('navigation roadmap and timeline views are localized in French and English', () => {
  const views = ['roadmap', 'timeline']

  for (const view of views) {
    const frNav = translations.fr.nav[view]
    const enNav = translations.en.nav[view]

    assert.ok(frNav, `Missing French translation for nav view: ${view}`)
    assert.ok(enNav, `Missing English translation for nav view: ${view}`)
    assert.notEqual(frNav.trim(), '')
    assert.notEqual(enNav.trim(), '')
  }

  assert.equal(translations.fr.nav.roadmap, 'Roadmap')
  assert.equal(translations.en.nav.roadmap, 'Roadmap')

  assert.equal(translations.fr.nav.timeline, 'Timeline')
  assert.equal(translations.en.nav.timeline, 'Timeline')
})
