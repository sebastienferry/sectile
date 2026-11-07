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

const taskSkillNames: Record<string, string> = {
  clarify: "Clarify",
  specify: "Specify",
  implement: "Implement",
  adjust: "Adjust",
  handoff: "Handoff",
  pickup: "Pickup",
  discuss: "Discuss",
  create_pr: "Create PR",
};

export function taskSkillName(skill?: string): string {
  const id = String(skill || "").trim().replace(/^[^:]*:/, "").replace(/-issue$/, "");
  if (!id) return "";
  return taskSkillNames[id] || id.charAt(0).toUpperCase() + id.slice(1);
}

function appendTaskSkill(title: string, skill?: string): string {
  const base = title.trim();
  const suffix = taskSkillName(skill);
  if (!base) return suffix;
  if (!suffix) return base;
  if (base.toLowerCase().endsWith(` - ${suffix.toLowerCase()}`)) return base;
  return `${base} - ${suffix}`;
}

function isSlashCommandTitle(title?: string): boolean {
  const trimmed = String(title || "").trim();
  return trimmed.startsWith("/") || trimmed.startsWith("$");
}

export function conversationTitle(
  snapshot?: { title?: string; events?: (string | { kind?: string; type?: string; text?: string; message?: string })[] | null },
  run?: Run,
  task?: Task,
): string {
  if (run?.taskId) {
    if (task?.title?.trim()) return appendTaskSkill(task.title.trim(), run.skill);
    if (snapshot?.title?.trim() && !isSlashCommandTitle(snapshot.title)) {
      return appendTaskSkill(snapshot.title.trim(), run.skill);
    }
    if (run.title?.trim() && !isSlashCommandTitle(run.title)) {
      return appendTaskSkill(run.title.trim(), run.skill);
    }
    if (run.taskKey?.trim()) return appendTaskSkill(run.taskKey.trim(), run.skill);
    return appendTaskSkill("Ticket", run.skill);
  }
  if (snapshot?.title?.trim()) return snapshot.title.trim();
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
  if (run?.prompt?.trim()) {
    return run.prompt.trim().split("\n")[0].slice(0, 100);
  }
  return "Conversation du projet";
}

