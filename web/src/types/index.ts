export type Priority = 'urgent' | 'high' | 'medium' | 'low'

/** One option of a Jira priority scheme and the level it maps to (#679). */
export interface PriorityMappingOption {
  id: string
  name: string
  level: Priority
  /** Matched by rank only: a write never sends it. */
  guessed?: boolean
  /** Set by a person: discovery never changes it. */
  manual?: boolean
}

/** A project's priority mapping, its options in the scheme's order. */
export interface PriorityMapping {
  options?: PriorityMappingOption[]
  /** Per level, the option a write sends when several sure ones share it. */
  preferred?: Partial<Record<Priority, string>>
}

/**
 * A Jira custom field an epic axis is mapped to (#680): a single select or a
 * two-level cascading select, picked from an epic's edit screen.
 */
export interface EpicAxisField {
  id: string
  name: string
  kind: 'select' | 'cascade'
  /** Axis value ("p1", "2026-Q4") to option path ("id", or "parentId/childId"). */
  options?: Record<string, string>
  /** Axis values a person set, changed or cleared by hand. */
  manual?: string[]
}

export interface EpicAxisFields {
  priority?: EpicAxisField
  quarter?: EpicAxisField
}

/** One option of a candidate field; a cascade's parents carry children. */
export interface EpicFieldOption {
  id: string
  value: string
  children?: EpicFieldOption[]
}

/** A closed-list custom field of an epic's edit screen, with deduced maps. */
export interface EpicFieldCandidate {
  id: string
  name: string
  kind: 'select' | 'cascade'
  options: EpicFieldOption[]
  deduced?: { priority?: Record<string, string>; quarter?: Record<string, string> }
}

/** What the settings read to map an axis to a field. */
export interface EpicAxisFieldDiscovery {
  epicKey?: string
  noEpic?: boolean
  candidates: EpicFieldCandidate[]
}

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

export type TaskSource = 'github' | 'gitlab' | 'jira' | 'local'

export type TerminalDockPosition = 'bottom' | 'left' | 'right'

export type ActivityStatus = 'queued' | 'pending' | 'running' | 'completed' | 'failed' | 'canceled'

export interface TaskActivity {
  id: string
  taskId: string
  projectId?: string
  /** The tracker a synchronisation read (#741). */
  trackerId?: string
  /** The project a task run works for (#741). */
  runProjectId?: string
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
  /** The provider a failed tracker write was refused for, for want of the person's own token (#645). */
  credentialMissing?: string
  duration?: string
  /** Set while a running session is blocked on the user. Cleared when it resumes or ends. */
  waitingSince?: string
  /**
   * Why a run waits when it is not a question asked in its session:
   * "repository" for a launch parked until its ticket is pinned to a repository.
   */
  waitingReason?: string
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
  /**
   * A roadmap project of the macro's project, a Jira key, when the story is
   * created there; it excludes targetProjectId, and the story stays in Jira (#632).
   */
  targetTrackerProject?: string
  /** Artefact d'origine. Absent vaut « saisie à la main ». */
  sourceKind?: MacroTodoSource
  /** Titre de l'entrée tel que l'artefact l'écrit, avant nettoyage. */
  sourceEntry?: string
}
export type EpicTodo = MacroTodo

/** What a batch creation did with one slicing line (#634). */
export interface MacroStoryOutcome {
  todoId: string
  status: 'created' | 'skipped' | 'failed'
  storyKey?: string
  task?: Task
  notice?: string
  error?: string
  /** `tracker_credential_missing` when the line failed for want of a token. */
  code?: string
  tracker?: string
}

