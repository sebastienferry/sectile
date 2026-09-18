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

/**
 * Les trackers qu'un projet peut réellement porter : ceux dont un adaptateur est
 * enregistré côté serveur (`trackerapi.NewDefaultRegistry`). GitLab n'y est pas,
 * ses paramètres se configurent sans qu'aucun projet puisse le choisir.
 *
 * Cette liste est partagée par la fiche projet et la vue de synchronisation.
 * Les deux avaient leur propre énumération, et elles ont divergé : Jira a
 * disparu de la fiche projet sans disparaître de la synchronisation, donc aucun
 * projet ne pouvait plus être posé dessus.
 */
export const PROJECT_TRACKERS: { id: IssueTracker; label: string }[] = [
  { id: 'local', label: 'Sectile (Local)' },
  { id: 'github', label: 'GitHub Issues' },
  { id: 'jira', label: 'Jira' },
]

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
 *
 * Le site aussi peut rester vide là où le serveur sait retomber sur une
 * instance publique, ce que font GitHub et GitLab. Jira n'a pas d'instance par
 * défaut, et son authentification porte sur un couple : site et e-mail y sont
 * donc exigés.
 */
export function canCheck(tracker: TrackerKind, values: { siteUrl?: string; email?: string }): boolean {
  if (!trackerFields(tracker).wantsEmail) return true
  return Boolean(values.siteUrl?.trim()) && Boolean(values.email?.trim())
}

/**
 * Pourquoi l'enregistrement est bloqué, ou une chaîne vide quand il ne l'est
 * pas. L'écran n'annonçait la règle que dans une infobulle, invisible tant
 * qu'on ne survole pas un bouton déjà grisé.
 */
export function saveBlockedReason(tracker: TrackerKind, values: { siteUrl?: string; email?: string }, checked: boolean): string {
  if (checked) return ''
  if (!canCheck(tracker, values)) {
    return trackerFields(tracker).wantsEmail
      ? "Renseignez le site et l'e-mail du compte, puis vérifiez les accès."
      : 'Renseignez les accès, puis vérifiez-les.'
  }
  return "Vérifiez les accès : l'enregistrement se débloque une fois que l'instance les a acceptés."
}
