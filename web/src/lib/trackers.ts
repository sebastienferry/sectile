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
  /**
   * Un jeton que le tracker attribue à un compte n'a de sens que personnel :
   * sur Jira, un commentaire porte le nom du compte qui l'a écrit. L'écran
   * n'offre alors pas d'enregistrer pour le serveur, dont le jeton ne reste
   * qu'un repli de configuration, hors interface.
   */
  personalOnly?: boolean
  /**
   * Le site fait partie de l'accès personnel : un compte Atlassian appartient
   * à un site, donc la personne, son instance et son jeton voyagent ensemble.
   */
  siteIsPersonal?: boolean
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
    tokenHint: "À créer sur id.atlassian.com, section jetons d'API. Il s'authentifie avec votre e-mail Atlassian, pas seul.",
    personalOnly: true,
    siteIsPersonal: true,
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
  const fields = trackerFields(tracker)
  if (!fields.wantsEmail) return true
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
      ? "Renseignez votre site et l'e-mail de votre compte, puis vérifiez les accès."
      : 'Renseignez les accès, puis vérifiez-les.'
  }
  return "Vérifiez les accès : l'enregistrement se débloque une fois que l'instance les a acceptés."
}

/**
 * Pour qui un jeton est enregistré. Un jeton serveur sert toutes les écritures,
 * y compris celles de la file de fond. Un jeton personnel porte le nom de la
 * personne sur ce qu'elle écrit, ce qui compte sur Jira où une écriture est
 * attribuée au compte du jeton.
 */
export type CredentialScope = 'server' | 'personal'

/** Un accès personnel déjà enregistré, tel que l'API le décrit : jamais le jeton. */
export interface StoredUserCredential {
  tracker: string
  siteUrl?: string
  email?: string
  sealed: boolean
  unlocked: boolean
  updatedAt?: string
}

/**
 * Ce que le scellement change, dit au moment du choix plutôt qu'après coup.
 * Une phrase de scellement rend le jeton illisible par le serveur seul, donc
 * inutilisable par tout ce qui tourne sans son propriétaire.
 */
export function sealingConsequence(sealed: boolean): string {
  return sealed
    ? "Scellé : personne ne peut l'ouvrir sans votre phrase, pas même le serveur. Les écritures parties en file de fond échoueront tant que vous ne l'aurez pas déverrouillé."
    : "Chiffré avec la clé du serveur, qui vit hors de la base. Une copie de la base ne le livre pas, et il reste utilisable par les écritures de fond."
}

/** L'état d'un accès personnel, en une phrase, pour l'écran. */
export function credentialState(credential?: StoredUserCredential): string {
  if (!credential) return "Aucun accès personnel : vos écritures partent avec le jeton du serveur."
  if (!credential.sealed) return 'Enregistré et chiffré avec la clé du serveur.'
  return credential.unlocked ? 'Scellé et déverrouillé pour cette session.' : 'Scellé et verrouillé : saisissez votre phrase pour le déverrouiller.'
}

/**
 * Pour qui l'écran peut enregistrer ce tracker. Un tracker qui attribue ses
 * écritures à un compte n'accepte que le personnel : son jeton serveur reste
 * un repli de configuration, jamais une case de l'interface.
 */
export function scopesFor(tracker: TrackerKind): CredentialScope[] {
  return trackerFields(tracker).personalOnly ? ['personal'] : ['server', 'personal']
}
