import { sameTask, tasksInProject } from '../lib/taskIdentity'
import React, { createContext, useContext, useState, useEffect, useCallback, useMemo, useRef, type ReactNode } from 'react'
import {
  loadBoardCardDisplayMode,
  saveBoardCardDisplayMode,
  toggleBoardCardDisplayMode as toggleDisplayModeValue,
} from '../lib/boardDisplayMode'
import type {
  MacroRequiredField,
  SkillEditorEntry,
  SkillMode,
  Task,
  CloneTaskRequest,
  Status,
  Priority,
  UserSettings,
  ViewMode,
  BoardGroupingMode,
  BoardCardDisplayMode,
  WorkflowStage,
  ToastMessage,
  Skill,
  TaskActivity,
  ActivityStats,
  CliStatus,
  TaskSource,
  Project,
  TrackerBoard,
  TaskComment,
  MacroMeta,
  MacroHorizon,
  MacroTodo,
  TrackerTeam,
  TeamMember,
  TeamWorkload,
  TaskFacetValue,
  CreateTaskPayload,
  RefineMacroResult,
  AutoSyncState,
  TrackerCheck,
  TrackerCredentials,
} from '../types'
import { translations, type TranslationSchema } from '../locales/translations'
import { resolveAccentAttribute } from '../lib/accents'
import { activeTaskIds } from '../lib/remoteRunIndicator'
import {
  INTERNAL_STATUS_BY_STAGE,
  resolveTaskStage, skillForStage,
  stageForInternalStatus,
  stageForTrackerStatuses,
  trackerStatusesForStage,
} from '../lib/workflow'

interface AppContextType {
  projects: Project[]
  selectedProjectId: string | 'all'
  setSelectedProjectId: (id: string | 'all') => void
  currentProject: Project | null
  createProject: (data: Partial<Project>) => Promise<Project | null>
  updateProject: (id: string, updates: Partial<Project>) => Promise<Project | null>
  deleteProject: (id: string) => Promise<boolean>
  fetchProjects: () => Promise<void>
  isProjectModalOpen: boolean
  setIsProjectModalOpen: (open: boolean) => void
  editingProject: Project | null
  setEditingProject: (p: Project | null) => void
  tasks: Task[]
  skills: Skill[]
  cliStatuses: CliStatus[]

  isFetchingGitStatus: boolean


  isLoading: boolean
  isSkillRunning: boolean
  isSyncing: boolean
  runningSkillId: string | null
  error: string | null
  activeView: ViewMode
  setActiveView: (view: ViewMode) => void
  boardGrouping: BoardGroupingMode
  setBoardGrouping: (mode: BoardGroupingMode) => void
  boardCardDisplayMode: BoardCardDisplayMode
  setBoardCardDisplayMode: (mode: BoardCardDisplayMode) => void
  toggleBoardCardDisplayMode: () => void
  searchQuery: string
  setSearchQuery: (query: string) => void
  statusFilter: Status | null
  setStatusFilter: (status: Status | null) => void
  priorityFilter: Priority | null
  setPriorityFilter: (priority: Priority | null) => void
  labelFilter: string | null
  taskFacets: {
    sprints: string[]
    teams: string[]
    macros: { key: string; title: string; count: number }[]
    noMacroCount: number
    assignees: string[]
    unassignedCount: number
    trackerStatuses: TaskFacetValue[]
    statuses: TaskFacetValue[]
    sources: TaskFacetValue[]
    labels: TaskFacetValue[]
    issueTypes: TaskFacetValue[]
    total: number
  }
  /** Types de tickets affichés. Vide veut dire « tous ». */
  issueTypeFilters: string[]
  setIssueTypeFilters: (types: string[]) => void
  /** État de la boucle de synchronisation de fond, rafraîchi avec les activités. */
  autoSync: AutoSyncState | null
  /** Écran de connexion au tracker, ouvert à la demande et jamais au démarrage. */
  isTrackerSetupOpen: boolean
  setIsTrackerSetupOpen: (open: boolean) => void
  /** Vérifie des accès tracker sans rien enregistrer. */
  checkTrackerCredentials: (params: TrackerCredentials) => Promise<TrackerCheck>
  /** Enregistre des accès déjà vérifiés, jeton en base ou dans un fichier à part. */
  saveTrackerCredentials: (params: TrackerCredentials) => Promise<boolean>
  /**
   * Statuts du tracker affichés. Vide veut dire « tous » : c'est le choix
   * explicite de ce qu'on regarde, board comme liste, et il remplace le
   * masquage des seules tâches terminées.
   */
  trackerStatusFilters: string[]
  setTrackerStatusFilters: (statuses: string[]) => void
  /** Équipes portées par les tickets du projet, avec leurs membres. */
  teams: TrackerTeam[]
  fetchTeams: () => Promise<void>
  /** Membres d'une équipe, par son nom : ce que porte un ticket. */
  membersForTeam: (teamName: string) => Promise<TeamMember[]>
  /** Relit les membres d'une équipe depuis le tracker. */
  refreshTeamMembers: (teamId: string) => Promise<TrackerTeam | null>
  fetchTeamWorkload: (teamName: string) => Promise<TeamWorkload | null>
  /** Recherche d'équipes sur l'instance, pour en choisir une hors de celles du board. */
  searchTrackerTeams: (query: string) => Promise<TrackerTeam[]>
  /** Change l'équipe d'un ticket, ou la retire avec un identifiant vide. */
  setTaskTeam: (taskId: string, teamId: string, teamName?: string) => Promise<Task | null>
  /** Qui peut recevoir ce ticket : l'équipe du ticket sans frappe, l'instance ensuite. */
  searchAssignableUsers: (taskId: string, query: string) => Promise<TeamMember[]>
  /** Déplace un ticket dans un sprint du board, ou au backlog avec un id vide. */
  setTaskSprint: (taskId: string, sprintId: string, sprintName?: string) => Promise<Task | null>
  /** Même chose pour un lot : la planification depuis la roadmap. */
  setTasksSprint: (projectId: string, taskIds: string[], sprintId: string, sprintName?: string) => Promise<boolean>
  /** Équipe d'un lot de tickets, en une seule activité. */
  setTasksTeam: (projectId: string, taskIds: string[], teamId: string, teamName?: string) => Promise<boolean>
  /** N'afficher que les tickets épinglés : le retour rapide aux chantiers en cours. */
  pinnedOnly: boolean
  setPinnedOnly: (value: boolean) => void
  /** N'afficher que les tickets qu'un agent est en train de traiter. */
  activeOnly: boolean
  setActiveOnly: (value: boolean) => void
  /** Les tickets portant une exécution distante vivante, pour le filtre et son compteur. */
  activeTasks: Set<string>
  sprintFilter: string | null
  setSprintFilter: (sprint: string | null) => void
  teamFilter: string | null
  setTeamFilter: (team: string | null) => void
  macroFilter: string | null
  setMacroFilter: (macro: string | null) => void
  setLabelFilter: (label: string | null) => void
  assigneeFilter: string | null
  setAssigneeFilter: (assignee: string | null) => void
  sourceFilter: 'all' | TaskSource
  setSourceFilter: (source: 'all' | TaskSource) => void
  /** Filters the board on a parent work item key (epic, or parent story). */
  parentFilter: string | null
  setParentFilter: (parentKey: string | null) => void
  /** Distinct parents present in the loaded tasks, most populated first. */
  availableParents: { key: string; title: string; type: string; count: number }[]
  /**
   * Resolves the display name of a workflow skill, honouring the project's
   * `skillOverrides`. Pass `projectId` to resolve against a specific project —
   * a task's project is not necessarily the one selected in the sidebar.
   */
  skillLabel: (skillId: string, fallback?: string, projectId?: string) => string
  /**
   * Resolves the slash command of a workflow skill. A project override is
   * treated as the command to invoke, normalised with a leading slash, so
   * renaming a skill also changes the command shown and run.
   */
  skillCommand: (skillId: string, fallback: string, projectId?: string) => string
  /** Docked workspace terminal on the right side of the app. */


  sidebarCollapsed: boolean
  setSidebarCollapsed: (collapsed: boolean | ((prev: boolean) => boolean)) => void
  selectedTask: Task | null
  setSelectedTask: (task: Task | null) => void


  /**
   * Session de terminal ouverte par son identifiant, sans passer par une tâche
   * chargée. Une exécution autonome crée sa session côté serveur, et elle doit
   * pouvoir s'ouvrir même quand la tâche est filtrée ou appartient à un autre
   * projet.
   */


  hideDone: boolean
  setHideDone: (hide: boolean | ((prev: boolean) => boolean)) => void
  toggleHideDone: () => void
  isQuickAddOpen: boolean
  setIsQuickAddOpen: (open: boolean) => void
  quickAddInitialStatus: Status
  setQuickAddInitialStatus: (status: Status) => void
  isCommandPaletteOpen: boolean
  setIsCommandPaletteOpen: (open: boolean) => void
  isProfileOpen: boolean
  setIsProfileOpen: (open: boolean) => void
  settings: UserSettings
  /**
   * `silent` évite le toast de confirmation : un basculement de thème ou
   * d'échelle se voit à l'écran, l'annoncer à chaque clic ne fait que du bruit.
   */
  updateSettings: (newSettings: Partial<UserSettings>, options?: { silent?: boolean }) => Promise<void>
  t: TranslationSchema
  toasts: ToastMessage[]
  addToast: (toast: Omit<ToastMessage, 'id'>) => void
  removeToast: (id: string) => void
  createTask: (task: { title: string; description?: string; status?: Status; priority?: Priority; labels?: string[]; assignee?: string; dueDate?: string | null; sprint?: string; source?: TaskSource; externalUrl?: string; projectId?: string; issueType?: string }) => Promise<Task | null>
  cloneTask: (taskId: string, req?: CloneTaskRequest, openAfterClone?: boolean) => Promise<Task | null>
  isCloneModalOpen: boolean
  setIsCloneModalOpen: (open: boolean) => void
  cloneSourceTask: Task | null
  setCloneSourceTask: (task: Task | null) => void
  openCloneModal: (task: Task) => void
  /**
   * assigneeAccountId accompagne un changement d'assigné : Jira n'assigne que
   * par identifiant de compte, jamais par nom affiché.
   */
  updateTask: (id: string, updates: Partial<Task> & { assigneeAccountId?: string }) => Promise<Task | null>
  moveTaskToTrackerStatus: (id: string, status: string) => Promise<Task | null>
  getTaskComments: (id: string) => Promise<TaskComment[]>


  postTaskComment: (id: string, body: string) => Promise<TaskComment[] | null>
  listProjectBoards: (projectId: string) => Promise<TrackerBoard[]>
  importProjectBoardColumns: (projectId: string, boardId: string) => Promise<Project | null>
  fetchProjectTrackerStatuses: (projectId: string) => Promise<string[]>
  /** Types de tickets que le tracker du projet expose, pour le réglage d'import. */
  fetchProjectIssueTypes: (projectId: string) => Promise<string[]>
  fetchProjectMacros: (projectId: string) => Promise<MacroMeta[]>
  fetchProjectEpics: (projectId: string) => Promise<MacroMeta[]>
  refineMacro: (key: string, projectId?: string) => Promise<RefineMacroResult | null>
  createBatchTasks: (reqs: CreateTaskPayload[]) => Promise<Task[]>

  saveMacroMeta: (projectId: string, key: string, patch: { title?: string; horizon?: MacroHorizon | ''; description?: string; framingComment?: string; todos?: MacroTodo[]; closed?: boolean }) => Promise<MacroMeta | null>
  saveEpicMeta: (projectId: string, key: string, patch: { title?: string; horizon?: MacroHorizon | ''; description?: string; framingComment?: string; todos?: MacroTodo[]; closed?: boolean }) => Promise<MacroMeta | null>
  createStoryFromMacroTodo: (projectId: string, macroKey: string, todoId: string) => Promise<{ macro: MacroMeta | null; epic: MacroMeta | null; storyKey: string } | null>
  createStoryFromEpicTodo: (projectId: string, epicKey: string, todoId: string) => Promise<{ macro: MacroMeta | null; epic: MacroMeta | null; storyKey: string } | null>
  pendingHorizonPushes: (projectId: string) => Promise<MacroMeta[]>
  /** Met la poussée des labels d'horizon en file d'activités. Retourne true si la file a accepté. */
  pushPendingHorizons: (projectId: string) => Promise<boolean>
  /**
   * Met le rattachement à une macro en file d'activités et renvoie le ticket tel
   * qu'il est déjà en local.
   */
  setTaskMacro: (taskId: string, macroKey: string) => Promise<Task | null>
  setTaskEpic: (taskId: string, epicKey: string) => Promise<Task | null>
  createStoryUnderMacro: (projectId: string, macroKey: string, title: string) => Promise<string>
  createStoryUnderEpic: (projectId: string, epicKey: string, title: string) => Promise<string>
  createMacro: (projectId: string, title: string, horizon?: MacroHorizon | '', fields?: Record<string, string>) => Promise<MacroMeta | null>
  createEpic: (projectId: string, title: string, horizon?: MacroHorizon | '', fields?: Record<string, string>) => Promise<MacroMeta | null>
  deleteMacro: (projectId: string, key: string) => Promise<boolean>
  deleteEpic: (projectId: string, key: string) => Promise<boolean>
  migrateMacro: (sourceProjectId: string, macroKey: string, targetProjectId: string, migrateTasks: boolean) => Promise<{ success: boolean; macro?: MacroMeta; epic?: MacroMeta; migratedTasks?: number; error?: string }>
  migrateEpic: (sourceProjectId: string, epicKey: string, targetProjectId: string, migrateTasks: boolean) => Promise<{ success: boolean; macro?: MacroMeta; epic?: MacroMeta; migratedTasks?: number; error?: string }>
  migrateTasks: (taskIds: string[], targetProjectId: string) => Promise<{ success: boolean; migratedCount?: number; error?: string }>
  /** Champs que le tracker impose pour créer une macro sur ce projet. */
  fetchMacroRequiredFields: (projectId: string) => Promise<MacroRequiredField[]>
  fetchEpicRequiredFields: (projectId: string) => Promise<MacroRequiredField[]>
  /** Met la découpe de macro en file d'activités. Retourne true si la file a accepté. */
  moveTasksToMacro: (projectId: string, taskIds: string[], targetMacroKey: string, newMacroTitle?: string, fields?: Record<string, string>) => Promise<boolean>
  moveTasksToEpic: (projectId: string, taskIds: string[], targetEpicKey: string, newEpicTitle?: string, fields?: Record<string, string>) => Promise<boolean>
  /** Transitions a task's agentic workflow stage/label, updating local state and queueing tracker sync. */
  transitionTaskStage: (taskIdOrKey: string, stage: string, note?: string, prUrl?: string, branch?: string) => Promise<{ success: boolean; task?: Task; activity?: TaskActivity; error?: string }>
  advanceTask: (taskId: string, auto?: boolean, mode?: SkillMode) => Promise<{ mode: string; skillId?: string; label?: string } | null>
  // Pas interactif en cours : la tâche dont la session TTY attend d'être clôturée.
  pendingInteractive: { taskId: string; taskKey: string; skillId: string; label: string } | null
  /** Tickets épinglés : la barre de bascule rapide entre chantiers en cours. */
  pinnedTasks: Task[]
  isPinned: (taskId: string) => boolean
  togglePin: (taskId: string) => Promise<void>
  hotSwitch: (taskId: string) => void
  /** Éditeur de skills : les cinq pas du workflow du projet courant. */
  /** Démarre l'agent du projet dans la session d'une tâche. */

