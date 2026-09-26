import type { LookupOption } from '../components/LookupField'
import type { EpicMeta, TrackerSprint, Project } from '../types'
import { foldForSearch } from './searchFold.ts'

export const isProjectCompatible = (
  p1: Project | null | undefined,
  p2: Project | null | undefined
): boolean => {
  if (!p1 || !p2) return false
  if (p1.id === p2.id) return false
  const t1 = (p1.issueTracker || 'local').toLowerCase().trim()
  const t2 = (p2.issueTracker || 'local').toLowerCase().trim()
  if (t1 === t2 || t1 === 'local' || t2 === 'local') return true
  const githubMilestones = (p: Project, t: string) => t !== 'gitlab' && (!!p.githubRepo || t === 'github')
  if (githubMilestones(p1, t1) && githubMilestones(p2, t2)) return true
  return false
}

/** What an empty tracker address of a project falls back to. */
export interface TrackerDefaults {
  jiraUrl?: string
  githubApiUrl?: string
  gitlabUrl?: string
  gitlabProject?: string
}

const DEFAULT_GITHUB_API = 'https://api.github.com'
const DEFAULT_GITLAB_API = 'https://gitlab.com/api/v4'

/** The tracker a project writes to, by the server's rule (Registry.ForProject). */
export const trackerKindOf = (p: Project): string => {
  const kind = (p.issueTracker || '').toLowerCase().trim()
  if (kind === '' || kind === 'local') return p.githubRepo?.trim() ? 'github' : 'local'
  return kind
}

/** A tracker URL reduced to what tells two sites apart, as the server does. */
export const trackerAddress = (raw: string | undefined): string => {
  let address = (raw || '').trim()
  const scheme = address.indexOf('://')
  if (scheme >= 0) address = address.slice(scheme + 3)
  address = address.replace(/\/+$/, '')
  const slash = address.indexOf('/')
  return slash >= 0 ? address.slice(0, slash).toLowerCase() + address.slice(slash) : address.toLowerCase()
}

/**
 * Whether a story created in target can take a macro of macroProject as its
 * parent: the same Jira site, the same GitHub repository, the same GitLab
 * project, or the local board on both sides. The server applies the same rule and has the last word; this
 * one only decides what the target picker offers.
 */
export const sameTrackerInstance = (macroProject: Project, target: Project, defaults: TrackerDefaults = {}): boolean => {
  if (macroProject.id === target.id) return true
  const kind = trackerKindOf(macroProject)
  if (kind !== trackerKindOf(target)) return false
  if (kind === 'local') return true
  if (kind === 'jira') {
    const site = (p: Project) => trackerAddress(p.trackerUrl?.trim() ? p.trackerUrl : defaults.jiraUrl)
    return site(macroProject) !== '' && site(macroProject) === site(target)
  }
  if (kind === 'github') {
    const api = (p: Project) => trackerAddress(p.githubApiUrl?.trim() || defaults.githubApiUrl?.trim() || DEFAULT_GITHUB_API)
    return api(macroProject) === api(target)
      && (macroProject.githubRepo || '').trim().toLowerCase() === (target.githubRepo || '').trim().toLowerCase()
  }
  if (kind === 'gitlab') {
    const api = (p: Project) => trackerAddress(p.gitlabUrl?.trim() || defaults.gitlabUrl?.trim() || DEFAULT_GITLAB_API)
    const path = (p: Project) => (p.gitlabProject?.trim() || defaults.gitlabProject?.trim() || '').replace(/^\/+|\/+$/g, '').toLowerCase()
    return api(macroProject) === api(target) && path(macroProject) === path(target)
  }
  return false
}

/** The other projects a slicing line's story may be created in. */
export const targetProjectOptions = (macroProject: Project, projects: Project[], defaults: TrackerDefaults = {}): Project[] =>
  projects.filter(p => p.id !== macroProject.id && sameTrackerInstance(macroProject, p, defaults))

