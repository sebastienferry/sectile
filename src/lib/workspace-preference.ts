import type { Project } from "../domain/board";

export type WorkspaceViewName =
  | "prompt"
  | "board"
  | "executions"
  | "orchestration"
  | "launch"
  | "project-settings"
  | "task";

export interface WorkspaceDestination {
  id: string;
  projectId: string;
  view: WorkspaceViewName;
  runId?: string;
  taskId?: string;
}

interface SavedWorkspace {
  projectId: string;
  view: WorkspaceViewName;
  runId?: string;
  taskId?: string;
}

const views = new Set<WorkspaceViewName>([
  "prompt",
  "board",
  "executions",
  "orchestration",
  "launch",
  "project-settings",
  "task",
]);

export function createWorkspaceDestination(
  projectId: string,
  view: WorkspaceViewName,
  runId?: string,
  taskId?: string,
): WorkspaceDestination {
  return {
    id: `${projectId}:${view}:${runId ?? ""}:${taskId ?? ""}`,
    projectId,
    view,
    runId,
    taskId,
  };
}

export function serializeWorkspaceDestination(
  destination: WorkspaceDestination,
): string {
  const { projectId, view, runId, taskId } = destination;
  return JSON.stringify({ projectId, view, runId, taskId });
}

export function resolveLaunchDestination(
  serialized: string,
  projects: Project[],
): WorkspaceDestination | null {
  let saved: Partial<SavedWorkspace>;
  try {
    saved = JSON.parse(serialized) as Partial<SavedWorkspace>;
  } catch {
    return defaultDestination(projects);
  }

  if (
    typeof saved.projectId === "string" &&
    typeof saved.view === "string" &&
    views.has(saved.view as WorkspaceViewName) &&
    projects.some((project) => project.id === saved.projectId)
  ) {
    return createWorkspaceDestination(
      saved.projectId,
      saved.view as WorkspaceViewName,
      typeof saved.runId === "string" ? saved.runId : undefined,
      typeof saved.taskId === "string" ? saved.taskId : undefined,
    );
  }

  return defaultDestination(projects);
}

function defaultDestination(projects: Project[]): WorkspaceDestination | null {
  const first = projects[0];
  return first ? createWorkspaceDestination(first.id, "board") : null;
}
