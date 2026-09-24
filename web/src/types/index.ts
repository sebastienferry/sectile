export type Priority = 'urgent' | 'high' | 'medium' | 'low'

export type Status = 
  | 'to_clarify'    // A clarifier (Label: #new)
  | 'clarified'     // Cadré (Label: #clarified)
  | 'to_implement'  // A implémenter (Label: #specified)
  | 'to_test'       // A tester (Label: #implemented)
  | 'to_close'      // En revue / PR (Label: #reviewed)
  | 'finished'      // Terminé (Label: #finished)
  | 'backlog'
  | 'specified'
  | 'in_progress'
  | 'to_validate'
  | 'done'

export type TaskSource = 'github' | 'jira' | 'local'

export type TerminalDockPosition = 'bottom' | 'left' | 'right'

export type ActivityStatus = 'queued' | 'pending' | 'running' | 'completed' | 'failed' | 'canceled'

export interface TaskActivity {
  id: string
  taskId: string
  projectId?: string
  taskKey?: string
  taskTitle?: string
  skillId: string
  skillName: string
  action: string
  status: ActivityStatus
  summary: string
  output: string
  steps: string[]
  prompt?: string
  createdAt: string
  startedAt?: string
  completedAt?: string
  error?: string
  duration?: string
  /** Set while a running session is blocked on the user. Cleared when it resumes or ends. */
  waitingSince?: string
  /** Who started the execution. Empty on records written before ownership existed. */
  userId?: string
  /** That person's display name or e-mail, resolved server side. */
  userName?: string
  /** Moteur et modèle réellement utilisés par ce run, vides si inconnus. */
  provider?: string
  model?: string
}

export interface ActivityStats {
  total: number
  queued: number
  running: number
  completed: number
  failed: number
  canceled: number
}

export interface TrackerColumn {
  name: string
  statuses: string[]
  /** Colonne retirée du board sans perdre son affectation de statuts. */
  hidden?: boolean
}

export interface TerminalSession {
  id: string
  cwd: string
  clients: number
  createdAt: string
  lastActiveAt: string
  historyBytes: number
  /** Un agent CLI a été démarré dans cette session. */
  agentRunning?: boolean
}

/** Résultat d'un démarrage d'agent ou d'une injection de skill dans un TTY. */
export interface TTYLaunchResult {
  sessionId: string
  agentLaunched?: boolean
  agentRunning?: boolean
  call?: string
  cwd?: string
  provider?: string
  launchCommand?: string
}

export interface TaskComment {
  id: string
  taskId?: string
  author: string
  /** The Sectile user behind a local comment; absent on tracker comments. */
  userId?: string
  body: string
  createdAt?: string
  source: string
}

export interface TrackerSprint {
  /** Identifiant du sprint côté tracker : l'API Agile ne déplace que par id. */
  id?: string
  name: string
  /** « active », « future » ou « closed » : ce qui sépare NOW de NEXT. */
  state: string
  /** Dates du board, qui donnent l'ordre chronologique réel des sprints. */
  startDate?: string
  endDate?: string
}

export type MacroHorizon = 'now' | 'next' | 'later' | 'hidden'
export type EpicHorizon = MacroHorizon

/**
 * Artefact d'où une ligne de découpe a été importée.
 *
 * L'absence de valeur vaut « saisie à la main », et c'est le cas le plus
 * intéressant de la liste : une ligne sans origine est un ajout que personne
 * n'a spécifié.
 *
 * « stories » est la seule qui ne décrive pas du travail à faire : la ligne
 * reprend un ticket qui existe déjà, et arrive donc rattachée.
 */
export type MacroTodoSource = 'tasks' | 'spec' | 'stories'

export interface MacroTodo {
  id: string
  text: string
  done: boolean
  /** Ticket créé depuis cette ligne de TODO, s'il existe. */
  storyKey?: string
  /** Projet où créer la story. Absent vaut « le projet de la macro ». */
  targetProjectId?: string
  /** Artefact d'origine. Absent vaut « saisie à la main ». */
  sourceKind?: MacroTodoSource
  /** Titre de l'entrée tel que l'artefact l'écrit, avant nettoyage. */
  sourceEntry?: string
}
export type EpicTodo = MacroTodo

export interface ProposedMacroTask {
  title: string
  issueType: string
  description: string
}

