# Plan #612 - Web card: model menu and copy-prompt button

## Stack

React 19 + TypeScript web app (`web/`), Tailwind classes, `lucide-react`
icons, Playwright browser component tests (`web/tests/*.browser.mjs`) and
`node --test` unit tests. No server, agent, desktop or database change; no
migration; no API change.

## Design

### 1. One prompt builder, two callers

`CopyTaskSkillMenu.tsx` owns today both the default command table
(`SKILL_COMMANDS`) and the prompt text (`promptFor`). Move them to a new pure
module so the card's icon cannot drift from the menu entry:

```ts
// web/src/lib/skillPrompt.ts
export const SKILL_COMMANDS: Record<string, string>   // clarify..handoff
export const PICKUP_SKILL = 'pickup'
export const PICKUP_COMMAND = '/pickup-issue'
/** The prompt pasted into an assistant to run `command` on `task`. */
export function taskSkillPrompt(command: string, taskId: string, projectId?: string): string
```

`taskSkillPrompt` returns exactly today's string:
`${command} ${taskId}. Use Sectile MCP to read the task and comments and record
workflow transitions. First call start_run and save its returned ID. Call
finish_run with that runId when this entire skill ends, including failure or
stopping for user input. Task primary key: ${taskId}.` followed by
` Project primary key: ${projectId}.` when `projectId` is set.

`CopyTaskSkillMenu` keeps resolving the command through
`skillCommand(skillId, SKILL_COMMANDS[skillId], task.projectId)` and calls
`taskSkillPrompt` with the result. Its rendering does not change.

### 2. The model list, rendered once

In `TaskCard.tsx`, extract the radio entries of the "Modèle des lancements"
sub-list (the configured entry and `offeredModels.map(...)`) into a local
render function `modelRadioItems()`. Both the `(...)` sub-list and the new
indicator menu render it, so FR2 holds by construction. Both call the
existing `chooseModel(model)`, which already calls `setLaunchModel` and
`saveLaunchModel`; the two lists read the same `launchModel` state of the
card, so a pick from one is shown by the other immediately (FR3, US2).
`chooseModel` also closes the indicator menu.

### 3. The indicator becomes a menu button

New state and refs in `TaskCard`:

```ts
const [isIndicatorMenuOpen, setIsIndicatorMenuOpen] = useState(false)
const indicatorRef = useRef<HTMLButtonElement>(null)
const indicatorMenuRef = useRef<HTMLDivElement>(null)
const [indicatorPos, setIndicatorPos] = useState<MenuPos | null>(null)
```

- Move the positioning of `openMenuAt` into a pure helper,
  `anchoredMenuPosition(anchor: HTMLElement, width: number)` in
  `web/src/lib/anchoredMenu.ts` (same zoom correction, viewport clamping,
  above/below choice and bottom reserve), so the card and the copy button's
  fallback panel share it. The `(...)` menu keeps its behaviour by calling it
  with `MENU_WIDTH`. *Changed during implementation: the plan first kept the
  helper inside `TaskCard.tsx`; the copy button needs it too.*
- `modelIndicator` keeps its three branches:
  - `cardModels.length > 0`: a `<button type="button" aria-haspopup="menu"
    aria-expanded={isIndicatorMenuOpen}>` with the current classes, title and
    label; when `launchedModel` is empty it renders `<Cpu size={11} />` with
    the title `t.shell.card.chooseModel` (FR4). `onClick` stops propagation,
    closes the `(...)` menu and toggles the indicator menu.
  - `engineUnknown`: unchanged passive `<span>` with `?`.
  - otherwise, when `launchedModel` is set: unchanged passive `<span>`;
    else nothing.
- The indicator menu is portalled to `document.body` with `role="menu"`,
  `aria-label={t.compactCard.advanceWithModel}`, fixed at `indicatorPos`,
  styled as the `(...)` menu, and contains `modelRadioItems()`. It stops
  click and mousedown propagation so the card neither opens nor drags.
- Keyboard: on open, focus the checked item (else the first). ArrowDown and
  ArrowUp move between `menuitemradio` items, wrapping; Home and End jump.
  The handling lives in `moveMenuFocus(menu, key)` next to the position
  helper; the `(...)` sub-list, which had only ArrowLeft, uses it too, and
  keeps its ArrowLeft behaviour.
