import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { getComposer, press, setCaret, typeIn, valueOf } from "./composer-editor";
import * as api from "../src/lib/conversation";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));

// jsdom has no ResizeObserver: a stub keeps the callbacks so a test can play a resize.
const observers: { callback: ResizeObserverCallback; targets: Element[] }[] = [];
class StubResizeObserver {
  entry: { callback: ResizeObserverCallback; targets: Element[] };
  constructor(callback: ResizeObserverCallback) { this.entry = { callback, targets: [] }; observers.push(this.entry); }
  observe(target: Element) { this.entry.targets.push(target); }
  unobserve() {}
  disconnect() { this.entry.targets = []; }
}
function resize(selector: string, height: number) {
  act(() => {
    for (const observer of observers) {
      const target = observer.targets.find(element => element.matches(selector));
      if (target) observer.callback([{ target, contentRect: { height } } as unknown as ResizeObserverEntry], observer as unknown as ResizeObserver);
    }
  });
}
beforeEach(() => {
  vi.resetAllMocks();
  observers.length = 0;
  vi.stubGlobal("ResizeObserver", StubResizeObserver);
  vi.mocked(api.controlConversation).mockResolvedValue({ accepted: true });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

const conversation = (id: string, busy = false) => ({
  id, busy, readOnly: false,
  events: [
    JSON.stringify({ kind: "user", text: "First prompt" }),
    JSON.stringify({ kind: "assistant", text: "First answer" }),
    JSON.stringify({ kind: "user", text: "Second prompt\nwith two lines\n\nRemote execution runId: 0a1b-2c." }),
    JSON.stringify({ kind: "assistant", text: "Second answer" }),
  ],
});
async function openConversation(busy = false) {
  vi.mocked(api.readConversation).mockResolvedValue(conversation("history-chat", busy));
  render(<ProjectPrompt projectId="p" runId="history-chat" onCreated={vi.fn()} />);
  await screen.findByText("Second answer");
  return getComposer();
}
const value = () => valueOf(getComposer());
const caret = setCaret;

test("ArrowUp in an empty conversation composer recalls prompts and ArrowDown clears past the newest", async () => {
  const input = await openConversation();
  press(input, "ArrowUp");
  expect(value()).toBe("Second prompt\nwith two lines");
  caret(input, 0);
  press(input, "ArrowUp");
  expect(value()).toBe("First prompt");
  press(input, "ArrowDown");
  expect(value()).toBe("Second prompt\nwith two lines");
  caret(input, value().length);
  press(input, "ArrowDown");
  expect(value()).toBe("");
});

test("arrows move the caret inside a recalled multi-line prompt and never replace a typed draft", async () => {
  const input = await openConversation();
  press(input, "ArrowUp");
  caret(input, value().length);
  press(input, "ArrowUp");
  expect(value()).toBe("Second prompt\nwith two lines");
  typeIn(input, "My own draft");
  press(input, "ArrowUp");
  press(input, "Escape");
  expect(value()).toBe("My own draft");
});

test("modifiers and IME composition never recall", async () => {
  const input = await openConversation();
  press(input, "ArrowUp", { shiftKey: true });
  press(input, "ArrowUp", { metaKey: true });
  expect(value()).toBe("");
  press(input, "ArrowUp", { isComposing: true });
  expect(value()).toBe("");
  press(input, "ArrowUp");
  expect(value()).toBe("Second prompt\nwith two lines");
});

test("Escape clears a recalled prompt without interrupting, then interrupts the running turn", async () => {
  const input = await openConversation(true);
  press(input, "ArrowUp");
  expect(value()).toBe("Second prompt\nwith two lines");
  press(input, "Escape");
  expect(value()).toBe("");
  expect(api.controlConversation).not.toHaveBeenCalledWith("history-chat", { action: "interrupt" });
  press(input, "Escape");
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("history-chat", { action: "interrupt" }));
});

test("a sent recall resets history to the newest prompt", async () => {
  const input = await openConversation();
  press(input, "ArrowUp");
  caret(input, 0);
  press(input, "ArrowUp");
  press(input, "Enter");
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("history-chat", { action: "message", message: "First prompt" }));
  expect(value()).toBe("");
});

test("the new prompt view keeps native arrows and both composers size against their view", async () => {
  render(<ProjectPrompt projectId="p" onCreated={vi.fn()} />);
  const prompt = screen.getByRole("textbox", { name: "Free prompt" }) as HTMLTextAreaElement;
  const notHandled = fireEvent.keyDown(prompt, { key: "ArrowUp" });
  expect(notHandled).toBe(true);
  expect(prompt.value).toBe("");
  // jsdom lays nothing out: the cap starts from the window height, then follows the observed view.
  expect(prompt.style.getPropertyValue("--composer-max-height")).toBe(`${Math.round(window.innerHeight * 0.4)}px`);
  resize(".project-prompt", 1000);
  expect(prompt.style.getPropertyValue("--composer-max-height")).toBe("400px");
  cleanup();

  const input = await openConversation();
  const wrapper = input.closest<HTMLElement>(".conversation-input")!;
  resize(".conversation-main-column", 800);
  expect(wrapper.style.getPropertyValue("--composer-max-height")).toBe("320px");
  resize(".conversation-main-column", 100);
  expect(wrapper.style.getPropertyValue("--composer-max-height")).toBe("150px");
});

test("the new prompt composer height follows its content", () => {
  render(<ProjectPrompt projectId="p" onCreated={vi.fn()} />);
  const input = screen.getByRole("textbox", { name: "Free prompt" }) as HTMLTextAreaElement;
  let scrollHeight = 48;
  Object.defineProperty(input, "scrollHeight", { configurable: true, get: () => scrollHeight });
  scrollHeight = 120;
  fireEvent.change(input, { target: { value: "one\ntwo\nthree\nfour\nfive" } });
  expect(input.style.height).toBe("120px");
  expect(input.style.overflowY).toBe("hidden");
  scrollHeight = 48;
  fireEvent.change(input, { target: { value: "" } });
  expect(input.style.height).toBe("48px");
});