export interface CreateTaskPayload {
  title: string
  description?: string
  status?: Status
  priority?: Priority
  labels?: string[]
  assignee?: string
  assigneeAvatar?: string
  creator?: string
  creatorAvatar?: string
  dueDate?: string | null
  sprint?: string
  source?: TaskSource
  externalUrl?: string
  projectId?: string
  issueType?: string
  parentKey?: string
  parentTitle?: string
  parentType?: string
}

export interface RefineMacroResult {
  key: string
  todos: MacroTodo[]
  proposedTasks?: ProposedMacroTask[]
  specFramework: string
}

export interface MacroMeta {
  projectId: string
  key: string
  /** Titre et statut du ticket macro lui-même, lus par la synchro. */
  title?: string
  status?: string
  /** La macro est dans une catégorie « terminé » côté tracker. */
  closed?: boolean
  /** Chaîne vide = macro non encore classée. */
  horizon: MacroHorizon | ''
  description: string
  framingComment?: string
  todos: MacroTodo[]
  updatedAt: string
}
export type EpicMeta = MacroMeta

export interface TrackerBoard {
  id: string
  name: string
  type: string
}

export type TtyMode = 'integrated' | 'external'

/**
 * A saved board view (#387): a personal, named selection of projects and
 * labels over the all-projects board. The server resolves it; the interface
 * only sends its id.
 */
export interface BoardView {
  id: string
  name: string
  projectIds: string[]
  labels: string[]
  createdAt: string
  updatedAt: string
}

export interface BoardViewPayload {
  name?: string
  projectIds?: string[]
  labels?: string[]
}

export interface Project {
  id: string
  name: string
  slug: string
  description: string
  icon: string
  color: AccentColor | string
  repoPath: string
  /**
   * Répertoires de travail connus du projet. Alimentée automatiquement :
   * dès qu'un ticket épingle un nouveau CWD, le chemin est enregistré ici.
   */
  repoPaths?: string[]
  /**
   * Chaque tâche travaille dans son propre worktree Git isolé, ou directement
   * dans le clone si l'option est désactivée. Vrai par défaut.
   */
  prCreationStage?: 'specified' | 'implemented'
  useWorktrees?: boolean
  /**
   * Mode d'exécution des skills quand ni le lancement ni la skill n'en fixe un.
   * Vide vaut « interactif », le comportement historique.
   */
  defaultSkillMode?: SkillMode
  /** Étape où s'arrête une exécution en chaîne. Vide vaut « reviewed ». */
  fullChainStopStage?: 'implemented' | 'reviewed'
  /** Board du tracker retenu pour ce projet. */
  boardId?: string
  /**
   * Colonnes du board, à la façon de Jira : un nom et les statuts du tracker
   * que la colonne regroupe. Importables depuis le board, puis modifiables.
   */
  trackerColumns?: TrackerColumn[]
  /** Sprints du board avec leur état, rafraîchis par la synchro. */
  sprints?: TrackerSprint[]
  /**
   * Types de tickets importés depuis le tracker. Vide vaut « les types par
   * défaut » (Task et Story). Un projet dont le tracker n'expose que son propre
   * type n'importe rien sans ce réglage.
   */
  issueTypes?: string[]
  /**
   * Vues optionnelles que le projet affiche, parmi `OPTIONAL_VIEWS`. Vide, la
   * valeur par défaut, veut dire « aucune » : Triage, Roadmap et Timeline
   * restent hors de la barre latérale tant que le projet ne les demande pas.
   */
  enabledViews?: OptionalViewMode[]
  /**
   * Les cartes portent la couleur de leur épic. Absent ou faux, la valeur par
   * défaut, elles restent telles qu'avant : le projet doit la demander.
   */
  epicColors?: boolean
  /**
   * Le projet tient dans un seul dépôt. La branche courante, son sélecteur et la
   * branche affichée sur une carte n'ont de sens que dans ce cas.
   */
  monoRepo?: boolean
  /** Étape du workflow agentique -> colonnes concernées (une ou plusieurs). */
  stageColumns?: Record<string, string[]>
  gitRemoteUrl?: string
  githubRepo: string
  /**
   * Paramètres de connexion propres au projet. Vide veut dire « ceux de la
   * configuration utilisateur ». Les jetons ne sont jamais renvoyés : seul le
   * drapeau `...TokenSet` dit qu'il y en a un.
   */
  githubApiUrl?: string
  githubToken?: string
  githubTokenSet?: boolean
  gitlabUrl?: string
  gitlabProject?: string
  gitlabToken?: string
  gitlabTokenSet?: boolean
  /** Jira project key the sync queries on, e.g. "PE". */
  jiraProject?: string
  issueTracker: IssueTracker
  /** Tracker project URL, or the Jira base URL (e.g. https://acme.atlassian.net). */
  trackerUrl?: string
  isDefault: boolean
  bookmarked?: boolean
  taskCount?: number
  skillOverrides?: Record<string, string>
  setupProviders?: string[]
  aiProvider?: AIProvider
  aiCommandTemplate?: string
  aiCommandTemplateAutonomous?: string
  /** Modèle du moteur pour ce projet. Vide : le réglage global s'applique. */
  aiModel?: string
  /** Modèle par compétence (skillId -> modèle) pour celles qui s'écartent d'aiModel. */
  aiSkillModels?: Record<string, string>
  specFramework?: SpecFramework
  /** Synchronisation automatique en arrière-plan activée pour ce projet. */
  autoSyncEnabled?: boolean
  /** Période de la synchronisation en arrière-plan (en minutes, entre 1 et 30 min). */
  autoSyncIntervalMin?: number
  /**
   * Compte propriétaire du projet, celui dont la synchronisation de fond
   * emprunte l'accès tracker. Renseigné par le serveur, jamais envoyé par le
   * client : le propriétaire décide du jeton emprunté (ADR 0018).
   */
  ownerUserId?: string
  /** Mode d'exécution des terminaux : 'integrated' (web xterm) ou 'external' (vrai terminal OS). */
  ttyMode?: TtyMode
  /** Commande ou application de terminal externe spécifique à ce projet (ex: 'Ghostty', 'iTerm', 'Terminal'). */
  externalTerminalCommand?: string
  createdAt: string
  updatedAt: string
}

