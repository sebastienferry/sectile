# Specification #606 - The user guide explains how a workflow skill shows up in a Claude desktop session

- Ticket: https://github.com/sebastienferry/sectile/issues/606
- Branch: `feat/606`
- Clarification: `docs/clarifications/606.md` (rounds 1 to 3, confirmed by the
  owner)
- Framework: Spec Kit
- Depends on: #603, merged on 2026-09-29 (`24444b68`)

## Summary

Since #603, a workflow skill run by hand in the Claude desktop app renames the
session, adds a status emoji to its title, writes ticket and pull request
links, files the session under a project group, marks chapters, opens the diff
pane, ends on the next step's command and sends a notification. The user
guide's **Use a prompt in Claude Code** section says none of this. This change
adds a subsection that explains it, so a user can read the sidebar and the
conversation without guessing.

## Scope

In scope: a new subsection `### What the session shows` at the end of **Use a
prompt in Claude Code** in `docs/USER_GUIDE.md`, and two owner-supplied
captures under `docs/images/user-guide/`.

Out of scope:

- The skill behaviour itself, owned by #603 and
  `internal/skills/fragments/contracts/session-title.md`.
- The reverse link from a run to its session (#602).
- Sectile Desktop's own indicators, already documented in `desktop/README.md`.
- `desktop/README.md`, `README.md` and any translation.
- A changelog line: #603 already added the user-facing lines for the feature.

## User stories

### US1 - Read the session title (P1)

- **Given** a skill identified ticket `#47`, **when** I read the guide, **then**
  it says the session is renamed `<ticket ID> - <ticket title>`, and that a
  batch reads `#47 (+2) - <title>`.
- **Given** the guide's table, **when** I look up ❓, ✅ or ❌, **then** it says
  what each means and when it appears, and that the title carries no emoji while
  the skill works.

### US2 - Find the ticket and the pull request (P1)

- **Given** a run on a GitHub project, **then** the guide says a `Ticket:` line
  follows the rename, a `PR:` line appears once a pull request exists, and the
  pull request is also bound to the session's PR bar.
- **Given** a GitLab project, **then** it says the `PR:` line is the only link.

### US3 - Recognise the rest of the session (P2)

- **Given** the guide, **then** it mentions the sidebar group named after the
  Sectile project, the chapters (one per stage under a pickup, one when a single
  skill starts in a session that already holds work), the diff pane after a
  skill changed code, the next step's command (`/sectile:<skill>` under a plugin
  install), and one notification on ❓, ✅ or ❌.
- **Given** a stage skill nested in a pickup, **then** it says the pickup owns
  all of the above.

### US4 - Know the limits (P2)

- **Given** another CLI, or an action the app refuses, **then** the guide says
  the skills use only what the host exposes, skip each missing or refused item
  and continue.
- **Given** a run launched by Sectile Desktop, **then** it says the links, the
  next step and the notification are left to Desktop, and points to the Desktop
  guide for Desktop's own indicators.

## Functional requirements

- **FR-001** The subsection sits at the end of **Use a prompt in Claude Code**.
- **FR-002** It describes behaviour present on `main` only, worded after the
  merged contract `internal/skills/fragments/contracts/session-title.md`.
- **FR-003** The status emojis are in a table with the meaning and the moment
  each appears; the text reads completely without the images.
- **FR-004** The plugin command form links to the README's
  `#install-sectile-in-your-coding-cli` section, and Sectile Desktop's
  indicators link to the Desktop guide.
- **FR-005** Each capture is a PNG under `docs/images/user-guide/`, referenced
  by relative path with descriptive alt text, demo data only.

## Open points

- **Captures.** Not supplied yet (owner, 2026-10-06): the text ships alone and
  the images are reported as pending. Blocks only FR-005.