  /** Tape l'appel d'une skill dans l'agent déjà démarré. */

  fetchSkillEditor: () => Promise<SkillEditorEntry[]>
  saveSkillContent: (skillId: string, content: string) => Promise<SkillEditorEntry | null>
  resetSkillContent: (skillId: string) => Promise<SkillEditorEntry | null>
  saveSkillMode: (skillId: string, mode: SkillMode) => Promise<SkillEditorEntry | null>
  importSkillFromRepo: (skillId: string) => Promise<SkillEditorEntry | null>
  launchInteractiveStep: (task: Task, skillId: string, label: string) => Promise<void>
  confirmInteractiveStep: (note?: string) => Promise<void>
  dismissInteractiveStep: () => void
  convertTask: (id: string, target: 'github') => Promise<Task | null>
  moveTask: (id: string, newStatus: Status, newPosition: number) => Promise<void>
  moveTaskWorkflowStage: (taskId: string, targetStage: WorkflowStage) => Promise<Task | null>
  deleteTask: (id: string) => Promise<boolean>
  runSkill: (taskId: string, skillId: string, prompt?: string, opts?: { withComments?: boolean; mode?: SkillMode }) => Promise<TaskActivity | null>
  syncAll: () => Promise<void>
  syncGithub: (repo?: string) => Promise<void>
  syncJira: (projectKey?: string) => Promise<void>
  syncCurrentProject: () => Promise<void>
  syncSingleTask: (taskId: string) => Promise<Task | null>
  fetchCliStatus: () => Promise<void>
  refreshTasks: () => Promise<void>
  activities: TaskActivity[]
  activityStats: ActivityStats
  selectedActivity: TaskActivity | null
  setSelectedActivity: (activity: TaskActivity | null) => void
  activeJobCount: number
  fetchActivities: () => Promise<void>
  fetchActivityStats: () => Promise<void>
  retryActivity: (id: string) => Promise<void>
  cancelActivity: (id: string) => Promise<void>
  deleteActivity: (id: string) => Promise<void>
  clearCompletedActivities: () => Promise<void>
  availableLabels: string[]
  availableAssignees: string[]
  /** Valeur sentinelle du filtre « non assigné » : le vide veut dire « pas de filtre ». */
  unassignedFilterValue: string


  startBatchPickup: (taskIds: string[]) => Promise<void>
}

/**
 * Vues connues. Ce qui sort du stockage local n'est pas fiable : une vue retirée
 * d'une version à l'autre laisserait un écran vide au démarrage.
 */
const VIEW_MODES: ViewMode[] = ['board', 'list', 'triage', 'roadmap', 'timeline', 'activities', 'sync', 'skills', 'team']

const defaultSettings: UserSettings = {
  id: 1,
  theme: 'dark',
  accentColor: 'orange',
  language: 'fr',
  density: 'standard',
  uiScale: 100,
  defaultView: 'board',
  detailMode: 'panel',
  userName: 'Developer',
  userEmail: 'dev@example.com',
  userAvatar: '',
  aiProvider: 'agy',
  aiCommandTemplate: 'agy -p "{prompt}"',
  aiModel: '',
  repoPath: '',
  issueTracker: 'local',
  githubRepo: '',
  jiraProject: '',
  jiraUrl: '',
  specFramework: 'speckit',
  promptClarify: '',
  promptSpecify: '',
  promptImplement: '',
  promptCreatePr: '',
  promptPick: '',
  editorCommand: 'code',
  externalTerminalCommand: '',
  updatedAt: new Date().toISOString(),
}

const AppContext = createContext<AppContextType | undefined>(undefined)

const API_BASE = '/api'

/**
 * Les quatre niveaux de zoom de l'interface, dans l'ordre du commutateur de la
 * barre d'état. Quatre crans est ce qu'un réglage rapide peut porter : un nombre
 * libre demanderait un écran de réglages, ce qui n'est pas ce que demande « c'est
 * trop petit, tout de suite ». La même liste borne la valeur côté serveur.
 */
export const UI_SCALE_OPTIONS = [90, 100, 112, 125]

// Le filtre « non assigné » a besoin d'une valeur : une chaîne vide voudrait dire
// « aucun filtre ». La même sentinelle est reconnue côté serveur.
const UNASSIGNED_FILTER_VALUE = '__unassigned__'

