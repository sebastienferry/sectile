import type { Project } from '../types'

/**
 * Reduces a Git remote, in URL or scp-like form, to host/path: lowercased,
 * without scheme, user, port, ".git" suffix or trailing slash. It mirrors the
 * server's models.RepositoryIdentity, so a remote the form refuses as a
 * duplicate is exactly one the server would refuse too.
 */
export function repositoryIdentity(remote: string): string {
  let value = remote.trim().replace(/\/+$/, '').replace(/\.git$/, '')
  const url = /^[a-z][a-z0-9+.-]*:\/\/([^/]*)(.*)$/i.exec(value)
  if (url && url[1]) {
    const host = url[1].replace(/^.*@/, '').replace(/:\d*$/, '')
    return (host + url[2].replace(/\/+$/, '')).toLowerCase()
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
 * The remotes a project declares besides its code remote, which the server
 * always lists first and derives from gitRemoteUrl rather than storing it.
 */
export function declaredRepositories(project: Pick<Project, 'gitRemoteUrl' | 'repositories'>): string[] {
  const code = project.gitRemoteUrl?.trim() ? repositoryIdentity(project.gitRemoteUrl) : ''
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
