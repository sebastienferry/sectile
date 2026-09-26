import type { IssueTracker, TrackerCredentials, UserSettings } from '../types'
import { translations, type TranslationSchema } from '../locales/translations.ts'

/**
 * Ce que chaque tracker demande pour se connecter. Un seul écran, un jeu de
 * champs par tracker : demander un site Jira et un e-mail Atlassian pour
 * configurer GitHub n'a jamais eu de sens.
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

export const TRACKER_CONFIGS: {
  id: TrackerKind
  label: string
  wantsEmail: boolean
  personalOnly?: boolean
  siteIsPersonal?: boolean
}[] = [
  {
    id: 'jira',
    label: 'Jira',
    wantsEmail: true,
    personalOnly: true,
    siteIsPersonal: true,
  },
  {
    id: 'github',
    label: 'GitHub',
    wantsEmail: false,
    personalOnly: true,
  },
  {
    id: 'gitlab',
    label: 'GitLab',
    wantsEmail: false,
    personalOnly: true,
  },
]

export function trackerFields(tracker: TrackerKind, t: TranslationSchema = translations.fr): TrackerFields {
  const config = TRACKER_CONFIGS.find(item => item.id === tracker) ?? TRACKER_CONFIGS[0]
  const tr = t.trackerCredentials.trackers[tracker as keyof typeof t.trackerCredentials.trackers] ?? t.trackerCredentials.trackers.jira
  return {
    ...config,
    siteLabel: tr.siteLabel,
    sitePlaceholder: tr.sitePlaceholder,
    projectLabel: tr.projectLabel,
    projectPlaceholder: tr.projectPlaceholder,
    tokenHint: tr.tokenHint,
  }
}

export function getTrackers(t: TranslationSchema = translations.fr): TrackerFields[] {
  return TRACKER_CONFIGS.map(config => trackerFields(config.id, t))
}

export function personalTrackers(t: TranslationSchema = translations.fr): TrackerFields[] {
  return getTrackers(t)
}

export const TRACKERS: TrackerFields[] = getTrackers(translations.fr)

/**
 * The trackers a personal credential is useful for: the ones the server can
 * drive. Every tracker of the list has an adapter since GitLab got one (#398).
 */
export const PERSONAL_TRACKERS: TrackerFields[] = personalTrackers(translations.fr)

/**
 * The trackers a project can actually use: the ones with an adapter registered
 * on the server (`trackerapi.NewDefaultRegistry`).
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
  { id: 'gitlab', label: 'GitLab' },
]

/**
 * What the ticket panel and the triage table can write on a task's tracker: a
 * person picked from the tracker's own list, a sprint, a team. It mirrors the
 * capabilities the server adapters declare (assign, sprint, team) for the
 * sources that have them, so the rule lives here once rather than as a
 * `source === 'jira'` test in every component.
 */
export type TrackerFeature = 'assigneeLookup' | 'sprint' | 'team'

const TRACKER_FEATURES: Record<string, TrackerFeature[]> = {
  jira: ['assigneeLookup', 'sprint', 'team'],
  gitlab: ['assigneeLookup', 'sprint', 'team'],
}

