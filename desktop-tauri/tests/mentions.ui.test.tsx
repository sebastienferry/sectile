import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { EditorView } from "@codemirror/view";
import { deleteCharBackward } from "@codemirror/commands";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { findComposer, getComposer, press, setCaret, typeIn, valueOf } from "./composer-editor";
import * as api from "../src/lib/conversation";
import * as mentions from "../src/lib/mentions";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));
vi.mock("../src/lib/mentions", () => ({ mentionFiles: vi.fn() }));

const files: mentions.MentionFile[] = [
  { root: "/work/app", rootLabel: "Working folder", path: "src", absolute: "/work/app/src", kind: "folder" },
  { root: "/work/app", rootLabel: "Working folder", path: "src/cache.ts", absolute: "/work/app/src/cache.ts", kind: "file" },
  { root: "/notes", rootLabel: "notes", path: "cache-plan.md", absolute: "/notes/cache-plan.md", kind: "file" },
];
const sources = {
  conversations: [
    { id: "chat", title: "This chat", createdAt: "2026-11-17" },
    { id: "older", title: "Cache design", createdAt: "2026-11-01", archived: true },
  ],
  tickets: [{ id: "gh-1-913", key: "#913", title: "Composer cache" }, { id: "gh-1-12", key: "#12", title: "Other" }],
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.controlConversation).mockResolvedValue({ accepted: true });
  vi.mocked(mentions.mentionFiles).mockImplementation(async (_project, _run, query) => files.filter(file => file.path.includes(query)));
});
afterEach(cleanup);

async function openChat(busy = false, events: string[] = [JSON.stringify({ kind: "assistant", text: "Ready" })]) {
  vi.mocked(api.readConversation).mockResolvedValue({ id: "chat", busy, readOnly: false, provider: "claude", events });
  render(<ProjectPrompt projectId="p" runId="chat" mentionSources={sources} onCreated={vi.fn()} />);
  await screen.findByText("Ready");
  return findComposer();
}
const menu = () => screen.queryByRole("listbox", { name: "Mentions" });
const labels = () => within(screen.getByRole("listbox", { name: "Mentions" })).getAllByRole("option").map(option => option.querySelector(".completion-menu-label")?.textContent);

test("@ lists files and folders, other conversations and tickets, with highlights", async () => {
  const editor = await openChat();
  typeIn(editor, "Look at @cache");
  await waitFor(() => expect(labels()).toEqual(["src/cache.ts", "cache-plan.md", "Cache design", "#913"]));
  expect(mentions.mentionFiles).toHaveBeenLastCalledWith("p", "chat", "cache");
  const list = screen.getByRole("listbox", { name: "Mentions" });
  expect(within(list).getByText("Archived")).toBeTruthy();
  expect(within(list).getByText("notes")).toBeTruthy();
  expect(Array.from(list.querySelectorAll("mark")).map(mark => mark.textContent)).toEqual(["cache", "cache", "Cache", "cache"]);
  expect(labels()).not.toContain("This chat");
});

test("no menu inside a word, and Escape closes it without interrupting", async () => {
  const editor = await openChat(true);
  typeIn(editor, "mail me@cache");
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 150)); });
  expect(menu()).toBeNull();
  typeIn(editor, "@");
  await waitFor(() => expect(menu()).toBeTruthy());
  press(editor, "Escape");
  expect(menu()).toBeNull();
  expect(api.controlConversation).not.toHaveBeenCalled();
});

test("a picked file becomes a chip deleted whole by Backspace", async () => {
  const editor = await openChat();
  typeIn(editor, "Read @cache.ts");
  await waitFor(() => expect(labels()?.[0]).toBe("src/cache.ts"));
  press(editor, "Enter");
  const token = "@[cache.ts](sectile:file/%2Fwork%2Fapp%2Fsrc%2Fcache.ts)";
  expect(valueOf(editor)).toBe(`Read ${token} `);
  expect(api.controlConversation).not.toHaveBeenCalled();
  const chip = editor.querySelector(".composer-chip-file");
  expect(chip?.textContent).toBe("cache.ts");
  expect(chip?.querySelector("svg")).toBeTruthy();
  setCaret(editor, `Read ${token}`.length);
  act(() => { deleteCharBackward(EditorView.findFromDOM(editor)!); });
  expect(valueOf(editor)).toBe("Read  ");
});

test("a picked folder is text that keeps browsing inside it", async () => {
  const editor = await openChat();
  typeIn(editor, "@sr");
  await waitFor(() => expect(labels()?.[0]).toBe("src/"));
  press(editor, "Tab");
  expect(valueOf(editor)).toBe("@src/");
  await waitFor(() => expect(mentions.mentionFiles).toHaveBeenLastCalledWith("p", "chat", "src/"));
  await waitFor(() => expect(labels()).toEqual(["src/cache.ts"]));
});

test("a clicked ticket and conversation are sent as tokens and shown as chips in the transcript", async () => {
  const editor = await openChat();
  typeIn(editor, "Compare @913");
  await waitFor(() => expect(labels()).toEqual(["#913"]));
  fireEvent.mouseDown(within(screen.getByRole("listbox")).getAllByRole("option")[0]);
  typeIn(editor, `${valueOf(editor)}with @design`);
  await waitFor(() => expect(labels()).toEqual(["Cache design"]));
  press(editor, "Enter");
  const message = "Compare @[#913](sectile:ticket/gh-1-913) with @[Cache design](sectile:conversation/older) ";
  expect(valueOf(editor)).toBe(message);
  expect(editor.querySelector(".composer-chip-ticket")?.textContent).toBe("#913");
  expect(editor.querySelector(".composer-chip-conversation")?.textContent).toBe("Cache design");
  press(editor, "Enter");
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("chat", { action: "message", message }));
});

test("the transcript shows a sent message's mentions as chips", async () => {
  await openChat(false, [JSON.stringify({ kind: "user", text: "See @[#913](sectile:ticket/gh-1-913) now" }), JSON.stringify({ kind: "assistant", text: "Ready" })]);
  const chip = document.querySelector(".prompt-transcript .composer-chip-ticket");
  expect(chip?.textContent).toBe("#913");
  expect(chip?.closest("p")?.textContent).toBe("See #913 now");
});

test("the new prompt composer mentions too, without a conversation", async () => {
  render(<ProjectPrompt projectId="p" mentionSources={sources} onCreated={vi.fn()} />);
  const editor = getComposer("Free prompt");
  typeIn(editor, "@plan");
  await waitFor(() => expect(within(screen.getByRole("listbox", { name: "Mentions" })).getAllByRole("option").length).toBe(1));
  expect(mentions.mentionFiles).toHaveBeenLastCalledWith("p", undefined, "plan");
});
