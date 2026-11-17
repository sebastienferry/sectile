import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { findComposer, getComposer, press, queryComposer, typeIn, valueOf } from "./composer-editor";
import { conversationFor } from "../src/lib/conversation-controller";
import * as api from "../src/lib/conversation";
import type { ConversationSnapshot } from "../src/lib/conversation";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));

/** What the fake Go agent reports for each conversation; a sent message starts a turn. */
let agent: Record<string, ConversationSnapshot> = {};
const line = (kind: string, text: string) => JSON.stringify({ kind, text });
const busy = (id: string): ConversationSnapshot => ({ id, busy: true, readOnly: false, events: [line("user", "Start"), line("assistant", "Working")] });
const idle = (id: string, ...events: string[]): ConversationSnapshot => ({ id, busy: false, readOnly: false, events: [line("user", "Start"), line("assistant", "Done"), ...events] });
const finishTurn = (id: string, ...events: string[]) => { agent[id] = { ...agent[id], busy: false, events: [...agent[id].events ?? [], line("assistant", "Done"), ...events] }; };
const sent = () => vi.mocked(api.controlConversation).mock.calls.filter(([, input]) => input.action === "message").map(([id, input]) => [id, (input as { message: string }).message]);
let ids = 0;
const fresh = (name: string) => `${name}-${++ids}`;

beforeEach(() => {
  vi.resetAllMocks();
  agent = {};
  vi.mocked(api.readConversation).mockImplementation(async id => {
    if (!agent[id]) throw new Error("Agent unavailable");
    return agent[id];
  });
  vi.mocked(api.controlConversation).mockImplementation(async (id, input) => {
    if (input.action === "message") agent[id] = { ...agent[id], busy: true, events: [...agent[id].events ?? [], line("user", input.message)] };
    return { accepted: true };
  });
});
afterEach(() => { cleanup(); vi.useRealTimers(); });

