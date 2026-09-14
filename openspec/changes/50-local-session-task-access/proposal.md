# Make the task interface usable by local skill sessions

## Why
A skill session launched by the local agent is told to read its ticket with `get_task` and its project settings with `get_project_context`. Neither call is usable today. `get_task` returns nothing at all when the server has no tracker credential, because a failed comment fetch discards the locally known task. `get_project_context` inlines the full body of every generated skill, producing a payload of roughly 136 KB that exceeds what a session can consume — while the session already holds its own skill text.

## What Changes
- `get_task` returns the task whenever it is known locally, with its comments when they can be read and an explicit retrieval error otherwise. It fails only when the task itself does not exist.
- `get_project_context` returns a bounded payload: project identity, execution settings, specification framework, pull-request creation stage, and per-skill references (id, directory, command name, resolved file paths) instead of inlined skill and command bodies.
- Both behaviours are covered by tests.

## Impact
No schema change, no migration, no protocol break. Consumers that relied on `get_project_context` returning full skill bodies read the referenced files instead. The generated skill routing, the managed-run gate, `start_run`/`finish_run` and the stage-validation rules are unchanged.

Out of scope: the agent/server split (#54), stage-validation rules, tracker credential provisioning, desktop UI, and the invalid `.git` suffix stored on some projects' `githubRepo`.
