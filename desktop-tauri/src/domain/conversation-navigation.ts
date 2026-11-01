import type { Task } from "./board.ts";
import type { Run } from "./execution.ts";
import { stageLabels, stages, taskStage } from "./workflow.ts";
import type { BoardMapping } from "../lib/workspace.ts";

export type ConversationGrouping = "recent" | "created" | "id" | "stage" | "tracker";

export function conversationGroups(
  runs: Run[],
  tasks: Task[],
  grouping: ConversationGrouping,
  board?: BoardMapping,
): { label: string; runs: Run[] }[] {
  const conversations = runs.filter(run => run.conversation);
  if (grouping === "recent") return [{ label: "", runs: conversations }];
  if (grouping === "created") {
    const sorted = [...conversations].sort((a, b) => {
      const timeA = Date.parse(a.createdAt) || 0;
      const timeB = Date.parse(b.createdAt) || 0;
      return timeB - timeA;
    });
    return [{ label: "", runs: sorted }];
  }
  if (grouping === "id") {
    const sorted = [...conversations].sort((a, b) => {
      const keyA = a.taskKey || a.taskId || "";
      const keyB = b.taskKey || b.taskId || "";
      if (!keyA && !keyB) return 0;
      if (!keyA) return 1;
      if (!keyB) return -1;
      return keyA.localeCompare(keyB, undefined, { numeric: true, sensitivity: "base" });
    });
    return [{ label: "", runs: sorted }];
  }
  const byTask = new Map(tasks.map(task => [task.id, task]));
  const groups = new Map<string, Run[]>();
  for (const run of conversations) {
    const task = byTask.get(run.taskId);
    const stage = task ? taskStage(task, board) : undefined;
    const label = !run.taskId ? "" : !task ? "Ticket indisponible" :
      grouping === "stage" ? stage ? stageLabels[stage] : "Étape inconnue" :
      task.trackerStatus?.trim() || "Statut distant indisponible";
    const group = groups.get(label);
    if (group) group.push(run);
    else groups.set(label, [run]);
  }
  if (grouping === "stage") {
    const stageOrder = ["", ...stages.map(stage => stageLabels[stage])];
    return [...groups].sort(([left], [right]) => {
      const leftIndex = stageOrder.indexOf(left);
      const rightIndex = stageOrder.indexOf(right);
      return (leftIndex < 0 ? stageOrder.length : leftIndex) - (rightIndex < 0 ? stageOrder.length : rightIndex);
    }).map(([label, runs]) => ({ label, runs }));
  }
  return [...groups].sort(([left], [right]) => (left === "" ? -1 : right === "" ? 1 : 0)).map(([label, runs]) => ({ label, runs }));
}

export function conversationTitle(
  snapshot?: { title?: string; events?: (string | { kind?: string; type?: string; text?: string; message?: string })[] | null },
  run?: Run,
  task?: Task,
): string {
  if (snapshot?.title?.trim()) return snapshot.title.trim();
  if (run?.taskId && task?.title?.trim()) return task.title.trim();
  if (run?.title?.trim()) return run.title.trim();
  if (snapshot?.events && Array.isArray(snapshot.events)) {
    for (const item of snapshot.events) {
      try {
        const parsed = typeof item === "string" ? JSON.parse(item) : item;
        const text = parsed?.text || parsed?.message;
        const kind = parsed?.kind || parsed?.type;
        if ((kind === "user" || !kind) && typeof text === "string" && text.trim()) {
          return text.trim().split("\n")[0].slice(0, 100);
        }
      } catch {
        // Ignore parse error
      }
    }
  }
  if (run?.taskId) {
    if (run.taskKey?.trim()) return run.taskKey.trim();
    return "Ticket";
  }
  if (run?.prompt?.trim()) {
    return run.prompt.trim().split("\n")[0].slice(0, 100);
  }
  return "Conversation du projet";
}

