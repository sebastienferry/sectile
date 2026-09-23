import type { PullRequestLink, Task } from '../types'

/** Ordered links, including the legacy single-URL fallback. */
export function taskPullRequestLinks(task: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>): PullRequestLink[] {
  if (task.prLinks && task.prLinks.length > 0) return task.prLinks
  return task.prUrl ? [{ url: task.prUrl, branch: task.branchName }] : []
}

/** The current PR is the last recorded link. */
export function currentPullRequest(links: PullRequestLink[]): string | undefined {
  return links.length > 0 ? links[links.length - 1].url : undefined
}

/** Append a new link without reordering or changing observed history. */
export function addPullRequestLink(links: PullRequestLink[], url: string, branch?: string): PullRequestLink[] {
  const trimmed = url.trim()
  if (!trimmed || links.some(l => l.url === trimmed)) return links
  return [...links, { url: trimmed, branch: branch?.trim() || undefined }]
}

/** Resolve the current link without deriving merge state from workflow. */
export function currentPullRequestLink(task: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>): PullRequestLink | undefined {
  return taskPullRequestLinks(task).at(-1)
}

export function pullRequestStateLabel(state?: string): string {
  switch (state) {
    case 'open': return 'PR ouverte'
    case 'conflicting': return 'PR en conflits'
    case 'merged': return 'PR fusionnée'
    case 'closed': return 'PR fermée sans fusion'
    default: return 'État de la PR inconnu'
  }
}
