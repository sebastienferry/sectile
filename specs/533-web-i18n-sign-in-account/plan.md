# Plan #533 - Sign-in and account translation

## Approach

1. `signIn` namespace (`web/src/locales/signIn.ts`, D1): `signIn.screen`,
   `signIn.status`, `signIn.apiKeys`, `signIn.agent`, `signIn.mcp`,
   `signIn.credentials`, `signIn.users`.
2. `SignInScreen.tsx`: `const [locale, setLocale] = useState(resolveInitialLocale)`;
   `const t = translations[locale]`; an effect calls
   `applyDocumentLocale(locale, t.app.documentTitle)`; a compact FR/EN
   segmented switch (`aria-label` from the catalog, `aria-pressed` on the
   active one) calls `rememberLocale`.
3. `lib/apiKeys.ts`: expiry helpers return a status value
   (`{ kind: 'expired' | 'expiresSoon' | 'valid' | 'unknown', days? }`) or
   accept the strings; components render it with the catalog and
   `plural`.
4. `session.ts` messages returned to the screen (passphrase refusal) take the
   catalog string or return a code.

## Documentation

`README.md` (web section) or `docs/` note: how the signed-out language is
chosen (remembered, then browser, fallback English, switch on the sign-in
screen).

## Target files

`web/src/locales/signIn.ts`, the components of FR3, `lib/apiKeys.ts`,
`lib/session.ts` if it returns prose, `tests/apiKeys.test.mjs`,
`tests/session.test.mjs` (updated to the new return shapes),
`web/tests/signInCatalog.test.mjs`, the documentation note.
