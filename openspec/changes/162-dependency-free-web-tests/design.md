# Design

## Context
Each of the five affected files opens the same way:

```js
import ts from 'typescript'
const source = await readFile(new URL('../src/lib/remoteRunIndicator.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } })
const { deriveRunIndicator } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
```

`web/tests/apiKeys.test.mjs` and six other files instead do:

```js
import { describeExpiry, expiryState, EXPIRY_WARNING_DAYS } from '../src/lib/apiKeys.ts'
```

Both reach the same exports. Only the first needs an installed package.

## Decision: import the `.ts` source directly
Node 26 (v26.3.0 locally, the version the repository runs on) strips TypeScript types natively, so a
`.mjs` test can import a `.ts` module with no transpilation step. This was verified in the `#162`
worktree with no `web/node_modules` present at all: each of the five subjects imports and exposes its
exports — `src/lib/commandTemplate.ts` (6), `shared/runStates.ts` (6), `src/lib/remoteRunIndicator.ts`
(3), `src/lib/terminalSkillCommand.ts` (1), `src/lib/workflow.ts` (12).

Named imports are used, matching the seven files that already do this, rather than a namespace
import: an export that disappears then fails at import time with its own name in the message.

The transpile preamble collapses to one import line per file. The rest of each file — helpers,
fixtures, assertions — is untouched.

### Rejected: require `npm ci --prefix web` before the suite
Fixes the symptom on a machine where someone remembers to run it, and leaves the gate dependent on
an install step that worktree creation does not perform. It also makes the suite the only consumer
forcing that install, when it does not need one.

### Rejected: provision `node_modules` in task worktrees as part of this change
The right fix for build and lint, and it is documented as the intended behaviour, but it is a Go
change to worktree creation under a ticket titled "fix the failing web tests". Split out.

### Rejected: vendor a transpiler, or precompile the sources into the test tree
Adds a build artifact and a step to keep in sync, to replace something the runtime already does.

## Risk
Node's type stripping rejects non-erasable TypeScript syntax (`enum`, `namespace`,
parameter properties). None of the five subjects uses any, and the seven files already importing
`.ts` directly prove the path. If a subject later gains such syntax, it breaks at import with a
precise error — the same failure mode the rest of the suite already carries.
