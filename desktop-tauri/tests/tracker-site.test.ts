import assert from "node:assert/strict";
import test from "node:test";
import { trackerApiSite } from "../src/tracker-site.ts";

test("GitLab website addresses resolve to the API root used by tracker writes", () => {
  assert.equal(trackerApiSite("gitlab", "https://gitlab.com"), "https://gitlab.com/api/v4");
  assert.equal(trackerApiSite("gitlab", "https://git.example/group/"), "https://git.example/group/api/v4");
  assert.equal(trackerApiSite("gitlab", "https://gitlab.com/api/v4/"), "https://gitlab.com/api/v4");
  assert.equal(trackerApiSite("gitlab", ""), "");
  assert.equal(trackerApiSite("jira", "https://example.atlassian.net/"), "https://example.atlassian.net");
});
