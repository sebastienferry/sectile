import type { IssueTracker, Project } from '../types'
import { foldForSearch } from './searchFold.ts'
import type { ProjectOpening } from './projectHistory.ts'

/**
 * What the project picker and the project overview list (#582): which fields a
 * search reads, in which order projects come, and how many rows the short menu
 * shows. The menu never scrolls, so every list it draws is bounded here.
 */

export type MatchField = 'name' | 'slug' | 'description' | 'repository' | 'tracker'

export const PICKER_RECENT_LIMIT = 3
export const PICKER_FAVORITE_LIMIT = 6
export const PICKER_RESULT_LIMIT = 6

const TRACKER_LABELS: Record<IssueTracker, string> = {
  github: 'GitHub',
  gitlab: 'GitLab',
  jira: 'Jira',
  local: 'Local',
}

export const PROJECT_TRACKERS: IssueTracker[] = ['github', 'gitlab', 'jira', 'local']

/** GitHub, GitLab, Jira or Local: product names, the same in every language. */
export function trackerLabel(tracker: IssueTracker | undefined): string {
  return TRACKER_LABELS[tracker || 'local'] ?? tracker ?? ''
}

/**
 * Shortens a remote to its path, `owner/name`, keeping the case it was typed
 * in. A value that is already a path (`githubRepo`, `gitlabProject`) is kept.
 */
export function repositoryLabel(remote: string): string {
  let value = remote.trim()
  const url = /^[a-z][a-z0-9+.-]*:\/\/[^/]*(.*)$/i.exec(value)
  if (url) value = url[1]
  else {
    // scp-like: git@github.com:owner/name.git
    const scp = /^[^/:]+:(.*)$/.exec(value)
    if (scp) value = scp[1]
  }
  return value.replace(/\/+$/, '').replace(/\.git$/, '').replace(/^\/+/, '')
}

