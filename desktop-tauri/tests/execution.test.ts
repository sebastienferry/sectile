import test from "node:test";
import assert from "node:assert/strict";
import {
  canStop,
  canLaunchTask,
  executionConnectionLabel,
  launchInput,
  projectRuns,
  statusLabel,
  type Run,
} from "../src/domain/execution.ts";
const run: Run = {
  id: "run",
  projectId: "p",
  taskId: "t",
  skill: "implement_issue",
  status: "running",
  createdAt: "2026-10-28T12:00:00Z",
};
test("connection status names the Fleet backend while execution stays on the workstation", () => {
  assert.equal(executionConnectionLabel("fleet"), "Local agent · Fleet backend");
  assert.equal(executionConnectionLabel("local"), "Local runtime");
  assert.equal(executionConnectionLabel(undefined), "Local runtime");
});
test("stop requires fresh live state and preserves cancellation acknowledgement", () => {
  assert.equal(canStop(run, true), true);
  for (const item of [
    { ...run, restored: true },
    { ...run, status: "completed" },
    { ...run, cancelRequested: true },
  ])
    assert.equal(canStop(item, true), false);
  assert.equal(canStop(run, false), false);
  assert.equal(
    statusLabel({ ...run, cancelRequested: true }),
    "Stopping…",
  );
});
test("runs stay scoped to their project and active tasks cannot be launched twice", () => {
  assert.equal(canLaunchTask("to_test", "t", [run]), false);
  assert.equal(canLaunchTask("finished", "other", []), false);
  assert.equal(
    canLaunchTask("to_test", "t", [{ ...run, status: "completed" }]),
    true,
  );
  assert.deepEqual(
    projectRuns([run, { ...run, id: "foreign", projectId: "other" }], "p"),
    [run],
  );
});
test("pickup remains autonomous while ordinary skills inherit project mode", () => {
  assert.equal(
    launchInput("t", "pickup_issue", "", "interactive").mode,
    "autonomous",
  );
  assert.equal(launchInput("t", "implement_issue", "", "").mode, "");
});


test("canonical pickup remains autonomous even through the advanced skill chooser", () => {
  assert.equal(launchInput("ticket", "pickup", "Instructions", "interactive").mode, "autonomous");
});

test("an idle conversation session is not labeled as an active response", () => {
  const conversation: Run = { ...run, conversation: true };
  assert.equal(statusLabel(conversation), "Active session");
  assert.equal(statusLabel(run), "Running");
  assert.equal(statusLabel({ ...conversation, cancelRequested: true }), "Stopping…");
  assert.equal(statusLabel({ ...conversation, waitingSince: "2026-10-29T00:00:00Z" }), "Action required");
});

test("former context is carried only by an interactive launch in the conversation view", () => {
  assert.equal(launchInput("t", "clarify", "Go on", "", { view: "conversation", contextRunId: "source" }).contextRunId, "source");
  assert.equal(launchInput("t", "clarify", "", "", { view: "terminal", contextRunId: "source" }).contextRunId, undefined);
  assert.equal(launchInput("t", "clarify", "", "", { contextRunId: "source" }).contextRunId, undefined);
  assert.equal(launchInput("t", "pickup", "", "", { view: "conversation", contextRunId: "source" }).contextRunId, undefined);
});

test("a start over skill launch stays interactive whatever the project default", () => {
  const input = launchInput("t", "clarify", "Go on", "interactive", { view: "conversation", contextRunId: "source" });
  assert.equal(input.mode, "interactive");
  assert.equal(input.contextRunId, "source");
});
