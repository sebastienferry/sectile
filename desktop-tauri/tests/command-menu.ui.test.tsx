import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { EditorView } from "@codemirror/view";
import { deleteCharBackward } from "@codemirror/commands";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { findComposer, press, setCaret, typeIn, valueOf } from "./composer-editor";
import * as api from "../src/lib/conversation";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.controlConversation).mockResolvedValue({ accepted: true });
});
afterEach(cleanup);

const commands = [
  { name: "review", description: "Review the work" },
  { name: "clarify-issue", description: "Clarify a ticket", argumentHint: "<ticket>" },
  { name: "compact", description: "Summarise the conversation" },
  { name: "$code-issue", description: "Implement a ticket" },
];
async function open(busy = false, list: typeof commands | null = commands) {
  vi.mocked(api.readConversation).mockResolvedValue({ id: "commands-chat", busy, readOnly: false, provider: "claude", commands: list, events: [JSON.stringify({ kind: "assistant", text: "Ready" })] });
  render(<ProjectPrompt projectId="p" runId="commands-chat" onCreated={vi.fn()} />);
  await screen.findByText("Ready");
  return findComposer();
}
const menu = () => screen.queryByRole("listbox");
const options = () => within(screen.getByRole("listbox")).getAllByRole("option");

test("typing / lists the slash commands ranked, with the match highlighted and ARIA wired", async () => {
  const editor = await open();
  expect(editor.getAttribute("aria-expanded")).toBe("false");
  typeIn(editor, "/c");
  const list = screen.getByRole("listbox", { name: "Slash commands" });
  expect(options().map(option => option.querySelector(".completion-menu-label")?.textContent)).toEqual(["/clarify-issue", "/compact"]);
  expect(list.querySelector("mark")?.textContent).toBe("c");
  expect(within(list).getByText("<ticket>")).toBeTruthy();
  expect(editor.getAttribute("aria-expanded")).toBe("true");
  expect(editor.getAttribute("aria-controls")).toBe(list.id);
  expect(editor.getAttribute("aria-activedescendant")).toBe(options()[0].id);
  expect(options()[0].getAttribute("aria-selected")).toBe("true");
});

test("$ lists Codex skills only, and nothing opens past the first token or in shell mode", async () => {
  const editor = await open();
  typeIn(editor, "$");
  expect(screen.getByRole("listbox", { name: "Skills" })).toBeTruthy();
  expect(options().map(option => option.textContent)).toEqual(["$code-issueImplement a ticket"]);
  typeIn(editor, "please /re");
  expect(menu()).toBeNull();
  typeIn(editor, "!ls /re");
  expect(menu()).toBeNull();
  typeIn(editor, "/zzz");
  expect(menu()).toBeNull();
});

test("arrows cycle, Enter and Tab insert the entry and a space, then a chip shows", async () => {
  const editor = await open();
  typeIn(editor, "/c");
  press(editor, "ArrowDown");
  expect(editor.getAttribute("aria-activedescendant")).toBe(options()[1].id);
  press(editor, "ArrowDown");
  expect(editor.getAttribute("aria-activedescendant")).toBe(options()[0].id);
  press(editor, "ArrowUp");
  press(editor, "Enter");
  expect(valueOf(editor)).toBe("/compact ");
  expect(menu()).toBeNull();
  expect(api.controlConversation).not.toHaveBeenCalled();
  const chip = editor.querySelector(".composer-chip-command");
  expect(chip?.textContent).toBe("/compact");
  typeIn(editor, "$co");
  press(editor, "Tab");
  expect(valueOf(editor)).toBe("$code-issue ");
  expect(editor.querySelector(".composer-chip-skill")?.textContent).toBe("$code-issue");
});

test("a click inserts the entry; hovering makes it active", async () => {
  const editor = await open();
  typeIn(editor, "/");
  fireEvent.mouseEnter(options()[2]);
  expect(editor.getAttribute("aria-activedescendant")).toBe(options()[2].id);
  fireEvent.mouseDown(options()[1]);
  expect(valueOf(editor)).toBe("/clarify-issue ");
});

test("Escape closes the menu without interrupting, then interrupts the running turn", async () => {
  const editor = await open(true);
  typeIn(editor, "/re");
  expect(menu()).toBeTruthy();
  press(editor, "Escape");
  expect(menu()).toBeNull();
  expect(valueOf(editor)).toBe("/re");
  expect(api.controlConversation).not.toHaveBeenCalled();
  press(editor, "Escape");
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("commands-chat", { action: "interrupt" }));
  // Typing on reopens the menu for the new token.
  typeIn(editor, "/rev");
  expect(menu()).toBeTruthy();
});

test("only a known first command becomes a chip, deleted whole by Backspace, and the text is sent as typed", async () => {
  const editor = await open();
  typeIn(editor, "/unknown now");
  expect(editor.querySelector(".composer-chip-token")).toBeNull();
  typeIn(editor, "/review the work");
  expect(editor.querySelector(".composer-chip-command")?.textContent).toBe("/review");
  setCaret(editor, 7);
  act(() => { deleteCharBackward(EditorView.findFromDOM(editor)!); });
  expect(valueOf(editor)).toBe(" the work");
  typeIn(editor, "/review the work");
  press(editor, "Enter");
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("commands-chat", { action: "message", message: "/review the work" }));
});

test("a draft starting with ! shows the shell indicator, also while queueing", async () => {
  const editor = await open(true);
  const box = editor.closest(".composer-box")!;
  expect(box.classList.contains("shell-mode")).toBe(false);
  typeIn(editor, "!git status");
  expect(box.classList.contains("shell-mode")).toBe(true);
  expect(within(box as HTMLElement).getByText("Shell")).toBeTruthy();
  expect(editor.getAttribute("aria-description")).toContain("shell command in this conversation's folder");
  expect(screen.getByRole("button", { name: "Send" })).toBeTruthy();
  typeIn(editor, "git status");
  expect(box.classList.contains("shell-mode")).toBe(false);
  expect(editor.getAttribute("aria-description")).toBeNull();
});

test("no command list means no menu", async () => {
  const editor = await open(false, null);
  typeIn(editor, "/");
  expect(menu()).toBeNull();
});

test("a recalled command prompt does not open the menu, so Up keeps walking history", async () => {
  vi.mocked(api.readConversation).mockResolvedValue({ id: "recall-chat", busy: false, readOnly: false, provider: "claude", commands, events: [JSON.stringify({ kind: "user", text: "Explain" }), JSON.stringify({ kind: "user", text: "/review" }), JSON.stringify({ kind: "assistant", text: "Ready" })] });
  render(<ProjectPrompt projectId="p" runId="recall-chat" onCreated={vi.fn()} />);
  await screen.findByText("Ready");
  const editor = await findComposer();
  act(() => { EditorView.findFromDOM(editor)!.focus(); });
  press(editor, "ArrowUp");
  expect(valueOf(editor)).toBe("/review");
  expect(menu()).toBeNull();
  press(editor, "ArrowUp");
  expect(valueOf(editor)).toBe("Explain");
});
