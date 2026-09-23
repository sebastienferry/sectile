# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the release note every Sectile surface shows: the web interface
serves it from the running server, and the desktop app embeds it at build time.
It is written for the people who use Sectile, not for the people who wrote it —
one line per user-visible change, in English, and nothing about refactorings,
test fixtures or internal plumbing.

## [Unreleased]

### Changed

- **Triage, Roadmap and Timeline are now hidden by default and enabled per
  project.** Project settings, under General, carry a "Vues de l'espace de
  travail" section where each project turns on the planning views it actually
  uses. A view that is off appears neither in the sidebar nor in the command
  palette, and switching to a project that does not use the view you are on
  returns you to the board. Existing projects start with all three off.

- Issue details show description and technical context directly below the title, alongside metadata, with pull requests below; narrow views keep the content first.

- Removed the permanent instructional hint below the desktop project list.

- Consolidated MCP setup into one per-engine configuration with three choices:
  remote HTTP (default), local HTTP proxy, and STDIO. The engine selectors
  offer Antigravity, Claude and Codex using the existing compact controls. API-key creation now lives in the same web
  view, and desktop shows only the selected connection configuration.

### Added

- **The Triage view is back.** It lists the work items that are missing a
  sprint, a macro, a team or an assignee, groups them by what they lack, and
  lets you fix several at once. It is off by default; enable it per project in
  the project settings.