describe("conversation controller queue", () => {
  beforeEach(() => { vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] }); });
  /** Runs pending promises without firing the 750 ms polling timer. */
  const flush = () => vi.advanceTimersByTimeAsync(0);
  async function loaded(id: string, snapshot: ConversationSnapshot) {
    agent[id] = snapshot;
    const controller = conversationFor(id);
    await controller.refresh();
    await flush();
    return controller;
  }
  async function read(controller: ReturnType<typeof conversationFor>) { await controller.refresh(); await flush(); }

  test("queued messages are sent one per turn, in order, with their own options, never interrupting", async () => {
    const id = fresh("order");
    const controller = await loaded(id, busy(id));
    expect(controller.submit("A", { model: "opus", mode: "plan" })).toBe("queued");
    expect(controller.submit("B")).toBe("queued");
    expect(api.controlConversation).not.toHaveBeenCalled();
    finishTurn(id);
    await read(controller);
    expect(sent()).toEqual([[id, "A"]]);
    expect(api.controlConversation).toHaveBeenCalledWith(id, { action: "message", message: "A", model: "opus", mode: "plan" });
    expect(controller.getSnapshot().queue.items.map(item => item.text)).toEqual(["B"]);
    await read(controller);
    expect(sent()).toEqual([[id, "A"]]);
    finishTurn(id);
    await read(controller);
    expect(sent()).toEqual([[id, "A"], [id, "B"]]);
    expect(controller.getSnapshot().queue).toEqual({ items: [] });
  });

  test("an idle conversation with an empty queue sends at once", async () => {
    const id = fresh("idle");
    const controller = await loaded(id, idle(id));
    expect(controller.submit("Now")).toBe("sent");
    await flush();
    expect(sent()).toEqual([[id, "Now"]]);
    expect(controller.getSnapshot().queue.items).toEqual([]);
  });

  test("a submission waits behind a paused queue even when the conversation is idle", async () => {
    const id = fresh("behind");
    const controller = await loaded(id, busy(id));
    controller.submit("A");
    await controller.control({ action: "interrupt" });
    expect(controller.getSnapshot().queue.paused).toBe("interrupt");
    finishTurn(id, line("notice", "Interrupted"));
    await read(controller);
    expect(controller.submit("B")).toBe("queued");
    await flush();
    expect(sent()).toEqual([]);
    controller.resumeQueue();
    await flush();
    expect(sent()).toEqual([[id, "A"]]);
    finishTurn(id);
    await read(controller);
    expect(sent()).toEqual([[id, "A"], [id, "B"]]);
  });

  test("a removed message is never sent and the others keep their order", async () => {
    const id = fresh("remove");
    const controller = await loaded(id, busy(id));
    controller.submit("A"); controller.submit("B"); controller.submit("C");
    controller.removeQueued(controller.getSnapshot().queue.items[1].id);
    finishTurn(id);
    await read(controller);
    finishTurn(id);
    await read(controller);
    expect(sent()).toEqual([[id, "A"], [id, "C"]]);
  });

  test("each queue only ever sends to its own conversation", async () => {
    const x = fresh("x"), y = fresh("y");
    const first = await loaded(x, busy(x));
    const second = await loaded(y, busy(y));
    first.submit("for X");
    second.submit("for Y");
    finishTurn(x);
    await read(first);
    await read(second);
    expect(sent()).toEqual([[x, "for X"]]);
    expect(second.getSnapshot().queue.items.map(item => item.text)).toEqual(["for Y"]);
    finishTurn(y);
    await read(second);
    expect(sent()).toEqual([[x, "for X"], [y, "for Y"]]);
  });

  test("a conversation holding a queue is never evicted from the cache", async () => {
    const id = fresh("retained");
    const controller = await loaded(id, busy(id));
    controller.submit("Keep me");
    agent[id] = idle(id);
    vi.mocked(api.controlConversation).mockRejectedValue(new Error("no"));
    for (let index = 0; index < 25; index++) conversationFor(fresh("other"));
    expect(conversationFor(id)).toBe(controller);
  });

  test("a failed send keeps the message, pauses the queue and is retried only on resume", async () => {
    const id = fresh("failure");
    const controller = await loaded(id, busy(id));
    controller.submit("Fragile");
    vi.mocked(api.controlConversation).mockRejectedValueOnce(new Error("Connection reset"));
    finishTurn(id);
    await read(controller);
    expect(controller.getSnapshot().queue).toMatchObject({ paused: "sendFailed", items: [{ text: "Fragile", status: "failed", error: "Error: Connection reset" }] });
    await read(controller);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(sent()).toEqual([[id, "Fragile"]]);
    controller.resumeQueue();
    await flush();
    expect(sent()).toEqual([[id, "Fragile"], [id, "Fragile"]]);
    expect(controller.getSnapshot().queue).toEqual({ items: [] });
  });

  test("snapshots arriving together dispatch the head only once", async () => {
    const id = fresh("double");
    const controller = await loaded(id, busy(id));
    controller.submit("Once");
    finishTurn(id);
    let acknowledge!: () => void;
    vi.mocked(api.controlConversation).mockImplementationOnce(async (target, input) => {
      await new Promise<void>(resolve => { acknowledge = resolve; });
      if (input.action === "message") agent[target] = { ...agent[target], busy: true };
      return {};
    });
    void controller.refresh(); void controller.refresh();
    await flush();
    void controller.refresh();
    await flush();
    expect(controller.getSnapshot().queue.items[0].status).toBe("sending");
    acknowledge();
    await flush();
    expect(sent()).toEqual([[id, "Once"]]);
  });

  test("a turn ending with an error or an interruption from another client pauses the queue", async () => {
    const failing = fresh("turn-error");
    const controller = await loaded(failing, busy(failing));
    controller.submit("Next");
    finishTurn(failing, line("error", "Claude Code stopped without a final result."));
    await read(controller);
    expect(controller.getSnapshot().queue.paused).toBe("turnError");
    const stopped = fresh("remote-interrupt");
    const other = await loaded(stopped, busy(stopped));
    other.submit("Next");
    finishTurn(stopped, line("notice", "Interrupted"));
    await read(other);
    expect(other.getSnapshot().queue.paused).toBe("interrupt");
    expect(sent()).toEqual([]);
  });

  test("a pending approval or a read-only conversation never receives queued messages", async () => {
    const asking = fresh("approval");
    const controller = await loaded(asking, busy(asking));
    controller.submit("Not an answer");
    agent[asking] = { ...idle(asking), approvals: [{ id: "approval-1", tool: "Bash" }] };
    await read(controller);
    const stopped = fresh("read-only");
    const other = await loaded(stopped, busy(stopped));
    other.submit("Never");
    agent[stopped] = { ...idle(stopped), readOnly: true };
    await read(other);
    expect(other.submit("More")).toBe("unavailable");
    expect(sent()).toEqual([]);
    expect(other.getSnapshot().queue.items.map(item => item.text)).toEqual(["Never"]);
  });

  test("a queue resumes on its own once the agent answers again", async () => {
    const id = fresh("reconnect");
    const controller = await loaded(id, busy(id));
    controller.submit("After the outage");
    const snapshot = agent[id];
    delete agent[id];
    await vi.advanceTimersByTimeAsync(750);
    expect(controller.getSnapshot().error).toContain("Agent unavailable");
    agent[id] = { ...snapshot, busy: false };
    await vi.advanceTimersByTimeAsync(5000);
    expect(sent()).toEqual([[id, "After the outage"]]);
  });
});

