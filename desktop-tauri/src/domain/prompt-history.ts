// Up/Down recall of the prompts already sent in a conversation (#913).
//
// Framework-free so the rules are unit tested without a DOM. A recall starts
// only from an empty composer, and stays active only while the composer shows
// exactly the recalled text: the moment the user edits it, it becomes an
// ordinary draft and the arrows move the caret again. No draft the user typed
// is ever replaced.

import type { ConversationEvent } from "../lib/conversation.ts";
import { cleanUserEventText } from "../lib/user-event-text.ts";

/** The prompt currently recalled: its index in the prompt list and its text. */
export type PromptRecall = { index: number; text: string } | null;

export interface HistoryKeyEvent {
  key: string;
  shiftKey?: boolean;
  ctrlKey?: boolean;
  altKey?: boolean;
  metaKey?: boolean;
  isComposing?: boolean;
}

export interface PromptHistoryInput {
  /** Sent prompts, oldest first. */
  prompts: readonly string[];
  value: string;
  selectionStart: number | null;
  selectionEnd: number | null;
  recall: PromptRecall;
  event: HistoryKeyEvent;
  /** True while an IME composition is in progress. */
  composing?: boolean;
  /**
   * Whether the caret is on the first ("up") or last ("down") rendered row.
   * Returns undefined when the layout cannot tell, and logical lines are used.
   */
  caretOnEdgeRow?: (direction: "up" | "down") => boolean | undefined;
}

export type PromptHistoryResult =
  | { handled: false; recall: PromptRecall }
  | { handled: true; value: string; recall: PromptRecall };

/** The text of every user message of the conversation, in order, empty ones dropped. */
export function userPrompts(events: readonly ConversationEvent[]): string[] {
  return events
    .filter(event => event.kind === "user" && typeof event.text === "string")
    .map(event => cleanUserEventText(event.text))
    .filter(text => text.length > 0);
}

/** Whether `recall` is still what the composer shows, over the current prompt list. */
export function isRecalling(prompts: readonly string[], value: string, recall: PromptRecall): boolean {
  return recall !== null && value === recall.text && prompts[recall.index] === recall.text;
}

function logicalEdge(value: string, caret: number, direction: "up" | "down"): boolean {
  return direction === "up" ? !value.slice(0, caret).includes("\n") : !value.slice(caret).includes("\n");
}

export function navigatePromptHistory(input: PromptHistoryInput): PromptHistoryResult {
  const { prompts, value, event } = input;
  const recall = isRecalling(prompts, value, input.recall) ? input.recall : null;
  const ignored: PromptHistoryResult = { handled: false, recall };
  if (input.composing || event.isComposing || event.shiftKey || event.ctrlKey || event.altKey || event.metaKey) return ignored;
  if (event.key !== "ArrowUp" && event.key !== "ArrowDown" && event.key !== "Escape") return ignored;

  if (!recall) {
    if (event.key !== "ArrowUp" || value.length !== 0 || prompts.length === 0) return ignored;
    const index = prompts.length - 1;
    return { handled: true, value: prompts[index], recall: { index, text: prompts[index] } };
  }
  const cleared: PromptHistoryResult = { handled: true, value: "", recall: null };
  if (event.key === "Escape") return cleared;

  const { selectionStart, selectionEnd } = input;
  if (selectionStart === null || selectionStart !== selectionEnd) return ignored;
  const direction = event.key === "ArrowUp" ? "up" : "down";
  const onEdge = input.caretOnEdgeRow?.(direction) ?? logicalEdge(value, selectionStart, direction);
  if (!onEdge) return ignored;

  if (direction === "up") {
    if (recall.index === 0) return ignored;
    const index = recall.index - 1;
    return { handled: true, value: prompts[index], recall: { index, text: prompts[index] } };
  }
  if (recall.index === prompts.length - 1) return cleared;
  const index = recall.index + 1;
  return { handled: true, value: prompts[index], recall: { index, text: prompts[index] } };
}
