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

- **Set what a project's Claude Code sessions are allowed, from Desktop.** A new **Sandbox** category in a project's settings sets Claude Code's sandbox (inherited, on or off), the network domains it may reach, the extra paths it may write, and the allow and deny rules of its permissions. The values stay on the workstation, add to Claude Code's own settings, and apply to every Claude Code launch of the project, conversations, terminal sessions and headless runs alike; a custom command template and other engines receive nothing, and the command preview shows the settings a launch adds. On Windows only the rules apply. A headless run now names in its activity each tool call Claude Code refused, and where to allow it. Upgrade the agent along with the desktop. (#700)
- **Cmd+Enter (Ctrl+Enter) validates a Desktop dialog.** It submits the form of the open dialog or settings page, such as **Create task**; dialogs of plain choices or confirmations are left alone.
- **Cmd+N adds a task in Desktop, and the command palette finds anything.** Cmd+N (Ctrl+N) opens a reworked **New task** dialog on the selected or last used project, with Cmd+Enter to create and, once created, **Clarify now**, **Launch task** or **Add another**. Cmd+K now searches actions, projects and the sidebar's executions in one keyboard-driven list, each action showing its shortcut.
- **A task's interactive launches open as a Claude conversation.** With **Settings → Appearance → Claude consoles** set to **Conversation**, a skill such as clarify or implement launched interactively from Desktop, and **Discussion (no skill)**, open in Desktop's conversation view instead of a terminal, in the task's worktree: a skill starts on its own command, a discussion waits for your first message, and **Stop** ends it. A tool call your Claude Code rules do not allow waits in its card for **Allow**, **Always allow** or **Deny**, and a question Claude asks you is answered in its card, picking options or writing your own, as in Claude Code. While one waits, or while a skill waits for your answer in the conversation, the conversation is marked waiting in the sidebar and Desktop notifies you, as for a terminal. While Claude answers, a message you send joins the answer in progress, and **Stop answer** (or Esc) interrupts the answer and keeps the conversation open, and **Terminal** opens a plain terminal window on the conversation's directory. Autonomous launches keep their read-only trace. A Claude engine with a launch template converses with its model, the template's other options left aside; another engine keeps the terminal. Upgrade the agent along with the desktop.
- **Hide a project from the Desktop sidebar.** **Hide from sidebar** in a project's `…` menu keeps its configuration and executions; the project stays in Settings, marked hidden, with **Show in sidebar** to bring it back, and **Add project** offers it again. Settings now list only the projects added to the workstation.
- **Add a project from the Desktop command palette.** Cmd+K (Ctrl+K) now offers **Add project**, which opens the same dialog as the `+` next to PROJECTS, also when the sidebar is collapsed. (#698)
- **Desktop groups a project's tasks by stage.** Choose **Group by stage** in a project's `…` menu to list its tasks in the sidebar from new to finished; the choice is kept per project. Every task key in the sidebar is now tinted after its stage, with the stage named in its tooltip. (#688)
- **An epic's framing shows on its Jira epic.** The **Framing comment** of a Jira epic, edited in the roadmap panel, is now copied as one comment on the epic, rewritten a few seconds after each save and never read back, beside the todos comment. A line under the framing says whether the copy is up to date, waiting or failed, with **Publish again**; GitHub milestones, GitLab and local projects, local keys and roadmap projects' epics keep their framing in Sectile and say so. (#636)
- **Add a folder from a discussion.** In Sectile Desktop, an **Add folder…** action in the Claude conversation composer and in the toolbar of a running ticket discussion attaches a folder of this workstation to the project, as the project settings do, without leaving the discussion. A conversation gives it to Claude from the next message; a Claude Code ticket discussion has `/add-dir` typed into its session, so Claude sees the folder at once and keeps its context. Another engine sees it at its next launch. Upgrade the agent along with the desktop. (#676)
- **The MCP connection settings show the command that registers Sectile.** On the web and in Desktop, Claude Code and Codex get a ready-to-copy `claude mcp add` or `codex mcp add` command for the selected mode, with its own Copy button, next to the configuration snippet. Antigravity has no such command. (#667)
- **Markdown files render in the Desktop Changes panel.** A selected `.md` or `.markdown` file opens as a formatted document, as it reads at the inspected state, uncommitted edits included, and the old version of a deleted file. The **Rendered** toggle turns back to the raw diff for the Markdown files of that execution; another execution opens rendered again. Web and mail links open in the browser, other links stay inert, images are not loaded and raw HTML shows as text. (#575)
- **Reword, reorder and publish an epic's todos.** In the roadmap panel, click a todo to reword it, and drag it by its handle, use its up and down arrows or press Alt+Up and Alt+Down to put the list in the order it should be done. The list is then copied on the tracker, one way: as one comment on a Jira epic, as a block at the end of a GitHub milestone description, rewritten after each change and never read back. A line under the todos says whether the copy is up to date, waiting or failed, with **Publish again**; GitLab and local projects and roadmap projects' epics keep their list in Sectile and say so. Agents read and save the list through the new `get_macro` and `update_macro_todos` MCP tools, and `refine-macro` saves the todos you confirm. Upgrade the agent along with the server. (#663)
- **A Jira project names the label prefixes of its epic axes.** A new **Roadmap: epic label prefixes** section of the Tracker settings sets the prefix of the epic priority, quarter and readiness labels, such as `prio-` for `prio-p1`, so the roadmap reads and writes your team's own convention; left empty, each axis keeps `priority:`, `quarter:` or `readiness:`. Changing a prefix rewrites nothing: the epics show up in the pending labels to push, and a label under the former prefix becomes a free label. (#635)
- An experimental Claude conversation view in Desktop, turned on in **Settings → Appearance → Claude consoles**: Claude project prompts open in it instead of a terminal, and a **Claude chat (test)** action opens an independent conversation in the selected execution's directory. It shows messages and tool calls, Claude's replies rendered as Markdown as the Changes panel renders a Markdown file (tables, task lists, code; raw HTML stays text and images do not load), each tool call as a card (an edit as a diff, a written file as its lines, a command as the command, the todo list as a checklist, reads and searches as one line), with what the tool answered inside its card and a failed call marked and opened, and each reply streamed as Claude writes it, continues the same Claude session between messages, runs a message starting with `!` in the shell, Claude seeing the command and its output with your next message, completes slash commands as you type `/`, shows the output of local commands such as `/usage` as laid out, answers `/mcp` with the health of each MCP server, shows whether Sectile's own MCP server is reachable from the composer, with the commands Claude Code offers in that directory, lets you pick the model, the permission mode (Ask before edits, Accept edits, Plan mode) and the reasoning effort of each message, shows how full the context window is, and supports stopping the conversation. Edits and Sectile's tools are accepted; other tools ask for approval.
- **The roadmap shows the epics of a Jira project's roadmap projects.** The projects listed under **Roadmap projects** in a Jira project's tracker settings now bring their epics to the roadmap. A **Projects** menu in the roadmap toolbar picks which of them to show, with the number of epics of each; the roadmap opens on the project's own epics and remembers the choice per browser. An epic of another project is marked read only: its horizon, priority and quarter stay in Sectile and its labels cannot be edited. One project that cannot be read does not stop the others, and the read names it. (#632)
- **A slicing line can create its story in a roadmap project.** The line's target menu lists the project's roadmap projects: the story is created in that Jira project, under the epic, and stays in Jira without entering the board. (#632)
- **Opt in to writing the priority and the quarter on a roadmap project's epics.** A new option under **Roadmap projects**, off by default, writes the priority and the quarter set from an epic's panel on that epic in Jira, one epic at a time. Seeding from titles and pushing the pending labels never write there. (#632)
- **Epics have their own priority and quarter.** On the roadmap, an epic takes a P0 to P3 priority and a quarter (for example 2026-Q4), set from its panel. On Jira they are written on the epic as `priority:pN` and `quarter:yyyy-qn` labels and read back, a bare `2026-Q3` label included. On GitHub milestones, GitLab and local projects they stay in Sectile. The toolbar filters and sorts epics on the priority, and **Seed from titles** proposes the values already written in epic titles such as `2026.Q4 [P2]`, writing only the ones you keep. (#627)
- **From a ticket to its epic and back.** A ticket with a parent opens its epic on the roadmap, from the board card menu, the list row actions or the ticket detail, on the epic's tab with its panel shown, switching to the ticket's project when needed. From the epic's panel, **Its tickets** goes back to the ticket view you came from, filtered on that epic. (#630)
- **Group and drag epics on the roadmap.** The toolbar's **Group macros** splits the NOW, NEXT, LATER and Unclassified tabs into sections by priority (P0 to P3, then *No priority*) or by quarter (the quarters in use plus the current one and the next three, then *No quarter*). Sections fold, and the grouping and the folded sections are remembered. Dragging an epic onto a section sets that priority or quarter; Ctrl or Cmd click and Shift click select several epics to move together, epics already at that value are skipped, and one message names any that were refused. (#628)
- **Sectile installs in Claude as a plugin.** The `sectile` plugin carries the workflow skills and the MCP server declaration and asks for the server URL and API key at install; build it with `make plugin`. The agent's direct setup (`sectile-agent init`, desktop **Initialize**) remains the alternative, and the only route for other CLIs. (#267)
- Two workstation settings under Settings → Execution defaults: **Custom project skills win** (a skill a project edited runs instead of the installed one, on by default) and **Installed skills source** (direct copy or Claude plugin, tried first). (#267)
- When a project's custom skill ran, the desktop's settings button shows a warning dot and Execution defaults lists which skills ran; the run's activity says so too. (#267)
- A user guide walks through Sectile sign-in, Jira access, project and workstation setup, Claude Code prompts, autonomous runs, and Desktop use. (#567)
- Projects can optionally push clarification and specification commits after each stage commit; the setting is off by default. (#459)
- Projects can open the draft pull request as soon as a clarification is confirmed: choose "Draft after clarification" as the PR creation stage in the project options. Update Sectile Desktop on every workstation first; older versions refuse the setting. (#580)
- Projects can choose the name of the branch Sectile creates for a task, in the project settings: a template such as `{key}` gives `AUC-1234`, the upper-case key Jira and GitLab link branches by. Empty keeps today's `feat/<key>` names, and a branch a task already has is never renamed. (#621)
- **Desktop consoles survive an agent restart.** After restarting or updating the local agent, the sidebar lists the same executions (skill runs, autonomous runs, macro runs, discussions and free consoles), and each one replays, read-only, what its console showed. A run that was still going when the agent stopped comes back canceled. The 100 most recently finished runs are kept, privately, in `~/.taskflow/runs/`, until **Clear finished consoles**. (#588)
- **Session titles show where a skill run stands.** The workflow skills prefix the agent session title with a status emoji: ❓ waiting for you, ✅ done, ❌ blocked. Among idle sessions, the sidebar tells which ones need you and how each run ended.
- **Agent sessions link to their ticket and pull request.** A workflow skill writes a link to the ticket at the start of the conversation, and one to the pull request or merge request once it exists; in the Claude desktop app, a GitHub pull request is also bound to the session's PR bar.
- **Claude desktop sessions follow work like Sectile Desktop.** A workflow skill files its session under a sidebar group named after the project, marks a chapter per stage, opens the diff pane once it changed code, ends with the command of the next step, and sends a notification when it waits for you, succeeds or is blocked.
- **Connect Jira with one consent screen.** When an admin configured an Atlassian OAuth app (**Administration → Jira connection**, or `SECTILE_JIRA_OAUTH_CLIENT_ID`, `SECTILE_JIRA_OAUTH_CLIENT_SECRET` and `SECTILE_JIRA_OAUTH_REDIRECT_URL`), **Profile → Tracker credentials** offers **Connect Jira** instead of pasting an API token: your Jira writes keep going out under your own account, renewed in the background with no passphrase to unlock, on every Jira site you allowed. **Reconnect Jira** appears if Atlassian stops honouring the connection. API tokens keep working, behind **Use an API token instead**. (#654)
- **A missing tracker token can be added from the error.** When Sectile refuses a tracker write because you have no personal token for that tracker, whether at once (creating a ticket, commenting) or later in the queued activity (moving a card, changing a stage, an assignee, a sprint), the error notification offers **Add my GitHub token** (or Jira, GitLab) and opens your profile on that tracker's credentials. (#645)
- The status emoji follows the state the skill reports to Sectile, so the session title and Sectile Desktop always agree. A run launched from Sectile Desktop writes no ticket or pull request links, no next step and no notification of its own: Desktop already shows them.
- Every workflow skill ends its replies with the same **Done / Remaining (Agent) / Pending (User)** block, so you always know what happened, what the agent still has to do and what waits for you.

- **The roadmap reopens as you left it.** Coming back to the roadmap, or reloading it, brings back its tab, the macro you selected in that project, the expanded or hidden panel and the folded framing sections. A new button hides the details panel so the macro list takes the whole width, and a rail on the right edge brings it back; an expanded panel now takes the whole view, toolbar included. A macro's Description and Framing notes open in a full-screen editor with their preview beside them, and **Copy** puts the macro's link on the clipboard, or its key and title when the tracker gives it no page. **Link** now opens the macro itself rather than one of its tickets. (#629)
- Open desktop Configuration with Cmd+, on macOS or Ctrl+, on Windows/Linux, including from a terminal; an already-open settings page keeps its current category and unsaved edits. (#547)
- **Epics have a readiness a person decides.** On the roadmap, each epic shows its readiness, Idea, Shaping or Ready. Until somebody decides it, the badge shows Sectile's suggestion followed by "?", read from the epic's tickets, framing and slicing; three chips in the epic's panel decide it, and a second click clears it. On Jira it is written on the epic as a `readiness:idea`, `readiness:shaping` or `readiness:ready` label and read back; on GitHub milestones, GitLab and local projects it stays in Sectile. The badge that summed up the tickets' progress now reads "Tickets: Specified" and the like, so the two are not confused. (#633)
- **The roadmap shows an epic's labels.** On Jira projects, the labels an epic carries on the tracker appear as badges on its roadmap row, a **Labels** filter in the toolbar narrows the roadmap to the epics carrying one of the chosen labels, and the epic's panel adds or removes them on the tracker. The horizon, priority and quarter labels (`roadmap:`, `priority:`, `quarter:`, and a bare `2026-Q3`) stay managed by their own controls. (#626)
- **Create a slicing's stories in one go.** In a macro's panel on the roadmap, tick the slicing lines and click **Créer les stories**: each story lands in its line's target project, lines that already have a story are skipped, a failing line does not stop the others, and the panel reports what happened to each line with a summary. Every slicing line also shows where it came from: tasks.md, spec.md, an existing story, or typed by hand. (#634)

### Changed

- **Desktop's Changes panel picks the file from a drop-down.** The changed files of an execution are listed in one **Changed file** drop-down above the diff instead of a column of buttons, so the diff keeps the panel's whole width, also when the console is shown beside it. (#710)
- **Desktop's "Mark reviewed" is now "Skip to Handoff".** On an implemented task, the button beside **Next: Adjust** says what it does: the pull request needs no more changes, so Adjust is skipped, the task moves to reviewed and Handoff follows the merge. The tickets' row menu names it the same way.
- **Desktop installs an engine's skills and registers its MCP server separately.** In **Settings → Deployment**, **Install skills** and **Register MCP** replace the single global setup button; the MCP server registers over HTTP to the remote server or to the local agent, whichever you pick.
- **Desktop asks the agent and the server far less.** A task row's skill status is read again only when its run changes, or every 30 s while it runs, instead of every 2 s, each read costing the server two requests; an open conversation is no longer sent its whole history several times a second when nothing changed.
- **The Desktop conversation shows when Claude is working.** A dot pulses before the composer status while Claude Code works, and the status turns amber with a still dot when Claude waits for your answer or approval. The dot stays still when the system asks for reduced motion. (#699)
- **The Desktop sidebar is more compact.** Projects and their tasks take about a fifth less height, so more of them fit without scrolling.
- **Ticket discussions, project consoles and Claude conversations receive the project's folders.** Like a skill run, they are now given the project's other repositories, its specifications folder and its attached folders: Claude Code and Codex through `--add-dir` when they start, and a conversation again at each message, so a folder attached meanwhile is not missed. (#676)
- **Refining a macro keeps its description in step.** When a clarification answer settles an open question, reverses a decision or moves something in or out of scope, `refine-macro` now proposes the matching edit of the macro description alongside the tickets, including for answers given after the run ended, and reports what it changed or left.
- The roadmap no longer applies the ticket views' parent filter: every epic keeps all its tickets, and the filter still applies when you go back to the tickets. (#630)
- The roadmap priority is now the epic's own. It no longer shows the highest priority among the epic's tickets, so an epic without a priority reads "No priority" until one is set or seeded. (#627)
- **Board cards no longer show the task's description.** A card now shows its key, title, badges and links, on every density; open the task to read its description. The list view keeps its one-line excerpt. (#622)
- **A card past the full chain runs its next step instead.** On the web board, once a task has reached the stage where its project's full chain stops (`reviewed` by default, or `implemented`), the card no longer offers the full chain, which would have nothing left to do: a full card shows in its place a button that runs the next step (Adjust or Handoff) autonomously, and a condensed card's menu keeps only **Avancer en autonome**. (#637)
- **A board card's model is where you change it.** On the web board, clicking the model shown on a card opens the list of models its launches can use; the pick is the same one the card's `(...)` menu offers and shows. Full cards also gain a copy button that puts the prompt of the task's next step on the clipboard, ready to paste into Claude Code, Codex or AGY. (#612)
- **The implementation skill is now `implement-issue`**, like the other stages (`/implement-issue`, `/sectile:implement-issue` in the plugin). `code-issue` stays as an alias that runs it with the same arguments, and a workstation that has only `code-issue` installed keeps running the implementation stage until its next **Initialize** or plugin update. (#608)
- **Five parallel executions by default.** A project that uses worktrees now runs up to five executions at a time when neither its settings nor the workstation's Execution defaults set a number, on the desktop app and on a headless agent alike; this applies to existing workstations that never changed the setting. A number you set, 1 included, is kept: set 1 under Parallel executions to go back to one execution at a time. (#594)
- **The web interface loads faster.** The server now sends its pages, scripts, styles and API answers gzip-compressed to the browsers that accept it: the interface's script goes down from about 1.3 MB to about 330 KB on a first visit and after each update. Live board updates are unchanged. (#601)
- Large boards display cards progressively as each column scrolls, while keeping complete column counts and batch selections. Engine reports are loaded once per project for the board.

- **The agent no longer writes skills or MCP registrations on its own.** A task or macro dispatch, an agent start or reconnection and a skills-editor save leave `~/.claude`, `~/.claude.json`, `~/.agents`, `~/.codex` and `~/.gemini` untouched. A dispatch runs the skill it finds installed, or fails with a message saying how to install one; a project's edited skill is handed to its run in a private file, so two projects no longer overwrite each other's skills. Saved desktop MCP connections are rewritten at start only when the server address, key or executable changed. (#267)
- A skill command name may carry a plugin namespace, such as `sectile:clarify-issue`. (#267)
- **The web project picker shows recent projects and reaches every project.** With an empty search it lists the three projects last opened in this browser and your favorites. Search now also matches descriptions, repositories and trackers, ignoring accents, and the menu can be driven with the arrow keys, Enter and Esc. A new "Browse projects…" entry opens an overview of every project, filterable by text and tracker, where favorites can be toggled. (#582)

- **Adjustment no longer forces corrections on a custom skill.** Sectile still makes every adjustment verify the existing pull request, never create one or push onto a merged one, collect review feedback and never merge; reviewing, fixing, running checks and pushing are now left to the adjustment skill. The bundled skill keeps doing all of it, and a custom skill that forbids corrections is obeyed. (#561)

- Each clarification round now publishes its full report section on the ticket, retaining Markdown history and avoiding duplicate final-round comments. (#459)

- Desktop task headers now put the title and execution history above the worktree path and actions; Console and Changes can be shown together with an adjustable divider. (#574)

- Desktop execution and skill-result indicators now appear in the task’s bottom status bar; the redundant current-skill badge is hidden. (#560)

- AI engine profiles have their own desktop settings page, separate from execution defaults.

- Desktop configuration project sections can be collapsed; expanding one closes the others while global settings remain visible.
- Configuration pages use consistent compact title spacing to keep navigation visually stable.

- **Initialization provider and skill command names are workstation settings.** Configure them once under Settings → Execution defaults for all projects; Deployment can explicitly select an engine for global setup. Existing project command names stay active until workstation command settings are saved.

- **Desktop configuration opens as a full page.** Settings and a project's configuration now share one full-page Configuration view with a Back button. Its sidebar lists General workstation categories first, followed by the selected project's categories, while existing save actions and configuration controls remain available. (#545)

### Removed

- **Gemini CLI, Cursor CLI and Mistral Vibe CLI are no longer AI engines.** Sectile runs Antigravity, Claude Code, Codex or a custom command; the desktop settings, `sectile-agent init --provider` and the MCP setup no longer offer the other three. On its first start after the upgrade, the local agent removes a workstation's engines, model lists and MCP choices for them, keeping a backup of the settings file beside it; the projects and tasks that used such an engine run their default engine. The Cursor editor ("Open in editor") is unaffected. (#614)

### Fixed

- **"Always allow" in a Desktop conversation no longer loses its rule with the task.** The rule was written in the task worktree's `.claude/settings.local.json`, removed with the worktree or committed by mistake; it is now added to the project's **Sandbox** allow rules, so the next tasks of the project apply it too, and you can remove it there. (#700)
- **Desktop settings controls stay clickable next to the macOS scrollbar.** In a settings panel that scrolls, such as Execution defaults, the controls on the right, the reset buttons first, no longer sit under the overlay scrollbar macOS shows while scrolling. (#703)
- **A Desktop execution default changed right after Save is kept.** Settings → Execution defaults said "Execution defaults saved" before it had reloaded the saved values, and that reload then undid a setting changed or reset in the meantime. The notice now appears once the reload is done, and the reload leaves a setting changed in the meantime as you left it.
- **The pull request lookup finds git again on Windows.** The agent started `glab` and `gh` with a PATH that kept only Sectile's own tool directories, so both failed with `git` not found and a task stopped with "PR lookup failed". The commands it runs, and the task terminals it opens, now keep your PATH. Upgrade the agent.
- **A repository prepared for a task but left unchanged no longer blocks its stage.** When a task opened a worktree in another repository and committed nothing there, implemented (and every later stage) now goes through with the pull requests of the repositories that did change, and the stage report names the skipped repository as prepared, unchanged. A repository with commits still needs its pull request, and an answer the agent cannot give keeps it required. Upgrade the agent along with the server. (#678)
- **The header search no longer empties the Timeline.** A search typed in a ticket view used to leave the Timeline's sprints, counters and backlog with only the matching tickets; the Timeline now ignores it, as the Roadmap does, and the search applies again when you return to the board, the list or the triage. The backlog keeps its own search field. (#636)
- **A macro skill can no longer drop a todo linked to a story.** `update_macro_todos` refuses a list that leaves out a todo already linked to a story, and names it: only the macro's panel, where the story is visible, removes it. `prepare_macro_worktree` also always returns the macro's todos, an empty list when there are none. (#647)
- The local agent no longer crashes when a terminal is opened, or kept open, on a console that is printing: a viewer joining or a connection keepalive could write to the terminal at the same moment as its output and stop the agent, closing every console.
- **A clarification whose report stays out of the repository resumes where it stopped.** When a project keeps its specifications out of the repository, a clarification continued from another worktree or another workstation no longer starts over at round 1: it rebuilds the report from the rounds already published on the ticket and asks the next round's questions. Specification and adjustment also read the clarification and the specification from the ticket when the files are not in their worktree. (#487)
- A card moved on a GitHub project by someone without a personal GitHub token no longer reports success: its activity fails and says the token is missing, as a stage change already did. (#645)
- **Preparing a task worktree on Windows no longer opens a console window.** The dependency install that follows a worktree's creation, and the `git` calls made to read a macro's specification files, used to flash a console window (or keep one open for the whole install) when the agent was started by Sectile Desktop. (#638)
- **The server stays responsive while autonomous runs stream their output.** Recording each chunk of a headless run's output no longer reads the ticket's whole run history, and ticket answers (the web detail view, MCP `get_task`, live board updates) no longer carry every past run's output. With several runs at once, the server used to run out of memory and stop answering long enough for Sectile Desktop and the agents to time out. A ticket's run history is still available from its activities.
- While a ticket is being created from the quick-add dialog, its button now reads "Creating…" ("Création…" in French) instead of "Création CLI...", which named a CLI that is not involved and stayed in French in the English interface.
- **The direct setup no longer installs one project's skills for all of them.** `sectile-agent init` and desktop **Initialize** now install the same generic skills as the Claude plugin, which read the project's specification framework and pull-request policy when they run, so a Spec Kit project and an OpenSpec project on one workstation each follow their own steps. Run **Initialize** (or `sectile-agent init`) once after updating: until then, a direct copy installed earlier keeps the steps of the project it was set up for, and the skills editor marks it DIVERGED. (#267)
- A Claude plugin disabled in a project's `.claude/settings.json` or `.claude/settings.local.json` is no longer used for that project's runs: the launch falls back to the direct copy, or fails with the message that says how to install a skill. (#267)
- A custom skill whose launch fails is no longer reported as used by the settings button's dot and the run's activity. (#267)
- The web interface no longer logs a `409 Conflict` on `/api/cli-status` at every load. On a shared server, `GET /api/cli-status` and `POST /api/open-editor` now reach the signed-in person's workstation instead of answering that no local agent is connected.
- The board and list toolbars fit on one line again: Pinned, In progress and priority stay in the toolbar, and status, types, macro, sprint, team and person move into a **Filters** panel whose button shows how many of them are active.
- **My Tasks** no longer shows an empty board when a tracker writes your name without its accents, as Jira often does ("Sebastien FERRY" for "Sébastien Ferry"): your account's name and e-mail now match regardless of case and accents.
- Large boards share pending engine lookups across task cards, preventing duplicate requests from exhausting browser resources.
- **A task that changed no repository can reach implemented and reviewed.** A configuration made through an API, a review or a follow-up has no pull request to give, and the stage used to be refused for want of one. A workflow skill now states that the task changed no repository, and the stage report says that no pull request was expected; the statement is refused when the task records a pull request on its branch or a changed repository. (#584)
- **A macro skill that asks you a question shows it.** `report_waiting` used to refuse a macro run (`refine-macro`, `realign-macro`) as "not found or no longer running", so the macro never showed that it waited for you. It now accepts the macro the run belongs to, the macro's run button reads **Waiting for your answer**, and the refusal says whether the run does not exist, belongs to something else or has ended. (#648)
- **A run's question mark clears after a server restart.** When the server restarted while a run launched from Sectile was waiting for you, the run's next Sectile call now ends the wait on the board and in Desktop, as it did before the restart, instead of keeping the ❓ until Enter or the end of the run. Update the agent with the server. (#498)

- When `prepare_repository_worktree` cannot prepare a repository, the refusal now says why: it names the workstation that answered and each folder attached to the project with what it is (gone, not a Git checkout, without origin, another origin, or the Git error that kept it from being read), and only advises attaching the folder when that is what is missing. A `transition_stage` refused over a pull request of an unprepared repository says to call `prepare_repository_worktree` first. (#589)
- The board again shows only the tickets its Sprint, Team and Assignee filters select, and each project or saved view keeps its remembered filters when you switch to it, even after a reload. (#581)
- Agent reconnection now retries promptly after a dropped session, and a launch waits briefly for a reconnecting agent. Abnormal WebSocket losses no longer claim the server deliberately closed the connection. (#568)
- Newly generated Sectile tracker reports use English headings for clarification, specification, implementation, review, and closure. (#549)
- Desktop Tickets Pickup (full chain) now runs autonomously even when the project or pickup skill defaults to interactive execution. (#565)
- Answering a question in a run no longer clears a newer question recorded at the same time by another server replica. (#496)
- New task and macro worktrees use filesystem-safe directory names so Vite can load source modules; existing checkouts remain available at their original locations. (#557)
- Marking a task reviewed now explains which local checkout and uncommitted files block validation, and how to resolve them.

- Switching desktop configuration categories or projects no longer accumulates Refresh from server buttons.
- **MCP connections behind a hosting proxy.** Servers whose ingress forwards over loopback can now allow their public hostname with `SECTILE_MCP_ALLOWED_HOSTS`, so Codex and other MCP clients can initialize and load tools instead of receiving `403 invalid Host header`. Authentication and protection against unlisted loopback hosts remain enforced.

## [0.3.0] - 2026-09-26

### Added

- **Attach folders to a project in Sectile Desktop.** Project settings → General → *Attached folders* adds any other folder of your workstation to a project: another repository, a library, notes, a Git checkout or a plain folder. Every execution of the project is then told where they are and what each one is. An attached Git repository with a remote is changed through a worktree on the ticket's branch and needs its own pull request before the ticket can be marked implemented; a folder without a remote is changed in place. The folders stay on your workstation and are never sent to the server. Update the local agent together with the server: an older agent cannot prepare a worktree in an attached repository. (#484)

- **Codex sees the task's other folders.** Like Claude, Codex now receives the task's other folders (project repositories, attached folders and the specifications folder) as additional directories, through its `--add-dir` option. (#484)

- **Open a worktree in your editor from the desktop.** Choose VS Code, Cursor, Zed, Sublime Text or a custom command under Settings → Execution defaults → Editor, and a code icon next to the selected execution's path opens its worktree in that editor. With no editor chosen, the path shows alone as before. (#535)

- **Download Sectile Desktop from every release.** Each release now publishes a ready-to-run Sectile Desktop for macOS (Apple Silicon and Intel), Linux x86-64 and Windows, with the Sectile agent included, so you no longer need to build it from the repository. The archives are on the release's GitHub Release page and in the GitLab package, next to the agent and server binaries, with checksums; the desktop README's "Install a release" section explains how to open the unsigned app on each system. (#428)

- **Project prompt for every desktop AI engine.** The former Open agent console action now offers the workstation engine catalogue, including Codex and custom engines, starts with the project default, and preserves the selected engine’s model and interactive command when launching or relaunching.

- **Several AI engines, switched per task from the desktop.** Sectile Desktop keeps a catalogue of named engines on your workstation, each a full AI CLI profile (provider, model, per-skill models, interactive and headless commands), edited under **Settings → Execution defaults → Engines**, one of them the default. The ticket list shows each task's engine as an icon; clicking it hands the task to the next engine, and every launch of that task on this workstation uses it, whether started from the desktop, the web, a relaunch or a full chain. A one-off launch model applies only on the project's default engine. (#510)

- **Light mode for Sectile Desktop.** The desktop app now has a light appearance next to its dark one. A new **Appearance** category in the desktop settings offers System, Dark and Light; System, the default, follows your computer's appearance, so a desktop on a light system turns light after the update. The whole window follows the choice at once, the console included, without a restart. The web interface keeps its own theme. (#507)

- **Run the server as several replicas.** Several servers can now share one PostgreSQL database behind a load balancer, with no sticky sessions. A new readiness probe, `GET /api/ready`, tells the balancer when a replica can take traffic. A replica asked to stop drains first: it reports not ready, keeps serving for `SECTILE_SHUTDOWN_GRACE` (5 seconds by default), then hands its agents over to the other replicas. The README's "Several replicas" section lists what the deployment must provide. (#410)

- **Sealed tracker credentials unlock on every server.** When several servers share a PostgreSQL database, a credential unlocked through one of them can be used through all of them, including servers started afterwards, and locking it holds on all of them at once. The passphrase is still never stored. (#409, #501)

- **GitLab as a project tracker.** A project can now be put on GitLab, gitlab.com or a self-managed instance, by naming its GitLab project (`group/project`) and optionally its instance. Its issues are synchronised and written back like GitHub and Jira ones: the stage is a `#<stage>` label and a closed issue is finished, a macro is a pair of `macro:` / `parent:` labels, the team a `team::<name>` label, the board columns are the GitLab board's lists, and project milestones as well as Premium iterations are sprints you can move tickets into, create and edit. Comments, assignees and related merge requests follow too. A personal GitLab token (scope `api`) makes your writes appear under your own GitLab account. (#398)

- **Choose how Board and Backlog cards are sorted.** A selector in the Board and Backlog toolbars orders the cards by priority (the default), by epic, by key or by last update, with a button that flips the direction. Epic keeps the tickets of one epic together in each column, the epic holding the most urgent ticket first, and the tickets without an epic last. The choice is shared by both views and remembered by the browser. In the Backlog it replaces the "Priorité" button; clicking a column header of the flat table still sorts that table until the selector changes. (#402)

- **Keep specifications out of the repository.** A new option in the web project settings leaves the clarifications and specifications of the project's tasks in their worktree instead of committing them: Git ignores them through the checkout's local exclude file, so branches and pull requests carry code only, and the stage reports on the ticket carry what the files said. Each workstation can override the choice in the desktop project settings, which also warn when the repository already tracks specifications (they stay in its history). When the project opens its pull request at specification, it is opened after implementation instead. (#487)

- **Copy a ticket's key from its page.** In the web ticket detail, a button next to the ticket's key, and one next to its parent's key, copies that key to the clipboard exactly as shown (`#431`, `SFE-123`, `M-7`) and confirms it with a check mark and a toast, or says when the browser does not give access to the clipboard. The keys still open the tracker. (#431)

- **Cmd+B (Ctrl+B) toggles the sidebar.** In the web app and in the desktop app, Cmd+B on macOS and Ctrl+B on Windows and Linux collapse and expand the sidebar, as its button does; the button's tooltip and the web command palette show the shortcut. A terminal keeps Ctrl+B, and the Markdown editor keeps it for bold. The web app now also remembers a collapsed sidebar across reloads, as the desktop app already did. (#474)

- **Start a project on a folder that is not a Git repository yet.** The desktop project settings offer to initialize the Local repository or the Specifications folder with an empty first commit, so Sectile can create worktrees from it. The repository stays on your workstation and is never pushed; since nothing is committed, worktrees start without the folder's existing files. A Git folder with no commit yet is offered the first commit alone. (#481)

- **Multi-repo projects run each task in its own repository.** A project lists the other repositories its tickets work in, by remote, in its web settings; each workstation gives each repository its folder in the desktop project settings, and a folder is accepted only if its checkout is that repository. A ticket is pinned to one of them from its detail, and every stage then runs in a worktree of that repository. A ticket pinned to none runs in the code repository. The agent is told where every folder of the project is: the others are handed to Claude as context it is told not to change (nothing enforces it), and a skill that must change one asks for a worktree there on the same branch. Each changed repository then needs its own pull request, and the transitions check every one of them. Working directories typed on tickets before are converted to repositories once, by the first workstation that sees the project; the ones it cannot resolve are dropped and listed in the project settings. (#456)

- **An Administration page.** Admins now open a full page from the sidebar or the command palette instead of a dialog. It shows how many people are using the board right now, how many runs are running, queued or pending, and the account totals, and refreshes on its own. The users list says who is online and when each account was last seen, next to the role and the block and delete actions.

- **Prometheus metrics.** The server exposes `/metrics` next to the interface: requests, latency and errors per controller, active users, active runs by status, and the build version.

- **A Grafana dashboard for those metrics.** `deploy/grafana/sectile.json` charts the running version, active users and runs, the traffic, errors and latency of each controller, and the memory and CPU of each replica. It imports into any Grafana, where you pick the datasource, the deployment and the replicas at the top of the page. (#467)

- **Runs whose client has gone quiet are shown, and closed in time.** A run whose agent session has made no call for four hours shows as *silent* on the board and in the activities view instead of *running*. After eight hours of silence, Sectile takes the client for dead and cancels the run, so it no longer holds the board or its chain; its owner can still report how it really ended. The second delay is set with `SECTILE_MCP_SESSION_ABANDON_AFTER`. (#319)

- **Close a client's run from the board.** A run started by an agent session rather than by your local agent shows *Close* on its badge, for its owner and for admins. Closing records it as disconnected, and its owner can still report the real outcome. The activities view's cancel now asks the same question: only the owner or an admin may cancel such a run. (#319)

- **A run waiting for your answer says so again.** Before a skill asks you a question it cannot continue without, it marks its run as waiting: the board shows the amber *waiting* badge with how long it has been waiting, and the desktop app raises its "waiting for you" notification for runs it launched. The mark clears by itself as soon as the session does anything else, and headless runs are never marked. Permission prompts of the agent's own tools are not detected. (#318)

- **The desktop skill badge shows a skill waiting for your answer.** While a skill run is marked as waiting, the skill badge in the run header and beside the task reads *? Waiting for your answer* in amber, next to the process state, instead of staying blank until the skill ends. (#454)

- **Realign a macro's specification with its slicing.** *Réaligner la spec*, in the macro panel, runs the new `realign-macro` skill on your local agent, in the desktop app's Run (or type `/realign-macro <KEY>` in an agent session), and can be stopped from the same panel. It brings the specification back in line with a slicing edited by hand: lines typed by hand become stub entries, renamed lines rename their entry, and entries no line points to any more are marked *to be removed*. It never rewrites the body of an entry and never deletes one, for Spec Kit and OpenSpec alike, and it commits and pushes the macro branch only when it wrote something. (#426)

- **Each macro gets its own worktree.** A macro's specification is written in `.tasks/worktrees/<KEY>` of the specifications folder, when it is a Git repository, on the macro's branch, started from the up-to-date default branch, so two macros specified at the same time no longer share untracked files. An existing worktree is reused with its uncommitted work; projects with worktrees off keep using the checkout. (#426)

- **Declare where a project's specifications live, on your workstation.** The desktop project settings have a *Specifications folder* used by every macro operation: the slicing import, the macro worktree and `realign-macro`. It defaults to the project's local repository unless you choose another folder. The folder may be a Git repository or a plain folder, and the settings show which: in a plain folder, macro skills write in place, with no branch, commit or push. Importing the slicing from the web now reads the specification on your workstation, so it needs the desktop app connected. The code repository stays the agents' working directory. (#426, #443)

- **Manage Jira sprints from the timeline.** On a Jira project, *+ Sprints* creates a batch on the board (name pattern with `{n}`, count, start date, one to four weeks each); renaming, changing dates, closing and deleting are written to Jira and the timeline shows Jira's answer, so the next synchronisation keeps them. Closing can first move the unfinished tickets to the next sprint or to the backlog. On a GitHub project the timeline is read-only. (#426)

- **Choose the project a slicing line's story is created in.** Each line offers the macro's project and the other projects of the same tracker instance (one Jira site, one GitHub repository); the story lands there, still under the macro's epic. A target on another tracker or site is refused by name, and nothing is created. (#426)

- **Attach stories from the other Jira projects your roadmap reads.** *Projets de roadmap*, in a Jira project's tracker options, lists other project keys whose stories attach to slicing lines on import. Sectile only reads them and never writes to those projects. (#426)

- **The "created" toast links to the new ticket.** After a quick add, or a story created from the Roadmap (typed or from a slicing line), the toast offers *Ouvrir <key>*, which opens the ticket's detail, and an icon to its GitHub or Jira page when it has one. Such a toast stays 8 s instead of 3.5 s and waits while the pointer or the keyboard is on it; other toasts are unchanged. (#432)

### Changed

- **The specifications folder defaults to the code checkout on every project.** Macro skills no longer refuse to run on a project that sets no specifications folder: they read and write the specifications in the project's local repository, unless the desktop settings name another folder. (#484)

- **Lighter execution history in the desktop toolbar.** The drop-down that switches between a task's executions in Sectile Desktop no longer looks like a boxed form field: it sits unboxed next to the toolbar icons, with a discreet chevron, a background on hover and the accent ring on keyboard focus, in both themes. (#525)

- **Next step and full chain from the desktop toolbar.** The console toolbar of Sectile Desktop launches the selected task's next step from a `>` button and its whole workflow, unattended, from a `>>` button, as the web task card does; a small badge next to them reads `Next: <step>` or, while something runs, `Current: <skill>`. (#515)

- **Rename a task from its sidebar row.** In Sectile Desktop, the **…** button of a task row gives way to a pencil that turns the task's title into a field: Enter or clicking away saves the local name, Escape cancels. Relaunch and Detach to native terminal stay in the toolbar of the selected task, and Archive on the row. (#513)

- **Desktop project settings pick a default engine.** The provider, model, per-skill model and command fields leave the project settings and the workstation defaults for a single "Default engine" choice among the workstation's engines. Existing settings become engines automatically on the first start of the updated agent, which keeps a copy of the previous settings file, and every project keeps running what it ran. (#510)

- **The desktop workflow button says what is running.** While an execution of the selected task is active or being launched, the console toolbar button reads `Current: <skill>` (for example `Current: Pickup`) instead of a greyed-out `Next:`, including on a finished task; it proposes `Next: <step>` again once the execution ends. (#500)

- **Execution settings belong to each workstation.** The AI provider, the models and per-skill models, the model list of each provider, the interactive and headless commands, the terminal, the editor, worktrees, parallelism, the extra agents that get the skills, and the command name each stage runs are now set in the desktop app, for the workstation and per project, and no longer in the web interface; the server stops storing or using them. An existing workstation takes over the values the server held, once, on its first connection, and keeps running what it ran before. The web model picker and the engine badge of a card now show what your connected workstation will run, and say "Engine unknown" when none of your agents is connected for the project. Upgrade the local agent together with the server: an older agent receives no execution setting from the new server and falls back to its own defaults. (#305)

- **Breaking: every change you make on a tracker needs your own tracker credential.** On GitHub and GitLab, a change you make without a personal tracker credential is now refused instead of being written under the server account, as Jira already did; add yours in *Profile → Tracker credentials*. Agent keys not tied to a user, such as the shared server key, can no longer write to a tracker: pair the desktop app or use a personal API key. Reading still works without a credential, and the synchronisation keeps using the server credential. (#482)

- **Every synchronisation uses the server credential of its tracker, set by an admin.** The automatic sync and a *Sync* started by hand now both read with one credential per provider (GitHub, Jira, GitLab), instead of the project owner's or your own token, so a sync keeps working when its owner leaves or locks their token. Admins set, check and clear these credentials from a new *Server tracker credentials* section of the Administration page, which shows the account each one authenticates as; they are encrypted in the database with the server key, and members can no longer change them. Tokens already saved in the server settings are encrypted and kept on upgrade; a server that has one to encrypt and no usable `SECTILE_SECRET_KEY` refuses to start rather than lose it. A Jira project that only synced through its owner's personal token now needs a Jira server credential. Your own writes (transitions, comments) still go with your own credential. (#464)

- **One *Skills & IA* tab on a ticket.** The web ticket view merges *Skills & Copilot* and *Cadrage & Specs* into a single tab. It opens with the prompts to copy for the next step and for the autonomous pickup, as in the card menu, and ends with the generated specification. The recommended next step now opens the *Details* tab, with its description and launch button. The tab no longer shows the Copilot banner, the macro and batch skills, the mode and model selectors, the additional instruction field (put extra context in the ticket's comments), or the *Passer direct au Code* shortcut, which launched the implementation without a specification. (#461)

- **A context menu on each desktop project.** A project row in the desktop sidebar now shows only its name and a *…* button. Right-click the row, or click *…*, to open tasks, switch to the execution queue, create a task, open the agent console, reach the project settings or remove the project from the desktop. The count of waiting executions moves to the *…* button.

- **A quieter status bar.** The MCP clients indicator and the active executions counter are gone from the status bar. Who is using the board and how many runs are in flight are now on the Administration page, and the activities view still lists every execution. `GET /api/mcp/sessions` still lists the live sessions.

- **The desktop sidebar reads as columns.** Each execution row now starts with its run state, then its task number, so the states of all your runs line up down one column, as in the tickets pane; the task numbers share one width, so the titles line up too, free consoles included. A longer number is still shown in full and only shifts its own title. (#446)

- **A run lost with its server can still be reported on.** When the server holding an agent session restarts, or one of several servers stops, the runs that session had started are canceled as before, but their owner can now report how they really ended through `finish_run`, as after any other disconnection. (#408)

- **The web quick add is roomier, files the ticket under a macro, and can hand it to an agent.** The dialog is wider, with the title and a taller Markdown description on the left and the ticket's settings on the right (stacked on a narrow window). A new *Macro* field lists the project's open macros and starts on the macro the board is filtered on; the ticket is attached on the tracker too, and if the tracker refuses, the ticket is kept and a warning says so. The *Destination* choice is gone: the ticket always goes to the project's tracker. *Après la création* lets you pick, before saving, either to rewrite the ticket as a user story (its detail modal opens with the proposal to review) or to clarify it in the background; nothing is launched unless you choose to. (#445)

- **A story created under a Jira epic now gets the epic as its parent on Jira**, not only on the Sectile board. If Jira refuses the parent, the story is kept and a warning says so. (#426)

- **Ending a discussion in the desktop app no longer reads as a cancellation.** Stopping a discussion, or stopping it after its console closed, now reports it *Finished* with the green check, here and in the task's activity history, and a discussion no longer shows a skill badge since it runs no skill. Stopping a skill run still cancels it, and a discussion whose CLI exits in error still fails. The run indicators also read better: the state now comes before the title in the sidebar and the header, its tooltip starts with *Process:* while the skill badge's starts with *Skill:*, and *Running* is a pulsing blue dot instead of a spinner (still under reduced motion), on the desktop notification and the web badges too. (#438)

- The priority field of the task detail, quick add and clone forms shows the same colour dot as the task's card, so the priority you pick reads the way the board will show it. (#434)

- **A queued run now makes its task busy.** A skill waiting for its turn in the queue will start an agent on the ticket, so launching another run on the same ticket is refused, as it already was for a running one, with a message saying the run is queued. The refusal covers the next step, the full chain and a retry too, which used to queue a second run. "Launch anyway" still starts one next to it, and a session you start yourself from a terminal is never refused.

### Removed

- **The Mono-repo project setting.** Every project now works the same way: its list of other repositories is always available in the web project settings, a ticket runs in the repository it is pinned to or else in the code repository, and an execution never waits for somebody to choose a repository. The Repository layout row and the repository picker of the desktop console are gone with it. (#484)

- **The web editors for execution settings**: the AI engine tab of the profile (its MCP configuration stays), the "Agent settings" category of the project settings (the PR creation stage moves to "Agentic workflow"), the local folder and per-skill command name fields. Also the project "TTY mode", which nothing used. (#305)

- **The old tracker credential variables and the per-project tokens.** `SECTILE_TRACKER_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, `JIRA_API_TOKEN` and `GITLAB_TOKEN` are no longer read as tracker credentials: set `SECTILE_GITHUB_TOKEN`, `SECTILE_JIRA_EMAIL` with `SECTILE_JIRA_TOKEN`, or `SECTILE_GITLAB_TOKEN` instead, or store the credential from the Administration page. The server still starts with one of them set, and logs a warning naming its replacement. The GitHub and GitLab tokens a project could carry are gone, and discarded on upgrade: one server credential serves every project of its provider. (#464)

### Fixed

- **The web interface speaks one language throughout.** With English selected, the sidebar and project picker, the board and backlog controls, task details, project settings, the roadmap, triage and team views, the sprint timeline, the skill editor, activities, synchronisation messages and notifications no longer fall back to French, tooltips and screen-reader labels included; with French selected they read as before. Counts agree with their number, and the provider descriptions in the project settings now describe the tracker API integration. Known messages the server writes in the activity history are shown in English too; task content, tracker names and statuses, and raw error details stay as they are. (#526, #527, #528, #529, #530, #531, #532)

- **Dates, the page title and the sign-in screen follow your language.** Dates and times use the language chosen in Sectile rather than the browser's, a sprint date no longer shifts by a day in time zones west of UTC, and the browser tab and page language match the interface. Signed out, Sectile uses the language this browser last used, or the browser's own language, and the sign-in screen offers a French/English switch; once signed in, your profile setting applies. (#533, #534)

- **Every ticket of a batch launched from the web shows the batch, and is busy while it runs.** Each ticket of a running batch now carries a `Lot <first ticket>` badge on its board card, its list row and its detail panel, with where it stands: *en cours* for the ticket the agent is working on, *en attente dans le lot* for the ones waiting their turn, and the badge alone once done. Launching a skill on any of them is refused with a message naming the batch until the batch run ends, and every indicator disappears when it does. (#522)

- **MCP clients keep one session behind a proxy, and abandoned sessions no longer pile up on the server.** The server now pings every connected MCP client every 25 seconds, so a proxy that cuts idle connections no longer drops the client's event stream and forces it to reconnect with a new session every two and a half minutes. A session that owns no run and whose client stopped answering is released after three missed pings, instead of staying in memory for eight hours. The interval and the number of missed pings are set with `SECTILE_MCP_KEEPALIVE_INTERVAL` and `SECTILE_MCP_KEEPALIVE_FAILURES`. (#517)

- **A run launched from the board is shown running and its outcome is recorded, even when the local agent reported it queued.** The session a launch starts can now take over its run with `start_run` and report how it ended with `finish_run` while the board still shows it *queued*, instead of being refused with "remote run does not match an active execution" and leaving the run unfinished. A refused run id now says whether the run already ended and what to do instead. (#499)

- **Sealed tracker tokens no longer lock themselves when the server restarts.** Once unlocked, a token sealed with a passphrase stays unlocked on every server instance while you are connected, from a browser tab or a running local agent, and locks itself 30 minutes after you leave, or at once when you sign out with nothing else of yours connected. (#501)

- **An outdated local agent is named instead of failing obscurely.** When the server asks your local agent for something its build cannot do, stage transitions and the other features that rely on it now say which agent is outdated and that restarting or updating the Sectile desktop app fixes it, instead of `unknown local operation`. The desktop app notices when the running agent is not the one it bundles, including after a rebuild that reports the same version, offers to restart it, and marks it as outdated in its settings until then. (#391)

- **Stage reports, tickets created by agents, converted tickets, stories created under a macro and macro milestones are no longer written under the server account.** They are written under the credential of the person who caused them. A stage recorded without one is kept on the board, and its activity now fails and says why instead of claiming the report was posted. (#482)

- **My Tasks finds the tickets assigned to you on GitHub and Jira.** The *My Tasks* button of the sidebar now keeps the tickets assigned to your GitHub login or your Jira display name, as the account of your personal credential for that tracker, and your local tickets by your name or e-mail, all at once over *All projects* or a saved view. It stays on when no ticket is yours, is remembered per project and view, and shows as a *My Tasks* chip in the filters. On a tracker where you have no confirmed personal credential it uses your name and e-mail, and its tooltip names that tracker: save or verify your personal credential in your profile to fix it. (#468)

- **Agents can file tickets on Jira-backed projects.** `create_task` no longer answers that remote creation is not supported for Jira: it creates the Jira issue under your own Jira account, with the issue type and parent epic the agent gives and its Markdown description rendered, and returns the new key and link. Without a personal Jira token in your profile the call is refused and nothing is created. When Jira refuses a creation, the error lists the fields the project makes mandatory for that issue type. Creating a task from the desktop app files it on Jira the same way. `get_task` also shows a Jira ticket's comments, read with your own token. (#472)

- **The waiting glyph clears once you answer.** Pressing Enter in a run's console, in the desktop app or the web terminal, now clears its *waiting* mark at once, on the desktop and on the board; other keys leave it. The mark also no longer stays after the server restarts or the desktop reconnects, and a session's next Sectile call ends its wait whichever server instance serves it. (#475)

- **The missing repository mapping error points at the right file.** When the agent cannot find a project's local repository, the error now names `~/.config/sectile/settings.json`, the file Sectile actually reads, instead of a `taskflow` path that does not exist.

- **The desktop app no longer fails to list projects after a long pause.** On a PostgreSQL server, a database connection left idle for a long time could be dropped by the network, and the next request to use it, often the desktop app's project list, answered *Cannot list projects*. Sectile now renews its connections before that happens, and logs the cause of such errors.

- **Search ignores case and accents.** The search bar finds `Équipe` whether you type `equipe`, `Equipe` or `ÉQUIPE`, on the board, the roadmap, triage, the activities view and the filter pickers, and `%` or `_` typed in a search now match those characters only. On a PostgreSQL server this needs the `unaccent` extension, which Sectile creates at start; a server whose database role cannot create it refuses to start and says so. (#447)

- **An agent session keeps working whichever server receives its requests.** With several servers behind one load balancer, a request for an MCP session reaches the server that holds it, so tool calls, runs and the event stream no longer fail with "session not found" halfway through. A session whose server stopped is refused as not found, and the client starts a new one. The sessions view lists the sessions of every server. (#408)

- **A run canceled after a long silence can be reported again.** A run that had gone silent and was then canceled as disconnected refused its owner's report of how it really ended; it now accepts it, like any other disconnected run. (#319)

- **A run you stopped stays stopped.** An agent reporting a run as running a moment after it was canceled, finished or failed used to bring it back as running on the board; a run that has ended now keeps its outcome.

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

- **Starting a second server on PostgreSQL no longer interrupts the first one's
  work.** A server used to mark every running job as failed and every client run
  as canceled when it started, including the work of another server sharing the
  same PostgreSQL database, which a rolling deploy does for a few seconds. Each
  server now only reclaims the work of servers that stopped answering for 45
  seconds. A single SQLite server still reclaims everything at start, as before.
  (#403)

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

## [0.2.0] - 2026-09-24

### Added

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

- **A server that fails to answer no longer looks like an empty deployment.**
  Reading the projects, the issues, the settings or the saved board views used
  to be discarded in silence when the server refused: the sidebar and the board
  simply showed nothing, with no way to tell a broken deployment from an empty
  one. A failed read now raises a toast naming the resource and what the server
  answered, and while the projects or the issues are failing a banner stays on
  screen, with a button to try again. Being signed out stays quiet, since it
  already sends you to the sign-in screen.

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

[Unreleased]: https://github.com/sebastienferry/sectile/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/sebastienferry/sectile/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/sebastienferry/sectile/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/sebastienferry/sectile/releases/tag/v0.1.0
