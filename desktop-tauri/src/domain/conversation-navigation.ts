import type { Task } from "./board.ts";
import type { Run } from "./execution.ts";
import { stageLabels, stages, taskStage } from "./workflow.ts";
import type { BoardMapping } from "../lib/workspace.ts";

export type ConversationGrouping = "recent" | "stage" | "tracker";

export function conversationGroups(
  runs: Run[],
  tasks: Task[],
  grouping: ConversationGrouping,
  board?: BoardMapping,
): { label: string; runs: Run[] }[] {
  const conversations = runs.filter(run => run.conversation);
  if (grouping === "recent") return [{ label: "", runs: conversations }];
  const byTask = new Map(tasks.map(task => [task.id, task]));
  const groups = new Map<string, Run[]>();
  for (const run of conversations) {
    const task = byTask.get(run.taskId);
    const stage = task ? taskStage(task, board) : undefined;
    const label = !run.taskId ? "Prompts libres" : !task ? "Ticket indisponible" :
      grouping === "stage" ? stage ? stageLabels[stage] : "Étape inconnue" :
      task.trackerStatus?.trim() || "Statut distant indisponible";
    const group = groups.get(label);
    if (group) group.push(run);
    else groups.set(label, [run]);
  }
  if (grouping === "stage") {
    const stageOrder = stages.map(stage => stageLabels[stage]);
    return [...groups].sort(([left], [right]) => {
      const leftIndex = stageOrder.indexOf(left);
      const rightIndex = stageOrder.indexOf(right);
      return (leftIndex < 0 ? stageOrder.length : leftIndex) - (rightIndex < 0 ? stageOrder.length : rightIndex);
    }).map(([label, runs]) => ({ label, runs }));
  }
  return [...groups].map(([label, runs]) => ({ label, runs }));
}
