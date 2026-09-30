# Plan #637 - Card: autonomous next step instead of an idle full chain

## Stack

React 19 + TypeScript web app (`web/`), Tailwind classes, `lucide-react`
icons, `node --test` unit tests and Playwright browser component tests
(`web/tests/*.browser.mjs`). No server, agent, desktop or database change; no
migration; no API change.

## Design

### 1. One rule, in `web/src/lib/workflow.ts`

```ts
/** Default of `Project.fullChainStopStage`, as documented in docs/CAPABILITIES.md. */
export const DEFAULT_FULL_CHAIN_STOP_STAGE: WorkflowStage = 'reviewed'

/**
 * Whether the full chain still has a stage to run for a task at `stage`: the
 * chain stops at the project's stop stage, so a task already there or past it
 * has nothing left for it.
 */
export const fullChainHasWork = (stage: WorkflowStage, project?: Project | null): boolean =>
  WORKFLOW_ORDER.indexOf(stage) < WORKFLOW_ORDER.indexOf(project?.fullChainStopStage || DEFAULT_FULL_CHAIN_STOP_STAGE)
```

`finished` is past every stop stage, so the rule answers `false` for it; the
card keeps finished tasks on their current rendering separately (FR2).

### 2. The card decides once

In `TaskCard.tsx`, next to `isFinishedTask`:

```ts
// The full chain stops at the project's stop stage: from there on it has
// nothing to run, so the card offers the next step, autonomously, instead.
const offersAutonomousStep = !isFinishedTask && !fullChainHasWork(nextStepInfo.currentStage, taskProject)
```

`nextStepInfo.currentStage` is the resolved stage (`resolveTaskStage`), the
one every other launch control of the card already uses.

### 3. Spinner ownership

`handleAdvance(auto, mode)` sets `advancing` to `'auto'` or `'step'`, which
picks the button that spins. It gains an optional third argument naming the
spinner, defaulting to today's choice:

```ts
const handleAdvance = async (auto: boolean, mode?: SkillMode, spinner: 'step' | 'auto' = auto ? 'auto' : 'step') => { ... setAdvancing(spinner) ... }
```

The shortcut, which sits where `>>` was, calls
`handleAdvance(false, 'autonomous', 'auto')` and spins on `advancing ===
'auto'`. The guard (`advancing !== null`) is unchanged, so every launch
control of the card stays disabled while one launch is pending.

### 4. Full card

In the bottom row, the `>>` button renders only when `!offersAutonomousStep`.
Otherwise a button with the same classes renders instead:

- icon `<Bot size={14} />`, spinner `<Loader2>` on `advancing === 'auto'`;
- `disabled={advancing !== null}`;
- `title` and `aria-label` =
  `format(t.shell.card.autonomousStep, { skill, description: nextStepInfo.stepDescription })`,
  where `skill = skillLabel(skillId, t.shell.card.skills[skillId])` and
  `skillId = skillForStage(nextStepInfo.currentStage)` (`adjust` or
  `handoff` here, both present in `t.shell.card.skills`);
- `onClick` stops propagation, then calls the advance as in §3.

### 5. Condensed card

In the condensed `(...)` menu, the "Chaîne complète" button renders only when
`!offersAutonomousStep`. `modeActions` (with "Avancer en autonome") is
unchanged. `CopyTaskSkillMenu` is not touched.

### 6. Strings

`web/src/locales/shell.ts`, card section (the English object has the shape of
the French one):

| Key | fr | en |
| --- | --- | --- |
| `autonomousStep` | Lancer {skill} en autonome : {description} | Run {skill} autonomously: {description} |

## Data contracts

None new. `Project.fullChainStopStage` (`'implemented' | 'reviewed'`,
optional) is already sent to the web client. `advanceTask(taskId, auto, mode,
model)` is unchanged.

## Target files

- `web/src/lib/workflow.ts`
- `web/src/components/TaskCard.tsx`
- `web/src/locales/shell.ts`
- `web/tests/workflowAdjustment.test.mjs` (rule)
- `web/tests/card-autonomous-step.browser.mjs` (new)
- `web/tests/condensed-card.browser.mjs` (its "Chaîne complète" assertions run on a `new` task: kept)
- `CHANGELOG.md`

## Rejected alternatives

- **Testing `currentStage === 'reviewed'`.** The owner reversed that reading in
  round 2: with the `implemented` stop stage, an implemented card has the same
  idle `>>`.
- **Hiding `>>` without a replacement.** The ticket asks for the autonomous
  shortcut in its place.
- **A new condensed menu entry.** "Avancer en autonome" is already the same
  launch (round 2, answer 2).
- **Asking the server whether the chain has work.** The rule is a pure
  function of data the client already holds; the server keeps refusing an
  out-of-range chain on its own.

## Risks

- `TaskCard.tsx` is large: edit it in small hunks.
- Browser tests need Playwright from the main checkout (`PLAYWRIGHT_MODULE`)
  and a `#`-free path; worktrees have no `node_modules` of their own.
