import test from "node:test";
import assert from "node:assert/strict";
import { commandChip, commandText, completionContext, isShellDraft, rankCommands } from "../src/domain/command-completion.ts";

const commands = [
  { name: "review", description: "Review the branch" },
  { name: "clarify-issue", description: "Clarify a ticket", argumentHint: "<ticket>" },
  { name: "issue", description: "Open an issue" },
  { name: "compact", description: "Summarise the conversation" },
  { name: "$code-issue", description: "Implement a ticket" },
  { name: "$review-pr", description: "Review a pull request" },
];

test("completion opens only inside the first token starting with / or $", () => {
  assert.deepEqual(completionContext("/rev", 4), { trigger: "/", query: "rev", from: 0, to: 4 });
  assert.deepEqual(completionContext("  $co", 5), { trigger: "$", query: "co", from: 2, to: 5 });
  assert.deepEqual(completionContext("/review now", 3), { trigger: "/", query: "re", from: 0, to: 7 });
  assert.equal(completionContext("/review now", 9), undefined);
  assert.equal(completionContext("please /review", 14), undefined);
  assert.equal(completionContext("/review", 0), undefined);
  assert.equal(completionContext("plain", 3), undefined);
});

test("ranking: exact, prefix, substring, then description, each trigger its own list", () => {
  const names = (trigger: "/" | "$", query: string) => rankCommands(commands, trigger, query).map(entry => entry.command.name);
  assert.deepEqual(names("/", "issue"), ["issue", "clarify-issue"]);
  assert.deepEqual(names("/", "c"), ["clarify-issue", "compact", "review"]);
  assert.deepEqual(names("/", "ticket"), ["clarify-issue"]);
  assert.deepEqual(names("/", ""), ["review", "clarify-issue", "issue", "compact"]);
  assert.deepEqual(names("$", "review"), ["$review-pr"]);
  assert.deepEqual(names("$", ""), ["$code-issue", "$review-pr"]);
  assert.deepEqual(rankCommands(commands, "/", "zzz"), []);
  assert.deepEqual(rankCommands(null, "/", ""), []);
  assert.equal(rankCommands(Array.from({ length: 80 }, (_, i) => ({ name: `c${i}` })), "/", "c").length, 50);
});

test("matches carry the highlighted range in the name or the description", () => {
  assert.deepEqual(rankCommands(commands, "/", "ARIF")[0].match, { field: "name", from: 2, to: 6 });
  assert.deepEqual(rankCommands(commands, "$", "co")[0].match, { field: "name", from: 1, to: 3 });
  assert.deepEqual(rankCommands(commands, "/", "branch")[0].match, { field: "description", from: 11, to: 17 });
});

test("a known first token followed by whitespace is a chip; an unknown one is not", () => {
  assert.equal(commandText(commands[0], "/"), "/review");
  assert.equal(commandText(commands[4], "$"), "$code-issue");
  assert.deepEqual(commandChip("/review now", commands), { from: 0, to: 7, kind: "command", label: "/review" });
  assert.deepEqual(commandChip(" $code-issue #42", commands), { from: 1, to: 12, kind: "skill", label: "$code-issue" });
  assert.equal(commandChip("/review", commands), undefined);
  assert.equal(commandChip("/unknown now", commands), undefined);
  assert.equal(commandChip("$review now", commands), undefined);
});

test("a draft starting with ! is a shell command", () => {
  assert.equal(isShellDraft("  !ls"), true);
  assert.equal(isShellDraft("ls !"), false);
});
