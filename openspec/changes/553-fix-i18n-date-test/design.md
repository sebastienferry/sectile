# Design

## Context

`web/tests/i18n.test.mjs` constructs a URL relative to `import.meta.url`, then calls `fileURLToPath` before embedding it in the child ESM source. That decoding exposes the `#` in the worktree name. The child interprets it as a URL fragment and cannot import the module.

## Decision

Use the existing URL's `.href` as the child import specifier, still serialized with `JSON.stringify`. Keep the child process, its two `TZ` settings, and all four formatting assertions. Node's ESM loader receives an encoded file URL and resolves the intended module.

## Alternatives

- Converting a filesystem path back with `pathToFileURL(...).href` would work, but the test already has a URL.
- Renaming worktrees or changing the browser harness would not address this import at its source.

## Verification

Run `node --test tests/i18n.test.mjs`, `npm test`, `npm run build`, and `npm run lint` from `web/` in the assigned `#553` worktree.
