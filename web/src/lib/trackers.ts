import type { IssueTracker, TrackerCredentials, UserSettings } from '../types'

/**
 * Ce que chaque tracker demande pour se connecter. Un seul écran, trois jeux de
 * champs : demander un site Jira et un e-mail Atlassian pour configurer GitHub
 * n'a jamais eu de sens.
 */
export type TrackerKind = TrackerCredentials['tracker']

export interface TrackerFields {
  id: TrackerKind
  label: string
  siteLabel: string
  sitePlaceholder: string
  /** Jira s'authentifie avec un e-mail ; GitHub et GitLab non. */
  wantsEmail: boolean
  projectLabel?: string
  projectPlaceholder?: string
  tokenHint: string
}

export const TRACKERS: TrackerFields[] = [
  {
    id: 'jira',
    label: 'Jira',
    siteLabel: 'Site Jira',
    sitePlaceholder: 'mon-org.atlassian.net',
    wantsEmail: true,
    projectLabel: 'Clé du projet par défaut',
    projectPlaceholder: 'PE',
    tokenHint: "À créer sur id.atlassian.com, section jetons d'API.",
  },
  {
    id: 'github',
    label: 'GitHub',
    siteLabel: "URL de l'API GitHub",
    sitePlaceholder: 'https://api.github.com',
    wantsEmail: false,
    projectLabel: 'Dépôt par défaut',
    projectPlaceholder: 'organisation/depot',
    tokenHint: 'Jeton personnel (PAT) avec la portée repo.',
  },
  {
    id: 'gitlab',
    label: 'GitLab',
    siteLabel: "URL de l'API GitLab",
    sitePlaceholder: 'https://gitlab.com/api/v4',
    wantsEmail: false,
    projectLabel: 'Projet par défaut',
    projectPlaceholder: 'groupe/projet',
    tokenHint: 'Jeton personnel avec la portée api.',
  },
]

export function trackerFields(tracker: TrackerKind): TrackerFields {
  return TRACKERS.find(t => t.id === tracker) ?? TRACKERS[0]
}

/** Le tracker proposé à l'ouverture : celui que le projet utilise déjà. */
export function initialTracker(issueTracker?: IssueTracker | string): TrackerKind {
  const known = TRACKERS.map(t => t.id as string)
  return known.includes(String(issueTracker)) ? (issueTracker as TrackerKind) : 'jira'
}

type StoredSettings = Partial<
  Pick<
    UserSettings,
    | 'jiraUrl'
    | 'jiraProject'
    | 'jiraEmail'
    | 'jiraApiTokenSet'
    | 'jiraApiTokenFromEnv'
    | 'githubApiUrl'
    | 'githubRepo'
    | 'githubTokenSet'
    | 'githubTokenFromEnv'
    | 'gitlabUrl'
    | 'gitlabProject'
    | 'gitlabTokenSet'
    | 'gitlabTokenFromEnv'
  >
>

/** Ce qui est déjà enregistré pour un tracker, pour préremplir l'écran. */
export function storedFor(settings: StoredSettings, tracker: TrackerKind) {
  switch (tracker) {
    case 'github':
      return {
        siteUrl: settings.githubApiUrl || '',
        project: settings.githubRepo || '',
        tokenIsSet: Boolean(settings.githubTokenSet),
        tokenFromEnv: Boolean(settings.githubTokenFromEnv),
      }
    case 'gitlab':
      return {
        siteUrl: settings.gitlabUrl || '',
        project: settings.gitlabProject || '',
        tokenIsSet: Boolean(settings.gitlabTokenSet),
        tokenFromEnv: Boolean(settings.gitlabTokenFromEnv),
      }
    default:
      return {
        siteUrl: settings.jiraUrl || '',
        project: settings.jiraProject || '',
        tokenIsSet: Boolean(settings.jiraApiTokenSet),
        tokenFromEnv: Boolean(settings.jiraApiTokenFromEnv),
      }
  }
}

/**
 * Le formulaire est vérifiable dès que les champs que le tracker exige sont
 * remplis. Le jeton peut rester vide : l'écran revérifie alors celui qui est
 * déjà enregistré, qu'il n'a jamais reçu en retour.
 */
export function canCheck(tracker: TrackerKind, values: { siteUrl?: string; email?: string }): boolean {
  const fields = trackerFields(tracker)
  if (!values.siteUrl?.trim()) return false
  return !fields.wantsEmail || Boolean(values.email?.trim())
}
