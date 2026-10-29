# Combined companion provider qualification

Date: 2026-10-29

The experimental `cmd/companion` executable was built from the dedicated Local
runtime branch and exercised through its real HTTP desktop gateway, using the
installed Claude Code and Codex CLIs and a new temporary workspace. The running
desktop installation was not used. A fresh installation manifest, local profile,
paired device and folder-only project were created through the backend APIs.

| Check | Claude Code | Codex |
| --- | --- | --- |
| Selected model | `haiku` | `gpt-6-luna` |
| Selected mode | `acceptEdits` | `workspace-write` |
| Shell tool writes the requested execution marker | Passed | Passed |
| Normalized tool event appears in conversation history | Passed | Passed |
| Explicit interruption returns to an idle, writable conversation | Passed | Passed |
| Explicit continuation recalls the initial random nonce | Passed | Passed |
| Interrupted delayed command produces no marker after 96 seconds | Passed | Passed |
| SIGKILL of the combined companion during a second delayed command | Passed | Passed |
| Relaunch restores the exact observed event prefix | Passed | Passed |
| Model, mode and writable idle state survive the crash | Passed | Passed |
| New continuation response recalls the nonce without a tool call | Passed | Passed |
| Crashed delayed command produces no marker after 96 seconds | Passed | Passed |

Each delayed tool appended `START` to its own counter, slept for 90 seconds,
then was supposed to create a separate completion file. Interruption and crash
were applied only after both providers' counters existed. Every counter remained
exactly `START`, and every delayed completion file remained absent. This checks
both lack of replay and termination of the original tool. Resumption sent a new,
explicit user message instructing the provider to recall context without tools.
The final assistant messages were inspected independently of the prior replies.

The combined process was stopped cleanly after qualification. Private local
snapshots and counters are retained under the temporary qualification directory;
credentials are not copied into this report.

This qualifies free conversations through the Go desktop gateway. It does not
qualify a rendered Tauri UI switch, ticket workflows with real MCP postbacks,
OAuth migration, Windows process ownership or every possible crash timing.
Those remain separate migration acceptance gates.