export const AppProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [tasks, setTasks] = useState<Task[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [cliStatuses, setCliStatuses] = useState<CliStatus[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isSkillRunning, setIsSkillRunning] = useState(false)
  const [isSyncing, setIsSyncing] = useState(false)
  const [runningSkillId, setRunningSkillId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  /**
   * L'écran affiché survit au rechargement.
   *
   * Recharger est un geste courant : un F5, une reprise après remplacement du
   * binaire, un onglet restauré. Repartir de force sur le board fait perdre sa
   * place, et il fallait revenir à la main dans la roadmap ou le triage à chaque
   * fois.
   *
   * Au tout premier lancement il n'y a rien à restaurer : la vue par défaut des
   * réglages prend alors le relais, ce qui est son rôle et n'était appliqué
   * nulle part jusqu'ici. Ensuite c'est la dernière vue visitée qui gagne, sinon
   * la mémorisation ne servirait à rien.
   */
  const [activeView, setActiveViewState] = useState<ViewMode>(() => {
    try {
      const stored = localStorage.getItem('sectile_active_view') ?? localStorage.getItem('taskacao_active_view')
      return stored && VIEW_MODES.includes(stored as ViewMode) ? (stored as ViewMode) : 'board'
    } catch {
      return 'board'
    }
  })
  // Vrai tant qu'aucune vue n'a été mémorisée : la vue par défaut des réglages
  // ne s'applique qu'à cet instant, jamais par-dessus un choix en cours.
  const defaultViewPending = useRef<boolean>(
    (() => {
      try {
        return !(localStorage.getItem('sectile_active_view') ?? localStorage.getItem('taskacao_active_view'))
      } catch {
        return false
      }
    })()
  )

  const setActiveView = useCallback((view: ViewMode) => {
    setActiveViewState(view)
    defaultViewPending.current = false
    try {
      localStorage.setItem('sectile_active_view', view)
    } catch {
      // stockage indisponible : la vue vaut pour cette session
    }
  }, [])

  const [boardGrouping, setBoardGroupingState] = useState<BoardGroupingMode>(() => {
    try {
      const val = (localStorage.getItem('sectile_board_grouping') ?? localStorage.getItem('taskacao_board_grouping')) as BoardGroupingMode
      return val || 'status'
    } catch {
      return 'status'
    }
  })

  const persistBoardGrouping = useCallback((mode: BoardGroupingMode) => {
    setBoardGroupingState(mode)
    try {
      localStorage.setItem('sectile_board_grouping', mode)
    } catch {
      // stockage indisponible : le mode vaut pour cette session
    }
  }, [])

  const [boardCardDisplayMode, setBoardCardDisplayModeState] = useState<BoardCardDisplayMode>(() => {
    return loadBoardCardDisplayMode()
  })

  const setBoardCardDisplayMode = useCallback((mode: BoardCardDisplayMode) => {
    setBoardCardDisplayModeState(mode)
    saveBoardCardDisplayMode(mode)
  }, [])

  const toggleBoardCardDisplayMode = useCallback(() => {
    setBoardCardDisplayModeState(prev => {
      const next = toggleDisplayModeValue(prev)
      saveBoardCardDisplayMode(next)
      return next
    })
  }, [])
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilterState] = useState<Status | null>(null)
  const [priorityFilter, setPriorityFilterState] = useState<Priority | null>(null)
  const [labelFilter, setLabelFilterState] = useState<string | null>(null)
  const [taskFacets, setTaskFacets] = useState<{
    sprints: string[]
    teams: string[]
    macros: { key: string; title: string; count: number }[]
    noMacroCount: number
    assignees: string[]
    unassignedCount: number
    trackerStatuses: TaskFacetValue[]
    statuses: TaskFacetValue[]
    sources: TaskFacetValue[]
    labels: TaskFacetValue[]
    issueTypes: TaskFacetValue[]
    total: number
  }>({
    sprints: [],
    teams: [],
    macros: [],
    noMacroCount: 0,
    assignees: [],
    unassignedCount: 0,
    trackerStatuses: [],
    statuses: [],
    sources: [],
    labels: [],
    issueTypes: [],
    total: 0,
  })
  const [issueTypeFilters, setIssueTypeFiltersState] = useState<string[]>([])
  const [autoSync, setAutoSync] = useState<AutoSyncState | null>(null)
  const [isTrackerSetupOpen, setIsTrackerSetupOpen] = useState(false)
  const [trackerStatusFilters, setTrackerStatusFiltersState] = useState<string[]>([])
  const [teams, setTeams] = useState<TrackerTeam[]>([])
  const [sprintFilter, setSprintFilterState] = useState<string | null>(null)
  const [pinnedOnly, setPinnedOnlyState] = useState<boolean>(false)
  const [activeOnly, setActiveOnlyState] = useState<boolean>(false)
  const [teamFilter, setTeamFilterState] = useState<string | null>(null)
  const [assigneeFilter, setAssigneeFilterState] = useState<string | null>(null)
  const [sourceFilter, setSourceFilter] = useState<'all' | TaskSource>('all')
  const [parentFilter, setParentFilterState] = useState<string | null>(null)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  // chatTask désigne la tâche dont le PTY est affiché. Il vit dans le panneau
  // latéral ancré, pas dans une modale : on garde le board visible à côté du
  // terminal, et une session par tâche reste accessible d'un clic.

  // Le pas interactif attend une confirmation humaine : la skill du dépôt ne
  // produit que du texte dans le terminal, elle ne touche jamais au ticket.
  const [pendingInteractive, setPendingInteractive] = useState<{
    taskId: string
    taskKey: string
    skillId: string
    label: string
  } | null>(null)


  const [pinnedTasks, setPinnedTasks] = useState<Task[]>([])


  // Ouvre une session par son identifiant. C'est le chemin qui marche toujours :
  // il ne dépend pas de la présence de la tâche dans la liste courante.


  const [hideDone, setHideDoneState] = useState<boolean>(() => {
    try {
      const val = localStorage.getItem('sectile_hide_done') ?? localStorage.getItem('taskacao_hide_done')
      return val === 'true'
    } catch {
      return false
    }
  })

  const setHideDone = useCallback((val: boolean | ((prev: boolean) => boolean)) => {
    setHideDoneState(prev => {
      const next = typeof val === 'function' ? val(prev) : val
      try {
        localStorage.setItem('sectile_hide_done', String(next))
      } catch {}
      return next
    })
  }, [])

  const toggleHideDone = useCallback(() => {
    setHideDone(prev => !prev)
  }, [setHideDone])

  const [isQuickAddOpen, setIsQuickAddOpen] = useState(false)
  const [quickAddInitialStatus, setQuickAddInitialStatus] = useState<Status>('backlog')
  const [isCloneModalOpen, setIsCloneModalOpen] = useState(false)
  const [cloneSourceTask, setCloneSourceTask] = useState<Task | null>(null)

  const openCloneModal = useCallback((task: Task) => {
    setCloneSourceTask(task)
    setIsCloneModalOpen(true)
  }, [])

  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false)
  const [isProfileOpen, setIsProfileOpen] = useState(false)
  const [settings, setSettings] = useState<UserSettings>(defaultSettings)
  const [toasts, setToasts] = useState<ToastMessage[]>([])

  // Projects State
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProjectId, setSelectedProjectIdState] = useState<string | 'all'>(() => {
    try {
      return localStorage.getItem('sectile_selected_project_id') || localStorage.getItem('taskacao_selected_project_id') || 'all'
    } catch {
      return 'all'
    }
  })
  const setSelectedProjectId = useCallback((id: string | 'all') => {
    setSelectedProjectIdState(id)
    try {
      localStorage.setItem('sectile_selected_project_id', id)
    } catch {}
  }, [])

  // Les filtres sont mémorisés par projet : sprint et équipe n'ont de sens que
  // dans le projet où ils ont été choisis, et on retrouve son contexte de
  // travail en revenant sur un projet ou après un rechargement.
  const filterStorageKey = (projectId: string) => `sectile_filters_${projectId || 'all'}`
  const legacyFilterStorageKey = (projectId: string) => `taskacao_filters_${projectId || 'all'}`

  const readStoredFilters = (projectId: string): Record<string, string | null> => {
    try {
      return JSON.parse(localStorage.getItem(filterStorageKey(projectId)) || localStorage.getItem(legacyFilterStorageKey(projectId)) || '{}') || {}
    } catch {
      return {}
    }
  }

  // L'écriture se fait dans les setters et non dans un effet : au changement de
  // projet, un effet verrait encore les filtres de l'ancien projet et les
  // écrirait sous la clé du nouveau.
  const persistFilter = useCallback((patch: Record<string, string | null>) => {
    try {
      const current = readStoredFilters(selectedProjectId)
      const merged = { ...current, ...patch }
      localStorage.setItem(filterStorageKey(selectedProjectId), JSON.stringify(merged))
    } catch {
      // stockage indisponible : les filtres restent simplement non mémorisés
    }
  }, [selectedProjectId])

  const setStatusFilter = useCallback((value: Status | null) => {
    setStatusFilterState(value)
    persistFilter({ status: value })
  }, [persistFilter])

  const setPriorityFilter = useCallback((value: Priority | null) => {
    setPriorityFilterState(value)
    persistFilter({ priority: value })
  }, [persistFilter])

  const setLabelFilter = useCallback((value: string | null) => {
    setLabelFilterState(value)
    persistFilter({ label: value })
  }, [persistFilter])

  const setSprintFilter = useCallback((value: string | null) => {
    setSprintFilterState(value)
    persistFilter({ sprint: value })
  }, [persistFilter])

  const setTeamFilter = useCallback((value: string | null) => {
    setTeamFilterState(value)
    persistFilter({ team: value })
  }, [persistFilter])

  const setPinnedOnly = useCallback((value: boolean) => {
    setPinnedOnlyState(value)
    persistFilter({ pinnedOnly: value ? '1' : null })
  }, [persistFilter])

  const setActiveOnly = useCallback((value: boolean) => {
    setActiveOnlyState(value)
    persistFilter({ activeOnly: value ? '1' : null })
  }, [persistFilter])

  const setTrackerStatusFilters = useCallback((values: string[]) => {
    setTrackerStatusFiltersState(values)
    // Mémorisé comme les autres filtres, en JSON puisque c'est une liste.
    persistFilter({ trackerStatuses: values.length > 0 ? JSON.stringify(values) : null })
  }, [persistFilter])

  const setIssueTypeFilters = useCallback((values: string[]) => {
    setIssueTypeFiltersState(values)
    persistFilter({ issueTypes: values.length > 0 ? JSON.stringify(values) : null })
  }, [persistFilter])

  const setAssigneeFilter = useCallback((value: string | null) => {
    setAssigneeFilterState(value)
    persistFilter({ assignee: value })
  }, [persistFilter])

  const setParentFilter = useCallback((value: string | null) => {
    setParentFilterState(value)
    persistFilter({ parent: value })
  }, [persistFilter])

  const [isProjectModalOpen, setIsProjectModalOpen] = useState(false)
  const [editingProject, setEditingProject] = useState<Project | null>(null)

  const currentProject = useMemo(() => {
    if (selectedProjectId === 'all') return null
    return projects.find(p => p.id === selectedProjectId || p.slug === selectedProjectId) || null
  }, [projects, selectedProjectId])

  /**
   * Changer de mode d'affichage convertit le filtre en cours plutôt que de le
   * laisser en travers : une étape du workflow et une colonne de statuts
   * désignent le même travail, le mapping du projet dit lequel. Sans conversion,
   * passer en mode statut avec un filtre d'étape actif laissait un board filtré
   * par quelque chose que l'écran n'affiche plus.
   *
   * Sans mapping (projet sans colonnes affectées), rien à convertir : le filtre
   * est simplement levé, ce qui vaut mieux qu'une correspondance devinée.
   */
  const setBoardGrouping = useCallback((mode: BoardGroupingMode) => {
    if (mode === boardGrouping) {
      persistBoardGrouping(mode)
      return
    }

    // Une conversion qui ne trouve rien ne doit pas lever le filtre : sans
    // mapping de colonnes (un projet dont le board n'a jamais été importé), le
    // filtre d'étape reste parfaitement applicable dans les deux modes, et le
    // supprimer en silence donnait un board qui change de contenu sans raison
    // visible.
    if (mode === 'status') {
      if (statusFilter) {
        const stage = stageForInternalStatus(statusFilter)
        const statuses = trackerStatusesForStage(currentProject, stage)
        if (statuses.length > 0) {
          setStatusFilter(null)
          setTrackerStatusFilters(statuses)
        }
      }
    } else if (trackerStatusFilters.length > 0) {
      const stage = stageForTrackerStatuses(currentProject, trackerStatusFilters)
      if (stage) {
        setTrackerStatusFilters([])
        setStatusFilter(INTERNAL_STATUS_BY_STAGE[stage])
      }
    }

    persistBoardGrouping(mode)
  }, [
    boardGrouping,
    persistBoardGrouping,
    currentProject,
    statusFilter,
    trackerStatusFilters,
    setStatusFilter,
    setTrackerStatusFilters,
  ])

  // Git Status & Branches State


  const [isFetchingGitStatus] = useState(false)


  // Activities & Queue State
  const [activities, setActivities] = useState<TaskActivity[]>([])
  // Les tickets qu'un agent traite, dérivés des activités comme le badge des
  // cartes : une seule passe, et le filtre ne peut pas contredire la pastille
  // affichée à côté de lui. Le jeu suit le rafraîchissement des activités, donc
  // un run qui démarre ou s'achève déplace son ticket sans geste de l'utilisateur.
  const activeTasks = useMemo(() => activeTaskIds(activities), [activities])
  const [activityStats, setActivityStats] = useState<ActivityStats>({
    total: 0,
    queued: 0,
    running: 0,
    completed: 0,
    failed: 0,
    canceled: 0,
  })
  const [selectedActivity, setSelectedActivity] = useState<TaskActivity | null>(null)
  const prevActiveActivitiesRef = useRef<Map<string, string>>(new Map())

  const addToast = useCallback((toast: Omit<ToastMessage, 'id'>) => {
    const id = Math.random().toString(36).substring(2, 9)
    const newToast: ToastMessage = { ...toast, id, duration: toast.duration || 3500 }
    setToasts(prev => [...prev, newToast])
  }, [])

  const removeToast = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const t = useMemo(() => translations[settings.language] || translations.fr, [settings.language])

  useEffect(() => {
    const root = document.documentElement
    const body = document.body

    if (settings.theme === 'light') {
      root.classList.add('light')
      root.classList.remove('dark')
    } else {
      root.classList.add('dark')
      root.classList.remove('light')
    }

    // Accent per project: the selected project's color drives the whole accent
    // scale (index.css [data-accent=…]). "All projects" keeps the brand orange.
    const accent = resolveAccentAttribute(currentProject?.color)
    body.setAttribute('data-accent', accent)
    root.setAttribute('data-accent', accent)

    const density = settings.density || 'standard'
    root.classList.remove('density-compact', 'density-standard', 'density-comfortable')
    body.classList.remove('density-compact', 'density-standard', 'density-comfortable')

    root.classList.add(`density-${density}`)
    body.classList.add(`density-${density}`)

    root.setAttribute('data-density', density)
    body.setAttribute('data-density', density)

    // Échelle de l'interface : un zoom sur la racine, parce que la moitié des
    // tailles de cette interface sont en pixels et qu'une taille de police
    // racine ne les touche pas. Le zoom est appliqué au document entier, donc
    // les panneaux, la barre latérale et les modales suivent ensemble.
    const scale = UI_SCALE_OPTIONS.includes(settings.uiScale || 100) ? settings.uiScale || 100 : 100
    root.style.zoom = scale === 100 ? '' : String(scale / 100)
    // --ui-zoom accompagne le zoom : les hauteurs d'écran s'en servent pour rester
    // dans la fenêtre, sinon la barre d'état passe sous le bord bas.
    root.style.setProperty('--ui-zoom', String(scale / 100))

    // Direct root font-size scaling for instantaneous global rem scaling
    if (density === 'compact') {
      root.style.fontSize = '12.5px'
    } else if (density === 'comfortable') {
      root.style.fontSize = '15.5px'
    } else {
      root.style.fontSize = '14px'
    }
  }, [settings.theme, settings.density, settings.uiScale, currentProject?.color])

  const fetchSettings = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/settings`)
      if (res.ok) {
        const data: UserSettings = await res.json()
        setSettings(data)
        // Premier lancement : aucune vue mémorisée, la vue par défaut des
        // réglages s'applique ici et nulle part ailleurs. C'est le seul moment
        // où l'on tient la valeur du serveur plutôt que celle de repli.
        if (defaultViewPending.current) {
          defaultViewPending.current = false
          if (data.defaultView && VIEW_MODES.includes(data.defaultView)) {
            setActiveView(data.defaultView)
          }
        }
      }
    } catch (err) {
      console.warn('Failed to load settings from server', err)
    }
  }, [setActiveView])

  const fetchSkills = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/skills`)
      if (res.ok) {
        const data: Skill[] = await res.json()
        setSkills(data)
      }
    } catch (err) {
      console.warn('Failed to load skills from server', err)
    }
  }, [])

  const fetchCliStatus = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/cli-status`)
      if (res.ok) {
        const data: CliStatus[] = await res.json()
        setCliStatuses(data)
      }
    } catch (err) {
      console.warn('Failed to load CLI statuses', err)
    }
  }, [])

  const fetchProjects = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/projects`)
      if (res.ok) {
        const data: Project[] = await res.json()
        const projectList = data || []
        setProjects(projectList)

        // Ensure a valid project is actively selected, preserving 'all' or active project with tasks
        setSelectedProjectIdState(prev => {
          if (prev === 'all') {
            return 'all'
          }
          if (prev && projectList.some(p => p.id === prev || p.slug === prev)) {
            return prev
          }
          try {
            const stored = localStorage.getItem('sectile_selected_project_id') || localStorage.getItem('taskacao_selected_project_id')
            if (stored === 'all') return 'all'
            if (stored && projectList.some(p => p.id === stored || p.slug === stored)) {
              return stored
            }
          } catch {}

          // Prioritize project with tasks or 'all'
          const projWithTasks = projectList.find(p => (p.taskCount || 0) > 0)
          if (projWithTasks) {
            try {
              localStorage.setItem('sectile_selected_project_id', projWithTasks.id)
            } catch {}
            return projWithTasks.id
          }
          return 'all'
        })
      }
    } catch (err) {
      console.warn('Failed to load projects', err)
    }
  }, [])

  const fetchActivities = useCallback(async () => {
    try {
      const params = new URLSearchParams()
      if (selectedProjectId && selectedProjectId !== 'all') {
        params.append('projectId', selectedProjectId)
      }
      const res = await fetch(`${API_BASE}/activities?${params.toString()}`)
      if (res.ok) {
        const data: TaskActivity[] = await res.json()
        setActivities(data)
      }
    } catch (err) {
      console.warn('Failed to load activities', err)
    }
  }, [selectedProjectId])

  const fetchActivityStats = useCallback(async () => {
    try {
      const params = new URLSearchParams()
      if (selectedProjectId && selectedProjectId !== 'all') {
        params.append('projectId', selectedProjectId)
      }
      const res = await fetch(`${API_BASE}/activities/stats?${params.toString()}`)
      if (res.ok) {
        const data: ActivityStats = await res.json()
        setActivityStats(data)
      }
    } catch (err) {
      console.warn('Failed to load activity stats', err)
    }
  }, [selectedProjectId])


  // Projet et filtres actifs, en un seul endroit : le rafraîchissement de fond
  // après une synchro ou une skill doit interroger exactement la même liste,
  // sinon il ramène tout le board et perd le contexte de travail.
  const buildTaskQuery = useCallback(() => {
    const params = new URLSearchParams()
    if (selectedProjectId && selectedProjectId !== 'all') {
      params.append('projectId', selectedProjectId)
    }
    // La roadmap se cherche par épic, pas par ticket. Envoyer la recherche au
    // serveur y amputerait les enfants de chaque épic : les compteurs de sprint
    // et le détail se videraient, et un épic dont aucun ticket ne correspond
    // disparaîtrait au lieu d'être trouvé. La vue filtre donc ses lignes
    // elle-même, sur des données complètes.
    if (searchQuery && activeView !== 'roadmap') params.append('q', searchQuery)
    if (statusFilter) params.append('status', statusFilter)
    if (priorityFilter) params.append('priority', priorityFilter)
    if (labelFilter) params.append('label', labelFilter)
    if (sprintFilter) params.append('sprint', sprintFilter)
    if (teamFilter) params.append('team', teamFilter)
    if (parentFilter) params.append('macro', parentFilter)
    // L'assigné se filtre côté serveur comme le reste : il n'était appliqué
    // nulle part, ce qui laissait « Mes tâches » sans effet.
    if (assigneeFilter) params.append('assignee', assigneeFilter)
    trackerStatusFilters.forEach(status => params.append('trackerStatus', status))
    issueTypeFilters.forEach(type => params.append('issueType', type))
    if (pinnedOnly) params.append('pinned', '1')
    return params.toString()
  }, [selectedProjectId, searchQuery, activeView, statusFilter, priorityFilter, labelFilter, sprintFilter, teamFilter, parentFilter, assigneeFilter, trackerStatusFilters, issueTypeFilters, pinnedOnly])

  // Resolve desktop deep links independently of board filters and pagination.
  useEffect(() => {
    const taskId = new URLSearchParams(window.location.search).get('task')
    if (!taskId) return
    const controller = new AbortController()
    fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}`, { signal: controller.signal })
      .then(async response => {
        if (!response.ok) throw new Error(`Unable to open task (HTTP ${response.status})`)
        const task: Task = await response.json()
        if (controller.signal.aborted) return
        if (task.projectId) setSelectedProjectId(task.projectId)
        setSelectedTask(task)
      })
      .catch(error => {
        if (!controller.signal.aborted) setError(error.message)
      })
    return () => controller.abort()
  }, [setSelectedProjectId])

  const fetchTasks = useCallback(async () => {
    try {
      setIsLoading(true)
      const res = await fetch(`${API_BASE}/tasks?${buildTaskQuery()}`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data: Task[] = await res.json()
      setTasks(data)
      setError(null)
    } catch (err: any) {
      setError(err.message || 'Failed to fetch tasks')
    } finally {
      setIsLoading(false)
    }
  }, [buildTaskQuery])

  // Restauration à l'ouverture et à chaque changement de projet. Les setters
  // bruts sont utilisés ici : réécrire ce qu'on vient de lire serait inutile.
  useEffect(() => {
    const stored = readStoredFilters(selectedProjectId)
    setStatusFilterState((stored.status as Status | null) ?? null)
    setPriorityFilterState((stored.priority as Priority | null) ?? null)
    setLabelFilterState(stored.label ?? null)
    setSprintFilterState(stored.sprint ?? null)
    setTeamFilterState(stored.team ?? null)
    setParentFilterState(stored.parent ?? null)
    setAssigneeFilterState(stored.assignee ?? null)
    try {
      const raw = stored.trackerStatuses
      setTrackerStatusFiltersState(raw ? JSON.parse(raw) : [])
    } catch {
      setTrackerStatusFiltersState([])
    }
    try {
      const raw = stored.issueTypes
      setIssueTypeFiltersState(raw ? JSON.parse(raw) : [])
    } catch {
      setIssueTypeFiltersState([])
    }
    setPinnedOnlyState(stored.pinnedOnly === '1')
    setActiveOnlyState(stored.activeOnly === '1')
  }, [selectedProjectId])

  const fetchTaskFacets = useCallback(async () => {
    try {
      const params = new URLSearchParams()
      if (selectedProjectId && selectedProjectId !== 'all') {
        params.append('projectId', selectedProjectId)
      }
      const res = await fetch(`${API_BASE}/tasks/facets?${params.toString()}`)
      if (!res.ok) return
      const data = await res.json()
      setTaskFacets({
        sprints: data?.sprints || [],
        teams: data?.teams || [],
        macros: data?.macros || [],
        noMacroCount: data?.noMacroCount || 0,
        assignees: data?.assignees || [],
        unassignedCount: data?.unassignedCount || 0,
        trackerStatuses: data?.trackerStatuses || [],
        statuses: data?.statuses || [],
        sources: data?.sources || [],
        labels: data?.labels || [],
        issueTypes: data?.issueTypes || [],
        total: data?.total || 0,
      })
    } catch {
      // A tracker that feeds neither field simply leaves the filters hidden.
    }
  }, [selectedProjectId])

  useEffect(() => {
    fetchTaskFacets()
  }, [fetchTaskFacets, tasks.length])

  const checkTrackerCredentials = useCallback(
    async (params: TrackerCredentials): Promise<TrackerCheck> => {
      try {
        const res = await fetch(`${API_BASE}/setup/tracker/check`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(params),
        })
        const data = await res.json().catch(() => ({}))
        if (!res.ok) return { ok: false, error: data.error || 'Vérification impossible' }
        return data
      } catch (err: any) {
        return { ok: false, error: err.message || 'Serveur injoignable' }
      }
    },
    []
  )

  const saveTrackerCredentials = useCallback(
    async (params: TrackerCredentials): Promise<boolean> => {
      try {
        const res = await fetch(`${API_BASE}/setup/tracker`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(params),
        })
        const data = await res.json().catch(() => ({}))
        if (!res.ok) throw new Error(data.error || 'Enregistrement refusé')
        setSettings(data)
        return true
      } catch (err: any) {
        addToast({ type: 'error', title: 'Accès non enregistrés', description: err.message })
        return false
      }
    },
    []
  )

  const fetchAutoSyncStatus = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/sync/auto`)
      if (!res.ok) return
      setAutoSync(await res.json())
    } catch {
      // Serveur injoignable : l'indicateur disparaît, ce qui est déjà le signal.
    }
  }, [])

  useEffect(() => {
    fetchAutoSyncStatus()
    // La boucle tourne côté serveur : l'interface se contente de regarder, à un
    // rythme qui n'a pas besoin d'être le sien.
    const timer = setInterval(fetchAutoSyncStatus, 30000)
    return () => clearInterval(timer)
  }, [fetchAutoSyncStatus])

  // Équipes du projet et personnes qu'elles portent. Le champ Équipe n'est pas
  // obligatoire côté tracker : une liste vide est une réponse normale, et les
  // vues concernées se contentent alors de ne rien proposer.
  const fetchTeams = useCallback(async () => {
    try {
      const params = new URLSearchParams({ members: '1' })
      if (selectedProjectId && selectedProjectId !== 'all') {
        params.append('projectId', selectedProjectId)
      }
      const res = await fetch(`${API_BASE}/teams?${params.toString()}`)
      if (!res.ok) {
        setTeams([])
        return
      }
      setTeams((await res.json()) || [])
    } catch {
      setTeams([])
    }
  }, [selectedProjectId])

  useEffect(() => {
    fetchTeams()
  }, [fetchTeams, tasks.length])

  const membersForTeam = useCallback(async (teamName: string): Promise<TeamMember[]> => {
    const name = (teamName || '').trim()
    if (!name) return []
    // Déjà chargée avec la liste des équipes : inutile de redemander au serveur.
    const known = teams.find(t => t.name === name)
    if (known?.members?.length) return known.members
    try {
      const res = await fetch(`${API_BASE}/teams/members?team=${encodeURIComponent(name)}`)
      if (!res.ok) return []
      return (await res.json()) || []
    } catch {
      return []
    }
  }, [teams])

  const refreshTeamMembers = useCallback(async (teamId: string): Promise<TrackerTeam | null> => {
    try {
      const res = await fetch(`${API_BASE}/teams/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ projectId: selectedProjectId === 'all' ? '' : selectedProjectId, teamId }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Lecture des membres refusée')
      addToast({
        type: 'success',
        title: `${data.name || 'Équipe'} rafraîchie`,
        description: `${data.memberCount || 0} personne(s) dans l'équipe`,
      })
      await fetchTeams()
      return data
    } catch (err: any) {
      addToast({ type: 'error', title: 'Membres non rafraîchis', description: err.message })
      return null
    }
  }, [selectedProjectId, fetchTeams])

  const searchTrackerTeams = useCallback(async (query: string): Promise<TrackerTeam[]> => {
    try {
      const params = new URLSearchParams({ q: query })
      if (selectedProjectId && selectedProjectId !== 'all') {
        params.append('projectId', selectedProjectId)
      }
      const res = await fetch(`${API_BASE}/teams/search?${params.toString()}`)
      if (!res.ok) return []
      return (await res.json()) || []
    } catch {
      return []
    }
  }, [selectedProjectId])

  const searchAssignableUsers = useCallback(async (taskId: string, query: string): Promise<TeamMember[]> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/assignable?q=${encodeURIComponent(query)}`)
      if (!res.ok) return []
      return (await res.json()) || []
    } catch {
      return []
    }
  }, [])

  const fetchTeamWorkload = useCallback(async (teamName: string): Promise<TeamWorkload | null> => {
    const name = (teamName || '').trim()
    if (!name) return null
    try {
      const params = new URLSearchParams({ team: name })
      if (selectedProjectId && selectedProjectId !== 'all') {
        params.append('projectId', selectedProjectId)
      }
      const res = await fetch(`${API_BASE}/teams/workload?${params.toString()}`)
      if (!res.ok) return null
      return await res.json()
    } catch {
      return null
    }
  }, [selectedProjectId])

  // Un filtre mémorisé peut ne plus exister : sprint clos, équipe renommée. Sans
  // ce garde-fou, le tableau paraîtrait vide avec un sélecteur qui n'affiche
  // rien de sélectionné.
  useEffect(() => {
    if (sprintFilter && taskFacets.sprints.length > 0 && !taskFacets.sprints.includes(sprintFilter)) {
      setSprintFilter(null)
    }
    if (teamFilter && taskFacets.teams.length > 0 && !taskFacets.teams.includes(teamFilter)) {
      setTeamFilter(null)
    }
    // Le filtre par personne était mémorisé sans être appliqué : une valeur
    // héritée de cette époque amputerait maintenant toutes les vues, dont la
    // roadmap qui n'affiche pas de barre de filtres. Un nom que le projet ne
    // porte plus est donc abandonné plutôt que gardé en silence.
    if (
      assigneeFilter &&
      assigneeFilter !== UNASSIGNED_FILTER_VALUE &&
      taskFacets.assignees.length > 0 &&
      !taskFacets.assignees.includes(assigneeFilter)
    ) {
      setAssigneeFilter(null)
    }
  }, [taskFacets, sprintFilter, teamFilter, assigneeFilter, setSprintFilter, setTeamFilter, setAssigneeFilter])

  // Initial load on mount
  useEffect(() => {
    fetchSettings()
    fetchSkills()
    fetchCliStatus()
    fetchProjects()
  }, [fetchSettings, fetchSkills, fetchCliStatus, fetchProjects])

  // Data reload on filter / project change
  useEffect(() => {
    fetchTasks()
    fetchActivities()
    fetchActivityStats()
  }, [fetchTasks, fetchActivities, fetchActivityStats])

  // Real-time SSE post-back and state update listener
  useEffect(() => {
    let eventSource: EventSource | null = null
    try {
      eventSource = new EventSource(`${API_BASE}/events`)
      eventSource.addEventListener('task_updated', (e: MessageEvent) => {
        try {
          const data = JSON.parse(e.data)
          if (data && data.task) {
            setSelectedTask(current => current && sameTask(current, data.task) ? data.task : current)
          }
          // Reapply the active project and all server-side filters, including
          // when an event introduces a new task or moves one out of this view.
          fetchTasks()

          fetchActivities()
          fetchActivityStats()
        } catch (err) {
          console.error('Error handling SSE task_updated event:', err)
        }
      })
    } catch (err) {
      console.error('Failed to initialize SSE EventSource:', err)
    }

    return () => {
      if (eventSource) {
        eventSource.close()
      }
    }
  }, [fetchTasks, fetchActivities, fetchActivityStats, selectedTask])

  // Active Job Count (queued or running)
  const activeJobCount = activities.filter(
    a => a.status === 'queued' || a.status === 'pending' || a.status === 'running'
  ).length

  // Smart background polling for queue execution & tasks
  useEffect(() => {
    const pollInterval = activeJobCount > 0 ? 3000 : 25000

    const interval = setInterval(async () => {
      try {
        const params = new URLSearchParams()
        if (selectedProjectId && selectedProjectId !== 'all') {
          params.append('projectId', selectedProjectId)
        }
        const [actRes, statsRes] = await Promise.all([
          fetch(`${API_BASE}/activities?${params.toString()}`),
          fetch(`${API_BASE}/activities/stats?${params.toString()}`),
        ])

        if (actRes.ok) {
          const newActivities: TaskActivity[] = await actRes.json()

          // Detect finished activities
          const prevMap = prevActiveActivitiesRef.current
          let needTaskRefresh = false

          newActivities.forEach(act => {
            const prevStatus = prevMap.get(act.id)
            if (prevStatus && (prevStatus === 'queued' || prevStatus === 'pending' || prevStatus === 'running')) {
              if (act.status === 'completed') {
                needTaskRefresh = true
                addToast({
                  type: 'success',
                  title: t.toasts.skillCompleted,
                  description: `${act.skillName} (${act.taskKey || 'Tâche'}) terminée avec succès !`,
                })
              } else if (act.status === 'failed') {
                needTaskRefresh = true
                addToast({
                  type: 'error',
                  title: t.toasts.error,
                  description: `Échec de ${act.skillName} (${act.taskKey || 'Tâche'})`,
                })
              }
            }
          })

          // Update tracking map
          const nextMap = new Map<string, string>()
          newActivities.forEach(a => nextMap.set(a.id, a.status))
          prevActiveActivitiesRef.current = nextMap

          // Only update activities state if changed
          setActivities(prev => {
            if (
              prev.length === newActivities.length &&
              prev.every(
                (a, i) =>
                  a.id === newActivities[i]?.id &&
                  a.status === newActivities[i]?.status &&
                  a.output?.length === newActivities[i]?.output?.length
              )
            ) {
              return prev
            }
            return newActivities
          })

          if (needTaskRefresh) {
            // Même requête que le chargement normal : sans les paramètres, ce
            // rafraîchissement remplaçait la liste par tout le board, tous
            // projets et tous filtres confondus.
            const taskRes = await fetch(`${API_BASE}/tasks?${buildTaskQuery()}`)
            if (taskRes.ok) {
              const freshTasks = await taskRes.json()
              setTasks(freshTasks)
              setSelectedTask(curr => {
                if (!curr) return null
                return freshTasks.find((t: Task) => t.id === curr.id) || curr
              })
            }
          }
        }

        if (statsRes.ok) {
          const newStats: ActivityStats = await statsRes.json()
          setActivityStats(prev => {
            if (
              prev.total === newStats.total &&
              prev.queued === newStats.queued &&
              prev.running === newStats.running &&
              prev.completed === newStats.completed &&
              prev.failed === newStats.failed &&
              prev.canceled === newStats.canceled
            ) {
              return prev
            }
            return newStats
          })
        }
      } catch (err) {
        console.warn('Queue polling error', err)
      }
    }, pollInterval)

    return () => clearInterval(interval)
  }, [activeJobCount, selectedProjectId, buildTaskQuery, t, addToast])

  const updateSettings = async (newSettings: Partial<UserSettings>, options?: { silent?: boolean }) => {
    const merged = { ...settings, ...newSettings }
    setSettings(merged)
    try {
      const res = await fetch(`${API_BASE}/settings`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(merged),
      })
      if (res.ok) {
        const saved = await res.json()
        setSettings(saved)
        if (!options?.silent) {
          addToast({
            type: 'success',
            title: t.toasts.settingsSaved,
          })
        }
        fetchCliStatus()
      }
    } catch (err) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: String(err),
      })
    }
  }

  const syncAll = async () => {
    setIsSyncing(true)
    try {
      const activeProj = selectedProjectId !== 'all' ? projects.find(p => p.id === selectedProjectId) : (projects.find(p => p.isDefault) || projects[0])
      const res = await fetch(`${API_BASE}/sync/all`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ projectId: activeProj?.id }),
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'Global sync failed')
      }
      const data = await res.json()
      if (data.activity) {
        setActivities(prev => [data.activity, ...prev.filter(a => a.id !== data.activity.id)])
      }
      fetchActivityStats()
      addToast({
        type: 'info',
        title: 'Synchronisation globale lancée',
        description: activeProj ? `Projet ${activeProj.name} — Suivi dans Activités.` : 'La tâche a été ajoutée à la file d\'attente.',
      })
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    } finally {
      setIsSyncing(false)
    }
  }

  const syncGithub = async (repo?: string) => {
    setIsSyncing(true)
    try {
      const activeProj = selectedProjectId !== 'all' ? projects.find(p => p.id === selectedProjectId) : (projects.find(p => p.isDefault) || projects[0])
      const targetRepo = repo || activeProj?.githubRepo || settings.githubRepo || ''
      const res = await fetch(`${API_BASE}/sync/github`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ repo: targetRepo, projectId: activeProj?.id }),
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'GitHub sync failed')
      }
      const data = await res.json()
      if (data.activity) {
        setActivities(prev => [data.activity, ...prev.filter(a => a.id !== data.activity.id)])
      }
      fetchActivityStats()
      addToast({
        type: 'info',
        title: 'Synchronisation GitHub lancée',
        description: targetRepo ? `Dépôt ${targetRepo} (${activeProj?.name || ''}) — Suivi dans Activités.` : 'Synchronisation GitHub en cours...',
      })
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    } finally {
      setIsSyncing(false)
    }
  }

  const syncJira = async (projectKey?: string) => {
    setIsSyncing(true)
    try {
      const activeProj = selectedProjectId !== 'all' ? projects.find(p => p.id === selectedProjectId) : (projects.find(p => p.isDefault) || projects[0])
      const targetKey = projectKey || activeProj?.jiraProject || settings.jiraProject || ''
      const res = await fetch(`${API_BASE}/sync/jira`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ projectKey: targetKey, projectId: activeProj?.id }),
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'Jira sync failed')
      }
      const data = await res.json()
      if (data.activity) {
        setActivities(prev => [data.activity, ...prev.filter(a => a.id !== data.activity.id)])
      }
      fetchActivityStats()
      addToast({
        type: 'info',
        title: 'Synchronisation Jira lancée',
        description: targetKey ? `Projet Jira ${targetKey} (${activeProj?.name || ''}) — Suivi dans Activités.` : 'Synchronisation Jira en cours...',
      })
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    } finally {
      setIsSyncing(false)
    }
  }

  const syncCurrentProject = async () => {
    const activeProj = currentProject || (projects.find(p => p.isDefault) || projects[0])
    const tracker = activeProj?.issueTracker || 'local'
    if (tracker === 'github') {
      await syncGithub(activeProj?.githubRepo)
    } else if (tracker === 'jira') {
      await syncJira(activeProj?.jiraProject)
    } else {
      await fetchTasks()
      addToast({
        type: 'info',
        title: 'Projet local à jour',
        description: `Tâches locales de ${activeProj?.name || 'ce projet'} rechargées depuis SQLite.`,
      })
    }
  }

  const createProject = async (data: Partial<Project>): Promise<Project | null> => {
    try {
      const res = await fetch(`${API_BASE}/projects`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'Failed to create project')
      }
      const created: Project = await res.json()
      await fetchProjects()
      setSelectedProjectId(created.id)
      addToast({
        type: 'success',
        title: 'Projet créé',
        description: `Le projet ${created.name} a été créé avec succès.`,
      })
      return created
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return null
    }
  }

  const updateProject = async (id: string, updates: Partial<Project>): Promise<Project | null> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'Failed to update project')
      }
      const updated: Project = await res.json()
      await fetchProjects()
      addToast({
        type: 'success',
        title: 'Projet mis à jour',
        description: `Le projet ${updated.name} a été actualisé.`,
      })
      return updated
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return null
    }
  }

  const deleteProject = async (id: string): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'Failed to delete project')
      }
      if (selectedProjectId === id) {
        setSelectedProjectId('all')
      }
      await fetchProjects()
      await fetchTasks()
      addToast({
        type: 'warning',
        title: 'Projet supprimé',
        description: 'Les tâches ont été réassignées au projet principal.',
      })
      return true
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return false
    }
  }

  const createTask = async (taskData: {
    title: string
    description?: string
    status?: Status
    priority?: Priority
    labels?: string[]
    assignee?: string
    dueDate?: string | null
    sprint?: string
    source?: TaskSource
    externalUrl?: string
    projectId?: string
    issueType?: string
  }): Promise<Task | null> => {
    try {
      const defaultProj = taskData.projectId || (selectedProjectId !== 'all' ? selectedProjectId : (projects[0]?.id || 'default'))
      const res = await fetch(`${API_BASE}/tasks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...taskData,
          projectId: defaultProj,
        }),
      })
      if (!res.ok) throw new Error('Creation failed')
      const created: Task = await res.json()
      setTasks(prev => [created, ...prev])
      fetchProjects()
      addToast({
        type: 'success',
        title: t.toasts.taskCreated,
        description: `${created.key}: ${created.title} (${(created.source || 'local').toUpperCase()})`,
      })
      return created
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return null
    }
  }

  const cloneTask = async (
    taskId: string,
    req?: CloneTaskRequest,
    openAfterClone: boolean = true
  ): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/clone`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: req ? JSON.stringify(req) : undefined,
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Erreur lors du clonage de la tâche')
      }
      const cloned: Task = await res.json()
      setTasks(prev => [cloned, ...prev])
      fetchProjects()
      addToast({
        type: 'success',
        title: 'Story clonée avec succès',
        description: `${cloned.key}: ${cloned.title} (${(cloned.source || 'local').toUpperCase()})`,
      })
      if (openAfterClone) {
        setSelectedTask(cloned)
      }
      return cloned
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return null
    }
  }

  const updateTask = async (id: string, updates: Partial<Task> & { assigneeAccountId?: string }): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      })
      if (!res.ok) throw new Error('Update failed')
      const updated: Task = await res.json()
      setTasks(prev => prev.map(t => (sameTask(t, updated) ? updated : t)))
      if (selectedTask && (sameTask(selectedTask, updated))) {
        setSelectedTask(updated)
      }
      addToast({
        type: 'success',
        title: t.toasts.taskUpdated,
        description: `${updated.key} ${t.toasts.taskUpdated.toLowerCase()}`,
      })
      return updated
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return null
    }
  }

  const syncSingleTask = async (id: string): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}/sync`, {
        method: 'POST',
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'Échec de la synchronisation')
      }
      const data = await res.json()
      const updated: Task = data.task
      if (updated) {
        setTasks(prev => prev.map(t => (sameTask(t, updated) ? updated : t)))
        if (selectedTask && (sameTask(selectedTask, updated))) {
          setSelectedTask(updated)
        }
        addToast({
          type: 'success',
          title: 'Synchronisation unitaire terminée',
          description: `${updated.key} réaligné avec le tracker distant`,
        })
      }
      return updated
    } catch (err: any) {
      addToast({
        type: 'error',
        title: 'Erreur de synchronisation',
        description: err.message,
      })
      return null
    }
  }

  // Déplacement par colonne de board : le statut local est écrit tout de suite,
  // donc la carte reste où elle a été lâchée, et la transition dans le tracker
  // part dans la file d'activités. Un refus du tracker apparaît alors comme une
  // activité en échec, et la synchronisation suivante remet la carte en place.
  const moveTaskToTrackerStatus = async (id: string, status: string): Promise<Task | null> => {
    const task = tasks.find(t => t.id === id)
    if (task) {
      const cleanSt = status.toLowerCase()
      let targetStage: WorkflowStage = 'new'
      const proj = projects.find(p => p.id === task.projectId) || currentProject

      // Resolve matching column name from tracker columns (e.g. "Code" status -> "In Progress" column)
      const matchingCol = proj?.trackerColumns?.find(
        c => c.name.toLowerCase() === cleanSt || (c.statuses && c.statuses.some(s => s.toLowerCase() === cleanSt))
      )
      const colName = matchingCol ? matchingCol.name.toLowerCase() : cleanSt

      let foundStage: WorkflowStage | null = null
      if (proj?.stageColumns) {
        for (const [stg, cols] of Object.entries(proj.stageColumns)) {
          if (cols.some(c => c.toLowerCase() === colName || c.toLowerCase() === cleanSt)) {
            foundStage = stg as WorkflowStage
            break
          }
        }
      }

      if (foundStage) {
        targetStage = foundStage
      } else if (cleanSt === 'closed' || cleanSt === 'done' || cleanSt === 'terminé' || cleanSt === 'finished') {
        targetStage = 'finished'
      } else if (cleanSt.includes('review') || cleanSt === 'to_close' || cleanSt.includes('pr')) {
        targetStage = 'reviewed'
      } else if (cleanSt.includes('test') || cleanSt === 'to_test' || cleanSt === 'to_validate' || cleanSt.includes('validate')) {
        targetStage = 'implemented'
      } else if (cleanSt.includes('progress') || cleanSt === 'to_implement' || cleanSt === 'in_progress' || cleanSt.includes('code') || cleanSt.includes('implement')) {
        targetStage = 'specified'
      } else if (cleanSt.includes('specify') || cleanSt.includes('spec')) {
        targetStage = 'clarified'
      } else if (cleanSt.includes('clarif') || cleanSt === 'to_clarify') {
        targetStage = 'new'
      }
      const targetLabel = `#${targetStage.replace(/^#+/, '')}`
      const cleanLabels = (task.labels || []).filter(
        l => !['untouched', 'new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished', 'closed'].includes(l.toLowerCase().replace(/^#+/, ''))
      )
      cleanLabels.push(targetLabel)
      setTasks(prev => prev.map(t => (t.id === id ? { ...t, trackerStatus: status, labels: cleanLabels } : t)))
    }

    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}/tracker-status`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(data.error || 'Transition refusée par le tracker')
      }
      const updated: Task | null = data.task || null
      if (updated) {
        setTasks(prev => prev.map(t => (t.id === updated.id ? updated : t)))
      }
      addToast({
        type: 'success',
        title: 'Transition en file',
        description: `${updated?.key || 'Ticket'} ➔ « ${status} ». Suivi dans les activités.`,
      })
      fetchActivities()
      return updated
    } catch (err: any) {
      fetchTasks()
      addToast({
        type: 'error',
        title: 'Déplacement impossible',
        description: err.message,
      })
      return null
    }
  }

  // Les commentaires vivent dans le tracker quand il y en a un : on les relit à
  // la demande plutôt que de les recopier en base, ce qui divergerait.
  // Les sessions PTY survivent à leurs spectateurs : la liste dit ce qui tourne
  // encore et permet d'y revenir.


  const getTaskComments = async (id: string): Promise<TaskComment[]> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}/comments`)
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Commentaires indisponibles')
      }
      return (await res.json()) || []
    } catch (err: any) {
      addToast({ type: 'error', title: 'Commentaires', description: err.message })
      return []
    }
  }

  const postTaskComment = async (id: string, body: string): Promise<TaskComment[] | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ body }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Publication refusée')
      }
      const comments: TaskComment[] = await res.json()
      addToast({ type: 'success', title: 'Commentaire publié' })
      return comments
    } catch (err: any) {
      addToast({ type: 'error', title: 'Commentaire non publié', description: err.message })
      return null
    }
  }

  const listProjectBoards = async (projectId: string): Promise<TrackerBoard[]> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/boards`)
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Boards indisponibles')
      }
      return (await res.json()) || []
    } catch (err: any) {
      addToast({ type: 'error', title: 'Boards du tracker', description: err.message })
      return []
    }
  }

  const importProjectBoardColumns = async (projectId: string, boardId: string): Promise<Project | null> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/board-columns`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ boardId }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Import des colonnes impossible')
      }
      const proj: Project = await res.json()
      setProjects(prev => prev.map(p => (p.id === proj.id ? proj : p)))
      addToast({
        type: 'success',
        title: 'Colonnes importées',
        description: `${proj.trackerColumns?.length || 0} colonnes reprises du board`,
      })
      return proj
    } catch (err: any) {
      addToast({ type: 'error', title: 'Import des colonnes', description: err.message })
      return null
    }
  }

  // Méta-macros : l'horizon est une décision produit, le cadrage et la TODO.
  const fetchProjectMacros = async (projectId: string): Promise<MacroMeta[]> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros`)
      if (!res.ok) {
        // Fallback to epics route if needed
        const fb = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/epics`)
        if (!fb.ok) return []
        return (await fb.json()) || []
      }
      return (await res.json()) || []
    } catch {
      return []
    }
  }
  const fetchProjectEpics = fetchProjectMacros

  const refineMacro = async (key: string, projectId?: string): Promise<RefineMacroResult | null> => {
    try {
      const targetProj = projectId || currentProject?.id || ''
      const url = targetProj
        ? `${API_BASE}/projects/${encodeURIComponent(targetProj)}/macros/${encodeURIComponent(key)}/refine`
        : `${API_BASE}/macros/${encodeURIComponent(key)}/refine`
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Erreur lors du raffinage de la macro')
      return {
        key: data.key || key,
        todos: data.todos || [],
        proposedTasks: data.proposedTasks || [],
        specFramework: data.specFramework || 'speckit',
      }
    } catch (err: any) {
      addToast({ type: 'error', title: 'Raffinage de macro échoué', description: err.message })
      return null
    }
  }

  const createBatchTasks = async (reqs: CreateTaskPayload[]): Promise<Task[]> => {
    if (!reqs || reqs.length === 0) return []
    try {
      const res = await fetch(`${API_BASE}/tasks/batch`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(reqs),
      })
      const data = await res.json().catch(() => ([]))
      if (!res.ok) throw new Error(data.error || 'Erreur lors de la création groupée de cartes')
      const created: Task[] = Array.isArray(data) ? data : []
      setTasks(prev => [...created, ...prev])
      addToast({
        type: 'success',
        title: 'Tickets créés',
        description: `${created.length} ticket(s) créé(s) avec succès.`,
      })
      return created
    } catch (err: any) {
      addToast({ type: 'error', title: 'Échec de création groupée', description: err.message })
      return []
    }
  }

  const saveMacroMeta = async (
    projectId: string,
    key: string,
    patch: { title?: string; horizon?: MacroHorizon | ''; description?: string; framingComment?: string; todos?: MacroTodo[]; closed?: boolean }
  ): Promise<MacroMeta | null> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key, ...patch }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Enregistrement refusé')
      const macro = data.macro || data.epic || data
      if (patch.title) {
        setTasks(prev => prev.map(t => (t.parentKey === key || t.parentTitle === key) ? { ...t, parentTitle: patch.title } : t))
      }
      return macro || null
    } catch (err: any) {
      addToast({ type: 'error', title: 'Macro non enregistrée', description: err.message })
      return null
    }
  }
  const saveEpicMeta = saveMacroMeta

  // Une ligne de TODO devient une story dans le tracker, sous sa macro.
  const createStoryFromMacroTodo = async (
    projectId: string,
    macroKey: string,
    todoId: string
  ): Promise<{ macro: MacroMeta | null; epic: MacroMeta | null; storyKey: string } | null> => {
    try {
      const res = await fetch(
        `${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/${encodeURIComponent(macroKey)}/story`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ todoId }),
        }
      )
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Création refusée')
      addToast({
        type: 'success',
        title: 'Story créée',
        description: `${data.storyKey} rattachée à ${macroKey}`,
      })
      fetchTasks()
      const m = data.macro || data.epic || null
      return { macro: m, epic: m, storyKey: data.storyKey || '' }
    } catch (err: any) {
      addToast({ type: 'error', title: 'Story non créée', description: err.message })
      return null
    }
  }
  const createStoryFromEpicTodo = createStoryFromMacroTodo

  // Rattrapage : les épics classés avant que le miroir en label existe, et ceux
  // dont la poussée a échoué, restent invisibles dans Jira jusqu'à ce qu'on les
  // pousse. C'est explicite, une édition de ticket par épic n'est pas anodine.
  // Rattacher un ticket existant à un épic, ou l'en détacher avec une clé vide.
  // Le changement d'équipe suit le même chemin que le reste des écritures : la
  // valeur locale part tout de suite, l'écriture Jira dans la file.
  const setTaskTeam = async (taskId: string, teamId: string, teamName?: string): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/team`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ teamId, teamName: teamName || '' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || "Changement d'équipe refusé")
      const updated: Task | null = data.task || null
      if (updated) {
        setTasks(prev => prev.map(t => (t.id === updated.id ? updated : t)))
        if (selectedTask && selectedTask.id === updated.id) setSelectedTask(updated)
      }
      addToast({
        type: 'success',
        title: teamId ? `Équipe ➔ ${teamName || teamId}` : 'Équipe retirée',
        description: 'Écriture Jira en file, suivi dans les activités.',
      })
      fetchActivities()
      return updated
    } catch (err: any) {
      addToast({ type: 'error', title: "Équipe non changée", description: err.message })
      return null
    }
  }

  // Le sprint appartient à l'API Agile du tracker, pas aux champs du ticket :
  // l'écriture passe donc par la file comme les autres, et la valeur locale part
  // tout de suite pour que la carte change de colonne sans attendre.
  const setTaskSprint = async (taskId: string, sprintId: string, sprintName?: string): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/sprint`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sprintId, sprintName: sprintName || '' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Changement de sprint refusé')
      const updated: Task | null = data.task || null
      if (updated) {
        setTasks(prev => prev.map(t => (t.id === updated.id ? updated : t)))
        if (selectedTask && selectedTask.id === updated.id) setSelectedTask(updated)
      }
      addToast({
        type: 'success',
        title: sprintId ? `Sprint ➔ ${sprintName || sprintId}` : 'Renvoyé au backlog',
        description: 'Écriture Jira en file, suivi dans les activités.',
      })
      fetchActivities()
      return updated
    } catch (err: any) {
      addToast({ type: 'error', title: 'Sprint non changé', description: err.message })
      return null
    }
  }

  const setTasksSprint = async (
    projectId: string,
    taskIds: string[],
    sprintId: string,
    sprintName?: string
  ): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/sprint-move`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ taskIds, sprintId, sprintName: sprintName || '' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Changement de sprint refusé')
      addToast({
        type: 'success',
        title: `${data.count || taskIds.length} ticket(s) ➔ ${sprintId ? sprintName || sprintId : 'backlog'}`,
        description: 'Écriture Jira en file, suivi dans les activités.',
      })
      fetchActivities()
      fetchTasks()
      return true
    } catch (err: any) {
      addToast({ type: 'error', title: 'Sprint non changé', description: err.message })
      return false
    }
  }

  const setTasksTeam = async (
    projectId: string,
    taskIds: string[],
    teamId: string,
    teamName?: string
  ): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/team-move`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ taskIds, teamId, teamName: teamName || '' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || "Changement d'équipe refusé")
      addToast({
        type: 'success',
        title: `${data.count || taskIds.length} ticket(s) ➔ ${teamId ? teamName || teamId : 'aucune équipe'}`,
        description: 'Écriture Jira en file, suivi dans les activités.',
      })
      fetchActivities()
      fetchTasks()
      return true
    } catch (err: any) {
      addToast({ type: 'error', title: 'Équipe non changée', description: err.message })
      return false
    }
  }

  // Le rattachement part dans la file d'activités : l'écriture tracker prend une à
  // deux secondes par ticket. Le tableau se rafraîchit tout seul quand l'activité se termine.
  const setTaskMacro = async (taskId: string, macroKey: string): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/macro`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ macroKey }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Rattachement refusé')
      const updated: Task | null = data.task || null
      if (updated) {
        setTasks(prev => prev.map(t => (t.id === updated.id ? updated : t)))
        if (selectedTask && selectedTask.id === updated.id) setSelectedTask(updated)
      }
      addToast({
        type: 'success',
        title: macroKey ? `Rattachement à ${macroKey} en file` : 'Détachement en file',
        description: 'Suivi dans les activités.',
      })
      fetchActivities()
      return updated
    } catch (err: any) {
      addToast({ type: 'error', title: 'Rattachement impossible', description: err.message })
      return null
    }
  }
  const setTaskEpic = setTaskMacro

  const createStoryUnderMacro = async (projectId: string, macroKey: string, title: string): Promise<string> => {
    try {
      const res = await fetch(
        `${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/${encodeURIComponent(macroKey)}/story`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ title }),
        }
      )
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Création refusée')
      addToast({ type: 'success', title: 'Story créée', description: `${data.storyKey} sous ${macroKey}` })
      fetchTasks()
      return data.storyKey || ''
    } catch (err: any) {
      addToast({ type: 'error', title: 'Story non créée', description: err.message })
      return ''
    }
  }
  const createStoryUnderEpic = createStoryUnderMacro

  const migrateMacro = async (
    sourceProjectId: string,
    macroKey: string,
    targetProjectId: string,
    migrateTasks: boolean
  ): Promise<{ success: boolean; macro?: MacroMeta; epic?: MacroMeta; migratedTasks?: number; error?: string }> => {
    try {
      const res = await fetch(
        `${API_BASE}/projects/${encodeURIComponent(sourceProjectId)}/macros/${encodeURIComponent(macroKey)}/migrate`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ targetProjectId, migrateTasks }),
        }
      )
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Échec de la migration de la macro')
      addToast({
        type: 'success',
        title: 'Macro migrée',
        description: `Macro ${macroKey} déplacée vers le projet cible (${data.migratedTasks || 0} tickets transférés)`
      })
      await Promise.all([fetchTasks(), fetchProjects()])
      const m = data.macro || data.epic
      return { success: true, macro: m, epic: m, migratedTasks: data.migratedTasks }
    } catch (err: any) {
      addToast({ type: 'error', title: 'Migration échouée', description: err.message })
      return { success: false, error: err.message }
    }
  }
  const migrateEpic = migrateMacro

  const migrateTasks = async (
    taskIds: string[],
    targetProjectId: string
  ): Promise<{ success: boolean; migratedCount?: number; error?: string }> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/migrate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ taskIds, targetProjectId }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Échec de la migration des tickets')
      addToast({
        type: 'success',
        title: 'Tickets migrés',
        description: `${data.migratedCount || 0} ticket(s) déplacé(s) vers le nouveau projet`
      })
      await Promise.all([fetchTasks(), fetchProjects()])
      return { success: true, migratedCount: data.migratedCount }
    } catch (err: any) {
      addToast({ type: 'error', title: 'Migration échouée', description: err.message })
      return { success: false, error: err.message }
    }
  }

  const fetchMacroRequiredFields = async (projectId: string): Promise<MacroRequiredField[]> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/fields`)
      if (!res.ok) return []
      const data = await res.json()
      if (!Array.isArray(data)) return []
      return data.filter(
        (f: unknown): f is MacroRequiredField =>
          Boolean(f) &&
          typeof (f as MacroRequiredField).id === 'string' &&
          typeof (f as MacroRequiredField).name === 'string' &&
          Array.isArray((f as MacroRequiredField).options)
      )
    } catch {
      return []
    }
  }
  const fetchEpicRequiredFields = fetchMacroRequiredFields

  const createMacro = async (
    projectId: string,
    title: string,
    horizon?: MacroHorizon | '',
    fields?: Record<string, string>
  ): Promise<MacroMeta | null> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, horizon: horizon || '', fields: fields || {} }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Création refusée')
      addToast({ type: 'success', title: `Macro ${data.key} créée` })
      return data
    } catch (err: any) {
      addToast({ type: 'error', title: 'Macro non créée', description: err.message })
      return null
    }
  }
  const createEpic = createMacro

  const deleteMacro = async (projectId: string, key: string): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Suppression échouée')
      addToast({ type: 'success', title: `Macro ${key} supprimée` })
      await fetchTasks()
      return true
    } catch (err: any) {
      addToast({ type: 'error', title: 'Macro non supprimée', description: err.message })
      return false
    }
  }
  const deleteEpic = deleteMacro

  // Le geste de coupe : un lot de tickets part vers une autre macro, créée à la
  // volée si on ne donne qu'un intitulé.
  const moveTasksToMacro = async (
    projectId: string,
    taskIds: string[],
    targetMacroKey: string,
    newMacroTitle?: string,
    fields?: Record<string, string>
  ): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/move`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ taskIds, targetEpicKey: targetMacroKey, targetMacroKey, newEpicTitle: newMacroTitle || '', newMacroTitle: newMacroTitle || '', fields: fields || {} }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Déplacement refusé')
      addToast({
        type: 'success',
        title: `Découpe de ${data.count || taskIds.length} ticket(s) en file`,
        description: targetMacroKey
          ? `Vers ${targetMacroKey}. Suivi dans les activités.`
          : 'Nouvelle macro créée pendant le traitement. Suivi dans les activités.',
      })
      fetchActivities()
      if (targetMacroKey) fetchTasks()
      return true
    } catch (err: any) {
      addToast({ type: 'error', title: 'Déplacement impossible', description: err.message })
      return false
    }
  }
  const moveTasksToEpic = moveTasksToMacro

  const transitionTaskStage = async (
    taskIdOrKey: string,
    stage: string,
    note?: string,
    prUrl?: string,
    branch?: string
  ): Promise<{ success: boolean; task?: Task; activity?: TaskActivity; error?: string }> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskIdOrKey)}/stage`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ stage, note: note || '', prUrl: prUrl || '', branch: branch || '' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Transition refusée')
      if (data.task) {
        setTasks(prev => prev.map(t => (sameTask(t, data.task) ? data.task : t)))
      }
      fetchActivities()
      addToast({
        type: 'success',
        title: `Étape #${stage} appliquée`,
        description: `Tâche ${data.task?.key || taskIdOrKey} passée à l'étape ${stage}.`,
      })
      return { success: true, task: data.task, activity: data.activity }
    } catch (err: any) {
      addToast({ type: 'error', title: 'Transition échouée', description: err.message })
      return { success: false, error: err.message }
    }
  }
  // l'étape de la tâche : l'interface ne fait qu'ouvrir le terminal quand le pas
  // est interactif.
  const advanceTask = async (taskId: string, auto?: boolean, mode?: SkillMode): Promise<{mode:string;skillId?:string;label?:string}|null> => {
    const task = tasks.find(task => task.id === taskId)
    if (!task) return null
    const project = projects.find(project => project.id === task.projectId)
    const skillId = auto ? 'pickup' : skillForStage(resolveTaskStage(task,project))
    if (!skillId) return null
    // Une chaîne complète est autonome par construction : elle force le mode au
    // lieu de laisser la précédence décider, sinon elle ouvrirait un terminal
    // que personne ne regarde. Le pas suivant lancé seul, lui, accepte la
    // surcharge ponctuelle.
    const activity = await runSkill(taskId,skillId,undefined,{mode:auto ? 'autonomous' : mode})
    return activity ? {mode:'remote',skillId} : null
  }

  const launchInteractiveStep = async (task: Task, skillId: string, _label: string): Promise<void> => {
    await runSkill(task.id,skillId)
  }

  // Clôture le pas interactif : c'est ici que le ticket bouge enfin (label
  // d'étape, statut, transition sur le tracker), le serveur faisant le même
  // travail que pour une skill autonome.
  const confirmInteractiveStep = async (note?: string): Promise<void> => {
    if (!pendingInteractive) return
    const { taskId, taskKey, skillId, label } = pendingInteractive
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/advance/confirm`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ skillId, note: note || '' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Confirmation refusée')
      setPendingInteractive(null)
      addToast({ type: 'success', title: `${label} confirmée`, description: `${taskKey} avance dans le workflow` })
      fetchTasks()
      fetchActivities()
    } catch (err: any) {
      addToast({ type: 'error', title: 'Confirmation impossible', description: err.message })
    }
  }

  // Ouvre la console d'une tâche : la tâche chargée quand elle est là, pour
  // l'étiquette et le worktree, sa session sinon.

  // Épingles. Elles vivent côté serveur : elles survivent au rechargement, et la
  // barre affiche un ticket même quand les filtres courants le cachent.
  const fetchPins = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/tasks/pins`)
      if (!res.ok) return
      setPinnedTasks((await res.json()) || [])
    } catch {
      // Serveur momentanément absent : la barre garde son dernier état.
    }
  }, [])

  useEffect(() => {
    fetchPins()
  }, [fetchPins])

  const isPinned = (taskId: string) => pinnedTasks.some(t => t.id === taskId)

  const togglePin = async (taskId: string) => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/pin`, { method: 'POST' })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Épinglage refusé')
      await fetchPins()
      // Le filtre « épinglés » lit la base : la liste doit suivre l'épingle qu'on
      // vient de poser ou de retirer.
      if (pinnedOnly) fetchTasks()
      await fetchTasks()
    } catch (err: any) {
      addToast({ type: 'error', title: 'Épinglage impossible', description: err.message })
    }
  }

  // Bascule à chaud : la console du ticket prend la main dans le panneau, et le
  // ticket devient celui sur lequel on travaille. C'est le geste qu'on répète
  // vingt fois par jour quand trois chantiers avancent en parallèle.
  const hotSwitch = (taskId: string) => {
    const task = pinnedTasks.find(task => task.id === taskId) || tasks.find(task => task.id === taskId)
    if (task) setSelectedTask(task)
  }

  const dismissInteractiveStep = () => setPendingInteractive(null)

  // Éditeur de skills. Le contenu vit en base, par projet, et le serveur
  // régénère les SKILL.md du dépôt à chaque enregistrement : l'éditeur est la
  // source, les fichiers sont le produit.
  const skillEditorBase = () => {
    const pid = currentProject?.id
    if (!pid) return ''
    return `${API_BASE}/projects/${encodeURIComponent(pid)}/skill-editor`
  }

  const fetchSkillEditor = async (): Promise<SkillEditorEntry[]> => {
    const base = skillEditorBase()
    if (!base) return []
    try {
      const res = await fetch(base)
      if (!res.ok) throw new Error('Lecture des skills impossible')
      return (await res.json()) || []
    } catch (err: any) {
      addToast({ type: 'error', title: 'Skills indisponibles', description: err.message })
      return []
    }
  }

  const skillEditorAction = async (
    path: string,
    init: RequestInit,
    successTitle: string
  ): Promise<SkillEditorEntry | null> => {
    const base = skillEditorBase()
    if (!base) return null
    try {
      const res = await fetch(`${base}${path}`, init)
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Action refusée')
      const entry = data as SkillEditorEntry
      addToast({
        type: 'success',
        title: successTitle,
        description: entry.paths?.length
          ? `${entry.paths.length} fichier(s) régénéré(s) dans le dépôt`
          : 'Enregistré en base, dépôt non accessible',
      })
      return entry
    } catch (err: any) {
      addToast({ type: 'error', title: 'Skill non enregistrée', description: err.message })
      return null
    }
  }

  const saveSkillContent = (skillId: string, content: string) =>
    skillEditorAction(
      `/${encodeURIComponent(skillId)}`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
      },
      'Skill enregistrée'
    )

  const saveSkillMode = (skillId: string, mode: SkillMode) =>
    skillEditorAction(
      `/${encodeURIComponent(skillId)}/mode`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode }),
      },
      'Mode d\'exécution enregistré'
    )

  const resetSkillContent = (skillId: string) =>
    skillEditorAction(`/${encodeURIComponent(skillId)}/reset`, { method: 'POST' }, 'Modèle intégré restauré')

  const importSkillFromRepo = (skillId: string) =>
    skillEditorAction(`/${encodeURIComponent(skillId)}/import`, { method: 'POST' }, 'Contenu du dépôt importé')

  const pendingHorizonPushes = async (projectId: string): Promise<MacroMeta[]> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/push-horizons`)
      if (!res.ok) {
        const fb = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/epics/push-horizons`)
        if (!fb.ok) return []
        return (await fb.json()) || []
      }
      return (await res.json()) || []
    } catch {
      return []
    }
  }

  const pushPendingHorizons = async (projectId: string): Promise<boolean> => {
    try {
      let res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/push-horizons`, { method: 'POST' })
      if (!res.ok) {
        res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/epics/push-horizons`, { method: 'POST' })
      }
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Poussée refusée')
      addToast({
        type: 'success',
        title: 'Poussée des horizons en file',
        description: 'Suivi dans les activités.',
      })
      fetchActivities()
      return true
    } catch (err: any) {
      addToast({ type: 'error', title: 'Poussée impossible', description: err.message })
      return false
    }
  }

  const fetchProjectIssueTypes = async (projectId: string): Promise<string[]> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/issue-types`)
      if (!res.ok) return []
      return (await res.json()) || []
    } catch {
      return []
    }
  }

  const fetchProjectTrackerStatuses = async (projectId: string): Promise<string[]> => {
    try {
      const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/tracker-statuses`)
      if (!res.ok) return []
      return (await res.json()) || []
    } catch {
      return []
    }
  }

  const convertTask = async (id: string, target: 'github'): Promise<Task | null> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}/convert`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target }),
      })
      if (!res.ok) {
        const errData = await res.json()
        throw new Error(errData.error || 'Conversion failed')
      }
      const updated: Task = await res.json()
      setTasks(prev => prev.map(t => (t.id === id ? updated : t)))
      if (selectedTask && selectedTask.id === id) {
        setSelectedTask(updated)
      }
      const activeProj = selectedProjectId !== 'all' ? projects.find(p => p.id === selectedProjectId) : projects[0]
      const repoLabel = activeProj?.githubRepo || settings.githubRepo || 'GitHub'
      addToast({
        type: 'success',
        title: 'Exporté vers GitHub',
        description: `${updated.key} (${repoLabel}) créé avec succès !`,
      })
      return updated
    } catch (err: any) {
      addToast({
        type: 'error',
        title: 'Erreur d\'export',
        description: err.message,
      })
      return null
    }
  }


  const moveTask = async (id: string, newStatus: Status, newPosition: number) => {
    const targetStage = stageForInternalStatus(newStatus)
    const targetLabel = `#${targetStage.replace(/^#+/, '')}`
    const existingTask = tasks.find(t => t.id === id)
    const cleanLabels = (existingTask?.labels || []).filter(
      l => !['untouched', 'new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished', 'closed'].includes(l.toLowerCase().replace(/^#+/, ''))
    )
    cleanLabels.push(targetLabel)

    setTasks(prev => {
      return prev.map(t => {
        if (t.id === id) {
          return { ...t, status: newStatus, position: newPosition, labels: cleanLabels }
        }
        return t
      })
    })

    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}/move`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus, position: newPosition }),
      })
      if (!res.ok) throw new Error('Move failed')
      const updated: Task = await res.json()
      setTasks(prev => prev.map(t => (t.id === id ? updated : t)))
      addToast({
        type: 'info',
        title: t.toasts.taskMoved,
        description: `${updated.key} ➔ ${t.status[newStatus]}`,
      })
    } catch (err: any) {
      fetchTasks()
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    }
  }

  const moveTaskWorkflowStage = async (taskId: string, targetStage: WorkflowStage): Promise<Task | null> => {
    const task = tasks.find(t => t.id === taskId || t.key === taskId)
    if (!task) return null

    const currentLabels = task.labels || []
    const targetLabel = `#${targetStage.replace(/^#+/, '')}`
    const cleanLabels = currentLabels.filter(
      l => !['untouched', 'new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished', 'closed'].includes(l.toLowerCase().replace(/^#+/, ''))
    )
    cleanLabels.push(targetLabel)

    // Stage and internal status are the same six-way split, so the fold needs no
    // per-project configuration; the server holds the same table.
    const proj = projects.find(p => p.id === task.projectId) || currentProject
    const mappedStatus: Status = INTERNAL_STATUS_BY_STAGE[targetStage] ?? task.status

    // Determine target tracker status if project has stageColumns mapping
    let mappedTrackerStatus = task.trackerStatus
    if (proj?.stageColumns && proj.stageColumns[targetStage]?.length) {
      const colName = proj.stageColumns[targetStage][0]
      const col = proj.trackerColumns?.find(c => c.name === colName)
      if (col?.statuses?.length) {
        mappedTrackerStatus = col.statuses[0]
      } else if (colName) {
        mappedTrackerStatus = colName
      }
    }

    const updated = await updateTask(task.id, {
      labels: cleanLabels,
      status: mappedStatus,
      ...(mappedTrackerStatus ? { trackerStatus: mappedTrackerStatus } : {}),
    })

    if (updated) {
      addToast({
        type: 'info',
        title: 'Étape Workflow Agentic mise à jour',
        description: `${updated.key} ➔ ${targetLabel}`,
      })
    }
    return updated
  }

  const runSkill = async (
    taskId: string,
    skillId: string,
    prompt?: string,
    opts?: { withComments?: boolean; mode?: SkillMode }
  ): Promise<TaskActivity | null> => {
    setIsSkillRunning(true)
    setRunningSkillId(skillId)
    addToast({
      type: 'info',
      title: t.toasts.skillQueued,
      description: `Moteur: ${settings.aiProvider.toUpperCase()} (${skillId}) - Poussée en file d'attente`,
    })

    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(taskId)}/run-skill`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        // `mode` absent veut dire « pas de surcharge » : la précédence retombe
        // sur la skill puis sur le projet. Ce n'est pas « interactif ».
        body: JSON.stringify({ skillId, prompt, withComments: opts?.withComments, mode: opts?.mode || undefined }),
      })
      if (!res.ok) {
        const errorData = await res.json()
        throw new Error(errorData.error || 'Skill execution failed')
      }

      const data = await res.json()
      const updatedTask: Task = data.task
      const activity: TaskActivity = data.activity

      setTasks(prev => prev.map(t => (t.id === taskId ? updatedTask : t)))
      if (selectedTask && selectedTask.id === taskId) {
        setSelectedTask(updatedTask)
      }
      setActivities(prev => [activity, ...prev.filter(a => a.id !== activity.id)])
      await fetchActivityStats()

      return activity
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return null
    } finally {
      setIsSkillRunning(false)
      setRunningSkillId(null)
    }
  }

  const retryActivity = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}/activities/${id}/retry`, { method: 'POST' })
      if (!res.ok) throw new Error('Retry failed')
      addToast({
        type: 'info',
        title: t.toasts.activityRetried,
      })
      await fetchActivities()
      await fetchActivityStats()
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    }
  }

  const cancelActivity = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}/activities/${id}/cancel`, { method: 'POST' })
      if (!res.ok) throw new Error('Cancel failed')
      addToast({
        type: 'warning',
        title: t.toasts.activityCanceled,
      })
      await fetchActivities()
      await fetchActivityStats()
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    }
  }

  const deleteActivity = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}/activities/${id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error('Delete failed')
      setActivities(prev => prev.filter(a => a.id !== id))
      if (selectedActivity && selectedActivity.id === id) {
        setSelectedActivity(null)
      }
      addToast({
        type: 'warning',
        title: t.toasts.activityDeleted,
      })
      await fetchActivityStats()
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    }
  }

  const clearCompletedActivities = async () => {
    try {
      const res = await fetch(`${API_BASE}/activities`, { method: 'DELETE' })
      if (!res.ok) throw new Error('Clear failed')
      addToast({
        type: 'info',
        title: t.toasts.activitiesCleared,
      })
      await fetchActivities()
      await fetchActivityStats()
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
    }
  }

  const deleteTask = async (id: string): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/tasks/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (!res.ok) throw new Error('Delete failed')
      setTasks(prev => prev.filter(t => t.id !== id))
      if (selectedTask && selectedTask.id === id) {
        setSelectedTask(null)
      }
      addToast({
        type: 'warning',
        title: t.toasts.taskDeleted,
      })
      return true
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.toasts.error,
        description: err.message,
      })
      return false
    }
  }

  // Filter tasks by active source filter (all / github / jira / local)
  // then by the active parent (epic or parent story), when one is selected.
  const filteredTasks = React.useMemo(() => {
    let out = sourceFilter === 'all'
      ? tasksInProject(tasks, selectedProjectId)
      : tasksInProject(tasks, selectedProjectId).filter(t => (t.source || 'local') === sourceFilter)
    if (parentFilter) {
      if (parentFilter === '__no_macro__' || parentFilter === 'none') {
        out = out.filter(t => !t.parentKey && !t.parentTitle)
      } else {
        out = out.filter(t => t.parentKey === parentFilter || t.parentTitle === parentFilter)
      }
    }
    return out
  }, [tasks, sourceFilter, parentFilter, selectedProjectId])

  // A project can rename any workflow skill through `skillOverrides`
  // (skillId -> custom label). Every place that shows a skill name goes through
  // this resolver, otherwise the setting would be write-only.
  const resolveSkillOverride = useCallback(
    (skillId: string, projectId?: string): string => {
      const proj = projectId
        ? projects.find(p => p.id === projectId) || currentProject
        : currentProject
      return (proj?.skillOverrides?.[skillId] || '').trim()
    },
    [projects, currentProject]
  )

  const skillLabel = useCallback(
    (skillId: string, fallback?: string, projectId?: string): string => {
      const override = resolveSkillOverride(skillId, projectId)
      // An override written as a command ("/clarify-workitem") reads badly as a
      // label, so strip the slash for display purposes.
      if (override) return override.replace(/^\//, '')
      if (fallback && fallback.trim() !== '') return fallback
      const known = skills.find(s => s.id === skillId)
      return known?.name || skillId
    },
    [resolveSkillOverride, skills]
  )

  const skillCommand = useCallback(
    (skillId: string, fallback: string, projectId?: string): string => {
      const override = resolveSkillOverride(skillId, projectId)
      if (!override) return fallback
      // Accept both "clarify-workitem" and "/clarify-workitem".
      return '/' + override.replace(/^\//, '')
    },
    [resolveSkillOverride]
  )

  // Distinct parents across the loaded tasks, ordered by how much work hangs
  // under each one. Drives the sidebar "Epics / Parents" filter.
  const availableParents = React.useMemo(() => {
    const byKey = new Map<string, { key: string; title: string; type: string; count: number }>()
    for (const t of tasks) {
      if (!t.parentKey) continue
      const existing = byKey.get(t.parentKey)
      if (existing) {
        existing.count += 1
        if (!existing.title && t.parentTitle) existing.title = t.parentTitle
      } else {
        byKey.set(t.parentKey, {
          key: t.parentKey,
          title: t.parentTitle || '',
          type: t.parentType || '',
          count: 1,
        })
      }
    }
    return Array.from(byKey.values()).sort(
      (a, b) => b.count - a.count || a.key.localeCompare(b.key)
    )
  }, [tasks])

  // Les labels viennent des facettes, donc du projet et non de la liste filtrée :
  // sinon choisir un label faisait disparaître tous les autres, celui-là compris.
  // Repli sur la liste affichée quand les facettes ne les portent pas encore.
  const availableLabels = useMemo(() => {
    if (taskFacets.labels.length > 0) return taskFacets.labels.map(l => l.value)
    return Array.from(new Set(tasks.flatMap(t => t.labels || []).filter(Boolean)))
  }, [taskFacets.labels, tasks])

  // Les personnes proposées viennent de deux sources : celles présentes sur les
  // tickets du projet, et les membres des équipes portées par ces tickets. La
  // seconde est ce qui permet de filtrer sur quelqu'un qui n'a encore rien.
  // Quand une équipe est sélectionnée, seule cette équipe compte.
  const availableAssignees = useMemo(() => {
    const names = new Set<string>()
    const scopedTeams = teamFilter ? teams.filter(tm => tm.name === teamFilter) : teams
    scopedTeams.forEach(tm => {
      (tm.members || []).forEach(m => {
        if (m.displayName) names.add(m.displayName)
      })
    })
    if (!teamFilter) {
      taskFacets.assignees.forEach(name => names.add(name))
    } else {
      tasks.forEach(t => {
        if (t.team === teamFilter && t.assignee) names.add(t.assignee)
      })
    }
    return Array.from(names).sort((a, b) => a.localeCompare(b))
  }, [teams, teamFilter, taskFacets.assignees, tasks])


  const startBatchPickup = async (taskIds: string[]): Promise<void> => {
    const batch = tasks.filter(task => taskIds.includes(task.id))
    if (!batch.length) return
    if (batch.some(task => task.projectId !== batch[0].projectId)) {
      addToast({type:'error',title:'Select tasks from one project for a batch'})
      return
    }
    await runSkill(batch[0].id,'pickup_issues','/pickup-issues '+batch.map(task=>task.id).join(' '))
  }

  // Global Keyboard Shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setIsCommandPaletteOpen(prev => !prev)
        return
      }

      const activeTag = (document.activeElement?.tagName || '').toLowerCase()
      const isXterm = Boolean(document.activeElement?.closest('.xterm') || document.activeElement?.classList.contains('xterm-helper-textarea'))
      const isInputActive = activeTag === 'input' || activeTag === 'textarea' || activeTag === 'select' || isXterm

      if (e.key === '/' && !isInputActive) {
        e.preventDefault()
        const searchInput = document.getElementById('global-search-input') as HTMLInputElement
        if (searchInput) {
          searchInput.focus()
          searchInput.select()
        }
        return
      }

      if ((e.key === 'n' || e.key === 'N' || e.key === 'c' || e.key === 'C') && !isInputActive && !e.metaKey && !e.ctrlKey) {
        e.preventDefault()
        setIsQuickAddOpen(true)
        return
      }

      if (!isInputActive && !e.metaKey && !e.ctrlKey && !e.altKey) {
        const key = e.key.toLowerCase()
        if (key === 'b') {
          e.preventDefault()
          setActiveView('board')
          return
        }
        if (key === 'l') {
          e.preventDefault()
          setActiveView('list')
          return
        }
        if (key === 'a') {
          e.preventDefault()
          setActiveView('activities')
          return
        }
        if (key === 's' && !e.shiftKey) {
          e.preventDefault()
          setActiveView('sync')
          return
        }

      }

      if (e.key === 'Escape') {
        if (isCommandPaletteOpen) {
          setIsCommandPaletteOpen(false)
        } else if (isQuickAddOpen) {
          setIsQuickAddOpen(false)
        } else if (selectedTask) {
          setSelectedTask(null)
        } else if (selectedActivity) {
          setSelectedActivity(null)
        } else if (isProfileOpen) {
          setIsProfileOpen(false)
        } else if (searchQuery) {
          setSearchQuery('')
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isCommandPaletteOpen, isQuickAddOpen, selectedTask, selectedActivity, isProfileOpen, searchQuery, setActiveView])

  return (
    <AppContext.Provider
      value={{
        projects,
        selectedProjectId,
        setSelectedProjectId,
        currentProject,
        createProject,
        updateProject,
        deleteProject,
        fetchProjects,
        isProjectModalOpen,
        setIsProjectModalOpen,
        editingProject,
        setEditingProject,
        tasks: filteredTasks,
        skills,
        cliStatuses,

        isFetchingGitStatus,


        isLoading,
        isSkillRunning,
        isSyncing,
        runningSkillId,
        error,
        activeView,
        setActiveView,
        boardGrouping,
        setBoardGrouping,
        boardCardDisplayMode,
        setBoardCardDisplayMode,
        toggleBoardCardDisplayMode,
        moveTaskWorkflowStage,
        searchQuery,
        setSearchQuery,
        statusFilter,
        setStatusFilter,
        priorityFilter,
        setPriorityFilter,
        labelFilter,
        taskFacets,
        pinnedOnly,
        setPinnedOnly,
        activeOnly,
        setActiveOnly,
        activeTasks,
        sprintFilter,
        setSprintFilter,
        teamFilter,
        setTeamFilter,
        setLabelFilter,
        assigneeFilter,
        setAssigneeFilter,
        trackerStatusFilters,
        setTrackerStatusFilters,
        issueTypeFilters,
        setIssueTypeFilters,
        autoSync,
        isTrackerSetupOpen,
        setIsTrackerSetupOpen,
        checkTrackerCredentials,
        saveTrackerCredentials,
        sourceFilter,
        setSourceFilter,
        parentFilter,
        setParentFilter,
        macroFilter: parentFilter,
        setMacroFilter: setParentFilter,
        availableParents,
        skillLabel,
        skillCommand,


        sidebarCollapsed,
        setSidebarCollapsed,
        selectedTask,
        setSelectedTask,


        hideDone,
        setHideDone,
        toggleHideDone,
        isQuickAddOpen,
        setIsQuickAddOpen,
        quickAddInitialStatus,
        setQuickAddInitialStatus,
        isCloneModalOpen,
        setIsCloneModalOpen,
        cloneSourceTask,
        setCloneSourceTask,
        openCloneModal,
        isCommandPaletteOpen,
        setIsCommandPaletteOpen,
        isProfileOpen,
        setIsProfileOpen,
        settings,
        updateSettings,
        t,
        toasts,
        addToast,
        removeToast,
        createTask,
        cloneTask,
        updateTask,
        moveTaskToTrackerStatus,
        getTaskComments,


        postTaskComment,
        listProjectBoards,
        importProjectBoardColumns,
        fetchProjectTrackerStatuses,
        fetchProjectIssueTypes,
        fetchProjectMacros,
        fetchProjectEpics,
        refineMacro,
        createBatchTasks,

        saveMacroMeta,
        saveEpicMeta,
        createStoryFromMacroTodo,
        createStoryFromEpicTodo,
        pendingHorizonPushes,
        pushPendingHorizons,
        setTaskMacro,
        setTaskEpic,
        createStoryUnderMacro,
        createStoryUnderEpic,
        createMacro,
        createEpic,
        deleteMacro,
        deleteEpic,
        migrateMacro,
        migrateEpic,
        migrateTasks,
        fetchMacroRequiredFields,
        fetchEpicRequiredFields,
        moveTasksToMacro,
        moveTasksToEpic,
        transitionTaskStage,
        advanceTask,
        pendingInteractive,
        pinnedTasks,
        isPinned,
        togglePin,
        hotSwitch,


        fetchSkillEditor,
        saveSkillContent,
        resetSkillContent,
        saveSkillMode,
        importSkillFromRepo,
        launchInteractiveStep,
        confirmInteractiveStep,
        dismissInteractiveStep,
        convertTask,
        moveTask,
        deleteTask,
        runSkill,
        syncAll,
        syncGithub,
        syncJira,
        syncCurrentProject,
        syncSingleTask,
        fetchCliStatus,
        refreshTasks: fetchTasks,
        activities,
        activityStats,
        selectedActivity,
        setSelectedActivity,
        activeJobCount,
        fetchActivities,
        fetchActivityStats,
        retryActivity,
        cancelActivity,
        deleteActivity,
        clearCompletedActivities,
        availableLabels,
        availableAssignees,
        unassignedFilterValue: UNASSIGNED_FILTER_VALUE,
        teams,
        fetchTeams,
        membersForTeam,
        refreshTeamMembers,
        fetchTeamWorkload,
        searchTrackerTeams,
        setTaskTeam,
        searchAssignableUsers,
        setTaskSprint,
        setTasksSprint,
        setTasksTeam,


        startBatchPickup,
      }}
    >
      {children}
    </AppContext.Provider>
  )
}

export const useApp = () => {
  const context = useContext(AppContext)
  if (!context) {
    throw new Error('useApp must be used within an AppProvider')
  }
  return context
}