/** The answer of a batch creation: one outcome per processed line. */
export interface MacroStoryBatch {
  macro: MacroMeta | null
  results: MacroStoryOutcome[]
  created: number
  skipped: number
  failed: number
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
  /** The epic's own priority, empty when none (#627). */
  priority?: EpicPriority | ''
  /** The epic's quarter, "2026-Q4", empty when none (#627). */
  quarter?: string
  /** The readiness a person decided, empty when nobody did (#633). */
  readiness?: EpicReadiness | ''
  /** False when the labels, the horizon among them, stay in Sectile: milestone, local key, foreign epic, tracker without epics. */
  labelsWritable?: boolean
  /**
   * Whether a panel edit of the priority or the quarter is written on the
   * tracker. It differs from labelsWritable on an epic of a roadmap project
   * whose project opted in (#632). Absent from an older server.
   */
  axesWritable?: boolean
  /** The Jira project key the epic's key carries, absent for a milestone or a local key (#632). */
  origin?: string
  /** An epic of another Jira project, which the roadmap reads without writing on it (#632). */
  foreign?: boolean
  /**
   * The epic's labels as the tracker returns them, horizon labels included.
   * Absent from a server older than #626.
   */
  labels?: string[]
  updatedAt: string
  /** The macro's own page on its tracker, absent when the tracker gives none. */
  externalUrl?: string
  /** Where the todos are copied on the tracker, and how that copy stands (#663). Absent from an older server. */
  todosMirror?: MacroTodosMirror
  /** Where the framing is copied: a comment on a Jira epic, or none (#636). Absent from an older server. */
  framingMirror?: MacroTodosMirror
}
export type EpicMeta = MacroMeta

/**
 * The one-way copy of a macro's todos on its tracker (#663): a comment on a
 * Jira epic, a block of a GitHub milestone description, or none, for the
 * reason given.
 */
export interface MacroTodosMirror {
  kind: 'jira_comment' | 'github_description' | ''
  /** Why the list stays in Sectile, when kind is empty. */
  reason?: string
  /** The last body written on the tracker is the one of the current list. */
  upToDate: boolean
  /** The last failure, kept until a write succeeds. */
  error?: string
  /** The tracker whose personal token the last failure lacked (#645). */
  credentialMissing?: string
  writtenAt?: string
  /** The comment, or the milestone. */
  url?: string
}
/** An epic's own priority, P0 the highest. */
export type EpicPriority = 'p0' | 'p1' | 'p2' | 'p3'
/** How far an epic has come from an idea to something ready to build, as a person judges it (#633). */
export type EpicReadiness = 'idea' | 'shaping' | 'ready'

export interface TrackerBoard {
  id: string
  name: string
  type: string
}

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

/** Workflow stage at which a project opens the draft pull request of its tasks. */
export type PRCreationStage = 'clarified' | 'specified' | 'implemented'

