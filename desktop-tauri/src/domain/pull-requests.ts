import type { Task } from "./board.ts";

export type PullRequestLink = NonNullable<Task["prLinks"]>[number];

export function taskPullRequests(task?: Task): PullRequestLink[] {
  if (!task) return [];
  const links = (task.prLinks ?? []).filter(link => /^https?:\/\//.test(link.url));
  if (links.length) return links;
  return task.prUrl && /^https?:\/\//.test(task.prUrl) ? [{ url: task.prUrl }] : [];
}

export function pullRequestStateLabel(link: PullRequestLink): string {
  if (link.missingToken) return "état indisponible · connexion forge requise";
  return ({ open: "ouverte", conflicting: "en conflit", merged: "fusionnée", closed: "fermée" } as Record<string, string>)[link.state ?? ""] ?? "état inconnu";
}

export function pullRequestLabel(link: PullRequestLink): string {
  try {
    const path = new URL(link.url).pathname;
    const github = path.match(/\/pull\/(\d+)\/?$/);
    if (github) return `PR #${github[1]}`;
    const gitlab = path.match(/\/merge_requests\/(\d+)\/?$/);
    if (gitlab) return `MR !${gitlab[1]}`;
  } catch { /* The native opener will reject malformed URLs. */ }
  return "PR/MR";
}
