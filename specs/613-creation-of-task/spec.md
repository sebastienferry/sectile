# Specification #613 - The quick-add button no longer says "Création CLI"

- Ticket: https://github.com/sebastienferry/sectile/issues/613
- Branch: `feat/613`
- Clarification: `docs/clarifications/613.md` (round 1, confirmed by the owner)
- Framework: Spec Kit

## Summary

While a ticket is being created from the quick-add dialog, its submit button
reads "Création CLI...". No command-line tool is involved in creating the
ticket, and the text stays in French when the interface is in English. With
this change the button reads "Création…" in French and "Creating…" in English
while the creation is in progress.

## Scope

In scope: the in-progress label of the quick-add dialog's submit button, in
both interface languages, and the changelog.

Out of scope:

- Any other wording of the quick-add dialog, including the idle label
  "Créer" / "Create".
- The other creation paths, which show no such label.
- How a ticket is created: the request, the server and the tracker call are
  unchanged.
- The desktop app, which has no quick-add dialog of its own.

## User stories

### US1 (P1) - A French interface says what is happening

As a person using Sectile in French, I read "Création…" on the submit button
while my ticket is being created.

1. Given the interface in French and the quick-add dialog open with a title
   and a project, when I submit, then until the creation ends the button shows
   the spinner and the text "Création…", and no text containing "CLI".
2. Given the same dialog, when the creation ends, then the button no longer
   shows "Création…".

### US2 (P1) - An English interface speaks English

As a person using Sectile in English, I read "Creating…" on the submit button
while my ticket is being created.

1. Given the interface in English and the quick-add dialog open with a title
   and a project, when I submit, then until the creation ends the button shows
   the spinner and the text "Creating…", and no French text.
2. Given the interface switched from French to English, when I submit from
   the quick-add dialog, then the in-progress text is "Creating…".

## Functional requirements

- FR1. While a quick-add submission is in progress, the submit button shows
  the spinner followed by the in-progress label of the current interface
  language.
- FR2. The in-progress label is "Création…" in French and "Creating…" in
  English, each ending with the single ellipsis character (U+2026), not three
  dots.
- FR3. The label comes from the interface translations, like the dialog's
  other texts; no French or English text for it is written in the component.
- FR4. The button's enabled and disabled states, its spinner, and its idle
  label are unchanged.
- FR5. `CHANGELOG.md` has one line under `Fixed` in `[Unreleased]` saying that
  the quick-add button reads "Creating…" (or "Création…" in French) instead of
  "Création CLI...", in the interface language.

## Acceptance criteria

- No text in the quick-add dialog contains "CLI".
- During a submission, a French interface shows "Création…" and an English
  interface shows "Creating…".
- The French and English translations hold the same keys, the new one
  included.
- The changelog carries the `Fixed` line.

## Open points

None. The wording and the use of translations were settled by the owner in
round 1 of the clarification.
