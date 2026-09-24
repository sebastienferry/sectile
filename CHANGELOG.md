# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the release note every Sectile surface shows: the web interface
serves it from the running server, and the desktop app embeds it at build time.
It is written for the people who use Sectile, not for the people who wrote it -
one line per user-visible change, in English, and nothing about refactorings,
test fixtures or internal plumbing.

## [Unreleased]

### Added

- **Realign a macro's specification with its slicing.** *Réaligner la spec*, in the macro panel, runs the new `realign-macro` skill on your local agent, in the desktop app's Run (or type `/realign-macro <KEY>` in an agent session), and can be stopped from the same panel. It brings the specification back in line with a slicing edited by hand: lines typed by hand become stub entries, renamed lines rename their entry, and entries no line points to any more are marked *to be removed*. It never rewrites the body of an entry and never deletes one, for Spec Kit and OpenSpec alike, and it commits and pushes the macro branch only when it wrote something. (#426)

- **Each macro gets its own worktree.** A macro's specification is written in `.tasks/worktrees/<KEY>` of the specifications repository, on the macro's branch, started from the up-to-date default branch, so two macros specified at the same time no longer share untracked files. An existing worktree is reused with its uncommitted work; projects with worktrees off keep using the checkout. (#426)

- **Declare where a project's specifications live.** The project options (Compétences IA & SDD) gain *Dépôt des spécifications*, for teams that keep their specifications apart from their code; the slicing import reads it, and the code repository stays the agents' working directory. On a workstation, the desktop project dialog has the matching *Specifications repository* field for macro skills. (#426)

- **Manage Jira sprints from the timeline.** On a Jira project, *+ Sprints* creates a batch on the board (name pattern with `{n}`, count, start date, one to four weeks each); renaming, changing dates, closing and deleting are written to Jira and the timeline shows Jira's answer, so the next synchronisation keeps them. Closing can first move the unfinished tickets to the next sprint or to the backlog. On a GitHub project the timeline is read-only. (#426)

- **Choose the project a slicing line's story is created in.** Each line offers the macro's project and the other projects of the same tracker instance (one Jira site, one GitHub repository); the story lands there, still under the macro's epic. A target on another tracker or site is refused by name, and nothing is created. (#426)

- **Attach stories from the other Jira projects your roadmap reads.** *Projets de roadmap*, in a Jira project's tracker options, lists other project keys whose stories attach to slicing lines on import. Sectile only reads them and never writes to those projects. (#426)

- **Zoom and density are in the status bar, and the zoom reaches further.** The bottom bar shows the current zoom and opens both settings where you are already looking, instead of four clicks away under Profile, Appearance. The ladder gains 80 %, 150 % and 175 %: stopping at 125 % left "it is too small" without an answer. The four levels you may already have chosen are unchanged, and a value written by another version snaps to the nearest step rather than being refused.

- **Group a macro's tickets by phase and by goal.** Two tabs in the macro panel, *Phases* and *Objectifs*, split the same tickets along two axes carried by prefixed labels: `phase:` says the order of the work, `goal:` says what you are trying to obtain, and a ticket can serve one without belonging to the other. Drag a ticket between groups to move it; only that axis's label changes. Naming a group labels nothing, so the group waits empty as a target and the label becomes real on the first ticket dropped into it. Names are normalised on the way to the tracker, spaces becoming hyphens as Jira requires, and the resulting label is shown before it is applied.

- **Take the existing stories back into a macro's slicing.** *Reprendre les stories*, next to the other import buttons, writes one todo line per ticket already created under the macro, each arriving attached to its own. It is the reverse of *Créer story*: a macro whose tickets were created elsewhere had an empty slicing although the work was already sliced. Running it again adds nothing and says the slicing is up to date. A line that carries a story now also links straight to it on the tracker, and the list of a macro's tickets reads as one row per ticket, with its title, type, sprint and assignee, like the phase and goal groups.

- **Import a macro's slicing from the repository's specification.** The macro panel, under Framing, offers *tasks.md* and *spec.md*: the first reads the group headings of the tasks file, one group being one story, the second the requirements or the prioritised user stories. Lines already there are kept, matched on their text rather than their position, so a ticked line keeps its tick and its story even when a group is inserted above it, and a line typed by hand survives. The two sources add up rather than replace each other. Nothing is written to the repository or the tracker, and no story is created: producing the slicing is a gesture you ask for, never a side effect of the synchronisation. When the specification is not merged yet, it is read from the macro's own branch, and the report says which file or branch it came from. A refusal names its cause: no repository configured, no specification folder for that key, or the chosen file missing next to the other one.

- The Backlog can be condensed to one row per ticket: the button left of the filters drops the description excerpt and reduces the macro to its key, on the title line. The two details that made a row taller go with it (the time spent in the current state, the creator below the assignee), and the macro's title stays in the tooltip. The board and the roadmap keep their own density, and the choice is remembered for the next visit.

- Projects can colour their cards per epic (project settings, General, "Couleur par épic"; off by default). A thin bar in the epic's colour, along the left edge, marks board cards, Backlog rows, sprint timeline items and Roadmap macros. The colour is derived from the epic key, so an epic looks the same in every view; tasks without an epic are unchanged.

- Saved board views: name a selection of several projects and labels, and
  reopen it from the sidebar's *Vues* section or a direct link. A ticket appears
  when it carries any of the view's labels, cards name their project, and the
  board filters are remembered per view (#387).

- Web and desktop PR indicators show the current GitHub or GitLab request as open, conflicting, merged, or closed without merge. State refresh uses grouped forge reads without synchronizing stories individually.

### Changed

- **A story created under a Jira epic now gets the epic as its parent on Jira**, not only on the Sectile board. If Jira refuses the parent, the story is kept and a warning says so. (#426)

- The priority field of the task detail, quick add and clone forms shows the same colour dot as the task's card, so the priority you pick reads the way the board will show it. (#434)

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
  longer matches - anything classified before the mirroring existed, or while
  the tracker was unreachable - and pushes them all in one click. Macros that
  can never carry a label, such as GitHub milestones and epics belonging to
  another project, are left out rather than reported as late for ever.
- **A condensed row for the roadmap.** **Condensed** in the roadmap toolbar
  reduces each macro to one line - its key, its title, its open/total count, its
  priority and a count of the tickets left to place - and puts NOW, NEXT and
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
  files one single synchronisation per project, bounded on the update date -
  Jira is asked for `updated >= -15m`, GitHub for issues changed `since` the
  previous pass - so the cost follows what actually changed rather than how
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

- **A saved view selects the same tickets on every server.** A view label with
  an accent, `Équipe`, matched its tickets or not depending on the locale the
  PostgreSQL database was created with. Labels are now compared the same way
  everywhere: upper and lower case are the same letter for A-Z, and any other
  character, an accented one included, has to be spelled as it is on the ticket.
  A view needing both `Équipe` and `équipe` lists the two labels.

- **Projects are listed again on a server upgraded from an earlier version.**
  The per-project view setting added a column to the schema in a place that only
  reaches a database created from scratch, so every existing deployment was left
  without it and answered an error to every project read: the project menu came
  up empty and the board showed nothing. The column is now added on start,
  whatever version the database comes from, and no setting is lost.

- **Live updates and cancellations reach every server sharing a database.** A
  board open on one server now shows a change made through another, and
  canceling a job stops it on the server that runs it. A job canceled while it
  ran, or before it started, keeps its canceled status instead of being
  overwritten by its own outcome, with one server as with several. (#405)

- **A local agent is reachable whichever server receives the request.** With
  several servers on one database, a stage transition, a launch or a workspace
  operation arriving on a server the agent is not connected to used to fail with
  "no local agent connected". The servers now forward the work to the one holding
  the agent, over an internal port (`SECTILE_INTERNAL_PORT`, 8092 by default), and
  the agent indicator lists the agents of every server. The indicator also
  refreshes as soon as an agent connects or disconnects. (#406)

- **Several servers sharing one database synchronise each project once.** The
  background synchronisation used to run in every server, so each project was
  read once per server per interval, and a tracker asking to slow down (rate
  limit) was only heard by the server it answered. The servers now share the
  loop's pacing: one of them claims a due project, a full read dated by any of
  them counts for all, and a rate limit pauses every server for ten minutes.
  The synchronisation status is the same whichever server answers. (#404)

- **A server that fails to answer no longer looks like an empty deployment.**
  Reading the projects, the issues, the settings or the saved board views used
  to be discarded in silence when the server refused: the sidebar and the board
  simply showed nothing, with no way to tell a broken deployment from an empty
  one. A failed read now raises a toast naming the resource and what the server
  answered, and while the projects or the issues are failing a banner stays on
  screen, with a button to try again. Being signed out stays quiet, since it
  already sends you to the sign-in screen.

- **Starting a second server on PostgreSQL no longer interrupts the first one's
  work.** A server used to mark every running job as failed and every client run
  as canceled when it started, including the work of another server sharing the
  same PostgreSQL database, which a rolling deploy does for a few seconds. Each
  server now only reclaims the work of servers that stopped answering for 45
  seconds. A single SQLite server still reclaims everything at start, as before.
  (#403)

- **A coordination project can record a pull request from another repository.**
  When a project has no code remote, or is not mono-repo, a stage transition
  whose `prUrl` points to another GitHub or GitLab repository is now checked
  against that repository. Its head commit is confirmed on a local checkout of
  that repository when the task or project knows one; otherwise the stage report
  says it was not verified locally. A project checkout without an `origin`
  remote is now named as such instead of failing with a Git exit status. (#392)

- **Turning off a project's custom agent now restores inheritance.** The project
  modal explicitly clears its provider and model override, rather than omitting
  them and leaving the previous provider stored. Reopening the project now keeps
  the global agent active. (#249)

- **A task worktree can build and lint from its first launch.** The local agent
  now runs `npm ci` in every package folder of a task worktree (its root and the
  folders one level down that hold a `package.json` and a `package-lock.json`)
  before the session starts. It installs again only when a manifest or lockfile
  changes. The main checkout is never linked to or touched, `.env` files are not
  shared, and a failed install is logged without blocking the launch. Switching
  a task's branch from the board no longer leaves the agent stuck on its own
  lock, and answers without waiting for the install, which runs in the
  background.
- Renaming your account preserves unsaved appearance and skill prompt changes in
  the open profile dialog. Closing and reopening the dialog reloads saved values.

- **The sidebar shows the name you set, not `Developer`.** The account button at
  the foot of the sidebar and the button in the status bar displayed a name that
  no screen could edit any more, so they stayed on the seeded `Developer`
  whatever you typed in Settings → Account. They now show the name your account
  carries - the same one your executions and your comments are signed with -
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
  GitLab project - whatever its issue tracker - was refused with
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
- **The suggested Jira board can be confirmed from the board picker.** On a
  project with no board recorded yet, the picker in the project settings now
  starts on "Choisir un board…" and marks the default board as "(suggéré)".
  Picking it records it and imports its columns, as picking any other board
  does, instead of waiting for the next synchronisation. (#375)
- **A locked personal GitHub token stops the call instead of borrowing the
  server's.** When somebody sealed their GitHub token behind a passphrase and
  had not unlocked it, Sectile quietly used the project or server token instead:
  the background synchronisation of a project they own read as the service
  account while its activity named them, and their own writes went out under an
  account they did not choose. Such a call made through the GitHub tracker
  adapter now fails and says the credential is locked, as Jira already did. The
  branch pull request lookup and the GitHub GraphQL reads, which resolve their
  credential through `trackerAs`, still fall back and are out of scope of this
  change. Somebody who stored no GitHub token at all still uses the project or
  server token.
- **Clicking beside a dialog closes it, as `Escape` does.** Ten dialogs - the
  quick add, the clone, the command palette, the task sheet and its expanded
  specification reader, the three roadmap dialogs, the sprint closing and the
  tracker setup - rendered a dark backdrop that reacted to nothing, and five of
  them had no `Escape` either, so the × was the only way out. All of them now
  close on a click beside them and on `Escape`, through the same path as their
  close button: the task sheet still saves what was edited. A dialog opened over
  another closes alone, and a selection begun inside a dialog and released on
  the backdrop no longer closes it - which the dialogs that already dismissed
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
  day, and a card's real history - a run, a transition, a comment - was buried
  under them. A background read that finds the ticket unchanged now leaves
  nothing behind. A read that finds a change still records it, and a read the
  tracker refuses is still kept: that one is the reason these entries are
  looked at.
- **A closed ticket no longer fills its own activity list.** The background
  synchronisation re-read every work item that was not marked finished in
  Sectile, and left one activity on the ticket each time. A ticket the tracker
  had closed - Jira's *Closed*, *Resolved*, *Won't Do*, GitHub's *closed* -
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
  named otherwise - the Blocker/Critical/Major/Minor/Trivial set, a renamed or
  translated one - refused every creation and every update with "The priority
  selected is invalid". Sectile now asks the screen that will receive the write
  which priorities it takes, and sends one of those. A project whose creation
  screen has no priority field at all is created without one - instead of being
  refused - and the level is set straight afterwards, so it is not lost.

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

### Fixed

- **Quiet agent runs are no longer canceled.** A run that stays silent for a
  long time - a long build, a question waiting for its owner - used to be
  canceled after fifteen minutes, which stopped an autonomous chain without a
  word. It now stays running, and its summary notes how long it has been quiet.
  A run canceled because its client disconnected can still be finished by the
  agent that owns it, so the chain carries on. (#315)

[Unreleased]: https://github.com/sebastienferry/sectile/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sebastienferry/sectile/releases/tag/v0.1.0
