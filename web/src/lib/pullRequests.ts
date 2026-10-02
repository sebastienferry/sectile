import type { PullRequestLink, Task } from '../types'
import { format } from './i18n.ts'

/** Ordered links, including the legacy single-URL fallback. */
export function taskPullRequestLinks(task: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>): PullRequestLink[] {
  if (task.prLinks && task.prLinks.length > 0) return task.prLinks
  return task.prUrl ? [{ url: task.prUrl, branch: task.branchName }] : []
}

/** The current PR is the last recorded link. */
export function currentPullRequest(links: PullRequestLink[]): string | undefined {
  return links.length > 0 ? links[links.length - 1].url : undefined
}

/**
 * Append a new link without changing observed history. A link of another
 * repository than the current one's goes before it, so the primary
 * repository's pull request stays the current one, as the server keeps it.
 */
export function addPullRequestLink(links: PullRequestLink[], url: string, branch?: string): PullRequestLink[] {
  const trimmed = url.trim()
  if (!trimmed || links.some(l => l.url === trimmed)) return links
  const repository = pullRequestRepository(trimmed)
  const added: PullRequestLink = { url: trimmed, branch: branch?.trim() || undefined }
  if (repository) added.repository = repository
  const current = links.at(-1)
  const primary = current ? repositoryOf(current) : ''
  if (current && primary && repository && repository !== primary) return [...links.slice(0, -1), added, current]
  return [...links, added]
}

/** Resolve the current link without deriving merge state from workflow. */
export function currentPullRequestLink(task: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>): PullRequestLink | undefined {
  return taskPullRequestLinks(task).at(-1)
}

/**
 * The repository a pull request URL names, as `host/path` in lower case, the
 * identity the server derives too: a GitHub pull request on a host naming
 * GitHub, or a GitLab merge request on any host. Empty for anything else.
 */
export function pullRequestRepository(url: string): string {
  let parsed: URL
  try {
    parsed = new URL(url.trim())
  } catch {
    return ''
  }
  if ((parsed.protocol !== 'https:' && parsed.protocol !== 'http:') || parsed.username || parsed.password || parsed.search || parsed.hash) return ''
  const host = parsed.hostname.toLowerCase()
  const parts = parsed.pathname.replace(/^\/+|\/+$/g, '').split('/')
  if (parts.some(part => part === '' || part === '.' || part === '..')) return ''
  const n = parts.length
  let repository = ''
  let number = ''
  if (n >= 5 && parts[n - 3] === '-' && parts[n - 2] === 'merge_requests') {
    repository = parts.slice(0, n - 3).join('/')
    number = parts[n - 1]
  } else if (n === 4 && parts[2] === 'pull' && host.includes('github')) {
    repository = parts.slice(0, 2).join('/')
    number = parts[3]
  }
  if (!repository || !/^[1-9]\d*$/.test(number)) return ''
  return (host + '/' + repository).toLowerCase()
}

/**
 * The repository of a link: the one it was recorded with, which stays put
 * while its URL is being edited, else the one its URL names.
 */
function repositoryOf(link: PullRequestLink): string {
  return link.repository ?? pullRequestRepository(link.url)
}

/** The pull requests of one repository a task changed. */
export interface RepositoryPullRequests {
  /** `host/path`, empty when no link of the group names a repository. */
  repository: string
  /** Positions of the group's links in the task's set, in recorded order. */
  indices: number[]
  /** The repository's current pull request: its last link. */
  current: PullRequestLink
}

/**
 * The links grouped per repository, the primary repository's first. The
 * server keeps the primary repository's current pull request last, so the
 * last link names the primary repository; a link naming none is folded into
 * it, as every link was before a task could span repositories.
 */
export function repositoryPullRequests(links: PullRequestLink[]): RepositoryPullRequests[] {
  if (links.length === 0) return []
  const primary = repositoryOf(links[links.length - 1])
  const groups = new Map<string, number[]>([[primary, []]])
  links.forEach((link, index) => {
    const key = repositoryOf(link) || primary
    groups.set(key, [...(groups.get(key) ?? []), index])
  })
  return [...groups].map(([repository, indices]) => ({ repository, indices, current: links[indices[indices.length - 1]] }))
}

/** The words of a pull request's state, naming the missing token when that is why it is unknown. */
export function pullRequestStateLabel(
  link: PullRequestLink | undefined,
  states: { open: string, conflicting: string, merged: string, closed: string, unknown: string, missingToken: string },
): string {
  const state = link?.state
  if (state === 'open' || state === 'conflicting' || state === 'merged' || state === 'closed') return states[state]
  if (link?.missingToken) return format(states.missingToken, { forge: link.missingToken === 'gitlab' ? 'GitLab' : 'GitHub' })
  return states.unknown
}
