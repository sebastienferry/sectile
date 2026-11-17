import { invoke } from "@tauri-apps/api/core";
export interface ConversationEvent {
  kind: string; text: string; interrupted?: boolean; detail?: string; tool?: string; input?: unknown; toolId?: string; error?: boolean;
}
export interface ApprovalChoice {
  decision: "allow" | "always" | "deny";
  label: string;
}
export interface Approval {
  id: string;
  tool: string;
  description?: string;
  reason?: string;
  input?: unknown;
  access?: unknown;
  suggestions?: Record<string, unknown>[];
  choices?: ApprovalChoice[];
}
export type ConversationMode = "plan" | "execute" | "default" | "acceptEdits" | "auto" | "read-only" | "workspace-write";
export interface ConversationCommand { name: string; description?: string; argumentHint?: string }
export interface ConversationSnapshot {
  checkingMcp?: boolean; sectileMcp?: {status:string;source?:string};
  title?: string; canRename?: boolean; provider?: string; model?: string; mode?: string; models?: { model: string }[] | null;
  connectionId?: string; canAdoptConnection?: boolean;
  id: string; busy: boolean; readOnly: boolean; events?: string[] | null; partial?: string; persistenceError?: string; approvals?: Approval[];
  /** Tokens in the model's context and its window, sent once the provider reports a window (#915). */
  context?: { used: number; window: number };
  /** Slash commands and Codex `$` skills the conversation's provider accepts (#916). */
  commands?: ConversationCommand[] | null;
}
export const createConversation = (projectId: string, provider: string, engineId?: string, contextRunId?: string) =>
  invoke<{ id: string }>("create_conversation", { projectId, provider, ...(engineId ? {engineId} : {}), ...(contextRunId ? {contextRunId} : {}) });
/** What a Start over from an archived conversation would give, without its text (#906). */
export interface FormerContextPreview { sourceId: string; sourceTitle: string; messages: number; omitted: number; toolsOmitted: boolean; characters: number }
export const formerContextPreview = (projectId: string, runId: string) =>
  invoke<FormerContextPreview>("former_context_preview", { projectId, runId });
export const readConversation = (runId: string) => invoke<ConversationSnapshot>("read_conversation", { runId });
export interface TurnOptions { model?: string; mode?: ConversationMode }
export const providerModels = (provider: string) => invoke<string[]>("provider_models", { provider });
export type ConversationAction = { action: "message"; message: string; model?: string; mode?: ConversationMode } | { action: "adoptConnection" } | { action: "checkMcp" } | { action: "interrupt" } | { action: "approval"; id: string; decision: "allow" | "always" | "deny" } | { action: "answer"; id: string; answers: Record<string, string> };
export const controlConversation = (runId: string, input: ConversationAction) => invoke("control_conversation", { runId, input });
export function decodeEvents(snapshot: ConversationSnapshot): ConversationEvent[] {
  return (snapshot.events ?? []).map(line => {
    try { return JSON.parse(line) as ConversationEvent; }
    catch { return { kind: "notice", text: line }; }
  });
}

export const renameConversation = (runId: string, title: string) => invoke("rename_conversation", {runId,title});