- Initialize a selected AI provider from desktop project Deployment, installing current server skills and configuring MCP with separate results and repeatable setup. (#389)

- **Launch a batch from the Backlog.** The compact "Batch" action ("Lot" in
  French) in selection bars can launch selected
  `new` or `clarified` tasks from one project. A shared preparation dialog lets
  you reorder the tickets and name the dedicated worktree before launching.
  Cancellation preserves the selection, and a refused launch keeps the chosen
  order and worktree name available for retry.

- **Select several stories on the board and launch them as one batch.** Cards
  still at the `new` or `clarified` stage show a checkbox on hover, and
  Ctrl/Cmd+click toggles them. A bar then launches `/pickup-issues` on the
  selection in board order, as the Curation, Triage and Sprint Timeline views
  already could.

- **MCP connection settings per AI engine.** Web and desktop settings explain
  Streamable HTTP and STDIO with copyable provider configurations. Desktop can
  update local provider files for remote authenticated access or an explicitly
  enabled local proxy without client credentials, preserving other MCP servers.

- **Agents CLI workstation settings and local inheritance (#359).** The desktop
  app now includes an **Agents CLI** category in its settings dialog to
  configure workstation-wide CLI defaults (AI provider, AI model, and
  interactive/autonomous command templates), saved in `~/.config/sectile/settings.json`.
  Desktop project settings inherit from these workstation defaults, and CLI command
  templates are removed from the central Web UI. Autonomous execution preflight
  validation is delegated to the local agent daemon.

- **Ticket creator and author attribution.** Synchronisation with GitHub and
  Jira captures the original issue creator and avatar, and local task creation
  attributes the task to the authenticated user. Authorship is displayed in the
  task detail panel and list view.
- **A macro's roadmap horizon now reaches the tracker.** Classifying a macro as
  NOW, NEXT or LATER writes a `roadmap:now`, `roadmap:next` or `roadmap:later`
  label on its epic and removes the other three, so the classification is
  readable from a Jira filter, a board or a JQL query instead of living only in
  Sectile. Un-classifying a macro removes the axis. The write goes through the
  activity queue, so a refusal from the tracker is visible rather than silent.
  Until now the classification was kept locally and announced as pushed.
- **The roadmap says how many classifications have not reached the tracker.** A
  counter in the roadmap toolbar lists the macros whose label is missing or no
  longer matches — anything classified before the mirroring existed, or while
  the tracker was unreachable — and pushes them all in one click. Macros that
  can never carry a label, such as GitHub milestones and epics belonging to
  another project, are left out rather than reported as late for ever.
- **A condensed row for the roadmap.** **Condensed** in the roadmap toolbar
  reduces each macro to one line — its key, its title, its open/total count, its
  priority and a count of the tickets left to place — and puts NOW, NEXT and
  LATER on it as three two-letter buttons, so a macro moves from one horizon to
  another without unfolding anything. The choice is remembered per browser. The
  **Hidden** tab keeps the unfolded row: none of the three buttons applies
  there, and every macro would read as unclassified.
- **Roadmap labels are read back on every synchronisation.** The horizon lives
  on the epic, which the sync imports as a container and never as a card, so a
  project whose epics are all classified on the tracker used to open with an
  entirely unclassified roadmap and nothing on screen saying why. Each sync now
  reads the `roadmap:` labels, along with each epic's own title and whether it
  is closed, and reports it as a step of the synchronisation activity. A tracker
  that cannot be reached leaves a note rather than undoing the import.
  **Re-read labels** in the roadmap toolbar runs the same read on demand. A
  macro classified on the tracker wins; one carrying no label keeps the
  classification made here.

### Changed

- Desktop console cleanup uses an unboxed broom icon, and task toolbar icons no longer have button frames. Icon controls show visible tooltips on hover and keyboard focus, including disabled actions.

- Desktop settings use a larger dialog, open on User profile, and list Agent connection, AI Engine CLI, Agent logs, and Changelog in that order, with Changelog at the bottom of the sidebar. The User profile no longer shows the Credential row, and Agent connection shows a green or orange dot beside the server link status, plus Start, Stop, and Restart controls beside the local agent.

- **The background synchronisation asks the tracker what moved, instead of
  re-reading every ticket one by one.** A pass used to queue one read per
  unfinished work item, every few minutes: four hundred tickets meant four
  hundred requests and four hundred activity rows a pass, for ever. It now
  files one single synchronisation per project, bounded on the update date —
  Jira is asked for `updated >= -15m`, GitHub for issues changed `since` the
  previous pass — so the cost follows what actually changed rather than how
  large the project is. A full read still runs every half hour, which is what
  notices a ticket that left the project's perimeter, and a tracker that cannot
  narrow a search is simply read in full. A pass that finds nothing leaves no
  activity behind; one the tracker refuses is still recorded, with the account
  whose credential was refused.
- Refreshed the web interface with Graphite light and dark surfaces, quieter navigation, and softer board cards that respect the selected density and project accent.
- **The server refuses to start rather than run on a half-applied schema.** It
  now records which schema changes a database has received and applies the ones
  it lacks when it starts; a change that fails leaves the database exactly as it
  was and stops the server, instead of letting it answer requests against a
  schema it does not have. A deployment upgrading from an earlier version is
  brought up to date on its first start, whichever database engine it uses.
- **The desktop settings panel dropped its bottom bar.** Its only button,
  **Close settings**, repeated the cross in the corner on every category, and
  the same bar showed up under dialogs that had nothing to put on it. The cross
  and Escape still close the panel, and the bar now appears only where a dialog
  really has something to save, such as a project's local configuration.
- **The desktop discussion header now leads with the task and its state.** The
  title, the execution's state and the skill result share one line; below them
  the worktree path is a control you click to copy. Relaunch, log export and the
  Console/Changes switch became icons, and that switch moved into the toolbar,
  so the header takes two rows instead of four.
- **The skill indicator no longer repeats the execution state.** It reported a
  running execution a second time as `In progress` and a stopped console as
  `Console stopped`. It now speaks only when it has something the state does not
  say: the server's verdict on the skill, a stop that has not taken effect, or
  an execution that ended cleanly without the skill being recorded.
- **Jira's Blocker/Critical/Major/Minor/Trivial priorities are read one level
  lower.** On the sites running that scheme, `Major` is where most work items
  sit, so it now arrives as medium rather than high, and `Critical` as high
  rather than urgent; `Blocker` stays urgent and `Minor` and `Trivial` stay low.
  Boards importing from such a project will see their ordinary work items move
  off the high level on the next synchronisation.

### Fixed

- Renaming your account preserves unsaved appearance and skill prompt changes in
  the open profile dialog. Closing and reopening the dialog reloads saved values.

- **The sidebar shows the name you set, not `Developer`.** The account button at
  the foot of the sidebar and the button in the status bar displayed a name that
  no screen could edit any more, so they stayed on the seeded `Developer`
  whatever you typed in Settings → Account. They now show the name your account
  carries — the same one your executions and your comments are signed with —
  falling back to your address, then your account id, for an account that has
  never been named. Settings → Account remains the one place to change it: a name
  sent to `/api/settings` is accepted and ignored, as the address already was.
  (#348)

- **Publishing a new branch no longer fails on a force push.** The PR creation
  and adjustment skills (and the pickup skills built on them) told the agent to
  use `git push --force-with-lease` after a rebase without saying when, so it
  forced branches the remote did not have yet and the push failed. They now
  push a new branch with `git push -u`, a fast-forward with a plain push, and
  keep `--force-with-lease` for published history a rebase actually rewrote. A
  push refused because the remote moved is resynced and retried once; an
  unguarded `--force` is never used. Regenerate or re-install the skills to get
  the new wording.
- **Projects hosted on GitLab can reach `implemented` and `reviewed`.** The stage
  evidence check only ever asked GitHub for the branch's pull request, so a
  GitLab project — whatever its issue tracker — was refused with
  `configure an explicit GitHub owner/repository` even with an open merge
  request on the checkout commit. The forge is now chosen from the code remote,
  and a GitLab merge request is read by the local agent with the `glab` login
  the workstation already has, under the same rules as a GitHub pull request:
  open or merged, same branch, head on the checkout commit, ready where
  adjustment needs it. Refusals speak of a merge request, a failed lookup is
  reported as such rather than as a missing merge request, and several open
  merge requests on the branch are refused as ambiguous.
- Running execution icons now spin in the desktop sidebar and discussion header, while respecting reduced-motion preferences.

- Terminal-owned executions now stop their child processes and report their exit when the supervisor receives a hangup or termination signal, preventing stale running entries and stop timeouts. Transient exit-report failures are retried, and Stop automatically recovers a run whose local terminal has already disappeared.

- **Clicking beside a dialog closes it, as `Escape` does.** Ten dialogs — the
  quick add, the clone, the command palette, the task sheet and its expanded
  specification reader, the three roadmap dialogs, the sprint closing and the
  tracker setup — rendered a dark backdrop that reacted to nothing, and five of
  them had no `Escape` either, so the × was the only way out. All of them now
  close on a click beside them and on `Escape`, through the same path as their
  close button: the task sheet still saves what was edited. A dialog opened over
  another closes alone, and a selection begun inside a dialog and released on
  the backdrop no longer closes it — which the dialogs that already dismissed
  used to do, taking the form with them.
- **Cutting stories out of a macro no longer answers success without moving
  anything.** The move, the horizon push and the required-field lookup were
  served under `/epics/` only, while the interface asked for them under
  `/macros/`. The request fell through to the generic macro route, which read
  the action's own name as a macro key: a cut reported "queued" while it had
  created an empty macro called `move`, and the list of classifications to push
  answered with every macro of the project.
- **A ticket's activity list shows what happened to it, not how often it was
  read.** The background synchronisation left one entry on a ticket every time
  it re-read it, whether or not anything had moved. On a project of a few
  hundred tickets that is tens of thousands of "synchronised successfully" a
  day, and a card's real history — a run, a transition, a comment — was buried
  under them. A background read that finds the ticket unchanged now leaves
  nothing behind. A read that finds a change still records it, and a read the
  tracker refuses is still kept: that one is the reason these entries are
  looked at.
- **A closed ticket no longer fills its own activity list.** The background
  synchronisation re-read every work item that was not marked finished in
  Sectile, and left one activity on the ticket each time. A ticket the tracker
  had closed — Jira's *Closed*, *Resolved*, *Won't Do*, GitHub's *closed* —
  counted as unfinished for as long as its workflow label said otherwise, so it
  was read again every few minutes, for ever. Sectile now asks the project's own
  board which columns land on the finished stage, falls back on the status name
  when a project declares none, and comes back to those tickets once an hour so
  a reopened one is still noticed.
- **A PostgreSQL deployment no longer leaves runs stuck after a restart.**
  Marking interrupted work as failed, and cancelling the remote runs whose
  client session died with the server, only ever ran on SQLite. On PostgreSQL
  those activities stayed `In progress` for good, holding their task and
  stalling any chain they belonged to, with nothing able to close them. They are
  now closed on every start, on both engines.
- **A PostgreSQL deployment upgraded from an earlier version gets the columns it
  is missing.** On a PostgreSQL database created before accounts could be
  blocked, the roster refused to open and blocking or unblocking an account
  failed with `column "blocked_at" does not exist`; an administrator could also
  silently read as an ordinary member, losing the screens their role opens. A
  new column only ever reached a newly created database, because the schema is
  created once and never altered on that engine. Sectile now adds the columns an
  older PostgreSQL database lacks each time the server starts, so restarting it
  is all this takes. Installations on SQLite, which includes the desktop
  application, were never affected.
- **Jira priorities follow the project's own scheme.** Sectile used to write the
  four names of Atlassian's default scheme, so a project whose priorities are
  named otherwise — the Blocker/Critical/Major/Minor/Trivial set, a renamed or
  translated one — refused every creation and every update with "The priority
  selected is invalid". Sectile now asks the screen that will receive the write
  which priorities it takes, and sends one of those. A project whose creation
  screen has no priority field at all is created without one — instead of being
  refused — and the level is set straight afterwards, so it is not lost.

### Security

- **An agent, MCP client or machine API call now needs a real workstation key.**
  A server that had issued no key used to accept any nonempty token and treat
  its holder as the `default` account, which may be an administrator. That
  fallback is gone, and deleting the last account holding a key no longer
  reopens it. `SECTILE_SERVER_TOKEN`, still deprecated, remains the one way to
  keep an agent running while you pair a workstation for it.

## [0.1.0] - 2026-09-22

First tagged release. Sectile had been running from `main` since its first
commit; this entry names what that unversioned history had built, and the
release mechanism that will keep the following entries short.

### Added

- **Versioning and releases.** Every executable is built from a Git tag and
  reports it: `sectile-server --version`, `sectile-agent --version`, the
  `GET /api/version` route, the web interface footer, and the General section of
  the desktop app's settings. A build cut outside a tag reports `dev` rather
  than claiming a release.
- **Changelog in the product.** This file is served by the server at
  `GET /api/changelog` and shown in the web interface from the version in the
  footer; the desktop app embeds it at build time and shows it in its settings.
- **Multi-project board.** Projects backed by GitHub, Jira or a local tracker,
  with per-project board columns, sprints, teams, epics and the mapping from a
  tracker status to a Sectile workflow stage.
- **Workflow stages.** Clarify, specify, implement and adjust, each driven by a
  skill, with the pull request as the deliverable and the human as the merger.
- **Local agent.** A workstation daemon paired to the server once, which runs
  the coding CLIs, owns the Git worktrees and exposes the Sectile MCP server to
  the agents it launches.
- **Desktop app.** Execution consoles, the worktree diff, the task queue and the
  local agent's own controls, in an Electron shell around the same agent.
- **Sign-in and roles.** Mandatory sign-in through OpenID Connect or a local
  e-mail identity, an admin and a member role, and executions owned by whoever
  started them.
- **Personal tracker credentials.** Each account stores its own tracker tokens,
  encrypted at rest and optionally sealed behind a passphrase, so a write lands
  under the name of whoever asked for it.
- **PostgreSQL as an alternative store.** `DB_DRIVER=postgres` alongside the
  default SQLite file, with a one-shot migration tool.
- **Container image and workstation binaries.** A distroless server image, and
  `sectile-server` / `sectile-agent` cross-compiled for macOS, Linux and Windows.

[Unreleased]: https://github.com/sebastienferry/sectile/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sebastienferry/sectile/releases/tag/v0.1.0
