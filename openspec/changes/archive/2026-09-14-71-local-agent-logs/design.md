# Design

Use the existing desktop-owned `agent.log` in Electron user data. Introduce an asynchronous bounded-tail reader in `desktop/electron/agent-log.cjs`; the IPC handler supplies the fixed application path and accepts no renderer path. Return path, text, missing and truncated metadata. Read at most 256 KiB from a regular file using a single opened descriptor and close it even on errors. Skip leading UTF-8 continuation bytes when the tail starts mid-character. Reject symlinks/non-regular files to avoid following unexpected sources or blocking on pipes.

Expose a narrow `agentLogs()` preload method. A persistent Agent logs toolbar action opens the shared dialog. Render contents with `textContent`, with a focusable scroll region, source path, status and Refresh. The action remains enabled while disconnected. Refresh clears stale text, disables duplicate reads, and replaces the snapshot; detached UI nodes ensure late responses cannot replace another dialog. Scroll to the newest output after successful reads. No polling or change to execution selection is introduced.

Explain that this is the desktop-owned diagnostic file: independently started agents may write to their original terminal instead. This avoids presenting historical desktop output as the connected CLI agent's current log.

Rejected alternatives: a server or agent endpoint would depend on the component being diagnosed and transfer local diagnostics unnecessarily; unbounded readFile would grow with the append-only log; automatic streaming adds lifecycle complexity beyond this request. Log rotation is an independent retention policy and is deferred.

Tests cover bounded tails, UTF-8 boundaries, missing/empty files, append/truncate/replacement, invalid file types and symlinks; Electron integration covers offline and connected access, literal rendering, refresh, errors, and unchanged selection. Run desktop build/UI suite and syntax/static checks. No Go or web production code changes are planned.
