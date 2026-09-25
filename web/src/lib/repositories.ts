import type { Project } from '../types'

/**
 * Reduces a Git remote, in URL or scp-like form, to host/path: lowercased,
 * without scheme, user, port, ".git" suffix or trailing slash. It mirrors the
 * server's models.RepositoryIdentity, so a remote the form refuses as a
 * duplicate is exactly one the server would refuse too.
 */
export function repositoryIdentity(remote: string): string {
  let value = remote.trim().replace(/\/+$/, '').replace(/\.git$/, '')
  const url = /^[a-z][a-z0-9+.-]*:\/\/([^/?#]*)([^?#]*)/i.exec(value)
  if (url && url[1]) {
    // Like Go's url.Hostname: no user, no port, no IPv6 brackets.
    const authority = url[1].slice(url[1].lastIndexOf('@') + 1)
    const bracketed = /^\[([^\]]*)\]/.exec(authority)
    const host = bracketed ? bracketed[1] : authority.replace(/:\d*$/, '')
    // Like Go's url.Path: without query or fragment, and decoded.
    let path = url[2]
    try {
      path = decodeURIComponent(path)
    } catch {
      // A malformed escape stays as typed, which Go would refuse outright.
    }
    return (host + path.replace(/\/+$/, '')).toLowerCase()
  }
  const at = value.indexOf('@')
  if (at >= 0 && at < value.indexOf(':')) value = value.slice(at + 1)
  return value.replace(':', '/').toLowerCase()
}

/**
 * The first remote of `candidates` whose identity is already taken, by the code
 * remote or by an earlier candidate. Empty when every remote is distinct.
 */
export function duplicateRepository(codeRemote: string, candidates: string[]): string {
  const seen = new Set<string>()
  if (codeRemote.trim()) seen.add(repositoryIdentity(codeRemote))
  for (const candidate of candidates) {
    if (!candidate.trim()) continue
    const identity = repositoryIdentity(candidate)
    if (seen.has(identity)) return candidate
    seen.add(identity)
  }
  return ''
}

/**
 * The identity of a project's code remote as the server derives it: its git
 * remote, else the GitHub repository its pull requests live in. Empty when the
 * project has neither.
 */
export function codeRepositoryIdentity(project: Pick<Project, 'gitRemoteUrl' | 'githubRepo'>): string {
  if (project.gitRemoteUrl?.trim()) return repositoryIdentity(project.gitRemoteUrl)
  const repo = (project.githubRepo || '').trim().replace(/^\/+|\/+$/g, '')
  if (!repo) return ''
  return /:\/\/|@/.test(repo) ? repositoryIdentity(repo) : ('github.com/' + repo).toLowerCase()
}

/**
 * The remotes a project declares besides its code remote, which the server
 * always lists first and derives rather than storing it.
 */
export function declaredRepositories(project: Pick<Project, 'gitRemoteUrl' | 'githubRepo' | 'repositories'>): string[] {
  const code = codeRepositoryIdentity(project)
  return (project.repositories || [])
    .filter(repository => repository.identity !== code)
    .map(repository => repository.url)
}

export interface DroppedRepositoryPath {
  path: string
  reason: string
  taskIds?: string[]
}

/**
 * The legacy paths the repositories migration could not convert. An empty,
 * malformed or fully converted report yields nothing to show.
 */
export function droppedRepositoryPaths(report?: string): DroppedRepositoryPath[] {
  if (!report?.trim()) return []
  try {
    const parsed = JSON.parse(report) as { dropped?: DroppedRepositoryPath[] }
    return Array.isArray(parsed.dropped) ? parsed.dropped.filter(entry => entry && entry.path) : []
  } catch {
    return []
  }
}
