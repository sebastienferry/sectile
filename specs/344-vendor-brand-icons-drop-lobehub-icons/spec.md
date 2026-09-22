# #344 — `npm ci` warns about a peer dependency nothing uses

Ticket: https://github.com/sebastienferry/sectile/issues/344
Type: Bug — "when running the make build there is an error in the npm ci".
Branch: `feat/344`.
Clarification: [`docs/clarifications/344.md`](../../docs/clarifications/344.md).

## Context

`make build` runs `cd web && npm ci`, which prints a block of `npm warn ERESOLVE`
lines: `@emoji-mart/react@1.1.1` declares `peer react@"^16.8 || ^17 || ^18"` while
the project is on `react@19.2.8`, so npm overrides the peer and says so.

The build does not fail. Reproduced on `feat/344` with node 26.9.0 / npm 11.13.0,
`npm ci --dry-run` emits the reported block verbatim and exits 0 (`added 518
packages`); every pasted line is prefixed `npm warn` and there is no `npm error`.
The defect is build-output noise sitting on top of a dependency tree the project
installs and never uses.

The cause chain is settled. `@lobehub/icons@5.18.0`, a direct dependency of
`web/package.json`, declares the **non-optional** peers `@lobehub/ui@^5.0.0` and
`antd@^6.1.1`. npm 7+ installs non-optional peers automatically, so
`@lobehub/ui@5.48.0` enters the tree, and its own dependency
`@emoji-mart/react@1.1.1` carries the react peer react 19 cannot satisfy. None of
it reaches the bundle: the icons barrel `es/index.js` re-exports only `./features`,
`./hooks/useFillId`, `./icons` and `./toc`, while `@lobehub/ui` is imported
exclusively from `es/components/*` and the `.mdx` docs. Satisfying that peer
declaration drags in `antd`, `@ant-design/*`, `emoji-mart`, `@splinetool/runtime`,
`leva`, `shiki`, `katex` and `beautiful-mermaid` for zero bundled bytes — and
`web/src` imports exactly four icons from the package.

This ticket removes the unused tree: `@lobehub/icons` is dropped and the four marks
the UI shows become local SVG components.

Out of scope: the React version, the `desktop/` and Dockerfile install steps,
`--strict-peer-deps` or any other build-failure policy, the `.install-stamp`
incremental mechanism in the `Makefile`, and any change to what the UI displays.
Reported and left alone: the repository root carries a `package-lock.json` named
`taskflow` with zero packages and no matching `package.json`; nothing installs from it.

This file states behaviour and acceptance criteria only. Implementation choices are
in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

The five open questions of the clarification were answered by the ticket owner in
Round 2, each in line with the Round 1 recommendation. No prior assumption was
reversed. They are restated here as the decisions this specification implements.

1. **Done means both.** The unused peer tree is removed *and* the ERESOLVE block
   disappears, the second as a consequence of the first and never as a suppression.
2. **The mechanism is vendoring.** `@lobehub/icons` leaves `web/package.json`; the
   four marks become local components under `web/src/components/icons/` behind a
   barrel. An `overrides` entry, a version pin and won't-fix are all rejected: only
   vendoring removes the cause.
3. **`web/` only.** `cd desktop && npm ci` (`Makefile:33`) and `Dockerfile:14` stay
   untouched. The same class of warning found there becomes its own issue.
4. **No `--strict-peer-deps`.** Turning overridden peers into hard errors is a
   build-policy change that needs its own decision.
5. **Exact marks.** The vendored components reproduce the package's SVG output
   verbatim, so nothing visible changes.

---

## User stories

### US1 — A developer builds the project and reads a clean log (P1)

As a developer running `make build`, I want `npm ci` to print no ERESOLVE block, so
that a genuine dependency warning is visible the day it appears instead of hiding in
noise the team has learned to scroll past.

**Acceptance**

- **Given** a clean checkout of `feat/344` with no `web/node_modules`,
  **when** I run `cd web && npm ci`,
  **then** the output contains no `npm warn ERESOLVE` line, no `Conflicting peer
  dependency` line and no mention of `@emoji-mart/react`, and the command exits 0.
- **Given** the same checkout, **when** I run `make build`,
  **then** it succeeds and its `web-deps` step prints no ERESOLVE block.

### US2 — The UI shows the same four brand marks (P1)

As a user of the Sectile web UI, I want the Antigravity, Claude, OpenAI and Cursor
marks to look exactly as before, so that dropping a dependency is invisible to me.

**Acceptance**

- **Given** the API keys panel, the MCP engine selector, the profile modal and the
  project modal, **when** they render after the change,
  **then** each shows the same mark, at the same size, in the same place as before.
