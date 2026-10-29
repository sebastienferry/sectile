import { invoke } from "@tauri-apps/api/core";
export interface ConversationEvent {
  kind: string; text: string; interrupted?: boolean; detail?: string; tool?: string; input?: unknown; toolId?: string; error?: boolean;
}
export interface Approval { id: string; tool: string; description?: string; reason?: string; input?: unknown }
export interface ConversationSnapshot {
  checkingMcp?: boolean; sectileMcp?: {status:string;source?:string};
  title?: string; canRename?: boolean; provider?: string; model?: string; mode?: string; models?: { model: string }[] | null;
  id: string; busy: boolean; readOnly: boolean; events?: string[] | null; partial?: string; persistenceError?: string; approvals?: Approval[];
}
export const createConversation = (projectId: string, provider: string, engineId?: string) =>
  invoke<{ id: string }>("create_conversation", { projectId, provider, ...(engineId ? {engineId} : {}) });
export const readConversation = (runId: string) => invoke<ConversationSnapshot>("read_conversation", { runId });
export interface TurnOptions { model?: string; mode?: "plan" | "execute" }
export const providerModels = (provider: string) => invoke<string[]>("provider_models", { provider });
export type ConversationAction = { action: "message"; message: string; model?: string; mode?: "plan" | "execute" } | { action: "checkMcp" } | { action: "interrupt" } | { action: "approval"; id: string; decision: "allow" | "deny" } | { action: "answer"; id: string; answers: Record<string, string> };
export const controlConversation = (runId: string, input: ConversationAction) => invoke("control_conversation", { runId, input });
export function decodeEvents(snapshot: ConversationSnapshot): ConversationEvent[] {
  return (snapshot.events ?? []).map(line => {
    try { return JSON.parse(line) as ConversationEvent; }
    catch { return { kind: "notice", text: line }; }
  });
}

export const renameConversation = (runId: string, title: string) => invoke("rename_conversation", {runId,title});
