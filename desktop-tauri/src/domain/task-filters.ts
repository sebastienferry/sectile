import type { Task } from "./board.ts";
import { taskStage } from "./workflow.ts";
import type { BoardMapping } from "../lib/workspace.ts";

export interface TaskFilters {
  query?: string;
  priority?: string;
  status?: string;
  statuses?: string[];
  assignee?: string;
  parent?: string;
  sprint?: string;
  issueType?: string;
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
  const statusTargets = (
    filters.statuses && filters.statuses.length > 0
      ? filters.statuses
      : filters.status?.trim()
        ? filters.status.split(",")
        : []
  )
    .map(value => value.trim().toLowerCase())
    .filter(Boolean);
  if (statusTargets.length > 0) {
    const stage = taskStage(task, board)?.toLowerCase() ?? "";
    const trackerStatus = (task.trackerStatus ?? "").toLowerCase();
    const rawStatus = (task.status ?? "").toLowerCase();
    const matchesAny = statusTargets.some(
      target => trackerStatus === target || stage === target || rawStatus === target,
    );
    if (!matchesAny) {
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
  if (filters.sprint?.trim()) {
    const target = filters.sprint.trim().toLowerCase();
    if (target === "__none__" || target === "no sprint" || target === "sans sprint") {
      if (task.sprint?.trim()) return false;
    } else {
      if (!(task.sprint ?? "").trim().toLowerCase().includes(target)) {
        return false;
      }
    }
  }
  if (filters.issueType?.trim()) {
    const target = filters.issueType.trim().toLowerCase();
    if (target === "__none__" || target === "no type" || target === "sans type") {
      if (task.issueType?.trim()) return false;
    } else {
      if ((task.issueType ?? "").trim().toLowerCase() !== target) {
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
