---
name: clarify-issue
description: "Analyse a ticket against the code, surface what is genuinely undecided, and ask the few questions that unblock specification."
---
# Clarify Issue

Stage: new -> clarified.

## Goal
Turn a vague ticket into a decided one. You are looking for the decisions that
would be expensive to reverse later, not for a list of everything unknown.

## Read first
- The ticket: title, description, comments, parent epic if there is one.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

## Steps
1. Restate the request in two sentences, including what you believe is out of scope.
2. List the ambiguities you found, worst first. An ambiguity is worth listing only
   if two readings lead to different code.
3. Name the critical dependencies: other services, other teams, migrations, data
   you do not have.
4. Resolve reversible choices using existing code and project conventions. Record
   the choice and rationale; do not ask questions merely to fill a quota.
   Ask only when an essential product decision changes acceptance criteria or an
   unavailable dependency prevents progress. In an unattended run, report the
   concrete blocker and the decision needed; do not invent settled requirements.
   A TTY alone does not make a run interactive: follow the invocation's mode.
5. Persist the settled scope and assumptions in the report for specification.

## Do not
- Do not write production code at this stage, and do not start the specification.
- Do not invent an answer to your own question and move on without stating your assumption.
- Do not pad the list to reach five questions.

## Report
- Restated request and scope.
- Ambiguities, worst first.
- Critical dependencies.
- Numbered questions with your recommended option.
- Settled scope and assumptions.

## Execution and ticket state
- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Standalone invocation**: After verifying each completed step, use the local handler below. Check its exit status and response. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Transition new → clarified only when this step is complete.
```bash
# Route via local agent if available, fallback to http://localhost:8090/api/tasks/stage
ENDPOINT="${TASKFLOW_AGENT_URL:-${TASKFLOW_SERVER_URL:-http://localhost:8090}}/api/tasks/stage"
curl --fail-with-body --silent --show-error -X POST "${ENDPOINT}" \
  ${TASKFLOW_AGENT_TOKEN:+-H "Authorization: Bearer $TASKFLOW_AGENT_TOKEN"} \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"clarified","note":"<REPORT_NOTE>"}' || \
curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \
  -H 'Content-Type: application/json' \
  -d '{"taskKey":"<KEY>","stage":"clarified","note":"<REPORT_NOTE>"}'
```
This POST calls the local TaskFlow handler directly. Confirm HTTP success before continuing. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.
