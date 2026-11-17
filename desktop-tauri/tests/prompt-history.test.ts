import test from "node:test";
import assert from "node:assert/strict";
import { navigatePromptHistory, userPrompts, type PromptHistoryInput, type PromptRecall } from "../src/domain/prompt-history.ts";
import { composerMaxHeight } from "../src/lib/composer-sizing.ts";

const prompts = ["first", "second\nline two", "third"];
function press(key: string, value: string, recall: PromptRecall, extra: Partial<PromptHistoryInput> = {}) {
  const caret = extra.selectionStart ?? value.length;
  return navigatePromptHistory({ prompts, value, selectionStart: caret, selectionEnd: caret, recall, event: { key }, ...extra });
}

test("ArrowUp in an empty composer recalls the most recent prompt", () => {
  assert.deepEqual(press("ArrowUp", "", null), { handled: true, value: "third", recall: { index: 2, text: "third" } });
});

test("a typed draft is never replaced and other keys are ignored", () => {
  assert.equal(press("ArrowUp", "draft", null).handled, false);
  assert.equal(press("ArrowDown", "", null).handled, false);
  assert.equal(press("Escape", "", null).handled, false);
  assert.equal(press("Enter", "", null).handled, false);
  assert.equal(navigatePromptHistory({ prompts: [], value: "", selectionStart: 0, selectionEnd: 0, recall: null, event: { key: "ArrowUp" } }).handled, false);
});

test("ArrowUp and ArrowDown step through history and ArrowDown clears past the newest", () => {
  let step = press("ArrowUp", "", null);
  step = press("ArrowUp", "third", step.recall);
  assert.equal(step.handled && step.value, "second\nline two");
  // Caret at the end of a two-line prompt: not on its first row, ArrowUp moves the caret.
  assert.equal(press("ArrowUp", "second\nline two", step.recall).handled, false);
  step = press("ArrowUp", "second\nline two", step.recall, { selectionStart: 3 });
  assert.equal(step.handled && step.value, "first");
  assert.deepEqual(press("ArrowUp", "first", step.recall), { handled: false, recall: { index: 0, text: "first" } });
  step = press("ArrowDown", "first", step.recall);
  assert.equal(step.handled && step.value, "second\nline two");
  // ArrowDown with the caret on the first of two rows moves the caret.
  assert.equal(press("ArrowDown", "second\nline two", step.recall, { selectionStart: 2 }).handled, false);
  step = press("ArrowDown", "second\nline two", step.recall);
  assert.equal(step.handled && step.value, "third");
  assert.deepEqual(press("ArrowDown", "third", step.recall), { handled: true, value: "", recall: null });
});

test("Escape clears a recalled prompt", () => {
  assert.deepEqual(press("Escape", "third", { index: 2, text: "third" }), { handled: true, value: "", recall: null });
});

test("an edited recall becomes an ordinary draft", () => {
  const edited = press("ArrowUp", "third!", { index: 2, text: "third" });
  assert.deepEqual(edited, { handled: false, recall: null });
  assert.equal(press("Escape", "third!", { index: 2, text: "third" }).handled, false);
});

test("a recall over a changed prompt list is dropped", () => {
  assert.deepEqual(press("ArrowUp", "third", { index: 1, text: "third" }), { handled: false, recall: null });
});

test("modifiers, IME composition and a selection never recall", () => {
  for (const modifier of ["shiftKey", "ctrlKey", "altKey", "metaKey", "isComposing"] as const) {
    assert.equal(navigatePromptHistory({ prompts, value: "", selectionStart: 0, selectionEnd: 0, recall: null, event: { key: "ArrowUp", [modifier]: true } }).handled, false, modifier);
  }
  assert.equal(press("ArrowUp", "", null, { composing: true }).handled, false);
  assert.equal(navigatePromptHistory({ prompts, value: "third", selectionStart: 0, selectionEnd: 3, recall: { index: 2, text: "third" }, event: { key: "ArrowUp" } }).handled, false);
});

test("the rendered-row check overrides logical lines when the layout knows", () => {
  const recall = { index: 2, text: "third" };
  assert.equal(press("ArrowUp", "third", recall, { caretOnEdgeRow: () => false }).handled, false);
  assert.equal(press("ArrowUp", "third", recall, { caretOnEdgeRow: () => undefined }).handled, true);
  const wrapped = press("ArrowUp", "second\nline two", { index: 1, text: "second\nline two" }, { caretOnEdgeRow: () => true });
  assert.equal(wrapped.handled && wrapped.value, "first");
});

test("user prompts are the cleaned text of user events, empty ones dropped", () => {
  const events = [
    { kind: "user", text: "Explain\n\nRemote execution runId: 0a1b-2c." },
    { kind: "assistant", text: "Answer" },
    { kind: "user", text: "  " },
    { kind: "user", text: "/pickup-issue gh-d7a52c1c-ba5b-4e78-863e-bc2e33d478a4-913" },
  ];
  assert.deepEqual(userPrompts(events), ["Explain", "/pickup-issue 913"]);
});

test("the composer cap is 40% of the view, never under 150px", () => {
  assert.equal(composerMaxHeight(1000), 400);
  assert.equal(composerMaxHeight(200), 150);
  assert.equal(composerMaxHeight(0), 150);
});
