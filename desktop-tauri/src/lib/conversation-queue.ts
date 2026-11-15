import type { ConversationEvent, TurnOptions } from "./conversation.ts";

/** Follow-up messages a conversation may hold before the user has to remove one. */
export const QUEUE_LIMIT = 20;
/** Same limit as the composer's textarea for a message sent at once. */
export const MESSAGE_LIMIT = 32000;

export type QueueItemStatus = "queued" | "sending" | "failed";
export interface QueuedMessage {
  /** Local identity, never sent to the agent. */
  id: string;
  text: string;
  /** Model and mode selected when the message was queued, used when it is dispatched. */
  options: TurnOptions;
  status: QueueItemStatus;
  /** Why the send failed, set when status is "failed". */
  error?: string;
}
export type PauseReason = "interrupt" | "turnError" | "sendFailed";
/** Messages waiting for their own turn, in dispatch order. Held in memory only. */
export interface MessageQueue { items: QueuedMessage[]; paused?: PauseReason }
export type EnqueueRefusal = "empty" | "full" | "tooLong";

export const emptyQueue = (): MessageQueue => ({ items: [] });

let sequence = 0;
const nextId = () => globalThis.crypto?.randomUUID?.() ?? `queued-${Date.now()}-${++sequence}`;

export function enqueue(queue: MessageQueue, text: string, options: TurnOptions = {}, id = nextId()): MessageQueue | EnqueueRefusal {
  if (!text.trim()) return "empty";
  if (text.length > MESSAGE_LIMIT) return "tooLong";
  if (queue.items.length >= QUEUE_LIMIT) return "full";
  return { ...queue, items: [...queue.items, { id, text, options: { ...options }, status: "queued" }] };
}

/** Drops a message that has not left yet. A message being sent stays: its outcome decides. */
export function remove(queue: MessageQueue, id: string): MessageQueue {
  const item = queue.items.find(candidate => candidate.id === id);
  if (!item || item.status === "sending") return queue;
  return settle({ ...queue, items: queue.items.filter(candidate => candidate !== item) });
}

/** An empty queue has nothing left to hold back, so it is never paused. */
export function settle(queue: MessageQueue): MessageQueue {
  return queue.items.length || !queue.paused ? queue : { items: queue.items };
}

/**
 * Reads how the last turn ended from the transcript the Go agent writes: after the last
 * `user` event, an "Interrupted" notice (or an interrupted partial answer) means the turn
 * was stopped, and an `error` event means it failed. Both strings come from
 * internal/agent/agent_conversation.go and agent_codex_conversation.go.
 */
export function turnOutcome(events: ConversationEvent[]): "ok" | "error" | "interrupted" {
  let start = 0;
  for (let index = events.length - 1; index >= 0; index--) {
    if (events[index].kind === "user") { start = index + 1; break; }
  }
  const tail = events.slice(start);
  if (tail.some(event => (event.kind === "notice" && event.text === "Interrupted") || event.interrupted)) return "interrupted";
  if (tail.some(event => event.kind === "error")) return "error";
  return "ok";
}
