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

export type TaskSource = 'linear' | 'github' | 'jira' | 'local'

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

export interface MacroTodo {
  id: string
  text: string
  done: boolean
  /** Ticket créé depuis cette ligne de TODO, s'il existe. */
  storyKey?: string
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
   * Le projet tient dans un seul dépôt. La branche courante, son sélecteur et la
   * branche affichée sur une carte n'ont de sens que dans ce cas.
   */
  monoRepo?: boolean
  /** Étape du workflow agentique -> colonnes concernées (une ou plusieurs). */
  stageColumns?: Record<string, string[]>
  gitRemoteUrl?: string
  linearTeam: string
  githubRepo: string
  /** Jira project key used as `acli --project`, e.g. "PE". */
  jiraProject?: string
  issueTracker: IssueTracker
  /** Linear project URL, or the Jira base URL (e.g. https://acme.atlassian.net). */
  trackerUrl?: string
  /**
   * "standard" for a delivery project, "personal" for a personal board. The
   * daily digest is only served for a personal project.
   */
  projectType?: ProjectType
  isDefault: boolean
  taskCount?: number
  stageMapping?: Record<WorkflowStage, string>
  skillOverrides?: Record<string, string>
  setupProviders?: string[]
  aiProvider?: AIProvider
  aiCommandTemplate?: string
  specFramework?: SpecFramework
  /** Synchronisation automatique en arrière-plan activée pour ce projet. */
  autoSyncEnabled?: boolean
  /** Période de la synchronisation en arrière-plan (en minutes, entre 1 et 30 min). */
  autoSyncIntervalMin?: number
  /** Mode d'exécution des terminaux : 'integrated' (web xterm) ou 'external' (vrai terminal OS). */
  ttyMode?: TtyMode
  /** Commande ou application de terminal externe spécifique à ce projet (ex: 'Ghostty', 'iTerm', 'Terminal'). */
  externalTerminalCommand?: string
  createdAt: string
  updatedAt: string
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
  position: number
  dueDate?: string | null
  branchName?: string
  prUrl?: string
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
   * Parent work item — an epic, or a parent story for a sub-task — carried as a
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

export type ViewMode = 'board' | 'list' | 'triage' | 'roadmap' | 'timeline' | 'activities' | 'sync' | 'digest' | 'skills' | 'team'

export type BoardGroupingMode = 'workflow' | 'status'

export type BoardCardDisplayMode = 'condensed' | 'expanded'

export type WorkflowStage = 'new' | 'clarified' | 'specified' | 'implemented' | 'reviewed' | 'finished'

export type DetailMode = 'modal' | 'panel'

export type AIProvider = 'agy' | 'vibe' | 'claude' | 'gemini' | 'codex' | 'cursor' | 'custom'

export type IssueTracker = 'linear' | 'github' | 'jira' | 'local'

export type ProjectType = 'standard' | 'personal'

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
   * Zoom de l'interface en pourcentage (90, 100, 112, 125). La densité ne bouge
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
  repoPath: string
  issueTracker: IssueTracker
  linearTeam: string
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
  /**
   * Remplace le prompt d'agenda du digest quotidien. Vide garde celui d'origine.
   * Marqueurs disponibles : {project}, {date}.
   */
  promptDigestAgenda?: string
  promptClarify: string
  promptSpecify: string
  promptImplement: string
  promptCreatePr: string
  promptPick: string
  editorCommand: string
  /** Commande ou nom de l'application de terminal externe (ex: "Terminal", "iTerm", "Ghostty"). */
  externalTerminalCommand?: string
  specFramework?: SpecFramework
  updatedAt: string
}

export interface DigestTaskRef {
  key: string
  title: string
  status: Status
  priority: Priority
  issueType?: string
  assignee?: string
  parentKey?: string
  parentTitle?: string
  externalUrl?: string
  branchName?: string
  prUrl?: string
  dueDate?: string
  ageDays: number
  isStale: boolean
  /** The tracker did not expose real dates at sync time. */
  datesUnknown?: boolean
  daysToDue?: number
}

export interface DigestMacroGroup {
  parentKey: string
  parentTitle?: string
  openCount: number
  doneCount: number
}
export type DigestEpicGroup = DigestMacroGroup

export interface DigestStats {
  totalOpen: number
  urgent: number
  high: number
  stale: number
  overdue: number
  awaitingReview: number
  doneLast7Days: number
  openDateUnknown: number
  closedDateUnknown: number
}

/** Une valeur filtrable et le nombre de tickets derrière elle. */
export interface TaskFacetValue {
  value: string
  count: number
}

/** Réponse de la vérification des accès au tracker. */
export interface TrackerCheck {
  ok: boolean
  error?: string
  identity?: { accountId: string; displayName: string; email?: string; siteUrl: string }
  /** Projets que ces accès peuvent lire, ce qui rend une erreur de site évidente. */
  projects?: { id: string; name: string }[]
}

/** État de la boucle de synchronisation de fond. */
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

export type DigestAIStatus = 'none' | 'queued' | 'running' | 'completed' | 'failed'

export interface DailyDigest {
  projectId: string
  projectName: string
  date: string
  /** Narrows the digest to one person; empty means the whole project. */
  assignee: string
  /** Every assignee present in the project's tasks, in the tracker's spelling. */
  assignees?: string[]
  focus: DigestTaskRef[]
  watch: DigestTaskRef[]
  stale: DigestTaskRef[]
  dueSoon: DigestTaskRef[]
  awaitingReview: DigestTaskRef[]
  recentlyDone: DigestTaskRef[]
  byMacro: DigestMacroGroup[]
  byEpic: DigestMacroGroup[]
  stats: DigestStats
  /** Markdown agenda produced by the project's AI agent (meetings). */
  agenda?: string
  aiStatus: DigestAIStatus
  aiError?: string
  aiActivityId?: string
  aiUpdatedAt?: string
  markdown: string
  generatedAt: string
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
