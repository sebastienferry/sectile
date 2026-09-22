# #344 — Implementation checklist

Ordered so the local components exist before anything imports them, and so the dependency
is dropped only once nothing depends on it. References: [`spec.md`](./spec.md),
[`plan.md`](./plan.md).

## 1. Capture the source of truth (D3)

- [x] T1.1 Read `web/node_modules/@lobehub/icons/es/<Name>/components/Mono.js` for
      `Antigravity`, `Claude`, `OpenAI` and `Cursor`, and record each `d` attribute
      together with `viewBox`, `fill` and `fillRule`. The main checkout holds the
      installed package if this worktree does not.
- [x] T1.2 Read `web/node_modules/@lobehub/icons/es/<Name>/style.js` and record each
      `TITLE` (`Antigravity`, `Claude`, `OpenAI`, `Cursor`) — it becomes the `<title>`.

## 2. Vendor the four marks (D1, D2, D3, D4, FR3, FR4)

- [x] T2.1 Create `web/src/components/icons/types.ts` exporting `BrandIconProps`
      (`Omit<SVGProps<SVGSVGElement>, 'size'> & { size?: string | number }`), with a
      comment saying why `size` is not an SVG attribute.
- [x] T2.2 Create `web/src/components/icons/Antigravity.tsx`: a `memo`-wrapped component
      taking `{ size = '1em', style, ...rest }`, rendering one `<svg>` with
      `fill="currentColor"`, `fillRule="evenodd"`, `viewBox="0 0 24 24"`,
      `xmlns`, `width={size}`, `height={size}`,
      `style={{ flex: 'none', lineHeight: 1, ...style }}`, `{...rest}`, a `<title>` and
      the path from T1.1.
- [x] T2.3 Same for `Claude.tsx`.
- [x] T2.4 Same for `OpenAI.tsx`.
- [x] T2.5 Same for `Cursor.tsx`.
- [x] T2.6 Diff each vendored `d` against the package output character by character and
      confirm they are identical.
- [x] T2.7 Create `web/src/components/icons/index.ts` re-exporting the four components by
      name and `export type { BrandIconProps }` (D6).

## 3. Repoint the call sites (D5, FR5, FR6)

- [x] T3.1 `web/src/components/ApiKeys.tsx:3` — change the specifier to `'./icons'`,
      keeping the four named imports and every JSX prop as they are.
- [x] T3.2 `web/src/components/MCPEngineConfig.tsx:3` — same.
- [x] T3.3 `web/src/components/ProfileModal.tsx:34` — same, three names.
- [x] T3.4 `web/src/components/ProjectModal.tsx:48` — same, three names.
- [x] T3.5 `grep -rn "@lobehub" web/src` returns only the provenance comments of the
      vendored files and their barrel — no import.

## 4. Drop the dependency (D8, FR1, FR2, US3, US4)

- [x] T4.1 Remove `"@lobehub/icons": "^5.18.0"` from `web/package.json` `dependencies`,
      adding nothing in its place.
- [x] T4.2 Run `cd web && npm install` to regenerate `web/package-lock.json`.
- [x] T4.3 Confirm the `version` field is still `0.1.0` in both manifests (AGENTS.md §4).
- [x] T4.4 Read the lockfile diff: it must contain the removal of the `@lobehub` subtree
      and its peers and nothing else. Revert any unrelated version bump.
- [x] T4.5 Confirm `@lobehub/icons`, `@lobehub/ui`, `@lobehub/streamdown`,
      `@lobehub/fluent-emoji`, `@emoji-mart/react` and `antd` are absent from the lockfile.

## 5. Test (D7, NFR3)

- [x] T5.1 Add `web/tests/brandIcons.test.mjs` reading sources with `readFile`, in the
      style of `aiModels.short.test.mjs`.
- [x] T5.2 Assert `web/package.json` no longer declares `@lobehub/icons`.
- [x] T5.3 Assert the lockfile mentions none of the four removed packages — the regression
      guard on the ERESOLVE block itself.
- [x] T5.4 Assert each of the four components carries `viewBox="0 0 24 24"`,
      `fill="currentColor"`, `fillRule="evenodd"`, the `'1em'` default, the
      `flex: 'none'` style spread, a `memo` wrapper and its own `<title>`.
- [x] T5.5 Assert the four call sites import from `./icons` and that none of them
      mentions `@lobehub` — the vendored files keep a provenance comment naming the
      package and version they were copied from, so the assertion targets the call sites.

## 6. Verify (NFR1, NFR2, NFR3, US1)

- [x] T6.1 `cd web && rm -rf node_modules && npm ci` — no `npm warn ERESOLVE`, no
      `Conflicting peer dependency`, no `@emoji-mart/react`, exit 0. The whole output is
      now `added 165 packages, and audited 166 packages in 42s` / `found 0
      vulnerabilities`; it was 518 packages and the ERESOLVE block before.
- [x] T6.2 `cd web && npm run build` — green, 2131 modules transformed.
- [x] T6.3 `cd web && npm run lint` — oxlint exits 0 and reports nothing under
      `src/components/icons/`; the warnings on the four edited files are pre-existing and
      sit on lines this change did not touch.
- [x] T6.4 `cd web && npm test` — 129 of 130 pass, the five new ones included. The single
      failure, `skillLaunchModel.test.mjs:134`, predates this change and is caused by the
      checkout's line endings: it reads `TaskCard.tsx`, which this branch does not touch,
      and its regex `/const modeActions = \([\s\S]*?\n {2}\)\n/` cannot match a CRLF file.
      The same regex matches once the CR characters are removed.
- [x] T6.5 `make build` — the `web-deps` and `build-server` halves are green and the
      `npm ci` step prints no ERESOLVE block. The run then fails further on at
      `build-app`, which builds `desktop/`: vite warns `The project root contains the "#"
      character` and dies on `EISDIR: illegal operation on a directory, read
      'C:\git\sectile\.tasks\worktrees'` — the worktree path truncated at `#344`. Nothing
      in this branch touches `desktop/`, and `make build-server` exits 0.
- [x] T6.6 Confirm `Makefile`, `Dockerfile`, `desktop/` and the root `package-lock.json`
      are untouched by the diff (FR7).

## 7. Document and hand over

- [x] T7.1 Leave `CHANGELOG.md` untouched (D9) and state why in the pull request.
- [x] T7.2 Re-read the whole diff as a reviewer would, then commit with a conventional
      message naming the cause rather than the symptom.