- Closing: a document effect active while `isIndicatorMenuOpen` closes it on
  Escape (focus back to `indicatorRef`), on a mousedown outside the menu and
  the indicator, on a captured scroll outside the menu, and on resize,
  mirroring the `(...)` effect. Opening `(...)` closes the indicator menu and
  vice versa, so only one popup exists per card.
- The ml-auto logic of the bottom row (today `launchedModel ? '' : 'ml-auto '`
  on the `>` button) is replaced by putting `ml-auto` on the first rendered
  element of the group indicator / copy icon / `>`; compute it from the same
  conditions rather than duplicating class strings.

### 4. The copy icon (full card only)

A small component in `web/src/components/CopyStepPromptButton.tsx`:

```tsx
export function CopyStepPromptButton({ task }: { task: Task })
```

- Resolves `stage = resolveTaskStage(task, project)` and
  `stageSkill = skillForStage(stage)` as `CopyTaskSkillMenu` does; renders
  nothing when `stageSkill` is null (the finished stage, FR6).
- `command = skillCommand(stageSkill, SKILL_COMMANDS[stageSkill],
  task.projectId)`; `title` and `aria-label` are
  `format(t.taskDetail.copySkill.copyCommand, { command })`.
- Icon: `Copy` from lucide, `size={14}`, same classes as the `>` button.
- On click (propagation stopped): `navigator.clipboard.writeText(prompt)`;
  on success `addToast({ type: 'info', title: t.shell.card.promptCopied,
  description: format(t.shell.card.promptCopiedDescription, { command, key:
  task.key }) })`; on rejection (or a missing `navigator.clipboard`) open the
  fallback panel.
- Fallback panel: portalled, anchored with `anchoredMenuPosition` (width
  about 280), `role="dialog"` with `aria-label` from
  `t.taskDetail.copySkill.clipboardBlocked`, the message in a `<p>` and the
  prompt in the same `<pre><code>` as `CopyTaskSkillMenu` (`select-text`).
  Escape, outside mousedown, scroll and resize close it and return focus to
  the icon. Its clicks and mousedowns stop propagation.

Placed in the full card's bottom row between `modelIndicator` and the `>`
button. The condensed branch does not render it.

### 5. Strings

`web/src/locales/shell.ts` (card section, French then English) and the
matching type in `translations.ts` if the card section is typed there:

| Key | fr | en |
| --- | --- | --- |
| `chooseModel` | Choisir le modèle des lancements | Choose the launch model |
| `promptCopied` | Prompt copié | Prompt copied |
| `promptCopiedDescription` | {command} pour {key}, à coller dans votre assistant | {command} for {key}, to paste into your assistant |

The indicator's title keeps `modelChosen` / `modelConfigured`; the copy
label reuses `copySkill.copyCommand`.

## Data contracts

None new. The per-task pick stays in `localStorage` under
`sectile_launch_models` through `loadLaunchModel` / `saveLaunchModel`.
`advanceTask(taskId, auto, mode, model)` is unchanged.

## Target files

- `web/src/lib/skillPrompt.ts` (new)
- `web/src/lib/anchoredMenu.ts` (new)
- `web/src/components/CopyTaskSkillMenu.tsx`
- `web/src/components/CopyStepPromptButton.tsx` (new)
- `web/src/components/TaskCard.tsx`
- `web/src/locales/shell.ts`, `web/src/locales/translations.ts` (types)
- `web/tests/skillPrompt.test.mjs` (new)
- `web/tests/card-model-menu.browser.mjs` (new)
- `web/tests/condensed-card.browser.mjs` (assert no copy icon)
- `web/tests/skillLaunchModel.test.mjs` (source-shape assertions follow the new indicator)
- `CHANGELOG.md`

## Rejected alternatives

- **A second, independent model state for the indicator.** Two stores could
  disagree; the owner asked that both entry points show one selection.
- **Rendering the indicator menu inline, not in a portal.** The card and the
  column clip overflow; the `(...)` menu is portalled for that reason.
- **Reusing `CopyTaskSkillMenu` inside a popover for the icon.** It would
  turn one click into two, against round 2 decision 1.
- **A per-engine prompt.** Out of scope by round 2 decision 2; the agent
  itself sends the same prompt to every engine.

## Risks

- `TaskCard.tsx` is large (about 1000 lines): edit it in small hunks.
- Browser tests need Playwright from the main checkout
  (`PLAYWRIGHT_MODULE`) and a `#`-free path; worktrees have no
  `node_modules` of their own.
