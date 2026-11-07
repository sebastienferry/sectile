export interface Project {
  id: string;
  name: string;
  path?: string;
  disconnected?: boolean;
  orchestratorEnabled?: boolean;
}
export interface TaskPullRequest {
  url: string;
  state?: string;
  branch?: string;
  repository?: string;
  missingToken?: string;
}
export interface Task {
  id: string;
  title: string;
  key?: string;
  source?: string;
  trackerStatus?: string;
  status: string;
  assignee?: string;
  priority?: string;
  parentKey?: string;
  parentTitle?: string;
  sprint?: string;
  labels?: string[];
  prUrl?: string;
  prLinks?: TaskPullRequest[];
  trackerId?: string;
  description?: string;
  issueType?: string;
}
export interface Column {
  name: string;
  tasks: Task[];
}

export function trackerColumns(
  tasks: Task[],
  filter: string | ((task: Task) => boolean) = "",
): Column[] {
  const groups = new Map<string, Task[]>();
  const predicate =
    typeof filter === "function"
      ? filter
      : (task: Task) => {
          const search = filter.trim().toLocaleLowerCase();
          return `${task.key ?? ""} ${task.title}`.toLocaleLowerCase().includes(search);
        };
  for (const task of tasks) {
    // Only known remote sources belong on this board; unknown sources fail closed.
    if (!["github", "gitlab", "jira"].includes(task.source ?? "")) continue;
    if (!predicate(task)) continue;
    const name = task.trackerStatus?.trim() || "Statut distant indisponible";
    const group = groups.get(name);
    if (group) group.push(task);
    else groups.set(name, [task]);
  }
  return [...groups].map(([name, tasks]) => ({ name, tasks }));
}
