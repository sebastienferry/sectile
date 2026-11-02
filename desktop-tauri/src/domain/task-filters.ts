import type { Task } from "./board.ts";
import { taskStage } from "./workflow.ts";
import type { BoardMapping } from "../lib/workspace.ts";

export interface TaskFilters {
  query?: string;
  priority?: string;
  status?: string;
  assignee?: string;
  parent?: string;
}

export function matchesTaskFilters(
  task: Task,
  filters: TaskFilters,
  board?: BoardMapping,
): boolean {
  if (filters.query?.trim()) {
    const search = filters.query.trim().toLocaleLowerCase();
    if (!`${task.key ?? ""} ${task.title}`.toLocaleLowerCase().includes(search)) {
      return false;
    }
  }
  if (filters.priority?.trim()) {
    const target = filters.priority.trim().toLowerCase();
    if ((task.priority ?? "").toLowerCase() !== target) {
      return false;
    }
  }
  if (filters.status?.trim()) {
    const target = filters.status.trim().toLowerCase();
    const stage = taskStage(task, board);
    const matchesTracker = (task.trackerStatus ?? "").toLowerCase() === target;
    const matchesStage = stage?.toLowerCase() === target;
    const matchesRawStatus = (task.status ?? "").toLowerCase() === target;
    if (!matchesTracker && !matchesStage && !matchesRawStatus) {
      return false;
    }
  }
  if (filters.assignee?.trim()) {
    if (filters.assignee === "__unassigned__") {
      if (task.assignee?.trim()) return false;
    } else {
      if ((task.assignee ?? "").trim().toLowerCase() !== filters.assignee.trim().toLowerCase()) {
        return false;
      }
    }
  }
  if (filters.parent?.trim()) {
    if (filters.parent === "__none__") {
      if (task.parentKey?.trim()) return false;
    } else {
      if ((task.parentKey ?? "").trim().toLowerCase() !== filters.parent.trim().toLowerCase()) {
        return false;
      }
    }
  }
  return true;
}

export function taskFilterPredicate(
  filters: TaskFilters,
  board?: BoardMapping,
): (task: Task) => boolean {
  return (task: Task) => matchesTaskFilters(task, filters, board);
}