/**
 * Project fields accepted by the create and update endpoints.
 *
 * A stored Project only exposes known providers. A write may additionally send
 * an empty provider to clear the optional project override; omitting the field
 * keeps the existing value on updates.
 */
export type ProjectSavePayload = Omit<Partial<Project>, 'aiProvider'> & {
  aiProvider?: AIProvider | ''
}

/**
 * Équipe du tracker telle que les tickets la portent, avec les personnes qu'elle
 * contient. Les membres viennent de l'API des équipes Atlassian, lus à chaque
 * synchronisation Jira.
 */
export interface TrackerTeam {
  id: string
  name: string
  memberCount: number
  members?: TeamMember[]
  /** Dernière lecture des membres depuis le tracker, ISO 8601. */
  syncedAt?: string
  /** Nombre de tickets synchronisés qui portent cette équipe. */
  taskCount: number
}

/** Une personne d'une équipe. accountId est ce par quoi Jira assigne. */
export interface TeamMember {
  teamId: string
  teamName?: string
  accountId: string
  displayName: string
  email?: string
  avatarUrl?: string
  active: boolean
}

/** Charge d'un membre : ses tickets dans l'équipe consultée. */
export interface TeamMemberLoad {
  member: TeamMember
  tasks: Task[]
  byStatus: Record<string, number>
  total: number
}

/** Charge d'une équipe, par personne. */
export interface TeamWorkload {
  team: TrackerTeam
  members: TeamMemberLoad[]
  /** Tickets de l'équipe que personne ne porte. */
  unassigned: Task[]
  /** Tickets de l'équipe assignés à quelqu'un qui n'en est pas membre. */
  outside: TeamMemberLoad[]
}

export interface DetectedStatus {
  id: string
  name: string
  type?: string
  color?: string
  source?: string
}

/**
 * Un lien de pull request porté par un ticket. La branche est conservée avec
 * l'URL : c'est elle qui distingue une PR de suite sur la même branche d'une PR
 * substituée à une autre, sans rapport.
 */
export type PullRequestState = 'open' | 'conflicting' | 'merged' | 'closed'

export interface PullRequestLink {
  state?: PullRequestState
  url: string
  branch?: string
}

