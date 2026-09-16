# Issue #101: Direct link to the WebUI

## Scope and implementation

The connected desktop header address now opens the current server board in the default browser. A native anchor supports keyboard activation and preserves focus across status polling. Disconnected and stopped status remains plain text.

- `desktop/src/main.js`: update the connection link in place and report opening errors.
- `desktop/src/style.css`: underline the link and expose keyboard focus.
- `desktop/electron/preload.cjs`: expose an argument-free board-opening action.
- `desktop/electron/main.cjs`: read current connection status, reject disconnected, malformed, non-HTTP(S), or credential-bearing destinations, preserve the base path, and clear task selection and fragment.
- `desktop/tests/board-link.ui.cjs`: exercise mouse and keyboard opening, refresh focus, changed addresses, invalid destinations, disconnection, stopped agent, and browser failure.
- `README.md` and `docs/CAPABILITIES.md`: document the shortcut.
- `openspec/changes/101-direct-link-to-webui/`: proposal, design, behavioral requirements, and checklist.

No architecture change, backend endpoint, or new dependency is needed. Existing task and PR opening remain unchanged.

## Validation

- `openspec validate 101-direct-link-to-webui --strict`: `Change '101-direct-link-to-webui' is valid`.
- `make server agent`: successful web build and both Go binaries.
- `make test`: all Go packages pass; web output reports `tests 22`, `pass 22`, `fail 0`; TypeScript and oxlint exit successfully.
- `go vet ./...`: exit 0 with no output.
- `npm --prefix desktop run build`: successful Vite build.
- `npm --prefix desktop test`: `tests 4`, `pass 4`, `fail 0`.
- JavaScript syntax checks and `git diff --check`: exit 0.
- `npm --prefix desktop run test:ui`: `tests 23`, `pass 23`, `fail 0` (107.5 seconds), including the new header-link regression.

Existing web lint warnings and the Vite chunk-size warning are in unchanged web files. Vite also warns about the assigned worktree's `#` character; both builds succeed.

## Review and workflow recovery

PR: https://github.com/sebastienferry/sectile/pull/103
Branch: `feat/101`.

The configured repository name `sectile` redirects to `sectile`; GitHub confirms the canonical repository and default branch `main`. The branch includes the fetched default branch. The existing PR has no review or inline feedback. Review verified specification coverage, current-status URL validation, focus persistence, and failure handling. Pre-existing generated skill changes remain outside the issue commits.

Clarification was recorded successfully. Two `specified` transition calls failed with `local operation not confirmed; check the agent before retrying: context deadline exceeded`. A subsequent task read confirms it remains clarified. The running local agent log contains `[Agent] Unknown message type: workspace_request` at 2026-09-14 20:58:48. The current source handles that message, indicating that the running agent is outdated. Related integration history exists in issue #50; no task-creation MCP tool is exposed. The failure is recorded on #101 and preserved here.

After active coding sessions finish, restart the updated local agent and replay `specified`, `implemented`, and `reviewed` with this PR and validation evidence. Do not bypass PR validation or change tracker labels directly. The specification is committed as `398a578`; implementation and review evidence remain available for recovery. No merge or ticket closure is performed.