export function trackerHas(source: string | undefined, feature: TrackerFeature): boolean {
  return (TRACKER_FEATURES[(source || '').toLowerCase()] || []).includes(feature)
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
export function saveBlockedReason(
  tracker: TrackerKind,
  values: { siteUrl?: string; email?: string },
  checked: boolean,
  t: TranslationSchema = translations.fr
): string {
  if (checked) return ''
  const reasons = t.trackerCredentials.saveBlockedReasons
  if (!canCheck(tracker, values)) {
    return trackerFields(tracker, t).wantsEmail
      ? reasons.wantsEmail
      : reasons.default
  }
  return reasons.needCheck
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
  /** The account the tracker confirmed the credential belongs to (#468). */
  account?: string
  sealed: boolean
  unlocked: boolean
  updatedAt?: string
}

/**
 * Un accès laissé sous une identité qu'aucun compte ne résout, tel que l'API le
 * décrit. Le détail — sous quelle identité, pour quel compte de tracker — n'est
 * servi qu'aux admins ; tout le monde reçoit le compte et les trackers occupés,
 * parce que c'est ce qui explique pourquoi son propre accès semble absent.
 */
export interface OrphanedCredential {
  userId: string
  tracker: string
  siteUrl?: string
  email?: string
  sealed: boolean
  updatedAt?: string
}

export interface OrphanedCredentialReport {
  count: number
  trackers: string[]
  /** Renseigné pour un admin seulement. */
  rows: OrphanedCredential[]
}

export const NO_ORPHANED_CREDENTIALS: OrphanedCredentialReport = { count: 0, trackers: [], rows: [] }

/** Lit le rapport d'accès orphelins d'une réponse de l'API, tolérante à son absence. */
export function orphanedCredentialsFrom(data: any): OrphanedCredentialReport {
  const count = typeof data?.orphanedCount === 'number' ? data.orphanedCount : 0
  if (count <= 0) return NO_ORPHANED_CREDENTIALS
  return {
    count,
    trackers: Array.isArray(data?.orphanedTrackers) ? data.orphanedTrackers : [],
    rows: Array.isArray(data?.orphaned) ? data.orphaned : [],
  }
}

/**
 * L'invitation à sceller, dite en termes de ce que la personne y gagne et de ce
 * qu'elle devra faire. Ce qui se passe sous le capot, clé et chiffrement, ne lui
 * apprend rien d'actionnable et n'a donc pas sa place ici.
 */
export const SEALING_INVITATION = translations.fr.trackerCredentials.sealingInvitation

export function sealingInvitation(t: TranslationSchema = translations.fr): string {
  return t.trackerCredentials.sealingInvitation
}

export type SealingConsequences = TranslationSchema['trackerCredentials']['sealingConsequences']

/** Ce que le choix change, dit au moment où il se fait. */
export function sealingConsequence(
  sealed: boolean,
  t?: TranslationSchema | SealingConsequences
): string {
  const c = !t
    ? translations.fr.trackerCredentials.sealingConsequences
    : 'trackerCredentials' in t
      ? t.trackerCredentials.sealingConsequences
      : t
  return sealed ? c.sealed : c.unsealed
}

export type CredentialStateMessages = TranslationSchema['trackerCredentials']['states']

/** L'état d'un accès personnel, en une phrase, pour l'écran. */
export function credentialState(
  credential?: StoredUserCredential,
  t?: TranslationSchema | CredentialStateMessages
): string {
  const states = !t
    ? translations.fr.trackerCredentials.states
    : 'trackerCredentials' in t
      ? t.trackerCredentials.states
      : t
  if (!credential) return states.none
  if (!credential.sealed) return states.unsealed
  return credential.unlocked ? states.unlocked : states.locked
}

/**
 * Dans l'interface utilisateur, tous les jetons saisis sont personnels.
 * Le jeton serveur n'est configurable que via l'environnement du serveur.
 */
export function scopesFor(_tracker: TrackerKind): CredentialScope[] {
  return ['personal']
}

/**
 * Un projet posé sur un tracker distant sans accès ne ramènera rien : autant le
 * dire à la création plutôt qu'après une synchronisation vide.
 *
 * Pour Jira, seul un jeton personnel permet d'agir. Pour GitHub, le jeton
 * personnel ou le jeton d'environnement du serveur permet la synchronisation.
 */
export function needsCredentialsFor(
  tracker: IssueTracker | string,
  settings: StoredSettings,
  mine: StoredUserCredential[]
): boolean {
  const kind = String(tracker)
  if (kind === 'local' || kind === '') return false
  if (mine.some(c => c.tracker === kind)) return false
  if (kind === 'jira') return true
  const stored = storedFor(settings, kind as TrackerKind)
  return !stored.tokenIsSet && !stored.tokenFromEnv
}

/**
 * Ce qu'un accès déjà enregistré remet dans le formulaire. Le site et l'e-mail
 * en font partie : sans eux, l'écran redemande à la personne ce qu'elle a déjà
 * donné, et le bouton de vérification reste gris faute d'un champ obligatoire.
 * Une valeur en cours de saisie n'est jamais écrasée.
 */
export function prefillFromCredential(
  credential: StoredUserCredential | undefined,
  current: { siteUrl?: string; email?: string }
): { siteUrl: string; email: string } {
  return {
    siteUrl: current.siteUrl?.trim() ? current.siteUrl : credential?.siteUrl || '',
    email: current.email?.trim() ? current.email : credential?.email || '',
  }
}
