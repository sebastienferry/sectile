import test from "node:test";
import assert from "node:assert/strict";
import { mentionChips, mentionContext, mentionToken, rankConversations, rankTickets, splitMentions } from "../src/domain/mentions.ts";

test("@ opens at the start or after whitespace, never inside a word or an inserted token", () => {
  assert.deepEqual(mentionContext("@src", 4), { query: "src", from: 0, to: 4 });
  assert.deepEqual(mentionContext("see @docs/ap now", 12), { query: "docs/ap", from: 4, to: 12 });
  assert.deepEqual(mentionContext("see @doc", 6), { query: "d", from: 4, to: 8 });
  assert.equal(mentionContext("mail me@example.com", 19), undefined);
  assert.equal(mentionContext("see @[a](sectile:file/x)", 10), undefined);
  assert.equal(mentionContext("see @x y", 8), undefined);
});

test("tokens are serialized safely and found again as chips and transcript parts", () => {
  const token = mentionToken("file", "/tmp/a b/[x].md", "[x].md");
  assert.equal(token, "@[x .md](sectile:file/%2Ftmp%2Fa%20b%2F%5Bx%5D.md)");
  const text = `Read ${token} and @[#4](sectile:ticket/gh-1-4).`;
  assert.deepEqual(mentionChips(text).map(chip => [chip.kind, chip.label, text.slice(chip.from, chip.to)]), [
    ["file", "x .md", token],
    ["ticket", "#4", "@[#4](sectile:ticket/gh-1-4)"],
  ]);
  assert.deepEqual(splitMentions(text), [
    { text: "Read " },
    { mention: { kind: "file", label: "x .md", id: "/tmp/a b/[x].md" } },
    { text: " and " },
    { mention: { kind: "ticket", label: "#4", id: "gh-1-4" } },
    { text: "." },
  ]);
});

test("conversations match their title, newest first; tickets put a key prefix first", () => {
  const conversations = [
    { id: "a", title: "Cache design", createdAt: "2026-11-01" },
    { id: "b", title: "Other", createdAt: "2026-11-03" },
    { id: "c", title: "Cache metrics", createdAt: "2026-11-02", archived: true },
  ];
  assert.deepEqual(rankConversations(conversations, "cache", 10).map(entry => entry.item.id), ["c", "a"]);
  const tickets = [
    { id: "t1", key: "#191", title: "Fix the cache" },
    { id: "t2", key: "#19", title: "Plan" },
    { id: "t3", key: "#219", title: "Docs" },
  ];
  assert.deepEqual(rankTickets(tickets, "#19", 10).map(entry => entry.item.id), ["t1", "t2", "t3"]);
  assert.deepEqual(rankTickets(tickets, "cache", 10).map(entry => [entry.item.id, entry.field, entry.from]), [["t1", "detail", 8]]);
});
