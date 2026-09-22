# #344 — Implementation checklist

Ordered so the local components exist before anything imports them, and so the dependency
is dropped only once nothing depends on it. References: [`spec.md`](./spec.md),
[`plan.md`](./plan.md).

## 1. Capture the source of truth (D3)

- [ ] T1.1 Read `web/node_modules/@lobehub/icons/es/<Name>/components/Mono.js` for
      `Antigravity`, `Claude`, `OpenAI` and `Cursor`, and record each `d` attribute
      together with `viewBox`, `fill` and `fillRule`. The main checkout holds the
      installed package if this worktree does not.
- [ ] T1.2 Read `web/node_modules/@lobehub/icons/es/<Name>/style.js` and record each
      `TITLE` (`Antigravity`, `Claude`, `OpenAI`, `Cursor`) — it becomes the `<title>`.

## 2. Vendor the four marks (D1, D2, D3, D4, FR3, FR4)

- [ ] T2.1 Create `web/src/components/icons/types.ts` exporting `BrandIconProps`
      (`Omit<SVGProps<SVGSVGElement>, 'size'> & { size?: string | number }`), with a
      comment saying why `size` is not an SVG attribute.
- [ ] T2.2 Create `web/src/components/icons/Antigravity.tsx`: a `memo`-wrapped component
      taking `{ size = '1em', style, ...rest }`, rendering one `<svg>` with
      `fill="currentColor"`, `fillRule="evenodd"`, `viewBox="0 0 24 24"`,
      `xmlns`, `width={size}`, `height={size}`,
      `style={{ flex: 'none', lineHeight: 1, ...style }}`, `{...rest}`, a `<title>` and
      the path from T1.1.
- [ ] T2.3 Same for `Claude.tsx`.
- [ ] T2.4 Same for `OpenAI.tsx`.
- [ ] T2.5 Same for `Cursor.tsx`.
- [ ] T2.6 Diff each vendored `d` against the package output character by character and
      confirm they are identical.
- [ ] T2.7 Create `web/src/components/icons/index.ts` re-exporting the four components by
      name and `export type { BrandIconProps }` (D6).

## 3. Repoint the call sites (D5, FR5, FR6)

- [ ] T3.1 `web/src/components/ApiKeys.tsx:3` — change the specifier to `'./icons'`,
      keeping the four named imports and every JSX prop as they are.
- [ ] T3.2 `web/src/components/MCPEngineConfig.tsx:3` — same.
- [ ] T3.3 `web/src/components/ProfileModal.tsx:34` — same, three names.
- [ ] T3.4 `web/src/components/ProjectModal.tsx:48` — same, three names.
- [ ] T3.5 `grep -rn "@lobehub" web/src` returns nothing.

## 4. Drop the dependency (D8, FR1, FR2, US3, US4)

- [ ] T4.1 Remove `"@lobehub/icons": "^5.18.0"` from `web/package.json` `dependencies`,
      adding nothing in its place.
- [ ] T4.2 Run `cd web && npm install` to regenerate `web/package-lock.json`.
- [ ] T4.3 Confirm the `version` field is still `0.1.0` in both manifests (AGENTS.md §4).
- [ ] T4.4 Read the lockfile diff: it must contain the removal of the `@lobehub` subtree
      and its peers and nothing else. Revert any unrelated version bump.
- [ ] T4.5 Confirm `@lobehub/icons`, `@lobehub/ui`, `@lobehub/streamdown`,
      `@lobehub/fluent-emoji`, `@emoji-mart/react` and `antd` are absent from the lockfile.

## 5. Test (D7, NFR3)

- [ ] T5.1 Add `web/tests/brandIcons.test.mjs` reading sources with `readFile`, in the
      style of `aiModels.short.test.mjs`.
- [ ] T5.2 Assert `web/package.json` no longer declares `@lobehub/icons`.
- [ ] T5.3 Assert the lockfile mentions none of the four removed packages — the regression
      guard on the ERESOLVE block itself.
- [ ] T5.4 Assert each of the four components carries `viewBox="0 0 24 24"`,
      `fill="currentColor"`, `fillRule="evenodd"`, the `'1em'` default, the
      `flex: 'none'` style spread, a `memo` wrapper and its own `<title>`.
- [ ] T5.5 Assert the four call sites import from `./icons` and that no file under
      `web/src` mentions `@lobehub`.

## 6. Verify (NFR1, NFR2, NFR3, US1)

- [ ] T6.1 `cd web && rm -rf node_modules && npm ci` — no `npm warn ERESOLVE`, no
      `Conflicting peer dependency`, no `@emoji-mart/react`, exit 0. Quote the output.
- [ ] T6.2 `cd web && npm run build` — green.
- [ ] T6.3 `cd web && npm run lint` — no new finding.
- [ ] T6.4 `cd web && npm test` — green, new suite included.
- [ ] T6.5 `make build` — green, and its `web-deps` step prints no ERESOLVE block.
- [ ] T6.6 Confirm `Makefile`, `Dockerfile`, `desktop/` and the root `package-lock.json`
      are untouched by the diff (FR7).

## 7. Document and hand over

- [ ] T7.1 Leave `CHANGELOG.md` untouched (D9) and state why in the pull request.
- [ ] T7.2 Re-read the whole diff as a reviewer would, then commit with a conventional
      message naming the cause rather than the symptom.
