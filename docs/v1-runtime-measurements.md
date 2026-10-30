# Local runtime measurements for the V1 candidate

The service-only comparison was measured on 2026-10-29 on the qualification
macOS arm64 host. A later exploratory desktop sample appears below. These are
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
(57,911,636 bytes combined). The macOS package still includes all three Go
binaries for Local/Fleet compatibility, so this binary comparison is not a
package-size reduction.

The latest retained clean-source, unsigned package pairs Tauri `d69c3ac` with
Go `f55e8768`: the arm64 app is 103.41 MiB and the DMG is 56,371,343 bytes
(53.76 MiB). App/DMG resource verification, the retained hash and `hdiutil verify`
passed. The [release checklist](v1-release-checklist.md) records its checksum
and open native/signing gates. This package measurement does not repeat the
startup/RSS benchmark above, which still uses Go `f5984122` without a desktop
renderer. Earlier package measurements remain historical evidence rather than
current artifact sizes.

## Exploratory desktop-inclusive Local sample (2026-10-30)

An isolated, unsigned arm64 **Sectile Brand QA** app built from Tauri
`32e76f2` used a private companion on `127.0.0.1:52965` and its existing QA
profile, one project and a stopped Codex conversation. The companion was a
temporary test-port build (SHA-256
`56a3a305cd05887d7ed50562ef52d0040cbb17f4e063feddba99bd1129e88c6b`);
its embedded Go build metadata does not identify a source commit. It was not
the retained release candidate and did not use port 52963.

For the first sample, quit the QA app and terminate only its identified QA
companion, verify no process listened on 52965, then launch the app through the
native computer-use interface. The first full accessibility observation after
launch showed **Agent connecté**, the restored project and conversation.
Repeat with the companion left running for a warm desktop restart. Measure
elapsed wall time around the launch and accessibility observation. Sample RSS
with macOS `ps` after the restored conversation is visible, summing the app,
companion and three WebKit XPC processes created at the same launch time.

| Sample | QA companion before launch | First ready UI observation | App RSS | Companion RSS | Three WebKit RSS | Sum of RSS |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| 1 | Stopped | 753 ms | 108.1 MiB | 67.1 MiB | 110.9 MiB | 286.1 MiB |
| 2 | Running | 750 ms | 113.3 MiB | 69.3 MiB | 110.0 MiB | 292.6 MiB |

The observation call includes automation and accessibility settling time, so
these numbers bound the observed ready state rather than isolate app startup
latency. RSS sums double-count shared pages and assign WebKit workers by launch
time, not a formal process ownership report. The snapshots were taken at about
12 and 8 seconds after launch, respectively, on a warm development host. A
third restart had a transient native-observation error after it appeared to
start the app; its follow-up observation cannot time launch and is excluded.
There is no matched Electron sample, provider workload, remote Fleet server,
clean-user install or signed release artifact. Repeat with controlled launch
instrumentation and exact release binaries before closing the desktop
performance gate.
