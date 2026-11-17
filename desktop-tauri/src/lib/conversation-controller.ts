import { useSyncExternalStore } from "react";
import { controlConversation, decodeEvents, readConversation, type ConversationAction, type ConversationSnapshot, type TurnOptions } from "./conversation";
import { emptyQueue, enqueue, remove, settle, withdraw, turnOutcome, type EnqueueRefusal, type MessageQueue, type QueuedMessage } from "./conversation-queue";

interface ConversationState {
  snapshot: ConversationSnapshot;
  loaded: boolean;
  pending: boolean;
  error: string;
  queue: MessageQueue;
}
export type SubmitResult = "sent" | "queued" | "unavailable" | EnqueueRefusal;

/** How long a conversation holding queued messages waits before reading again after a failed read. */
const QUEUE_RETRY_MS = 5000;

/**
 * Go owns history and execution. This store only observes it, serializes local sends and
 * holds the follow-up messages queued while a turn runs, which it sends one per turn.
 */
export class ConversationController {
  private state: ConversationState = { snapshot: { id: "", busy: false, readOnly: false }, loaded: false, pending: false, error: "", queue: emptyQueue() };
  private listeners = new Set<() => void>();
  private timer?: ReturnType<typeof setTimeout>;
  private reading = false;
  private sending = false;
  private controlling = false;
  private revision = 0;
  private refreshQueued = false;
  /** Whether the last applied snapshot showed a turn running, to notice the turn ending. */
  private lastBusy = false;
  constructor(readonly id: string) {}
  getSnapshot = () => this.state;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };
  get retained() { return this.listeners.size > 0 || this.reading || this.sending || this.controlling || this.state.queue.items.length > 0 || (this.state.snapshot.busy || this.state.snapshot.checkingMcp); }
  private update(next: Partial<ConversationState>) {
    this.state = { ...this.state, ...next };
    for (const listener of this.listeners) listener();
  }
  refresh = async () => {
    if (this.reading || this.sending) { this.refreshQueued = true; return; }
    clearTimeout(this.timer);
    this.reading = true;
    const revision = this.revision;
    let fresh = false;
    this.update({ pending: true, error: "" });
    try {
      const snapshot = await readConversation(this.id);
      // A read begun before a mutation must not restore stale idle/approval state.
      if (revision === this.revision) {
        fresh = true;
        this.update({ snapshot, loaded: true, queue: this.pauseAfterTurn(snapshot) });
        this.lastBusy = snapshot.busy;
      }
    } catch (error) {
      if (revision === this.revision) this.update({ error: String(error) });
    } finally {
      this.reading = false;
      this.update({ pending: this.sending });
      if (this.refreshQueued && !this.sending) {
        this.refreshQueued = false;
        void this.refresh();
      } else if ((this.state.snapshot.busy || this.state.snapshot.checkingMcp) && !this.state.error && !this.sending) {
        this.timer = setTimeout(() => void this.refresh(), 750);
      } else if (this.state.error && this.dispatchable && !this.sending) {
        // Keep reading so a queue resumes after reconnection, even when the view is closed.
        this.timer = setTimeout(() => void this.refresh(), QUEUE_RETRY_MS);
      } else if (fresh) {
        this.dispatchNext();
      }
    }
  };
  /** A turn that ended with an error or an interruption holds the queue back until the user resumes it. */
  private pauseAfterTurn(snapshot: ConversationSnapshot) {
    const queue = this.state.queue;
    if (!this.lastBusy || snapshot.busy || !queue.items.length || queue.paused) return queue;
    const outcome = turnOutcome(decodeEvents(snapshot));
    return outcome === "ok" ? queue : { ...queue, paused: outcome === "error" ? "turnError" as const : "interrupt" as const };
  }
  private get dispatchable() {
    const queue = this.state.queue;
    return !queue.paused && queue.items[0]?.status === "queued";
  }
  async send(message: string, newlyCreated = false, options: TurnOptions = {}) {
    if (this.sending || this.state.snapshot.busy || (!newlyCreated && (!this.state.loaded || this.state.pending || this.state.error || this.state.snapshot.readOnly)) || !message.trim()) return false;
    this.sending = true;
    this.revision++;
    clearTimeout(this.timer);
    this.update({ pending: true, error: "" });
    try {
      await controlConversation(this.id, { action: "message", message, ...options });
      this.lastBusy = true;
      this.refreshQueued = true;
    } catch (error) {
      // A lost acknowledgement is ambiguous. Never replay a prompt automatically.
      this.refreshQueued = false;
      this.update({ error: String(error) });
    } finally {
      this.sending = false;
      this.update({ pending: this.reading });
      if (this.refreshQueued && !this.reading) { this.refreshQueued = false; void this.refresh(); }
    }
    return true;
  }
  /**
   * The composer's single entry point: sends at once when the conversation is idle and
   * nothing waits before this message, queues it otherwise so the order is kept.
   */
  submit(message: string, options: TurnOptions = {}): SubmitResult {
    const { snapshot, loaded, pending, error, queue } = this.state;
    if (!loaded || snapshot.readOnly) return "unavailable";
    if (snapshot.busy || pending || this.sending || queue.items.length) return this.enqueue(message, options);
    if (!message.trim()) return "empty";
    if (error) return "unavailable";
    void this.send(message, false, options);
    return "sent";
  }
  enqueue(message: string, options: TurnOptions = {}): "queued" | "unavailable" | EnqueueRefusal {
    if (!this.state.loaded || this.state.snapshot.readOnly) return "unavailable";
    const queue = enqueue(this.state.queue, message, options);
    if (typeof queue === "string") return queue;
    this.update({ queue });
    return "queued";
  }
  removeQueued(id: string) {
    const queue = remove(this.state.queue, id);
    if (queue !== this.state.queue) this.update({ queue });
  }
  /** Lifts the pause, gives a failed message one more explicit try, and dispatches after a fresh read. */
  /** Takes a queued or failed message out of the queue and returns it, for the composer to edit. */
  withdrawQueued(id: string): QueuedMessage | undefined {
    const withdrawn = withdraw(this.state.queue, id);
    if (!withdrawn) return undefined;
    this.update({ queue: withdrawn.queue });
    return withdrawn.item;
  }
  resumeQueue() {
    const items = this.state.queue.items.map(item => item.status === "failed" ? { id: item.id, text: item.text, options: item.options, status: "queued" as const } : item);
    this.update({ queue: { items } });
    void this.refresh();
  }
  /** Sends the head of the queue only right after a fresh snapshot shows the conversation idle. */
  private dispatchNext() {
    const { snapshot, loaded, error, queue } = this.state;
    if (!loaded || error || this.reading || this.sending || snapshot.busy || snapshot.checkingMcp || snapshot.readOnly || snapshot.approvals?.length || !this.dispatchable) return;
    const head = queue.items[0];
    // Taken synchronously: a second snapshot arriving now finds a send in flight and backs off.
    this.sending = true;
    this.revision++;
    clearTimeout(this.timer);
    this.update({ pending: true, queue: { ...queue, items: queue.items.map(item => item === head ? { ...item, status: "sending" as const } : item) } });
    void (async () => {
      try {
        await controlConversation(this.id, { action: "message", message: head.text, ...head.options });
        this.lastBusy = true;
        const current = this.state.queue;
        this.update({ queue: settle({ ...current, items: current.items.filter(item => item.id !== head.id) }) });
      } catch (cause) {
        // A lost acknowledgement is ambiguous: keep the message, pause, and let the user decide.
        const current = this.state.queue;
        this.update({ queue: { paused: "sendFailed", items: current.items.map(item => item.id === head.id ? { ...item, status: "failed" as const, error: String(cause) } : item) } });
      } finally {
        this.sending = false;
        this.refreshQueued = false;
        this.update({ pending: this.reading });
        if (!this.reading) void this.refresh();
      }
    })();
  }
  async control(input: Exclude<ConversationAction, { action: "message" }>) {
    if (this.controlling) throw new Error("An action is already in progress.");
    this.controlling = true;
    this.revision++;
    clearTimeout(this.timer);
    const queue = this.state.queue;
    // The user stopped this turn: the next queued message must not start right behind it.
    if (input.action === "interrupt" && queue.items.length && !queue.paused) this.update({ queue: { ...queue, paused: "interrupt" } });
    try {
      await controlConversation(this.id, input);
      await this.refresh();
    } finally { this.controlling = false; }
  }
}

const sessions = new Map<string, ConversationController>();
export function conversationFor(id: string) {
  let session = sessions.get(id);
  if (!session) {
    if (sessions.size >= 20) {
      for (const [key, value] of sessions) {
        if (!value.retained) { sessions.delete(key); break; }
      }
    }
    session = new ConversationController(id);
    sessions.set(id, session);
  }
  return session;
}
export function useConversation(id: string) {
  const controller = conversationFor(id);
  return { controller, ...useSyncExternalStore(controller.subscribe, controller.getSnapshot) };
}
