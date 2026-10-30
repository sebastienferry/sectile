import { useSyncExternalStore } from "react";
import { controlConversation, readConversation, type ConversationAction, type ConversationSnapshot, type TurnOptions } from "./conversation";

interface ConversationState {
  snapshot: ConversationSnapshot;
  loaded: boolean;
  pending: boolean;
  error: string;
}

/** Go owns history and execution. This store only observes it and serializes local sends. */
export class ConversationController {
  private state: ConversationState = { snapshot: { id: "", busy: false, readOnly: false }, loaded: false, pending: false, error: "" };
  private listeners = new Set<() => void>();
  private timer?: ReturnType<typeof setTimeout>;
  private reading = false;
  private sending = false;
  private controlling = false;
  private revision = 0;
  private refreshQueued = false;
  constructor(readonly id: string) {}
  getSnapshot = () => this.state;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };
  get retained() { return this.listeners.size > 0 || this.reading || this.sending || this.controlling || (this.state.snapshot.busy || this.state.snapshot.checkingMcp); }
  private update(next: Partial<ConversationState>) {
    this.state = { ...this.state, ...next };
    for (const listener of this.listeners) listener();
  }
  refresh = async () => {
    if (this.reading || this.sending) { this.refreshQueued = true; return; }
    clearTimeout(this.timer);
    this.reading = true;
    const revision = this.revision;
    this.update({ pending: true, error: "" });
    try {
      const snapshot = await readConversation(this.id);
      // A read begun before a mutation must not restore stale idle/approval state.
      if (revision === this.revision) this.update({ snapshot, loaded: true });
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
      }
    }
  };
  async send(message: string, newlyCreated = false, options: TurnOptions = {}) {
    if (this.sending || this.state.snapshot.busy || (!newlyCreated && (!this.state.loaded || this.state.pending || this.state.error || this.state.snapshot.readOnly)) || !message.trim()) return false;
    this.sending = true;
    this.revision++;
    clearTimeout(this.timer);
    this.update({ pending: true, error: "" });
    try {
      await controlConversation(this.id, { action: "message", message, ...options });
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
  async control(input: Exclude<ConversationAction, { action: "message" }>) {
    if (this.controlling) throw new Error("Une action est déjà en cours.");
    this.controlling = true;
    this.revision++;
    clearTimeout(this.timer);
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
