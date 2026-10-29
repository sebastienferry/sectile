import test from "node:test";
import assert from "node:assert/strict";
import {
  canStop,
  canLaunchTask,
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
    "Arrêt demandé…",
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
