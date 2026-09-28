import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'
import { describeExpiry } from '../src/lib/apiKeys.ts'
import { format, plural, resolveInitialLocale } from '../src/lib/i18n.ts'

const fr = translations.fr.signIn
const en = translations.en.signIn
const now = new Date('2026-09-17T12:00:00Z')
const inDays = n => new Date(now.getTime() + n * 24 * 60 * 60 * 1000).toISOString()

test('the sign-in screen is titled in both languages', () => {
  assert.equal(fr.screen.title, 'Se connecter à Sectile')
  assert.equal(en.screen.title, 'Sign in to Sectile')
  assert.equal(en.screen.continueWithProvider, 'Continue with the identity provider')
  assert.equal(fr.screen.continueWithProvider, 'Continuer avec le fournisseur d\'identité')
})

test('the language switch has an accessible name and names each language in itself', () => {
  assert.equal(fr.language.switchLabel, 'Langue de l\'interface')
  assert.equal(en.language.switchLabel, 'Interface language')
  for (const catalog of [fr, en]) {
    assert.equal(catalog.language.fr, 'Français')
    assert.equal(catalog.language.en, 'English')
  }
})

test('a signed-out visitor gets the remembered language, else French for a French browser, else English', () => {
  assert.equal(resolveInitialLocale('en', ['fr-FR']), 'en')
  assert.equal(resolveInitialLocale(null, ['fr-CA', 'en']), 'fr')
  assert.equal(resolveInitialLocale(null, ['de-DE']), 'en')
  assert.equal(resolveInitialLocale(null, []), 'en')
})

test('a passphrase refusal names the trackers in either language', () => {
  assert.equal(
    format(en.screen.unlockRefused, { trackers: 'gitlab' }),
    'Sealing passphrase refused for gitlab: those tokens stay locked, unlock them from your profile.',
  )
  assert.match(format(fr.screen.unlockRefused, { trackers: 'gitlab' }), /^Phrase de scellement refusée pour gitlab/)
})

test('expiry warnings follow each language\'s plural rules', () => {
  assert.equal(describeExpiry(inDays(3), en.apiKeys, 'en', now), 'Expires in 3 days, renew it')
  assert.equal(describeExpiry(inDays(0.5), en.apiKeys, 'en', now), 'Expires in 1 day, renew it')
  assert.equal(describeExpiry(inDays(3), fr.apiKeys, 'fr', now), 'Expire dans 3 jours, prolongez-la')
  assert.equal(describeExpiry(inDays(0.5), fr.apiKeys, 'fr', now), 'Expire dans 1 jour, prolongez-la')
  assert.equal(describeExpiry(null, fr.apiKeys, 'fr', now), 'Sans expiration')
  assert.match(describeExpiry(inDays(-1), fr.apiKeys, 'fr', now), /^Expirée le /)
  assert.match(describeExpiry(inDays(30), fr.apiKeys, 'fr', now), /\(30 jours\)$/)
})

test('pairing code validity and workstation counts are worded in both languages', () => {
  assert.equal(en.apiKeys.expiryUnknown, 'expiry unknown')
  assert.equal(format(en.apiKeys.validUntil, { time: '10:00' }), 'Valid until 10:00')
  assert.equal(format(fr.apiKeys.validUntil, { time: '10:00' }), 'Valable jusqu\'à 10:00')
  assert.equal(en.apiKeys.copyUnavailable, 'Copy unavailable.')
  assert.equal(en.agent.copyServerUrl, 'Copy Server URL')
  assert.equal(plural('en', 1, en.apiKeys.machines), '1 machine')
  assert.equal(plural('en', 0, en.apiKeys.machines), '0 machines')
  assert.equal(plural('fr', 0, fr.apiKeys.machines), '0 machine')
  assert.equal(plural('fr', 2, fr.apiKeys.machines), '2 machines')
  assert.equal(plural('en', 90, en.apiKeys.renewed, { label: 'laptop' }), 'laptop renewed for 90 days.')
})
