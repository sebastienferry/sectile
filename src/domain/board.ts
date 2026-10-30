export interface Project {
  id: string;
  name: string;
  path?: string;
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
  labels?: string[];
  prUrl?: string;
  prLinks?: { url: string; state?: string; branch?: string; repository?: string; missingToken?: string }[];
  trackerId?: string;
  description?: string;
}
export interface Column {
  name: string;
  tasks: Task[];
}

export function trackerColumns(tasks: Task[], query = ""): Column[] {
  const groups = new Map<string, Task[]>();
  const search = query.trim().toLocaleLowerCase();
  for (const task of tasks) {
    // Only known remote sources belong on this board; unknown sources fail closed.
    if (!["github", "gitlab", "jira"].includes(task.source ?? "")) continue;
    if (!`${task.key ?? ""} ${task.title}`.toLocaleLowerCase().includes(search))
      continue;
    const name = task.trackerStatus?.trim() || "Statut distant indisponible";
    const group = groups.get(name);
    if (group) group.push(task);
    else groups.set(name, [task]);
  }
  return [...groups].map(([name, tasks]) => ({ name, tasks }));
}