export interface Task {
  id: string
  projectId?: string
  key: string
  title: string
  description: string
  status: Status
  priority: Priority
  labels: string[]
  assignee: string
  assigneeAvatar?: string
  creator?: string
  creatorAvatar?: string
  position: number
  dueDate?: string | null
  branchName?: string
  /** Pull request courante du ticket : toujours le dernier lien de `prLinks`. */
  prUrl?: string
  /** Ensemble ordonné des pull requests du ticket, de la plus ancienne à la courante. */
  prLinks?: PullRequestLink[]
  /** Répertoire de travail propre au ticket. Vide = hérite du projet, puis du réglage global. */
  repoPath?: string
  /** Statut brut du tracker, tel qu'il l'écrit (« Dev Test », « To Merge »…). */
  trackerStatus?: string
  /** Sprint / itération du tracker (champ Sprint côté Jira). */
  sprint?: string
  /** Équipe du tracker (champ Team côté Jira). Facultative sur un ticket. */
  team?: string
  /** Identifiant de l'équipe Atlassian : c'est lui qui donne accès à ses membres. */
  teamId?: string
  /** Dates du tracker, distinctes de createdAt / updatedAt qui datent l'import. */
  trackerCreatedAt?: string
  trackerUpdatedAt?: string
  /**
   * Entrée dans la catégorie de statut courante : c'est de là que se compte le
   * temps passé. La catégorie, pas le statut précis : deux colonnes d'une même
   * catégorie ne la font pas bouger.
   */
  statusChangedAt?: string
  source?: TaskSource
  externalUrl?: string
  /** Tracker work item type. Only "Task" and "Story" are imported. */
  issueType?: string
  /**
   * Parent work item - an epic, or a parent story for a sub-task - carried as a
   * property of the task rather than as a card of its own.
   */
  parentKey?: string
  parentTitle?: string
  parentType?: string
  activities?: TaskActivity[]
  createdAt: string
  updatedAt: string
}

export interface CloneTaskRequest {
  title?: string
  projectId?: string
  status?: Status
  priority?: Priority
  sprint?: string
  assignee?: string
  assigneeAvatar?: string
  includeDescription?: boolean
  includeLabels?: boolean
  includeParent?: boolean
  includeSprint?: boolean
  includeAssignee?: boolean
  source?: TaskSource
}

export interface GitDiffFile {
  path: string
  oldPath?: string
  status: 'modified' | 'added' | 'deleted' | 'renamed' | string
  additions: number
  deletions: number
  diff: string
}

export interface GitDiffResult {
  taskKey: string
  branch: string
  baseBranch: string
  repoPath: string
  worktreePath?: string
  isClean: boolean
  filesChanged: number
  insertions: number
  deletions: number
  files: GitDiffFile[]
  rawDiff: string
  prUrl?: string
  error?: string
}

export interface WorktreeInfo {
  taskKey: string
  branch: string
  worktreePath: string
  exists: boolean
  mainRepoPath: string
}

export interface GitStatusInfo {
  repoPath: string
  isGitRepo: boolean
  branch: string
  baseBranch?: string
  isClean: boolean
  modifiedCount: number
  untrackedCount: number
  ahead: number
  behind: number
  remoteName?: string
  remoteUrl?: string
  latestCommit?: string
  error?: string
}

export interface Skill {
  id: string
  name: string
  command: string
  description: string
  inputStatus: Status
  outputStatus: Status
  icon: string
  color: string
  steps: string[]
}

export type AccentColor = 
  | 'indigo'
  | 'violet'
  | 'emerald'
  | 'amber'
  | 'rose'
  | 'cyan'
  | 'blue'
  | 'orange'
  | 'neon-cyan'
  | 'neon-purple'
  | 'neon-green'
  | 'neon-amber'

export type Theme = 'dark' | 'light' | 'system'

export type Language = 'fr' | 'en'

export type Density = 'compact' | 'standard' | 'comfortable'

export type ViewMode = 'board' | 'list' | 'triage' | 'roadmap' | 'timeline' | 'activities' | 'sync' | 'skills' | 'team'

/**
 * Vues de planification qu'un projet active à la demande. Elles répondent à un
 * besoin (trier ce qui n'est pas classé, poser les macros sur des horizons,
 * lire le calendrier des sprints) qu'un projet suivant un seul flux de tickets
 * n'a jamais, et une entrée vide dans la barre coûte plus qu'elle ne rapporte.
 */
export type OptionalViewMode = 'triage' | 'roadmap' | 'timeline'

export type BoardGroupingMode = 'workflow' | 'status'

export type BoardCardDisplayMode = 'condensed' | 'expanded'

export type WorkflowStage = 'new' | 'clarified' | 'specified' | 'implemented' | 'reviewed' | 'finished'

