import { test } from "node:test";
import assert from "node:assert/strict";
import { pullRequestLabel, pullRequestStateLabel, taskPullRequests } from "../src/domain/pull-requests.ts";

test("task pull requests preserve multiple forge states and the legacy primary link", () => {
  const task = { id: "t", title: "Task", status: "to_review", prUrl: "https://gitlab.example.com/a/b/-/merge_requests/7", prLinks: [
    { url: "https://github.com/a/b/pull/42", state: "merged" },
    { url: "https://gitlab.example.com/a/b/-/merge_requests/7", state: "open" },
  ] };
  const links = taskPullRequests(task);
  assert.deepEqual(links.map(link => `${pullRequestLabel(link)} · ${pullRequestStateLabel(link)}`), ["PR #42 · fusionnée", "MR !7 · ouverte"]);
  assert.deepEqual(taskPullRequests({ id: "old", title: "Old", status: "to_review", prUrl: task.prUrl }), [{ url: task.prUrl }]);
  assert.equal(pullRequestStateLabel({ url: task.prUrl, missingToken: "gitlab" }), "état indisponible · connexion forge requise");
});
