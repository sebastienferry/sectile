export interface Run {
  id: string;
  conversation?: boolean;
  provider?: string;
  model?: string;
  title?: string;
  prompt?: string;
  projectId: string;
  taskId: string;
  taskKey?: string;
  skill: string;
  status: string;
  createdAt: string;
  startedAt?: string;
  waitingSince?: string;
  cancelRequested?: boolean;
  authenticationRequired?: boolean;
  engineName?: string;
  directory?: string;
  folders?: {path:string;name:string;role:string;attached?:boolean}[];
  branch?: string;
  headless?: boolean;
  trace?: boolean;
  restored?: boolean;
  sessionId?: string;
  engineId?: string;
  /** The archived conversation this run started over from (#906). */
  contextRunId?: string;
  contextTitle?: string;
  contextOmitted?: number;
}
export interface Snapshot {
  status: {
    connected: boolean;
    server?: string;
    runtimeMode?: "local" | "fleet" | "unconfigured";
    capabilities?: string[];
    contractError?: string;
    text?: string;
  };
  runs: Run[];
}

export function executionConnectionLabel(runtimeMode?: Snapshot["status"]["runtimeMode"]) {
  return runtimeMode === "fleet" ? "Local agent · Fleet backend" : "Local runtime";
}
export interface AgentProfile {
  id: string;
  name: string;
  file: string;
  guidelines: string;
  guidelinesError?: string;
}
export interface Orchestration {
  orchestratorEnabled: boolean;
  orchestratorMaxConcurrent: number;
  orchestratorActiveRuns?: number;
  agents: AgentProfile[];
}
export interface OrchestrationReport {
  evaluatedCount: number;
  activeRunsAfter: number;
  maxConcurrent: number;
  launched: { id: string; key: string; title: string }[];
  skipped: { id: string; key: string; reason: string }[];
}
export type TaskLaunchView = "conversation" | "terminal";
export interface LaunchOptions {
  view?: TaskLaunchView;
  provider?: "claude" | "codex";
  engineId?: string;
  model?: string;
  conversationMode?: string;
  /** Archived conversation given as former context; conversation view only. */
  contextRunId?: string;
}
export interface LaunchInput extends LaunchOptions {
  taskId: string;
  skillId: string;
  prompt: string;
  mode: string;
  batchTaskIds?: string[];
}
const TASK_LAUNCH_VIEW_KEY = "sectile:taskLaunchView";
export function readTaskLaunchView(): TaskLaunchView {
  try {
    return window.localStorage.getItem(TASK_LAUNCH_VIEW_KEY) === "terminal"
      ? "terminal"
      : "conversation";
  } catch {
    return "conversation";
  }
}
export function saveTaskLaunchView(view: TaskLaunchView): void {
  try {
    if (view === "terminal") {
      window.localStorage.setItem(TASK_LAUNCH_VIEW_KEY, "terminal");
    } else {
      window.localStorage.removeItem(TASK_LAUNCH_VIEW_KEY);
    }
  } catch {
    // Ignore storage errors in restricted environments.
  }
}
const activeStates = new Set([
  "queued",
  "preparing",
  "running",
  "pending",
  "waiting",
]);
export function isActive(run: Run) {
  return activeStates.has(run.status);
}
export function canStop(run: Run, fresh: boolean) {
  return fresh && isActive(run) && !run.cancelRequested && !run.restored;
}
export function projectRuns(runs: Run[], projectId: string) {
  return runs
    .filter((r) => r.projectId === projectId)
    .sort(
      (a, b) =>
        Number(isActive(b)) - Number(isActive(a)) ||
        b.createdAt.localeCompare(a.createdAt) ||
        a.id.localeCompare(b.id),
    );
}
export function statusLabel(run: Run) {
  if (run.authenticationRequired) return "Provider sign-in required";
  if (run.cancelRequested && isActive(run)) return "Stopping…";
  if (run.waitingSince && !run.waitingSince.startsWith("0001") && isActive(run))
    return "Action required";
  // A conversation process stays available between turns. Its turn state comes
  // from the conversation snapshot, not the longer-lived agent process state.
  if (run.conversation && run.status === "running") return "Active session";
  return (
    (
      {
        queued: "Queued",
        preparing: "Preparing",
        running: "Running",
        pending: "Pending",
        waiting: "Waiting",
        completed: "Completed",
        finished: "Completed",
        failed: "Failed",
        canceled: "Stopped",
        stopped: "Stopped",
      } as Record<string, string>
    )[run.status] ?? run.status
  );
}
export function launchInput(
  taskId: string,
  skillId: string,
  prompt: string,
  mode: string,
  options?: LaunchOptions,
): LaunchInput {
  const effectiveMode = ["pickup", "pickup_issue", "pickup_issues"].includes(skillId)
    ? "autonomous"
    : mode;
  const result: LaunchInput = {
    taskId,
    skillId,
    prompt,
    mode: effectiveMode,
  };
  if (effectiveMode !== "autonomous" && options) {
    if (options.view === "terminal") result.view = "terminal";
    if (options.provider) result.provider = options.provider;
    if (options.engineId) result.engineId = options.engineId;
    if (options.model) result.model = options.model;
    if (options.conversationMode) result.conversationMode = options.conversationMode;
    if (options.contextRunId && options.view === "conversation") result.contextRunId = options.contextRunId;
  }
  return result;
}
export function canLaunchTask(taskStatus: string, taskId: string, runs: Run[]) {
  return (
    !["finished", "done"].includes(taskStatus) &&
    !runs.some((r) => r.taskId === taskId && isActive(r))
  );
}