export type DetailMode = 'modal' | 'panel'

export type AIProvider = 'agy' | 'vibe' | 'claude' | 'gemini' | 'codex' | 'cursor' | 'custom'

export type IssueTracker = 'github' | 'jira' | 'local'

/**
 * Spec-Driven Design frameworks Sectile can scaffold into a project.
 * - `speckit`  : GitHub Spec Kit (`specify` CLI, `.specify/` + `specs/`)
 * - `openspec` : OpenSpec (`openspec` CLI, `openspec/changes/` + `openspec/specs/`)
 */
export type SpecFramework = 'speckit' | 'openspec'

export interface SpecFrameworkStatus {
  framework: SpecFramework
  frameworkLabel: string
  repoPath: string
  cliAvailable: boolean
  cliCommand: string
  initialized: boolean
  markerPaths?: string[]
  installHint?: string
}

export interface SpecFrameworkStep {
  label: string
  command: string
  success: boolean
  skipped: boolean
  output?: string
  error?: string
}

export interface SpecFrameworkInstallResult {
  framework: SpecFramework
  frameworkLabel: string
  repoPath: string
  installed: boolean
  alreadyInit: boolean
  version?: string
  markerPaths?: string[]
  steps: SpecFrameworkStep[]
  message: string
  error?: string
}

export interface UserSettings {
  id: number
  theme: Theme
  accentColor: AccentColor
  language: Language
  density: Density
  /**
   * Zoom de l'interface en pourcentage, sur un des crans de lib/uiScale.
   * La densité ne bouge
   * que la taille de police racine, ce qui laisse intactes toutes les tailles
   * fixées en pixels : l'échelle, elle, zoome toute l'interface.
   */
  uiScale?: number
  /**
   * Boucle de synchronisation de fond. Elle ne relit que ce qui a changé depuis
   * sa passe précédente : une passe complète coûte une requête par centaine de
   * tickets, une passe incrémentale une seule requête.
   */
  autoSyncEnabled?: boolean
  /** Période de cette boucle, en secondes. Plancher à 30. */
  autoSyncIntervalSec?: number
  defaultView: ViewMode
  detailMode: DetailMode
  userName: string
  userEmail: string
  userAvatar: string
  aiProvider: AIProvider
  aiCommandTemplate: string
  aiCommandTemplateAutonomous?: string
  /** Modèle du moteur. Vide : le CLI garde son défaut. */
  aiModel?: string
  /** Modèle par compétence (skillId -> modèle). */
  aiSkillModels?: Record<string, string>
  /**
   * Modèles proposés par moteur (provider -> liste ordonnée). C'est ce que les
   * surfaces de lancement offrent : un modèle absent d'ici ne peut pas être
   * choisi au lancement. Un moteur sans liste retombe sur celle livrée.
   */
  aiProviderModels?: Record<string, string[]>
  repoPath: string
  issueTracker: IssueTracker
  githubRepo: string
  jiraProject?: string
  jiraUrl?: string
  /** Identifiants de l'API Jira, requis pour importer Sprint et Team. */
  jiraEmail?: string
  /**
   * Jamais renvoyé par l'API. En écriture, une chaîne vide conserve le jeton
   * existant et la sentinelle `__clear__` l'efface.
   */
  jiraApiToken?: string
  /** Un jeton est configuré, en base ou par variable d'environnement. */
  jiraApiTokenSet?: boolean
  /** Le jeton vient de SECTILE_JIRA_API_TOKEN et prime sur la base. */
  jiraApiTokenFromEnv?: boolean
  /** Instance GitHub, vide pour api.github.com. */
  githubApiUrl?: string
  /** Instance GitLab et projet par défaut, l'équivalent de githubRepo. */
  gitlabUrl?: string
  gitlabProject?: string
  /**
   * Jetons GitHub et GitLab : mêmes règles que jiraApiToken, jamais renvoyés,
   * vide conserve, `__clear__` efface.
   */
  githubToken?: string
  githubTokenSet?: boolean
  /** Aucun jeton en base, mais l'environnement du serveur en fournit un. */
  githubTokenFromEnv?: boolean
  gitlabToken?: string
  gitlabTokenSet?: boolean
  gitlabTokenFromEnv?: boolean
  promptClarify: string
  promptSpecify: string
  promptImplement: string
  promptAdjust: string
  promptHandoff: string
  promptCreatePr?: string
  promptPick: string
  editorCommand: string
  /** Commande ou nom de l'application de terminal externe (ex: "Terminal", "iTerm", "Ghostty"). */
  externalTerminalCommand?: string
  specFramework?: SpecFramework
  updatedAt: string
}

