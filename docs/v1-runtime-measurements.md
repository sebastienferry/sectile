# Local runtime measurements for the V1 candidate

Measured on 2026-10-29 on the qualification macOS arm64 host. These are
development-branch observations, not release-binary performance guarantees.

## Method

- Build the companion, server and agent from Go commit `f5984122` on the same
  host. For the separate server only, change the Local test port constant from
  52963 to 52965 in a disposable worktree. The companion reads 52965 from its
  isolated installation manifest. No production process or data is used.
- Create one disposable SQLite installation, account and device key. Copy its
  server database for the separate-process topology. Run both topologies with
  an isolated home and MCP refresh disabled.
- Start each topology three times. Measure wall-clock time from process spawn
  until the local agent reports `connected: true` on `/desktop/status`. After
  two more seconds, count the service processes and sum their resident memory
  from macOS `ps` RSS. Stop and reap every process between attempts.
- The first attempt includes the host's current filesystem/cache state. The
  next two are warm restarts against an already prepared database. No Tauri or
  Electron renderer, provider CLI, tracker synchronization or user workload is
  included.

| Topology | Connected, first / warm 2 / warm 3 | Idle service processes | RSS at two seconds, first / warm 2 / warm 3 |
| --- | --- | --- | --- |
| Embedded companion | 546 / 54 / 59 ms | 1 | 27.36 / 20.97 / 22.80 MiB |
| Separate server + agent | 2251 / 134 / 220 ms | 2 | 51.42 / 51.14 / 40.20 MiB |

The companion used fewer processes and less resident memory in every sample.
The warm startup samples were also shorter. The first-start comparison is not
a controlled cold-cache benchmark: the topologies ran sequentially, and the
separate server had a test-port-only source edit. Repeat on signed release
artifacts and include the desktop renderer before promising an end-user
startup or memory improvement.

The development binaries measured 42,003,778 bytes for the companion versus
37,279,250 bytes for the separate server and 20,632,386 bytes for the agent
(57,911,636 bytes combined). The current macOS package still includes all
three Go binaries for Local/Fleet compatibility, so this binary comparison is
not a package-size reduction. The latest arm64 app is 102.28 MiB and its DMG
is 51.70 MiB; the [release checklist](v1-release-checklist.md) records its
source commit and checksum.