/** The project's repositories as short labels, the code remote first, without duplicates. */
export function projectRepositories(project: Project): string[] {
  const remotes = project.repositories && project.repositories.length > 0
    ? project.repositories.map(r => r.url)
    : [project.gitRemoteUrl || '']
  const labels = [...remotes, project.githubRepo, project.gitlabProject || '']
    .map(remote => (remote ? repositoryLabel(remote) : ''))
    .filter(Boolean)
  const seen = new Set<string>()
  return labels.filter(label => {
    const key = label.toLowerCase()
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

export interface ProjectMatch {
  field: MatchField
  /** The field's text that contains the query, for highlighting. */
  text: string
}

/**
 * The first field of the project that contains the query, in the order name,
 * slug, description, repository, tracker. Case and accents are ignored the way
 * the board's search ignores them (#447). A blank query matches on the name.
 */
export function matchProject(project: Project, query: string): ProjectMatch | null {
  const q = foldForSearch(query.trim())
  if (!q) return { field: 'name', text: project.name }
  const contains = (text: string | undefined) => !!text && foldForSearch(text).includes(q)
  if (contains(project.name)) return { field: 'name', text: project.name }
  if (contains(project.slug)) return { field: 'slug', text: project.slug }
  if (contains(project.description)) return { field: 'description', text: project.description }
  const repository = projectRepositories(project).find(contains)
  if (repository) return { field: 'repository', text: repository }
  const tracker = trackerLabel(project.issueTracker)
  if (contains(tracker)) return { field: 'tracker', text: tracker }
  if (contains(project.jiraProject)) return { field: 'tracker', text: project.jiraProject || '' }
  return null
}

/**
 * The "tracker · repository" line of a row or a card, showing the field that
 * matched whenever it is one of them: the Jira key next to the tracker, the
 * slug or the matching repository in place of the first repository.
 */
export interface ProjectLocation {
  tracker: string
  /** The Jira key, shown only when the query matched it. */
  key: string
  location: string
  /** Which part holds the match, to highlight it; null when none does. */
  matched: 'tracker' | 'key' | 'location' | null
}

export function projectLocation(project: Project, match: ProjectMatch | null): ProjectLocation {
  const tracker = trackerLabel(project.issueTracker)
  const key = match?.field === 'tracker' && match.text !== tracker ? match.text : ''
  const location = match?.field === 'slug'
    ? project.slug
    : match?.field === 'repository'
    ? match.text
    : projectRepositories(project)[0] || project.slug
  const matched = key
    ? 'key'
    : match?.field === 'tracker'
    ? 'tracker'
    : match?.field === 'slug' || match?.field === 'repository'
    ? 'location'
    : null
  return { tracker, key, location, matched }
}

/**
 * Favorites first, A–Z; then the other projects, the most recently opened
 * first, and A–Z for those never opened in this browser.
 */
export function orderProjects(projects: Project[], history: ProjectOpening[], locale: string): Project[] {
  const rank = new Map(history.map((entry, index) => [entry.id, index]))
  const byName = (a: Project, b: Project) => a.name.localeCompare(b.name, locale, { sensitivity: 'base' })
  const favorites = projects.filter(p => p.bookmarked).sort(byName)
  const others = projects.filter(p => !p.bookmarked).sort((a, b) => {
    const ra = rank.get(a.id) ?? Infinity
    const rb = rank.get(b.id) ?? Infinity
    return ra === rb ? byName(a, b) : ra - rb
  })
  return [...favorites, ...others]
}

export interface PickerRow {
  project: Project
  match: ProjectMatch
}

export interface PickerModel {
  /** Empty search only: recently opened projects that are not favorites. */
  recent: PickerRow[]
  favorites: PickerRow[]
  /** Search only: the matches that are not favorites. */
  others: PickerRow[]
  /** Empty search: favorites left out of the menu. */
  hiddenFavorites: number
  /** Search: matches left out of the menu, favorites included. */
  hiddenMatches: number
}

/** The rows of the short menu for a query, blank or not. */
export function pickerModel(
  projects: Project[],
  history: ProjectOpening[],
  query: string,
  locale: string,
): PickerModel {
  const ordered = orderProjects(projects, history, locale)
  if (!query.trim()) {
    const byId = new Map(projects.map(p => [p.id, p]))
    const recent = history
      .map(entry => byId.get(entry.id))
      .filter((p): p is Project => !!p && !p.bookmarked)
      .slice(0, PICKER_RECENT_LIMIT)
      .map(project => ({ project, match: { field: 'name' as const, text: project.name } }))
    const allFavorites = ordered.filter(p => p.bookmarked)
    return {
      recent,
      favorites: allFavorites
        .slice(0, PICKER_FAVORITE_LIMIT)
        .map(project => ({ project, match: { field: 'name' as const, text: project.name } })),
      others: [],
      hiddenFavorites: Math.max(0, allFavorites.length - PICKER_FAVORITE_LIMIT),
      hiddenMatches: 0,
    }
  }
  const matches = ordered.flatMap(project => {
    const match = matchProject(project, query)
    return match ? [{ project, match }] : []
  })
  const shown = matches.slice(0, PICKER_RESULT_LIMIT)
  return {
    recent: [],
    favorites: shown.filter(row => row.project.bookmarked),
    others: shown.filter(row => !row.project.bookmarked),
    hiddenFavorites: 0,
    hiddenMatches: matches.length - shown.length,
  }
}

/** Every project of the overview, ordered, filtered by text and tracker. */
export function overviewProjects(
  projects: Project[],
  history: ProjectOpening[],
  query: string,
  tracker: IssueTracker | 'all',
  locale: string,
): Project[] {
  return orderProjects(projects, history, locale)
    .filter(p => tracker === 'all' || (p.issueTracker || 'local') === tracker)
    .filter(p => matchProject(p, query) !== null)
}

/**
 * Folds `text` one code point at a time, remembering where each folded
 * character came from: stripping diacritics changes lengths, so an offset in
 * the folded text is not an offset in the original.
 */
function foldWithOffsets(text: string): { folded: string; origin: number[] } {
  let folded = ''
  const origin: number[] = []
  let offset = 0
  for (const char of text) {
    const piece = foldForSearch(char)
    for (let i = 0; i < piece.length; i++) origin.push(offset)
    folded += piece
    offset += char.length
  }
  origin.push(offset)
  return { folded, origin }
}

/** Where the query occurs in the original text, or null. */
function findMatch(text: string, query: string): { start: number; end: number } | null {
  const q = foldForSearch(query.trim())
  if (!q) return null
  const { folded, origin } = foldWithOffsets(text)
  const index = folded.indexOf(q)
  if (index < 0) return null
  const start = origin[index]
  // The end is the start of the character after the last matched one.
  let end = origin[index + q.length]
  if (end === start) end = origin[origin.length - 1]
  return { start, end }
}

export interface HighlightPart {
  text: string
  match: boolean
}

/** Splits the text around the first occurrence of the query, for a <mark>. */
export function highlightParts(text: string, query: string): HighlightPart[] {
  const found = findMatch(text, query)
  if (!found) return [{ text, match: false }]
  return [
    { text: text.slice(0, found.start), match: false },
    { text: text.slice(found.start, found.end), match: true },
    { text: text.slice(found.end), match: false },
  ].filter(part => part.text !== '')
}

/** How much of the description is kept before the match. */
const EXCERPT_LEAD = 20

/**
 * The description from a little before the match, with a leading ellipsis
 * when it is cut. The end is left to the row's own truncation.
 */
export function descriptionExcerpt(text: string, query: string): string {
  const found = findMatch(text, query)
  if (!found || found.start <= EXCERPT_LEAD) return text
  const cut = text.lastIndexOf(' ', found.start - EXCERPT_LEAD)
  const start = cut > 0 && found.start - cut <= EXCERPT_LEAD * 2 ? cut + 1 : found.start - EXCERPT_LEAD
  return '…' + text.slice(start)
}
