import test from "node:test";
import assert from "node:assert/strict";
import { trackerColumns } from "../src/domain/board.ts";

test("board only includes remote tracker tasks and preserves remote statuses", () => {
  const tasks = [
    {
      id: "1",
      title: "Fix login",
      key: "APP-1",
      source: "jira",
      trackerStatus: "In review",
      status: "to_clarify",
    },
    { id: "2", title: "Local task", source: "local", status: "to_test" },
    { id: "3", title: "Unknown source", status: "to_test" },
    {
      id: "4",
      title: "Missing remote status",
      source: "github",
      status: "finished",
    },
  ];
  const columns = trackerColumns(tasks);
  assert.deepEqual(
    columns.map((c) => c.name),
    ["In review", "Statut distant indisponible"],
  );
  assert.equal(columns.flatMap((c) => c.tasks).length, 2);
  assert.equal(trackerColumns(tasks, "app-1")[0].tasks[0].id, "1");
  assert.deepEqual(trackerColumns(tasks, "no match"), []);
});
