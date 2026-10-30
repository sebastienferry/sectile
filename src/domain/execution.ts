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
}
export interface Snapshot {
  status: {
    connected: boolean;
    runtimeMode?: "local" | "fleet" | "unconfigured";
    capabilities?: string[];
    contractError?: string;
  };
  runs: Run[];
}

export function executionConnectionLabel(runtimeMode?: Snapshot["status"]["runtimeMode"]) {
  return runtimeMode === "fleet" ? "Agent local · backend Fleet" : "Exécution locale";
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
export interface LaunchInput {
  taskId: string;
  skillId: string;
  prompt: string;
  mode: string;
  batchTaskIds?: string[];
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
  if (run.authenticationRequired) return "Connexion au fournisseur requise";
  if (run.cancelRequested && isActive(run)) return "Arrêt demandé…";
  if (run.waitingSince && !run.waitingSince.startsWith("0001") && isActive(run))
    return "Votre intervention est requise";
  // A conversation process stays available between turns. Its turn state comes
  // from the conversation snapshot, not the longer-lived agent process state.
  if (run.conversation && run.status === "running") return "Session active";
  return (
    (
      {
        queued: "En file",
        preparing: "Préparation",
        running: "En cours",
        pending: "En attente",
        waiting: "En attente",
        completed: "Terminé",
        finished: "Terminé",
        failed: "Échec",
        canceled: "Arrêté",
        stopped: "Arrêté",
      } as Record<string, string>
    )[run.status] ?? run.status
  );
}
export function launchInput(
  taskId: string,
  skillId: string,
  prompt: string,
  mode: string,
): LaunchInput {
  return {
    taskId,
    skillId,
    prompt,
    mode: ["pickup", "pickup_issue", "pickup_issues"].includes(skillId)
      ? "autonomous"
      : mode,
  };
}
export function canLaunchTask(taskStatus: string, taskId: string, runs: Run[]) {
  return (
    !["finished", "done"].includes(taskStatus) &&
    !runs.some((r) => r.taskId === taskId && isActive(r))
  );
}
