import test from "node:test";
import assert from "node:assert/strict";
import type { Task } from "../src/domain/board.ts";
import { matchesTaskFilters, taskFilterPredicate } from "../src/domain/task-filters.ts";

const tasks: Task[] = [
  {
    id: "1",
    key: "APP-1",
    title: "Login page crash",
    source: "jira",
    trackerStatus: "In Progress",
    status: "to_implement",
    priority: "urgent",
    assignee: "Alice",
    parentKey: "AUTH-1",
    parentTitle: "Authentication",
  },
  {
    id: "2",
    key: "APP-2",
    title: "Reset password email",
    source: "jira",
    trackerStatus: "To Do",
    status: "to_specify",
    priority: "high",
    assignee: "Bob",
    parentKey: "AUTH-1",
    parentTitle: "Authentication",
  },
  {
    id: "3",
    key: "APP-3",
    title: "Dark mode setting",
    source: "jira",
    trackerStatus: "To Do",
    status: "to_clarify",
    priority: "low",
    assignee: undefined,
    parentKey: undefined,
  },
];

test("matchesTaskFilters filters by query", () => {
  assert.equal(matchesTaskFilters(tasks[0], { query: "crash" }), true);
  assert.equal(matchesTaskFilters(tasks[1], { query: "crash" }), false);
});

test("matchesTaskFilters filters by priority", () => {
  assert.equal(matchesTaskFilters(tasks[0], { priority: "urgent" }), true);
  assert.equal(matchesTaskFilters(tasks[0], { priority: "low" }), false);
  assert.equal(matchesTaskFilters(tasks[2], { priority: "low" }), true);
});

test("matchesTaskFilters filters by status (tracker status and workflow stage)", () => {
  // trackerStatus
  assert.equal(matchesTaskFilters(tasks[0], { status: "in progress" }), true);
  assert.equal(matchesTaskFilters(tasks[1], { status: "in progress" }), false);
  // workflow stage (status to_implement maps to stage specified)
  assert.equal(matchesTaskFilters(tasks[0], { status: "specified" }), true);
  assert.equal(matchesTaskFilters(tasks[2], { status: "new" }), true);
});

test("matchesTaskFilters filters by assignee (including unassigned)", () => {
  assert.equal(matchesTaskFilters(tasks[0], { assignee: "Alice" }), true);
  assert.equal(matchesTaskFilters(tasks[1], { assignee: "Alice" }), false);
  assert.equal(matchesTaskFilters(tasks[2], { assignee: "Alice" }), false);
  assert.equal(matchesTaskFilters(tasks[2], { assignee: "__unassigned__" }), true);
  assert.equal(matchesTaskFilters(tasks[0], { assignee: "__unassigned__" }), false);
});

test("matchesTaskFilters filters by parent (including unparented)", () => {
  assert.equal(matchesTaskFilters(tasks[0], { parent: "AUTH-1" }), true);
  assert.equal(matchesTaskFilters(tasks[2], { parent: "AUTH-1" }), false);
  assert.equal(matchesTaskFilters(tasks[2], { parent: "__none__" }), true);
  assert.equal(matchesTaskFilters(tasks[0], { parent: "__none__" }), false);
});

test("matchesTaskFilters combines multiple filters with AND semantics", () => {
  const predicate = taskFilterPredicate({
    priority: "high",
    parent: "AUTH-1",
    assignee: "Bob",
  });
  const filtered = tasks.filter(predicate);
  assert.equal(filtered.length, 1);
  assert.equal(filtered[0].key, "APP-2");
});
