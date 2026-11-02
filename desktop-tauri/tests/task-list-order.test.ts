import test from "node:test";
import assert from "node:assert/strict";
import type { Task } from "../src/domain/board.ts";
import {
  DEFAULT_SORT,
  compareIdentity,
  nextSort,
  orderedTasks,
} from "../src/domain/task-list-order.ts";

const t1: Task = { id: "1", key: "APP-1", title: "Alpha issue", priority: "low", status: "to_clarify" };
const t2: Task = { id: "2", key: "APP-2", title: "Beta issue", priority: "urgent", status: "specified" };
const t3: Task = { id: "3", key: "APP-10", title: "Gamma issue", priority: "medium", status: "implemented" };
const t4: Task = { id: "4", key: "APP-9", title: "Delta issue", priority: "high", status: "to_close" };

test("nextSort flips ascending if the same field is clicked", () => {
  assert.deepEqual(nextSort({ field: "priority", ascending: false }, "priority"), {
    field: "priority",
    ascending: true,
  });
  assert.deepEqual(nextSort({ field: "priority", ascending: true }, "priority"), {
    field: "priority",
    ascending: false,
  });
});

test("nextSort defaults to descending for priority and ascending for other columns", () => {
  assert.deepEqual(nextSort({ field: "key", ascending: true }, "priority"), {
    field: "priority",
    ascending: false,
  });
  assert.deepEqual(nextSort({ field: "priority", ascending: false }, "title"), {
    field: "title",
    ascending: true,
  });
  assert.deepEqual(nextSort({ field: "priority", ascending: false }, "stage"), {
    field: "stage",
    ascending: true,
  });
});

test("compareIdentity naturally sorts keys and ids numerically", () => {
  assert.ok(compareIdentity({ id: "1", key: "APP-9", title: "" }, { id: "2", key: "APP-10", title: "" }) < 0);
  assert.ok(compareIdentity({ id: "1", key: "#9", title: "" }, { id: "2", key: "#100", title: "" }) < 0);
});

test("orderedTasks defaults to sorting by priority descending", () => {
  const sorted = orderedTasks([t1, t2, t3, t4], DEFAULT_SORT);
  assert.deepEqual(sorted.map(t => t.key), ["APP-2", "APP-9", "APP-10", "APP-1"]);
});

test("orderedTasks sorts by title alphabetically", () => {
  const asc = orderedTasks([t3, t1, t4, t2], { field: "title", ascending: true });
  assert.deepEqual(asc.map(t => t.key), ["APP-1", "APP-2", "APP-9", "APP-10"]);

  const desc = orderedTasks([t3, t1, t4, t2], { field: "title", ascending: false });
  assert.deepEqual(desc.map(t => t.key), ["APP-10", "APP-9", "APP-2", "APP-1"]);
});

test("orderedTasks sorts by key naturally", () => {
  const asc = orderedTasks([t3, t4, t1, t2], { field: "key", ascending: true });
  assert.deepEqual(asc.map(t => t.key), ["APP-1", "APP-2", "APP-9", "APP-10"]);

  const desc = orderedTasks([t3, t4, t1, t2], { field: "key", ascending: false });
  assert.deepEqual(desc.map(t => t.key), ["APP-10", "APP-9", "APP-2", "APP-1"]);
});

test("orderedTasks sorts by stage following workflow stages", () => {
  // stages: to_clarify (new) -> specified (specified) -> implemented (implemented) -> to_close (reviewed)
  const asc = orderedTasks([t3, t4, t1, t2], { field: "stage", ascending: true });
  assert.deepEqual(asc.map(t => t.key), ["APP-1", "APP-2", "APP-3" in asc ? "APP-3" : "APP-10", "APP-9"]);
});
