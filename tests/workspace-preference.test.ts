import assert from "node:assert/strict";
import test from "node:test";
import {
  createWorkspaceDestination,
  resolveLaunchDestination,
  serializeWorkspaceDestination,
} from "../src/lib/workspace-preference.ts";

const projects = [
  { id: "project-a", name: "Project A" },
  { id: "project-b", name: "Project B" },
];

test("restores a valid project view and its conversation identity", () => {
  const saved = serializeWorkspaceDestination(
    createWorkspaceDestination("project-b", "prompt", "conversation-7"),
  );

  assert.deepEqual(resolveLaunchDestination(saved, projects), {
    id: "project-b:prompt:conversation-7:",
    projectId: "project-b",
    view: "prompt",
    runId: "conversation-7",
    taskId: undefined,
  });
});

test("falls back to the first project board when the saved project disappeared", () => {
  const saved = JSON.stringify({ projectId: "removed", view: "prompt" });

  assert.deepEqual(resolveLaunchDestination(saved, projects), {
    id: "project-a:board::",
    projectId: "project-a",
    view: "board",
    runId: undefined,
    taskId: undefined,
  });
});

test("opens setup when no project remains", () => {
  assert.equal(
    resolveLaunchDestination('{"projectId":"removed","view":"board"}', []),
    null,
  );
});

test("ignores malformed and unsupported saved views", () => {
  assert.equal(resolveLaunchDestination("not-json", projects)?.view, "board");
  assert.equal(
    resolveLaunchDestination(
      JSON.stringify({ projectId: "project-a", view: "settings" }),
      projects,
    )?.view,
    "board",
  );
});
