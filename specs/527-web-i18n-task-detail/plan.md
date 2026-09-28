# Plan #527 - Task detail translation

## Approach

Move Sectile-owned literals to the `taskDetail` namespace
(`web/src/locales/taskDetail.ts`, D1), grouped as `taskDetail.fields`,
`taskDetail.lookups`, `taskDetail.spec`, `taskDetail.rewrite`,
`taskDetail.workflow`, `taskDetail.pr`, `taskDetail.move`,
`taskDetail.comments`, `taskDetail.clone`, `taskDetail.copySkill`,
`taskDetail.toasts`. Replace `toLocaleString`/`toLocaleDateString` calls with
`formatDateTime`/`formatDate` from `lib/i18n.ts`, locale from
`settings.language`.

## Data contracts

None.

## Target files

`web/src/locales/taskDetail.ts`, the components of FR1,
`web/tests/taskDetailCatalog.test.mjs`.

## Tests

Catalog test on representative keys (type fallback, move confirmation with
its placeholders, copy feedback) in both languages; the French browser
harnesses (`issue-detail-layout`, `pr-state`, `priority-select`) keep
passing unchanged.
