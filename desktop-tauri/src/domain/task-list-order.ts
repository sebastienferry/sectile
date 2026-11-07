import type { Task } from "./board.ts";
import { stages, taskStage } from "./workflow.ts";
import type { BoardMapping } from "../lib/workspace.ts";

export const PRIORITY_RANK: Record<string, number> = {
  urgent: 4,
  high: 3,
  medium: 2,
  low: 1,
};

export type SortField = "key" | "title" | "issueType" | "stage" | "priority" | "assignee" | "sprint";
export const SORTABLE_FIELDS: readonly SortField[] = ["key", "title", "issueType", "stage", "priority", "assignee", "sprint"] as const;

export interface SortState {
  field: SortField;
  ascending: boolean;
}

export const DEFAULT_SORT: SortState = {
  field: "priority",
  ascending: false,
};

const text = (value: unknown) => String(value ?? "");
const natural = (a: unknown, b: unknown) => text(a).localeCompare(text(b), undefined, { numeric: true, sensitivity: "base" });

export function compareIdentity(a: Task, b: Task): number {
  return natural(a.key || a.id, b.key || b.id) || natural(a.id, b.id);
}

const priorityRank = (task: Task) => PRIORITY_RANK[text(task.priority).toLowerCase()] || 0;

function stageRank(task: Task, board?: BoardMapping): number {
  const stage = taskStage(task, board);
  const index = stage ? stages.indexOf(stage) : -1;
  return index === -1 ? stages.length : index;
}

export function compareBy(field: SortField, board?: BoardMapping): (a: Task, b: Task) => number {
  switch (field) {
    case "key":
      return compareIdentity;
    case "title":
      return (a, b) => text(a.title).localeCompare(text(b.title), undefined, { sensitivity: "base" });
    case "issueType":
      return (a, b) => {
        const left = text(a.issueType).trim();
        const right = text(b.issueType).trim();
        if (!left && !right) return 0;
        if (!left) return 1;
        if (!right) return -1;
        return left.localeCompare(right, undefined, { sensitivity: "base" });
      };
    case "stage":
      return (a, b) => stageRank(a, board) - stageRank(b, board);
    case "assignee":
      return (a, b) => {
        const left = text(a.assignee).trim();
        const right = text(b.assignee).trim();
        if (!left && !right) return 0;
        if (!left) return 1;
        if (!right) return -1;
        return left.localeCompare(right, undefined, { sensitivity: "base" });
      };
    case "sprint":
      return (a, b) => {
        const left = text(a.sprint).trim();
        const right = text(b.sprint).trim();
        if (!left && !right) return 0;
        if (!left) return 1;
        if (!right) return -1;
        return natural(left, right);
      };
    case "priority":
    default:
      return (a, b) => priorityRank(a) - priorityRank(b);
  }
}

export function orderedTasks(
  tasks: Task[],
  sort: SortState = DEFAULT_SORT,
  board?: BoardMapping,
): Task[] {
  const compare = compareBy(sort.field, board);
  return [...tasks].sort((a, b) => {
    const result = compare(a, b);
    if (result) return sort.ascending ? result : -result;
    return compareIdentity(a, b);
  });
}

export function nextSort(current: SortState, field: SortField): SortState {
  if (current?.field === field) {
    return { field, ascending: !current.ascending };
  }
  return { field, ascending: field !== "priority" };
}
