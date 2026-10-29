# Desktop folder projects

`POST /api/projects` accepts `folderOnly: true` with a name and no tracker fields.
It creates project metadata without a tracker association and disables automatic
synchronization. Existing tracker-backed creation keeps requiring a tracker.

The native desktop selects a local directory with its OS folder picker. Paths
stay in workstation settings, via `POST /desktop/projects` with `folderOnly: true`.
This explicit flag permits a non-Git directory. Structured free conversations
accept mapped non-Git folders; task execution retains Git validation. No Git
initialization or ticket creation occurs when opening a folder.

The desktop selects an existing project when its mapped path matches the
canonical selected folder. If creation succeeds but mapping fails, it retains
the created ID and offers association retry rather than creating another project.

## Manual Codex verification (2026-10-28)

Verified through the packaged macOS desktop using a disposable folder without Git:

- Native folder selection created a project and opened the prompt composer.
- Codex ran `pwd`, wrote `probe.txt` and read it back. The transcript displayed
  tool calls, command details, tool results and the expected `SECTILE_CODEX_OK` answer.
- Interruption of a shell command waiting 90 seconds returned the UI to idle and
  displayed an interruption acknowledgement. A new turn succeeded with `RESUME_OK`.
- **Failure:** after the original 90-second deadline, that shell command still
  wrote its marker. The provider's interrupted-turn acknowledgement did not stop
  the shell tool. Its result appeared late, after the resumed answer. Interruption
  is therefore not validated as a reliable tool stop.
- Navigating to the board and reopening the conversation restored the transcript.
- A spurious non-Git attached-folder warning was fixed in `projectFolderMap` and
  covered by `TestProjectFolderMapAcceptsMappedNonGitFolder`. Existing running
  provider sessions were not restarted to load this fix.

A future interruption fix must stop the tool execution as well as the model turn,
then repeat the delayed-marker test past its deadline. Merely changing the UI label
or accepting a provider turn acknowledgement is insufficient.

## Interruption fix verified (2026-10-28)

The corrected Go agent was exercised twice with the real installed Codex CLI in
a disposable local runtime. Both runs interrupted a `sleep 90` followed by a file
write, waited 95 seconds after interruption, and confirmed the delayed marker was
absent. The stricter second run first waited for a separate `started-marker.txt`
written by the shell, proving the tool was running before interruption. Both runs
resumed the same Codex conversation and received `RESUME_OK` after a tool checked
that the delayed marker remained absent. The corrected companion was then loaded
by the desktop and its connected status verified.

See ADR 0061 for process-tree termination and thread-resume behavior.

## Claude Code verification (2026-10-28)

The real installed Claude Code CLI passed the equivalent isolated-runtime test:

- A shell command wrote and read `SECTILE_CLAUDE_OK`; the transcript included an
  assistant response, the shell tool call and its result.
- The test waited for `started-marker.txt` from the running shell before asking
  to interrupt its `sleep 90` followed by a delayed file write.
- After waiting 95 seconds following interruption, `delayed-marker.txt` was absent.
- The same conversation resumed, checked the file's absence through a shell tool
  and answered `RESUME_OK`.

Separately, a Claude prompt was sent through the packaged desktop UI in the
disposable test project. The rendered conversation displayed the Bash call, its
`SECTILE_CLAUDE_UI_OK` result, the matching assistant response and the idle state.
No Claude runtime interruption changes were required for this scenario. The
opt-in fixture preserves USER as well as HOME so the installed macOS CLI can
resolve its existing authentication. The ordinary fixture remains isolated.
