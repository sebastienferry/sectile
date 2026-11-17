import assert from "node:assert/strict";
import test from "node:test";
import { emptyQueue, enqueue, MESSAGE_LIMIT, QUEUE_LIMIT, mergeDraft, remove, turnOutcome, withdraw, type MessageQueue } from "../src/lib/conversation-queue.ts";
import type { ConversationEvent } from "../src/lib/conversation.ts";

const queued = (queue: MessageQueue | string): MessageQueue => {
  assert.equal(typeof queue, "object", `expected a queue, got ${String(queue)}`);
  return queue as MessageQueue;
};

test("enqueue appends in insertion order and captures the turn options", () => {
  const options = { model: "opus", mode: "plan" as const };
  let queue = queued(enqueue(emptyQueue(), "first", options, "a"));
  options.model = "changed later";
  queue = queued(enqueue(queue, "second", {}, "b"));
  assert.deepEqual(queue.items.map(item => [item.id, item.text, item.status]), [["a", "first", "queued"], ["b", "second", "queued"]]);
  assert.deepEqual(queue.items[0].options, { model: "opus", mode: "plan" });
});

test("enqueue refuses empty drafts, oversized messages and a full queue", () => {
  assert.equal(enqueue(emptyQueue(), "   \n"), "empty");
  assert.equal(enqueue(emptyQueue(), "x".repeat(MESSAGE_LIMIT + 1)), "tooLong");
  assert.equal(typeof enqueue(emptyQueue(), "x".repeat(MESSAGE_LIMIT)), "object");
  let queue = emptyQueue();
  for (let index = 0; index < QUEUE_LIMIT; index++) queue = queued(enqueue(queue, `message ${index}`));
  assert.equal(queue.items.length, 20);
  assert.equal(enqueue(queue, "one too many"), "full");
});

test("remove keeps the order of the others and refuses a message being sent", () => {
  let queue = queued(enqueue(emptyQueue(), "A", {}, "a"));
  queue = queued(enqueue(queue, "B", {}, "b"));
  queue = queued(enqueue(queue, "C", {}, "c"));
  assert.deepEqual(remove(queue, "b").items.map(item => item.text), ["A", "C"]);
  const sending = { ...queue, items: queue.items.map(item => item.id === "a" ? { ...item, status: "sending" as const } : item) };
  assert.equal(remove(sending, "a"), sending);
  assert.equal(remove(queue, "unknown"), queue);
});

test("removing the last message clears the pause", () => {
  const queue = { ...queued(enqueue(emptyQueue(), "A", {}, "a")), paused: "sendFailed" as const };
  assert.deepEqual(remove(queue, "a"), { items: [] });
  const twoItems = { ...queued(enqueue(queue, "B", {}, "b")), paused: "interrupt" as const };
  assert.equal(remove(twoItems, "a").paused, "interrupt");
});

const event = (kind: string, text = "", extra: Partial<ConversationEvent> = {}): ConversationEvent => ({ kind, text, ...extra });

test("turnOutcome reads the end of the last turn from the Go transcript", () => {
  assert.equal(turnOutcome([]), "ok");
  assert.equal(turnOutcome([event("user", "Hi"), event("assistant", "Hello"), event("notice", "Ready for your next message")]), "ok");
  // Strings written by internal/agent when a turn fails or is interrupted.
  assert.equal(turnOutcome([event("user", "Hi"), event("error", "Claude Code stopped without a final result.")]), "error");
  assert.equal(turnOutcome([event("user", "Hi"), event("notice", "Interrupted", { detail: "Claude Code stopped this answer." })]), "interrupted");
  assert.equal(turnOutcome([event("user", "Hi"), event("assistant", "Half an answer", { interrupted: true })]), "interrupted");
});

test("turnOutcome ignores errors from earlier turns", () => {
  assert.equal(turnOutcome([event("user", "Hi"), event("error", "Old failure"), event("user", "Again"), event("assistant", "Fine")]), "ok");
  assert.equal(turnOutcome([event("user", "Hi"), event("notice", "Interrupted"), event("user", "Again"), event("assistant", "Fine")]), "ok");
});

test("withdraw takes a queued or failed message back, never one being sent", () => {
  const queue: MessageQueue = { paused: "sendFailed", items: [
    { id: "a", text: "A", options: { model: "opus" }, status: "failed", error: "boom" },
    { id: "b", text: "B", options: {}, status: "queued" },
  ] };
  const first = withdraw(queue, "a");
  assert.equal(first?.item.text, "A");
  assert.deepEqual(first?.item.options, { model: "opus" });
  assert.deepEqual(first?.queue.items.map(item => item.id), ["b"]);
  assert.equal(first?.queue.paused, "sendFailed");
  // The last message withdrawn leaves nothing to hold back.
  assert.deepEqual(withdraw(first!.queue, "b")?.queue, { items: [] });
  assert.equal(withdraw({ items: [{ id: "c", text: "C", options: {}, status: "sending" }] }, "c"), undefined);
  assert.equal(withdraw(queue, "missing"), undefined);
});

test("a withdrawn message joins the draft after a blank line", () => {
  assert.equal(mergeDraft("", "Queued"), "Queued");
  assert.equal(mergeDraft("  \n", "Queued"), "Queued");
  assert.equal(mergeDraft("Draft\n", "Queued"), "Draft\n\nQueued");
});
