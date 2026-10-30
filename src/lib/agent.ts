import { invoke, isTauri } from "@tauri-apps/api/core";
import type { Project, Task } from "../domain/board";

function requireDesktop() {
  if (!isTauri())
    throw new Error(
      "Ouvrez l’application de bureau avec npm run tauri dev pour connecter l’agent.",
    );
}
export async function listProjects(): Promise<Project[]> {
  requireDesktop();
  return (await invoke<Project[] | null>("list_projects")) ?? [];
}
export async function listTasks(projectId: string): Promise<Task[]> {
  requireDesktop();
  return (await invoke<Task[] | null>("list_tasks", { projectId })) ?? [];
}
export async function getTask(projectId:string,taskId:string):Promise<Task> {
  requireDesktop();
  return invoke<Task>("get_task",{projectId,taskId});
}

import { Channel } from "@tauri-apps/api/core";
import type {
  Snapshot,
  Orchestration,
  OrchestrationReport,
  LaunchInput,
} from "../domain/execution";
export async function executionSnapshot(): Promise<Snapshot> {
  requireDesktop();
  return invoke("execution_snapshot");
}
export async function projectSkills(
  projectId: string,
): Promise<{ id: string }[]> {
  requireDesktop();
  return invoke("project_skills", { projectId });
}
export async function launchTask(
  projectId: string,
  input: LaunchInput,
): Promise<unknown> {
  requireDesktop();
  return invoke("launch_task", { projectId, input });
}
export async function stopRun(runId: string): Promise<void> {
  requireDesktop();
  return invoke("stop_run", { runId });
}
export async function getOrchestration(
  projectId: string,
): Promise<Orchestration> {
  requireDesktop();
  return invoke("orchestration", { projectId });
}
export async function runOrchestration(
  projectId: string,
  agentId: string,
): Promise<OrchestrationReport> {
  requireDesktop();
  return invoke("run_orchestration", { projectId, agentId });
}
export type Output =
  | { kind: "bytes"; data: number[] }
  | { kind: "error"; data: string }
  | { kind: "closed" };
export async function watchRun(
  runId: string,
  onOutput: (value: Output) => void,
): Promise<() => Promise<void>> {
  requireDesktop();
  const subscription = crypto.randomUUID();
  const output = new Channel<Output>();
  output.onmessage = onOutput;
  await invoke("watch_run", { runId, subscription, output });
  return () => invoke("unwatch_run", { subscription });
}
