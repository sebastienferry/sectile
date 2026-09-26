# Plan #534 - Locale formatting, document language, regression checks

## Stack

React 19 + TypeScript web client (`web/`), Vite build, `node --test` unit
tests (`web/tests/*.test.mjs`, TypeScript loaded natively by Node 22.18+),
Playwright browser harnesses (`web/tests/*.browser.mjs`, `useApp` mocked).

## Contract: `web/src/lib/i18n.ts`

```ts
export type Locale = 'fr' | 'en'
export const intlLocale = (l: Locale) => (l === 'en' ? 'en-US' : 'fr-FR')
export function format(template: string, params: Record<string, string | number>): string
export function plural(locale: Locale, count: number, forms: { one: string; other: string }): string
// forms may contain {count}; plural() formats it.
export function formatDate(locale: Locale, value: DateInput, options?: Intl.DateTimeFormatOptions): string
export function formatDateTime(locale: Locale, value: DateInput, options?: Intl.DateTimeFormatOptions): string
export function formatTime(locale: Locale, value: DateInput, options?: Intl.DateTimeFormatOptions): string
export function formatNumber(locale: Locale, value: number, options?: Intl.NumberFormatOptions): string
export const EMPTY_VALUE = '-'
export function rememberLocale(locale: Locale): void        // localStorage, guarded
export function resolveInitialLocale(): Locale              // remembered, else navigator, else 'en'
export function applyDocumentLocale(locale: Locale, title: string): void
```

`formatDate` defaults to `{ day: 'numeric', month: 'short', year: 'numeric' }`,
`formatDateTime` adds hour and minute. A `YYYY-MM-DD` string is built as a
local `Date(y, m - 1, d)`.

## Catalog layout (D1)

`web/src/locales/<surface>.ts` modules, each:

```ts
const fr = { … }
export type ShellStrings = typeof fr   // widened to string by a helper type
const en: ShellStrings = { … }
export const shell = { fr, en }
```

A `Strings<T>` helper type maps literal types to `string` so English values
type-check. `translations.ts` adds `shell: ShellStrings` etc. to
`TranslationSchema` and spreads each module's language into `fr` and `en`.
New `app.documentTitle` key for the title.

## Document language

- `main.tsx`: `applyDocumentLocale(resolveInitialLocale(), …)` before render.
- `AppContext.tsx`: an effect on `settings.language` applies the locale and
  title, and calls `rememberLocale` once the settings came from the server.
- `index.html`: keeps a neutral `Sectile` title; `lang` is set at startup.

## Target files

`web/src/lib/i18n.ts` (new), `web/src/locales/*.ts` (new modules),
`web/src/locales/translations.ts`, `web/src/main.tsx`, `web/index.html`,
`web/src/context/AppContext.tsx`, `web/src/components/AdminView.tsx`,
`web/src/components/UsersPanel.tsx`, `web/tests/i18n.test.mjs`,
`web/tests/translationCatalog.test.mjs`, `web/tests/i18n-shell.browser.mjs`,
`docs/web-translation-checks.md`. The other date call sites are migrated by
the tickets that own their files.

## Rejected alternatives

- An i18n library (i18next, FormatJS): a new dependency for what
  `Intl.PluralRules` and a 10-line `format` cover; the catalog stays.
- A source scan for French characters: flags proper nouns and user content.
