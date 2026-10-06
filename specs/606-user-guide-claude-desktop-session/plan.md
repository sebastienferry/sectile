# Plan #606 - The user guide explains how a workflow skill shows up in a Claude desktop session

## Stack

Markdown only. No code, no build step, no test harness for prose.

## Target files

| File | Change |
| --- | --- |
| `docs/USER_GUIDE.md` | New subsection `### What the session shows` at the end of **Use a prompt in Claude Code**. |
| `docs/images/user-guide/session-sidebar.png` | Owner-supplied capture (a), when available. |
| `docs/images/user-guide/session-links.png` | Owner-supplied capture (b), when available. |

## Source of truth

The subsection paraphrases `internal/skills/fragments/contracts/session-title.md`
as merged by #603. Points to carry over, in the guide's plain second-person
style:

1. **Title.** `<ticket ID> - <ticket title>`, batch `#47 (+2) - <title>`. A
   macro skill uses the macro's ID and title. No emoji while the skill works.
2. **Emoji table.** ❓ (the skill waits for an answer: before a blocking
   question, or when it stops with open questions; removed when work resumes),
   ✅ (the skill reached its goal), ❌ (it stopped on a failure or a blocker it
   cannot resolve). Each one is set at the moment Sectile records the matching
   run state, so the board and the title agree.
3. **Links.** `Ticket:` (or `Macro:`) line after the rename, skipped without an
   external URL; `PR:` line once a pull or merge request is created or found;
   GitHub pull request bound to the PR bar; GitLab: the line only.
4. **Session experience.** Sidebar group named after the Sectile project
   (reused when it exists); chapters; diff pane when implement, adjust or a
   pickup changed code (otherwise the worktree path); next step's command on ✅,
   or what to answer or fix on ❓ / ❌; one notification on ❓, ✅ or ❌, none for
   routine progress.
5. **Nesting.** A stage skill nested in a pickup leaves the title, links,
   group, chapters, pane, next step and notification to the pickup.
6. **Launched by Sectile.** When Desktop launches the run, the links, the next
   step and the notification are skipped, since Desktop shows them.
7. **Other hosts and refusals.** Only what the host exposes is used; a missing
   capability or a refused or unapproved action is skipped, and the run
   continues. A host that cannot rename shows the state through Sectile alone.

## Decisions

- **Prose plus one table**, consistent with the guide's other sections; no
  numbered procedure, since the user does nothing.
- **Images after the text they illustrate**, and only once supplied. Without
  them the subsection is complete; adding them later is a one-line change each.
- **No changelog line**, per the clarification.

## Rejected alternatives

- A section of its own or a Desktop guide entry: rejected in clarification
  round 2 (placement settled).
- Agent-drawn mock-ups of the Claude desktop app: rejected in round 3; the owner
  captures a demo project instead.
