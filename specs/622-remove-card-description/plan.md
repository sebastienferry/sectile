# Plan #622 - Remove the description from the card view

## Stack

Web front end only: React and TypeScript in `web/src`, Playwright browser
regressions in `web/tests/*.browser.mjs`. Sectile Desktop loads the built web
bundle, so it inherits the change without desktop code. No server, agent,
database or tracker change.

## Current behaviour

`web/src/components/TaskCard.tsx`, in the full card's JSX, right after the
title `<h4>`:

```tsx
{/* Ligne 2 : Description tronquée */}
{task.description && (
  <p className="text-[11px] text-[var(--text-muted)] line-clamp-2 mb-2 leading-relaxed">
    {task.description}
  </p>
)}
```

The condensed card (`compact` prop, compact density) does not render this
block, which is why `web/tests/condensed-card.browser.mjs` already asserts that
the text `Hidden description` is absent there. Line 36 of that test switches to
the standard and comfortable densities and waits for `Hidden description` to
appear, pinning the behaviour this change removes.

## Changes

1. `web/src/components/TaskCard.tsx`: delete the block above, its comment
   included. The block is deleted rather than hidden with CSS, so the text is
   neither rendered nor exposed to assistive technology. The title keeps its
   `mb-1.5` margin; the metadata row below it provides the spacing the
   description's `mb-2` used to add, so no other class changes.
2. `web/tests/condensed-card.browser.mjs` line 36: for each of `standard` and
   `comfortable`, render the full card, wait for it (for example its title), and
   assert `page.getByText('Hidden description').count()` equals 0. The fixture
   keeps `description: 'Hidden description'`, so the test proves a described
   task shows no description on any density.
3. `CHANGELOG.md`, `## [Unreleased]`, `### Changed` (create the subsection if it
   is missing): one line such as "Board cards no longer show an excerpt of the
   task's description; open the task to read it. The list view keeps its
   excerpt. (#622)".

## Rejected alternatives

- **A display setting.** Rejected by the owner in the clarification: it would
  add a stored key, a settings entry and translations for a text the detail
  panel already shows.
- **Removing the list view excerpt too.** Rejected by the owner: the list is a
  denser reading surface where the excerpt helps scanning.
- **Hiding with CSS** (`hidden`, `sr-only`). Rejected: it keeps dead markup and
  would still be read by screen readers in the `sr-only` case.

## Data contracts

None change. `Task.description` stays in the type, the API and the tracker
synchronisation; `ListView` and the detail panel still read it.

## Risks

- Other browser tests carrying a `description` in their fixture
  (`pr-state.browser.mjs`, `card-model-menu.browser.mjs`) assert nothing about
  it; they stay green.
- No translated string is removed: the deleted block renders the raw
  description, not a locale key.
