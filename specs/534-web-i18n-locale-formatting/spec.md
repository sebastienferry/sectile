# Spec #534 - Web i18n: document language, date formatting and translation regression checks

Clarification: `docs/clarifications/534.md` (decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - The page speaks the UI language

As an English user, I see `lang="en"` on the page and an English browser
title; as a French user, `lang="fr"` and a French title, from the first paint,
after switching language and after reloading.

- **Given** the UI language is English, **when** the application has started,
  **then** `document.documentElement.lang` is `en` and `document.title` is the
  English title.
- **Given** the UI language is English, **when** I switch to French in the
  profile, **then** the language and title change at once, without reload.
- **Given** I reload the page, **when** it starts, **then** the remembered UI
  language applies before the settings arrive, and the signed-out screen uses
  it too.

### US2 (P1) - Dates and numbers follow the UI language

As a user whose browser language differs from my Sectile language, I see
dates, times and numbers formatted in the Sectile language.

- **Given** a browser in German and Sectile in French, **when** a date is
  shown, **then** it is formatted `fr-FR`.
- **Given** a date-only value `2026-09-01`, **when** it is shown in any time
  zone, **then** the calendar day is 1 September, never 31 August.
- **Given** an instant, **when** it is shown, **then** it is converted to the
  viewer's time zone.
- **Given** a missing or invalid value, **when** it is shown, **then** a
  neutral placeholder (`-`) appears instead of `Invalid Date`.

### US3 (P1) - Counts agree with their number

- **Given** 0, 1 and 5 items, **when** a count is shown in French, **then** 0
  and 1 take the singular and 5 the plural; in English only 1 is singular.

### US4 (P2) - Regressions are caught

- **Given** a key missing or empty in one language, or a placeholder present
  in one language only, **when** the web tests run, **then** they fail and
  name the key.
- **Given** representative components rendered with the English catalog,
  **when** the browser checks run, **then** their accessible names are English
  and user or tracker text is shown unchanged.

## Functional requirements

- FR1 `web/src/lib/i18n.ts` exports `Locale`, `intlLocale(locale)`,
  `format(template, params)`, `plural(locale, count, forms)`,
  `formatDate`, `formatDateTime`, `formatTime`, `formatNumber`, and the
  pre-authentication helpers `rememberLocale`, `resolveInitialLocale`.
- FR2 `format` replaces every occurrence of each `{name}` and leaves unknown
  placeholders visible.
- FR3 Date formatters accept `string | number | Date | null | undefined`,
  treat `YYYY-MM-DD` strings as calendar dates, and return `-` for missing or
  invalid input.
- FR4 The document language and title are applied at startup from the
  remembered locale, by the authenticated shell from the settings, and by the
  sign-in screen from its own locale.
- FR5 The date call sites listed in the ticket use the shared formatters.
- FR6 A catalog parity test covers every namespace, both languages: same key
  set, non-empty strings, same placeholder set.
- FR7 `docs/web-translation-checks.md` holds the browser check matrix (shell,
  board/backlog, details, project configuration, planning, skills,
  activities/sync, sign-in/profile, admin, quick-add #455).
- FR8 No accent or ASCII detector over sources.

## Out of scope

Screen copy (#526 to #533), pre-authentication language selection UI (#533).