export interface Project {
  id: string
  name: string
  slug: string
  description: string
  icon: string
  color: AccentColor | string
  /**
   * Other Jira project keys whose epics the roadmap also reads and whose story
   * keys the slicing attaches (#632). Nothing existing of theirs is changed,
   * except what roadmapAxisWrites opens.
   */
  roadmapProjects?: string[]
  /** Whether a panel edit writes the priority and the quarter on a roadmap project's epic. */
  roadmapAxisWrites?: boolean
  /**
   * The label prefixes of the epic priority, quarter and readiness (#635). An
   * empty or missing field is the default prefix of that axis.
   */
  epicAxisPrefixes?: { priority?: string; quarter?: string; readiness?: string }
  /**
   * How the Jira priority scheme maps to Sectile's levels (#679). Empty until
   * the first discovery, and absent on other trackers.
   */
  priorityMapping?: PriorityMapping
  /**
   * The Jira custom fields the epic priority and quarter are mapped to
   * (#680). Absent or empty maps none: both axes stay labels only.
   */
  epicAxisFields?: EpicAxisFields
  /** Stage at which the workflow opens the pull request. */
  prCreationStage?: PRCreationStage
  /**
   * Whether the tasks' clarification and specification files are committed
   * with the code ("keep", the default) or left in the task worktree and
   * ignored by Git ("drop"). A workstation may override it.
   */
  specArtifacts?: 'keep' | 'drop'
  /**
   * Mode d'exécution des skills quand ni le lancement ni la skill n'en fixe un.
   * Vide vaut « interactif », le comportement historique.
   */
  defaultSkillMode?: SkillMode
  /** Étape où s'arrête une exécution en chaîne. Vide vaut « reviewed ». */
  fullChainStopStage?: 'implemented' | 'reviewed'
  pushStageCommits?: boolean
  /** Template of the branch of a task that has none yet (#621). Empty means feat/{key_lower}. */
  branchNameFormat?: string
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
  /** Étape du workflow agentique -> colonnes concernées (une ou plusieurs). */
  stageColumns?: Record<string, string[]>
  gitRemoteUrl?: string
  /**
   * The repositories the project's tickets work in, the code remote
   * (gitRemoteUrl) always first. Derived server side, never stored as such.
   */
  repositories?: ProjectRepository[]
  /**
   * JSON report of the conversion of the legacy working directories into
   * repositories. Empty until that conversion ran.
   */
  repositoriesMigration?: string
  githubRepo: string
  /**
   * The project's own connection parameters. Empty means "those of the user
   * configuration". A project carries no token: the server credential of its
   * provider serves every project (#464).
   */
  githubApiUrl?: string
  gitlabUrl?: string
  gitlabProject?: string
  /** Jira project key the sync queries on, e.g. "PE". */
  jiraProject?: string
  issueTracker: IssueTracker
  /** Tracker project URL, or the Jira base URL (e.g. https://acme.atlassian.net). */
  trackerUrl?: string
  isDefault: boolean
  bookmarked?: boolean
  taskCount?: number
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
  /**
   * The trackers the project selects its tickets from, in order (#741). The
   * first is the default unless defaultTrackerId names another.
   */
  trackers?: ProjectTrackerRef[]
  /** The label a ticket carries to belong to the project. Empty: every ticket of its trackers. */
  label?: string
  /** Where the project's new tickets go (#741). */
  defaultTrackerId?: string
  createdAt: string
  updatedAt: string
}

/** One tracker a project selects, by id and identity (#741). */
export interface ProjectTrackerRef {
  trackerId: string
  identity: string
}

/** The providers a tracker can be recorded on (#741); local boards are each project's own. */
export type TrackerProvider = 'jira' | 'github' | 'gitlab'

/** What a member sees of a tracker, to pick it for a project (GET /api/trackers). */
export interface TrackerSummary {
  id: string
  name: string
  provider: TrackerProvider | 'local'
  /** The tracker's own address, empty when it uses the deployment's. */
  site: string
  /** A Jira key, a GitHub owner/repo or a GitLab project path. */
  scope: string
  identity: string
}

/**
 * A tracker as an admin configures it (GET /api/admin/trackers): its source and
 * its board mirror, one per tracker (#741).
 */
export interface Tracker extends TrackerSummary {
  boardId?: string
  trackerColumns?: TrackerColumn[]
  stageColumns?: Record<string, string[]>
  sprints?: TrackerSprint[]
  issueTypes?: string[]
  autoSyncEnabled: boolean
  autoSyncIntervalMin: number
  createdAt?: string
  updatedAt?: string
  /** How many tickets and epics it holds, read only: one holding some keeps its source. */
  ticketCount?: number
}

/**
 * Project fields accepted by the create and update endpoints. Execution
 * settings (provider, model, templates, checkout, terminal) are not among
 * them: the workstation's local file owns them (#305).
 */
export type ProjectSavePayload = Omit<Partial<Project>, 'repositories'> & {
  /** Remote URLs of the full declared list; the code remote may be included or not. */
  repositories?: string[]
}

/** One repository of a project: its remote URL and its host/path identity. */
export interface ProjectRepository {
  url: string
  identity: string
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
  /** The repository the URL names (`host/path`), derived by the server. */
  repository?: string
  /** The forge whose token the last state refresh lacked. */
  missingToken?: 'github' | 'gitlab'
}

/** Where a ticket stands in the batch run that covers it. */
export type BatchMemberState = 'waiting' | 'processing' | 'done'

/** A ticket's place in a running batch, as the server reports it on the task. */
export interface TaskBatch {
  /** The batch run, which sits on the lead ticket. */
  runId: string
  leadTaskId: string
  leadKey: string
  /** 1 for the lead, in launch order. */
  position: number
  size: number
  state: BatchMemberState
}

