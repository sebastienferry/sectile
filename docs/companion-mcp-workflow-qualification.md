# Combined companion MCP workflow qualification

Date: 2026-10-29

The experimental combined companion completed a real Claude Code (`haiku`)
workflow through its process-scoped `sectile_companion` MCP connection. A
project-scoped pickup skill override described a small Python greeting task.
All work ran in a temporary Git worktree against a disposable loopback GitLab
HTTP fixture and a real bare Git repository. No production tracker was used.
The companion process received its explicit MCP origin through
`SECTILE_COMPANION_MCP_URL`; launcher integration must preserve that setting.

## Verified results

- `get_task`, `get_project_context`, `start_run`, `transition_stage` and
  `finish_run` executed through MCP; the existing launcher run ID was reused.
- Clarification and OpenSpec artifacts were committed, and `clarified` and
  `specified` were accepted by the backend and synchronized to the fixture.
- Implementation added `greeting.py` and a unittest for `greet("Ada")`.
- `implemented` and `reviewed` were accepted after real Git/forge evidence
  checks. The final backend task status was `to_close`, with label `#reviewed`.
- The final workflow activity was `completed`. The fixture issue remained open;
  its single merge request was ready, open and unmerged.
- Independent verification reran the unittest successfully, checked the clean
  worktree, confirmed local HEAD matched the pushed `feat/1` branch, and found
  exactly three feature commits: clarification, specification and implementation.

## Fixture corrections and recovery

The initial sync needed the fixture's unattended server credential as well as
its personal credential. The first workflow reached `specified`, implemented
and tested the code, pushed its branch and created the MR, but failed the
`implemented` transition because the repository-local forge CLI configuration
was incomplete. The run correctly reported `failed` and retained its evidence.

After configuring the fixture host, API host and HTTP protocol in the
worktree's private glab configuration, a second pickup run resumed from the
last accepted stage. It reused the existing commits and MR, completed the two
remaining transitions and reported success. The failed activity remains in the
history; no stage or database record was forged to pass verification.

This qualifies a complete MCP workflow and recovery from a real validation
failure on the combined process. It uses a fixture skill override and a GitLab
test service; it does not qualify every bundled skill, production forge, Codex
workflow, or the rendered Tauri launcher. The native Tauri launcher's later
companion migration is qualified separately in the runtime contract; this
workflow's fixture execution did not run through that desktop path.
