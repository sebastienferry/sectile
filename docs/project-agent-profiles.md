# Project agent profiles

A project agent profile is a reusable set of supplemental agent instructions.
It is independent of the workstation engine catalogue: the engine selects the
provider and model, while the profile describes how that agent should work.

In Desktop, use the **+** beside a mapped project, or its **Project prompt**
menu item. Select an engine and an optional **Agent profile**. **None** keeps
the ordinary launch behavior. The selection affects only this session and
never changes a project default. Enter your request after the session opens.
The global **PROJECTS +** still adds a project.

## Creating and sharing profiles

Choose **Create profile**, enter a name and reusable agent instructions, and
choose **Save profile**. A successful save selects the profile without starting
a session. A failed save keeps your draft. **Cancel profile** or Escape closes
the creation form without saving. The checkout must be available and writable.

Profiles are flat Markdown files in the mapped primary checkout:

```text
.sectile/agents/code-reviewer.md
```

The filename without `.md` is the profile's name and ID. Use 1–64 lowercase
ASCII letters or digits, with single internal hyphens. Examples: `reviewer`,
`backend-reviewer`, `agent-2`. Windows device names such as `con`, `nul`,
`com1` and `lpt1` are rejected on every platform so profiles stay portable.
No front matter is interpreted. The whole file is
the prompt, including meaningful leading and trailing whitespace. Prompts must
be valid UTF-8, nonblank, without NUL, and at most 64 KiB. Desktop normalizes
CRLF to LF when creating a file. Existing names are never overwritten.

Discovery is sorted, nonrecursive, and limited to this primary checkout.
Attached context repositories are not searched or written. Invalid or unreadable
Markdown files appear as warnings and cannot be selected. Symbolic links in the
profile path are refused, including a symlinked checkout or ancestor directory;
map the actual canonical checkout path instead. Listing creates no files.

Use your normal editor and Git workflow to maintain and share the files.
Desktop does not edit or delete profiles and never commits or pushes them.

## Supported engines and views

Built-in **Claude Code** and **Codex** engines support profiles in both Terminal
and Conversation views. Custom command templates and other providers do not
support a selected profile; choose a supported engine or explicitly choose None.
Older local agents without the profile API keep ordinary launches available and
show that profiles are unavailable.

Claude must expose `--append-system-prompt`. Codex CLI 0.162 or newer is required;
its installed effective configuration must be readable through the app-server
protocol. Unsupported or unavailable interfaces reject the profiled launch
before a run is admitted. CLI/provider startup and authentication errors still
follow the existing console or conversation failure reporting.

Claude receives additive system instructions; normal CLAUDE.md discovery stays
active. Codex receives the profile after any effective native developer
instructions, through a session override. Normal AGENTS.md discovery and base
instructions remain active. Profile loading does not change sandbox, approval,
permission, model, or folder settings. The prompt is data, not shell syntax,
and is never submitted as an initial user request.

The agent rereads and validates the selected file at launch. If it disappeared
or became invalid, the launch fails rather than silently omitting it. A running
session uses its launch-time snapshot for subsequent turns and process
reconnects. New sessions read the current file. A restarted Sectile agent only
restores read-only console history; it does not resume provider processes.
Profile IDs/names survive in that history, but the prompt is not copied into the
run store or sent to the Sectile server. As with other session instructions, the
selected prompt is sent to the chosen AI provider when that session runs.