- **Given** any of the four vendored components, **when** it renders,
  **then** its `path` data, `viewBox`, `fill`, `fillRule` and `<title>` are byte
  identical to `@lobehub/icons@5.18.0`'s `es/<Name>/components/Mono.js`.

### US3 — The dependency tree no longer carries the unused packages (P2)

As a maintainer, I want the installed tree to stop carrying `@lobehub/ui`, `antd`
and `@emoji-mart/react`, so that install time, disk footprint and audit surface
reflect what the application actually uses.

**Acceptance**

- **Given** `web/package.json` after the change, **when** I read its
  `dependencies`, **then** `@lobehub/icons` is absent and no replacement package
  was added.
- **Given** `web/package-lock.json` after the change, **when** I search it,
  **then** `@lobehub/icons`, `@lobehub/ui`, `@lobehub/streamdown`,
  `@lobehub/fluent-emoji`, `@emoji-mart/react` and `antd` no longer appear, and the
  installed package count is materially lower than 518.
- **Given** the repository, **when** I grep `web/` for `@lobehub`,
  **then** no *import* resolves to it. The only remaining mentions are the provenance
  comments at the top of the vendored files and of their barrel, which name the package
  and version each mark was copied from — they are how a reviewer checks the copy, so
  they are deliberate.

### US4 — The release procedure is unaffected (P3)

As whoever cuts the next tag, I want the manifests untouched by this fix, so that
the version a release bumps is still the only thing that moves.

**Acceptance**

- **Given** `web/package.json` and `web/package-lock.json` after the change,
  **when** I read their `version` fields, **then** both are still `0.1.0`
  (AGENTS.md §4).

---

## Functional requirements

- **FR1** — `web/package.json` no longer declares `@lobehub/icons`, and declares no
  package in its place.
- **FR2** — `web/package-lock.json` is regenerated by npm (never edited by hand) and
  is consistent with the new manifest, so `npm ci` installs from it without
  resolution work.
- **FR3** — `web/src/components/icons/` holds one component file per vendored mark —
  `Antigravity`, `Claude`, `OpenAI`, `Cursor` — plus an `index.ts` barrel re-exporting
  the four by name.
- **FR4** — Each vendored component honours the contract measured on
  `@lobehub/icons@5.18.0`'s `Mono` export:
  - a single `<svg>` with `fill="currentColor"`, `fillRule="evenodd"`,
    `viewBox="0 0 24 24"` and `xmlns="http://www.w3.org/2000/svg"`;
  - `width` and `height` both bound to a `size` prop defaulting to `'1em'`;
  - `style={{ flex: 'none', lineHeight: 1, ...style }}`, so a caller's `style`
    overrides the defaults;
  - a `<title>` child carrying the mark's name (`Antigravity`, `Claude`, `OpenAI`,
    `Cursor`);
  - every remaining prop spread onto the `<svg>`, so `className` and the rest keep
    working;
  - the component wrapped in `memo`.
- **FR5** — `ApiKeys.tsx`, `MCPEngineConfig.tsx`, `ProfileModal.tsx` and
  `ProjectModal.tsx` import the four marks from the local barrel. Their JSX is
  otherwise unchanged: the `size` and `className` props already passed stay exactly
  as they are.
- **FR6** — No call site gains or loses a mark. `Cursor` stays used only by
  `ApiKeys.tsx` and `MCPEngineConfig.tsx`.
- **FR7** — `Makefile`, `Dockerfile`, `desktop/` and the root `package-lock.json` are
  not modified by this change.

## Non-functional requirements

- **NFR1** — `cd web && npm run build` (`tsc -b && vite build`) succeeds with no new
  TypeScript error, under `noUnusedLocals`, `noUnusedParameters` and
  `verbatimModuleSyntax`.
- **NFR2** — `cd web && npm run lint` (oxlint) reports no new warning or error.
- **NFR3** — `cd web && npm test` passes, including the new suite this change adds.
- **NFR4** — The vendored components are readable source, not the package's babel
  output: TypeScript with the project's own style, no `_objectSpread` helpers.
- **NFR5** — Code, comments and documentation added by this change are written in
  English.

## Out of scope

- Upgrading, downgrading or pinning React, `vite`, or any other dependency.
- `cd desktop && npm ci` (`Makefile:33`) and `Dockerfile:14`.
- Adding `--strict-peer-deps`, `legacy-peer-deps`, an `.npmrc`, or a `--loglevel`
  flag anywhere.
- The `web/node_modules/.install-stamp` incremental rule.
- The stray root `package-lock.json` named `taskflow`.
- Any visual change to the UI.

## Open requirements

None. The clarification closed with zero open product questions.