/**
 * Sources de recherche pour les champs de type lookup.
 *
 * Deux familles de valeurs cohabitent dans l'outil. Celles qui vivent dans le
 * tracker et qu'aucune liste locale ne contient (les personnes, les équipes de
 * l'instance) sont cherchées par un appel réseau. Celles que la synchronisation a
 * déjà ramenées (les sprints du board, les épics du projet) sont filtrées ici,
 * sans réseau : un projet porte cent quarante épics, ce qui rend une liste
 * déroulante inutilisable, mais reste minuscule à filtrer en mémoire.
 *
 * Toutes renvoient une promesse, pour que le champ de recherche traite les deux
 * familles de la même façon.
 */

/** Nombre de propositions rendues sans frappe : au delà, la liste ne se lit plus. */
const DEFAULT_LIMIT = 40

const matches = (haystack: string, query: string): boolean =>
  foldForSearch(haystack).includes(foldForSearch(query.trim()))

/**
 * Macros du projet, cherchées par clé ou par titre. Les macros terminées sont exclues
 * par défaut : on ne rattache pas un ticket à un chantier clos, mais la macro
 * courante d'un ticket reste proposée par l'appelant si elle est hors liste.
 */
export const macroLookup =
  (macros: EpicMeta[], options?: { includeClosed?: boolean }) =>
  async (query: string): Promise<LookupOption[]> => {
    const pool = options?.includeClosed ? macros : macros.filter(macro => !macro.closed)
    const found = pool.filter(macro => {
      if (!query.trim()) return true
      return matches(macro.key, query) || matches(macro.title || '', query)
    })
    return found.slice(0, DEFAULT_LIMIT).map(macro => ({
      id: macro.key,
      label: macro.key,
      sublabel: macro.title || undefined,
    }))
  }

export const epicLookup = macroLookup

/**
 * Sprints du board, cherchés par nom. Les sprints clos sont écartés, ainsi que
 * ceux dépourvus d'identifiant : l'API Agile ne déplace un ticket que par
 * identifiant, donc un sprint sans le sien ne serait pas applicable.
 */
export const sprintLookup =
  (sprints: TrackerSprint[]) =>
  async (query: string): Promise<LookupOption[]> => {
    const pool = sprints.filter(sprint => sprint.id && sprint.state !== 'closed')
    const found = pool.filter(sprint => (query.trim() ? matches(sprint.name, query) : true))
    return found.slice(0, DEFAULT_LIMIT).map(sprint => {
      const parts = [sprintKindLabel(sprint.id), sprint.state === 'active' ? 'sprint en cours' : ''].filter(Boolean)
      return {
        id: sprint.id as string,
        label: sprint.name,
        sublabel: parts.length ? parts.join(' · ') : undefined,
      }
    })
  }

/**
 * The kind of a GitLab sprint, which its id carries (milestone:<id> or
 * iteration:<id>): both are sprints there, and the picker says which. Any
 * other tracker's sprint has one kind and no label.
 */
export const sprintKindLabel = (id: string | undefined): string => {
  if (id?.startsWith('milestone:')) return 'Jalon'
  if (id?.startsWith('iteration:')) return 'Itération'
  return ''
}

/**
 * Valeurs déjà présentes sur les tickets, pour les filtres : un sprint, une
 * équipe ou une personne que le board porte réellement. Le compteur donne le
 * poids de chaque valeur, ce qui aide à choisir.
 */
export const valueLookup =
  (values: string[], counts?: Record<string, number>) =>
  async (query: string): Promise<LookupOption[]> => {
    const found = values.filter(value => (query.trim() ? matches(value, query) : true))
    return found.slice(0, DEFAULT_LIMIT).map(value => ({
      id: value,
      label: value,
      sublabel: counts?.[value] ? `${counts[value]} ticket(s)` : undefined,
    }))
  }