describe("conversation composer queue", () => {
  const view = (id: string) => render(<ProjectPrompt projectId="p" runId={id} agentAvailable onCreated={vi.fn()} />);

  test("while the agent responds, Enter queues the draft beside Interrupt and clears it", async () => {
    const id = fresh("composer");
    agent[id] = busy(id);
    view(id);
    const box = await findComposer();
    await screen.findByRole("button", { name: "Queue message" });
    expect(screen.getByRole("button", { name: "Interrupt" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Send" })).toBeNull();
    typeIn(box, "Then add tests");
    press(box, "Enter");
    const strip = await screen.findByRole("region", { name: "Message queue" });
    // The strip sits at the top of the composer box, before the editor.
    expect(strip.parentElement).toBe(box.closest(".composer-box"));
    expect(within(strip).getByText("Then add tests")).toBeTruthy();
    expect(within(strip).getByText("Queued")).toBeTruthy();
    expect(within(strip).queryByRole("button", { name: "Expand queued messages" })).toBeNull();
    expect(valueOf(box)).toBe("");
    expect(api.controlConversation).not.toHaveBeenCalled();
    fireEvent.click(within(strip).getByRole("button", { name: "Remove queued message 1" }));
    expect(screen.queryByRole("region", { name: "Message queue" })).toBeNull();
  });

  test("a full queue keeps the draft and says why", async () => {
    const id = fresh("full");
    agent[id] = busy(id);
    view(id);
    const box = await findComposer();
    await screen.findByRole("button", { name: "Queue message" });
    for (let index = 0; index < 20; index++) conversationFor(id).enqueue(`message ${index}`);
    await screen.findByLabelText("20 messages");
    typeIn(box, "One more");
    expect(screen.getByRole("button", { name: "Queue message" }).hasAttribute("disabled")).toBe(true);
    press(box, "Enter");
    expect(await screen.findByText("Queue full (20 messages). Remove one to add another.")).toBeTruthy();
    expect(valueOf(box)).toBe("One more");
  });

  test("a paused queue shows its reason and resumes on demand", async () => {
    const id = fresh("paused");
    agent[id] = busy(id);
    view(id);
    await screen.findByRole("button", { name: "Queue message" });
    conversationFor(id).enqueue("Next step");
    fireEvent.click(screen.getByRole("button", { name: "Interrupt" }));
    finishTurn(id, line("notice", "Interrupted"));
    expect(await screen.findByText("Queue paused · after an interrupt")).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole("button", { name: "Interrupt" })).toBeNull());
    expect(sent()).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    await waitFor(() => expect(sent()).toEqual([[id, "Next step"]]));
  });

  test("a stopped conversation keeps its queue visible, copyable and removable, never resumable", async () => {
    const id = fresh("stopped");
    agent[id] = busy(id);
    const controller = conversationFor(id);
    await controller.refresh();
    controller.enqueue("Left behind");
    controller.enqueue("Also left");
    agent[id] = { ...idle(id), readOnly: true };
    await controller.control({ action: "interrupt" });
    view(id);
    const list = await screen.findByRole("list", { name: "Queued messages" });
    expect(screen.queryByRole("button", { name: "Resume" })).toBeNull();
    expect(queryComposer()).toBeNull();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    fireEvent.click(within(list).getByRole("button", { name: "Copy queued message 1" }));
    expect(writeText).toHaveBeenCalledWith("Left behind");
    fireEvent.click(within(list).getByRole("button", { name: "Remove queued message 1" }));
    expect(within(list).queryByText("Left behind")).toBeNull();
    expect(within(list).getByText("Also left")).toBeTruthy();
    expect(sent()).toEqual([]);
  });
  test("Edit moves a queued message back into the composer, after the draft, with its turn options", async () => {
    const id = fresh("edit");
    agent[id] = busy(id);
    view(id);
    const box = await findComposer();
    await screen.findByRole("button", { name: "Queue message" });
    conversationFor(id).enqueue("Queued with plan", { mode: "plan" });
    const strip = await screen.findByRole("region", { name: "Message queue" });
    typeIn(box, "My draft");
    fireEvent.click(within(strip).getByRole("button", { name: "Edit queued message 1" }));
    expect(valueOf(box)).toBe("My draft\n\nQueued with plan");
    expect(document.activeElement).toBe(box);
    expect((screen.getByLabelText("Conversation mode") as HTMLSelectElement).value).toBe("plan");
    expect(screen.queryByRole("region", { name: "Message queue" })).toBeNull();
    expect(conversationFor(id).getSnapshot().queue.items).toEqual([]);
    expect(sent()).toEqual([]);
  });

  test("several queued messages expand into a list keeping pause, resume and failure", async () => {
    const id = fresh("expand");
    agent[id] = busy(id);
    view(id);
    await screen.findByRole("button", { name: "Queue message" });
    const controller = conversationFor(id);
    controller.enqueue("First queued");
    controller.enqueue("Second queued");
    const strip = await screen.findByRole("region", { name: "Message queue" });
    expect(within(strip).getByLabelText("2 messages")).toBeTruthy();
    expect(within(strip).getByText("First queued")).toBeTruthy();
    expect(within(strip).queryByText("Second queued")).toBeNull();
    fireEvent.click(within(strip).getByRole("button", { name: "Expand queued messages" }));
    const list = within(strip).getByRole("list", { name: "Queued messages" });
    expect(within(list).getByText("Second queued")).toBeTruthy();
    fireEvent.click(within(list).getByRole("button", { name: "Edit queued message 2" }));
    expect(valueOf(getComposer())).toBe("Second queued");
    // Down to one message, the strip collapses to its single line.
    expect(within(strip).queryByRole("list", { name: "Queued messages" })).toBeNull();
    controller.enqueue("Third queued");
    vi.mocked(api.controlConversation).mockImplementation(async (target, input) => {
      if (input.action === "message") throw new Error("network down");
      return { accepted: true };
    });
    finishTurn(id);
    expect(await within(strip).findByText("Queue paused · send failed: Error: network down")).toBeTruthy();
    expect(within(strip).getByText("Failed: Error: network down")).toBeTruthy();
    fireEvent.click(within(strip).getByRole("button", { name: "Expand queued messages" }));
    expect(within(strip).getByRole("button", { name: "Resume" })).toBeTruthy();
    expect(within(within(strip).getByRole("list", { name: "Queued messages" })).getByText("Failed: Error: network down")).toBeTruthy();
  });
  test("Edit past the length limit shows exactly what Send would send", async () => {
    const id = fresh("long-edit");
    agent[id] = busy(id);
    view(id);
    const box = await findComposer();
    await screen.findByRole("button", { name: "Queue message" });
    conversationFor(id).enqueue("q".repeat(15000));
    const strip = await screen.findByRole("region", { name: "Message queue" });
    typeIn(box, "d".repeat(20000));
    fireEvent.click(within(strip).getByRole("button", { name: "Edit queued message 1" }));
    expect(valueOf(box).length).toBe(20000 + 2 + 15000);
  });
});