export interface Task {
  id: string
  /**
   * The project the ticket is shown for: the scoped one when a project is
   * listed, else its first project (#741). Computed by the server.
   */
  projectId?: string
  /** Every project the ticket belongs to: those selecting its tracker and its label (#741). */
  projectIds?: string[]
  /** The tracker the ticket belongs to (#741). Empty for a row written before trackers. */
  trackerId?: string
  key: string
  title: string
  description: string
  status: Status
  priority: Priority
  /** Why a creation reached the tracker without its priority (#679). */
  priorityNotice?: string
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
  /** Legacy free-text working directory, ignored by the agent. Superseded by `repository`. */
  repoPath?: string
  /** Identity of the repository the ticket is pinned to, e.g. "github.com/o/b". Empty = not pinned. */
  repository?: string
  /** Identities of the repositories the ticket's work changed. */
  changedRepositories?: string[]
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
  /** The ticket's place in a running batch (#522); absent when it is in none. */
  batch?: TaskBatch
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

export type ViewMode = 'board' | 'list' | 'triage' | 'roadmap' | 'timeline' | 'activities' | 'sync' | 'skills' | 'team' | 'admin'
  /** A tracker's tickets in no project (#741). */
  | 'tracker-backlog'

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

export type AIProvider = 'agy' | 'claude' | 'codex' | 'custom'

export type IssueTracker = 'github' | 'gitlab' | 'jira' | 'local'

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
  issueTracker: IssueTracker
  githubRepo: string
  jiraProject?: string
  jiraUrl?: string
  /**
   * Never returned: the server credentials live in the Administration page
   * (#464). The flags below are what the API answers instead.
   */
  jiraEmail?: string
  jiraApiToken?: string
  /** A Jira server credential is stored. */
  jiraApiTokenSet?: boolean
  /** None is stored, but the server environment provides one. */
  jiraApiTokenFromEnv?: boolean
  /** Instance GitHub, vide pour api.github.com. */
  githubApiUrl?: string
  /** Instance GitLab et projet par défaut, l'équivalent de githubRepo. */
  gitlabUrl?: string
  gitlabProject?: string
  /** GitHub and GitLab server credentials: the same flags as Jira's. */
  githubToken?: string
  githubTokenSet?: boolean
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
  specFramework?: SpecFramework
  updatedAt: string
}

/**
 * What the caller's own workstation reported for a project (#305): the engine
 * and models its next run would use. `unknown` means no agent of the caller is
 * connected for the project, or it predates the report; the web then names no
 * model and offers no picker. Empty values may be omitted by the server.
 */
export interface EngineReport {
  state: 'reported' | 'unknown'
  provider?: AIProvider | string
  /** Project-wide model, used by skills without their own entry. */
  model?: string
  /** Per-skill models (skillId -> model), outranking `model`. */
  skillModels?: Record<string, string>
  /** The models the workstation offers at launch. */
  models?: string[]
  /** Whether the reported command line carries a model at all. */
  modelSlot?: boolean
  /** Whether a headless run is possible on that workstation. */
  headless?: boolean
  reportedAt?: string
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
  /** A tracker a credential can be stored for: every remote one. */
  tracker: Exclude<IssueTracker, 'local'>
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
  /** The pacing of each tracker the loop reads, or of one project's trackers (#741). */
  trackers?: TrackerAutoSyncState[]
}

/** One tracker's background synchronisation (#741). */
export interface TrackerAutoSyncState {
  trackerId: string
  name: string
  provider: string
  enabled: boolean
  intervalMin: number
  lastPassAt?: string
  lastFullSyncAt?: string
}

// A link a toast offers to the thing it announces: opened in the app, and on
// its tracker page when it has one.
export interface ToastLink {
  label: string
  onOpen: () => void
  externalUrl?: string
}

export interface ToastMessage {
  id: string
  type: 'success' | 'info' | 'warning' | 'error'
  title: string
  description?: string
  duration?: number
  link?: ToastLink
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
