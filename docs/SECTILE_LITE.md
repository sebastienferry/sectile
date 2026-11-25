# Sectile Lite: setup and usage guide

Sectile Lite is a Claude Code plugin that runs the Sectile workflow on Jira
tickets from Claude alone, with no Sectile server, agent or database. Each
ticket goes through the same stages as in Sectile: clarify, specify,
implement, review and hand over. Jira stays the source of truth. The stage is
recorded as a label on the ticket and the stage report as a Jira comment.

This guide is for whoever installs and uses the plugin. Why it works this way
is in [ADR 0064](adrs/0064-sectile-lite-runs-the-skills-on-jira-without-a-server.md).

- [What you need](#what-you-need)
- [Install](#install)
- [First run: `/sectile-lite:setup`](#first-run-sectile-litesetup)
- [The skills](#the-skills)
- [The ticket workspace](#the-ticket-workspace)
- [Stages and labels](#stages-and-labels)
- [A typical day](#a-typical-day)
- [Configuration files](#configuration-files)
- [Update](#update)
- [Troubleshooting](#troubleshooting)
- [Publish a new version (maintainers)](#publish-a-new-version-maintainers)

## What you need

| Need | Why | Check |
| --- | --- | --- |
| Claude Code, ideally in the Claude desktop app (Code tab) | Runs the skills. The desktop app can also start one session per ticket and publish the board. | `claude --version` |
| The **Atlassian connector** added on claude.ai (Settings, Connectors) | Every Jira read and write goes through it, with your own Jira access. | `/sectile-lite:setup` |
| `git` 2.5 or later, with `user.name` and `user.email` set | Branches, worktrees and commits. | `git --version` |
| `gh` logged in, for repositories on GitHub | Pull requests. | `gh auth status` |
| `glab` logged in, for repositories on GitLab | Merge requests. | `glab auth status --hostname <host>` |
| A local clone of each repository your tickets change | The ticket sessions work in a worktree of it. | |
| A ticket workspace, and `uv` (Spec Kit) or `npm` (OpenSpec) | Each ticket's specification and notes, in a Spec-Driven Design framework. `setup` creates the workspace and installs the framework. | `/sectile-lite:setup` |

The plugin itself declares no MCP server and asks for nothing at install time.

## Install

Sectile Lite is distributed as a Claude Code marketplace: a Git repository that
holds the generated plugin. Add the marketplace your team publishes, then
install the plugin from it:

```bash
claude plugin marketplace add <marketplace git URL>
claude plugin install sectile-lite@sectile-lite
```

The marketplace URL is either an SSH address (`git@host:group/sectile-lite.git`,
for a private repository you reach with your SSH key) or an HTTPS one.

Without a published marketplace, build one from this repository and add the
folder instead:

```bash
make plugin VERSION=0.1.0 OUT=/tmp/sectile-lite ARGS="-lite -marketplace"
claude plugin marketplace add /tmp/sectile-lite
claude plugin install sectile-lite@sectile-lite
```

Start a new Claude Code session afterwards: plugins are loaded when a session
starts. The skills are then available as `/sectile-lite:<skill>`.

## First run: `/sectile-lite:setup`

Run `/sectile-lite:setup` once on each workstation. It:

1. checks the Atlassian connector: the sites it reaches, your Jira identity,
   one search, and the write tools the ticket skills use;
2. asks once for your team: the Jira site, the team's name, its Jira projects
   and its ticket query. It runs the query, shows what it returns, and writes
   `~/.config/sectile-lite/config.json` only after you say yes;
3. sets up the [ticket workspace](#the-ticket-workspace): asks for its folder
   (`~/sectile-workspace` by default) and whether to sync it with your team
   through a Git remote, then creates or clones it after you say yes;
4. detects its Spec-Driven Design framework (Spec Kit or OpenSpec) and, when
   there is none, proposes one and installs it after you say yes, showing each
   command first;
5. checks the host: whether it can start sessions (needed by `dispatch` and
   `daily`) and publish pages (needed by `board`);
6. checks `git`, and `gh` or `glab` for the forges your repositories live on,
   with one read-only call on a real repository;
7. checks the plugin's local files, `daily.json` and `repositories.json`;
8. ends with one table: ✓ passed, ✗ failed or not needed, and the command
   that fixes each failure.

Apart from the workspace and its framework, which it sets up only after you
say yes, it only reads: it logs in to nothing, never prints a token, and
writes nothing to Jira.

| Command | Does |
| --- | --- |
| `/sectile-lite:setup` | The full run: checks, plus the team question when `config.json` is missing. |
| `/sectile-lite:setup check` | The diagnosis only: asks nothing and writes nothing. Run it when a skill fails. |
| `/sectile-lite:setup team` | Changes the team only. |
| `/sectile-lite:setup workspace` | Changes the ticket workspace only. |

## The skills

### Ticket skills

Each one takes a Jira key, reads the ticket through the connector, does its
stage, posts its report as a Jira comment headed `Sectile · <stage>`, and moves
the stage label on.

| Skill | From → to | Does |
| --- | --- | --- |
| `/sectile-lite:clarify-issue PE-123` | `#new` → `#clarified` | Analyses the ticket against the code and asks you the open questions, in rounds, until you confirm the scope. |
| `/sectile-lite:specify-issue PE-123` | `#clarified` → `#specified` | Writes the executable specification in the repository's Spec-Driven Design framework, before any code. |
| `/sectile-lite:implement-issue PE-123` | `#specified` → `#implemented` | Implements the specification and proves it with the project's build, linters and tests, then opens the pull request. |
| `/sectile-lite:adjust-issue PE-123` | `#implemented` → `#reviewed` | Reviews the branch as a peer would, fixes what the review finds, and updates the pull request. Never merges. |
| `/sectile-lite:handoff-issue PE-123` | `#reviewed` → `#finished` | Confirms the merge, writes the handover and the acceptance checklist, and cleans the local workspace. |
| `/sectile-lite:pickup-issue PE-123` | `#new` → `#reviewed` | Runs every stage on its own, up to the pull request. |
| `/sectile-lite:pickup-issues PE-1 PE-2` | `#new` → `#reviewed` | Same for a batch, in one worktree and one combined pull request. |
| `/sectile-lite:create-pr` | none | Creates or reuses the pull request of the current branch, without moving the stage. |
| `/sectile-lite:rewrite-story PE-123` | none | Rewrites the ticket's description as a structured story. |

Lite never changes a ticket's Jira **status**: the status belongs to your
team's workflow. Move it yourself, or through your usual Jira automation.

### Workstation skills

| Skill | Does |
| --- | --- |
| `/sectile-lite:setup` | Remembers your team and checks the tools (see above). |
| `/sectile-lite:board [JQL]` | Publishes your own **Sectile Board**, a private claude.ai page of your Jira tickets in the six stage columns, read live through your connector. Run it again after a plugin update to update the same page. Dragging a card, or its "Move to" menu, rewrites the ticket's stage label. Its buttons hand the next step to a session running `dispatch`. |
| `/sectile-lite:dispatch PE-123 PE-124` | Opens one Claude Code session per ticket, each in its own worktree of the ticket's repository, starting with the command of the ticket's stage. At most three run at once; the rest wait in a queue. It only relays what the sessions reach and never does the ticket work itself. Add a skill after a key to force it (`PE-123 specify-issue`), or `auto` to run `pickup-issue`. |
| `/sectile-lite:daily` | The morning brief, then the day's master session (see below). |

### The daily brief

`/sectile-lite:daily`:

- asks once which tickets to read (JQL queries, a Jira filter, a board), the
  triage pool and a few limits, offering your team from `config.json` by
  default, and saves the answers in `daily.json` after you say yes;
- writes the brief: **Needs me** (mentions, questions, decisions and reviews
  waiting on you), **Triage**, **In flight** (with stale tickets flagged), and
  **Moved since the last brief**;
- triages the new tickets of the pool: into the open sprint, up in priority,
  or left in the backlog, three proposals at most, each with its reason. After
  one approval for the whole batch, it applies them in Jira, reads them back,
  and comments the reason (`Daily triage: <reason>`);
- ranks the **Next** tickets to run today, each with its command: what blocks
  someone or is due first, then what is already in flight, then priority;
- asks which ones to start, hands them to `dispatch`, and stays open for the
  day, relaying each ticket session and starting the next queued one.

| Command | Does |
| --- | --- |
| `/sectile-lite:daily` | The brief, the triage and the day's dispatcher. |
| `/sectile-lite:daily configure` | Changes the sources and the triage pool. |
| `/sectile-lite:daily dry-run` | The brief only: applies no triage, starts nothing and does not record the run. |
| `/sectile-lite:daily PE-123 PE-124` | The brief, then starts these tickets right away. |

In the master session, say "brief", "refresh" or "what's next" to get the
brief again with only what changed since the morning.

## The ticket workspace

Each ticket gets a clarification report, a specification and working notes.
They are written in the **ticket workspace**, a folder of your workstation,
rather than in the code repository: the code branch then carries code only,
and the specifications of every ticket sit in one place.

```text
<workspace>/
  docs/clarifications/PE-123.md      the clarification rounds and decisions
  specs/PE-123-short-title/          the Spec Kit specification (spec, plan, tasks)
  openspec/changes/PE-123-.../       or the OpenSpec change, with OpenSpec
  notes/PE-123.md                    the working notes
```

- **Review in Claude.** Each time a stage writes or changes one of these files,
  the session opens it for you as Markdown, in the Claude desktop app's file
  pane, so you review the clarification or the specification where you answer
  the session. When it waits for your confirmation, it asks once the file is
  open.
- **Notes.** `notes/<KEY>.md` is yours as much as the skills'. Write context,
  constraints or decisions there before a stage starts: every stage reads it
  first. Each stage then appends a dated section with what it decided, what is
  still open and its links. Nothing already there is rewritten.
- **Git by default.** The workspace is a Git repository, so every stage
  commits the ticket's files with its message.
- **Synced or local.** `setup` asks whether to sync it with your team:
  - **synced**: the workspace is a clone of a shared repository. Each ticket
    works in its own worktree of it, `<workspace>/.tasks/worktrees/<KEY>`, on a
    branch named like the ticket's code branch, pushed after each stage. You
    open the merge request of that branch to have the specification reviewed
    by the team; `handoff-issue` removes the worktree once it is merged;
  - **local**: the files are committed in the workspace itself, never pushed.
- **Framework.** The workspace holds the Spec-Driven Design framework the
  specification follows, Spec Kit (`.specify/`) or OpenSpec (`openspec/`),
  recorded in `config.json`.
- Without a workspace, the skills write in the ticket's code worktree, as
  before, and say so.

## Stages and labels

A ticket's stage is one Jira label: `#new`, `#clarified`, `#specified`,
`#implemented`, `#reviewed` or `#finished`. A ticket without one is `#new`.
Setting a stage replaces the previous stage label and leaves every other label
alone. These are Sectile's own labels, so Sectile, Sectile Lite and the board
read each other's tickets.

Each stage posts one Jira comment headed `Sectile · <stage>`: the
clarification, the specification, the implementation report, the review, the
handover. Pull requests are linked to the ticket as remote links.

## A typical day

1. Open a session in the Claude desktop app's Code tab and run
   `/sectile-lite:daily`.
2. Read the brief, approve or edit the triage, and pick the tickets to start.
3. Each ticket opens in its own session and worktree, titled
   `<KEY> · <skill>`. Answer its questions there.
4. When a session ends its stage, the daily session relays it and starts the
   next queued ticket. Start the next stage of a ticket from the daily session,
   or from the board's button.
5. Keep the Sectile Board open beside it to see every ticket's stage and to
   move cards by hand.

## Configuration files

All of them live in `~/.config/sectile-lite/` and belong to the workstation.
No skill writes them without asking, except the timestamps noted below.

### `config.json`: your team

Written by `setup`. The board and the daily brief take their defaults from
its `team`; the ticket skills write in its `workspace`.

```json
{
  "site": "example.atlassian.net",
  "team": {
    "name": "Payments",
    "projects": ["ABC"],
    "jql": "project in (ABC) AND statusCategory != Done ORDER BY Rank ASC"
  },
  "workspace": {
    "path": "/Users/me/sectile-workspace",
    "sync": true,
    "framework": "speckit"
  },
  "checkedAt": "2026-11-25T09:00:00+01:00"
}
```

`checkedAt` is updated without asking when a full check passes.

### `daily.json`: the brief's sources

Written by `daily`.

```json
{
  "site": "example.atlassian.net",
  "sources": [
    { "name": "Mine", "jql": "assignee = currentUser() AND statusCategory != Done ORDER BY Rank ASC" }
  ],
  "triage": {
    "jql": "project in (ABC) AND statusCategory != Done AND (sprint is EMPTY OR sprint not in openSprints())",
    "sprintField": "customfield_10020"
  },
  "limit": 3,
  "next": 5,
  "staleDays": 5,
  "lastRunAt": "2026-11-24T08:30:00+01:00"
}
```

`lastRunAt` and `triage.sprintField` are updated without asking.

### `repositories.json`: where each ticket is worked

Written by `dispatch`, when you agree to remember an answer.

```json
{
  "repositories": [
    { "project": "ABC", "component": "Web", "path": "/Users/me/Sources/abc-web", "remote": "github.com/acme/abc-web" },
    { "project": "ABC", "path": "/Users/me/Sources/abc-core" }
  ]
}
```

A rule matches a Jira project, narrowed by a `component` or a `label`; the
most specific rule wins. `remote` lets the dispatcher recognise the
repository's pull requests. When no rule matches, the dispatcher asks once and
offers to save the answer.

## Update

```bash
claude plugin marketplace update sectile-lite
claude plugin update sectile-lite@sectile-lite
```

Then start a new session. Run `/sectile-lite:board` once more to bring your
board page to the new version.

## Troubleshooting

Start with `/sectile-lite:setup check`: its table names what fails and the
command that fixes it.

| Symptom | Cause and fix |
| --- | --- |
| `/sectile-lite:...` is not recognised | The plugin is not loaded in this session: check `claude plugin list`, then start a new session. |
| "No Atlassian connector" | Add the Atlassian connector on claude.ai (Settings, Connectors), then start a new session. |
| A Jira write failed | The skill keeps its work and gives you the exact comment or label to apply by hand. Check your Jira permission on the project. |
| `dispatch` gives commands to paste instead of starting sessions | The host cannot start sessions (a plain terminal, for example). Use the Claude desktop app's Code tab, or paste each command into its own session. |
| `dispatch` asks for a folder each time | No rule matches the ticket: say yes when it offers to remember the answer in `repositories.json`. |
| The pull request cannot be created | `gh auth login`, or `glab auth login --hostname <host>`. |
| The board shows nothing | Its first opening asks you to allow the connector. Check its query in the page's query field. |

## Publish a new version (maintainers)

The marketplace repository holds generated files only; never edit them by
hand. From this repository:

```bash
make plugin VERSION=X.Y.Z OUT=/tmp/sectile-lite-gen ARGS="-lite -marketplace -force"
```

Copy the generated tree into a clone of the marketplace repository, keeping its
`.git`, its `README.md` and its `docs/` folder, refresh the copy of this guide
there, then commit and push:

```bash
rsync -a --delete --exclude .git --exclude README.md --exclude docs /tmp/sectile-lite-gen/ <marketplace clone>/
cp docs/SECTILE_LITE.md <marketplace clone>/docs/SECTILE_LITE.md
git -C <marketplace clone> add -A
git -C <marketplace clone> commit -m "chore(release): sectile-lite X.Y.Z"
git -C <marketplace clone> push
```

Bump the version every time: `claude plugin update` only picks up a version
it has not installed yet.