export interface TaskFacetValue {
  value: string
  count: number
}

/**
 * Ce que l'écran de connexion envoie au serveur. `tracker` décide des champs qui
 * comptent : site et e-mail pour Jira, instance et jeton pour GitHub, instance,
 * projet et jeton pour GitLab.
 */
export interface TrackerCredentials {
  /**
   * GitLab n'est pas dans `IssueTracker` : ses paramètres se configurent, mais
   * aucun adaptateur GitLab n'est enregistré, donc un projet ne peut pas encore
   * le choisir comme tracker. Jira et GitHub, eux, en ont un.
   */
  tracker: IssueTracker | 'gitlab'
  siteUrl: string
  /** Dépôt GitHub (`owner/repo`) ou projet GitLab (`groupe/projet`). */
  project?: string
  email?: string
  token?: string
}

export interface TrackerCheck {
  ok: boolean
  error?: string
  /** Compte auquel les accès GitHub ou GitLab appartiennent. */
  account?: string
  identity?: { accountId: string; displayName: string; email?: string; siteUrl: string }
  /** Projets que ces accès peuvent lire, ce qui rend une erreur de site évidente. */
  projects?: { id: string; name: string }[]
}

export interface AutoSyncState {
  enabled: boolean
  intervalSec: number
  running: boolean
  lastRunAt?: string
  lastError?: string
  lastImported: number
  passes: number
  imported: number
  backoffUntil?: string
}

export interface CliStatus {
  tool: string
  available: boolean
  path: string
  authStatus: string
  details: string
}

export interface ToastMessage {
  id: string
  type: 'success' | 'info' | 'warning' | 'error'
  title: string
  description?: string
  duration?: number
}

export interface InstalledSkillInfo {
  id: string
  name: string
  installed: boolean
  path: string
  description: string
}

export interface ProjectSkillsStatus {
  projectId: string
  projectName: string
  repoPath: string
  pathExists: boolean
  isGitRepo?: boolean
  gitBranch?: string
  installedAll: boolean
  specFramework?: SpecFramework
  worktreesCount?: number
  worktreePaths?: string[]
  skills: InstalledSkillInfo[]
}

export interface GitBranchItem {
  name: string
  isCurrent: boolean
  isRemote: boolean
  commit?: string
  message?: string
}

export interface GitBranchesInfo {
  repoPath: string
  currentBranch: string
  branches: GitBranchItem[]
}

export interface ProjectGitInitResult {
  repoPath: string
  isGitRepo: boolean
  branch: string
  message: string
  initialized: boolean
}

/**
 * Une skill du workflow telle que l'éditeur la voit. Le contenu vient de la base
 * Sectile, le modèle intégré sert de valeur par défaut, et `diverged` signale
 * qu'un SKILL.md a été retouché à la main dans le dépôt.
 */
export interface SkillEditorEntry {
  overrideOrigin?: string
  legacyConflicts?: string[]
  legacyContents?: Record<string, string>
  requiresReconciliation?: boolean
  id: string
  name: string
  dirName: string
  command: string
  description: string
  fromStage: string
  toStage: string
  scope?: 'task' | 'macro' | string
  /** Mode propre à la skill. Vide : pas d'avis, le défaut du projet décide. */
  mode: SkillMode
  content: string
  defaultContent: string
  isCustom: boolean
  updatedAt?: string
  installed: boolean
  paths: string[]
  diverged: boolean
  repoContent?: string
  repoPath?: string
}

/**
 * Mode d'exécution d'un run. `autonomous` lance la CLI en headless et laisse le
 * worker poser la transition ; `interactive` ouvre un terminal que l'utilisateur
 * répond et confirme. La chaîne vide est le troisième état : « pas d'avis », qui
 * laisse la précédence retomber sur le niveau suivant.
 */
export type SkillMode = '' | 'interactive' | 'autonomous'

/** Champ que le tracker impose à la création d'une macro, avec ses valeurs permises. */
export interface MacroRequiredField {
  id: string
  name: string
  options: { id: string; value: string }[]
}
export type EpicRequiredField = MacroRequiredField
