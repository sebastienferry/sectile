import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Target,
  Layers,
  Goal,
  CalendarRange,
  Route,
  Compass,
  HelpCircle,
  EyeOff,
  Eye,
  CalendarDays,
  ExternalLink,
  Plus,
  Check,
  Trash2,
  AlertTriangle,
  Save,
  X,
  Filter,
  Scissors,
  Search,
  Loader2,
  FileCode,
  Pencil,
  ArrowRightLeft,
  Sparkles,
  ListChecks,
  ListFilter,
  FolderGit2,
  Maximize2,
  Minimize2,
  ChevronDown,
  ChevronRight,
  MessageSquare,
  Tag,
  RefreshCw,
  Rows3,
  Copy,
  PanelRightClose,
  PanelRightOpen,
  Lock,
} from 'lucide-react'
import type { RefineMacroResult } from '../types'
import { useApp } from '../context/AppContext'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { LookupField } from './LookupField'
import { MarkdownEditor } from './Markdown'
import { EpicBar, useEpicColors } from './EpicMarker'
import { MacroLabelGroups } from './MacroLabelGroups'
import { EpicLabelFilter } from './EpicLabelFilter'
import { EpicOriginFilter } from './EpicOriginFilter'
import { EpicLabelEditor } from './EpicLabelEditor'
import { MacroTaskRow } from './MacroTaskRow'
import { sprintLookup, isProjectCompatible, targetProjectOptions } from '../lib/lookups'
import { format, plural } from '../lib/i18n'
import {
  buildMacroRows,
  placementIssues,
  placementOf,
  matchesMacroSearch,
  epicLabelInventory,
  freeEpicLabels,
  matchesEpicLabels,
  pruneSelectedLabels,
  HORIZON_META,
  MATURITY_META,
  PLACEMENT_META,
  PRIORITY_META,
  READINESS_META,
  type MacroRow,
  type Horizon,
  type HorizonTab,
  tasksBySprintOrder,
  sprintLabelOf,
  macroCopyPayload,
  pruneTodoSelection,
  selectableTodoIds,
  batchSummary,
  todoOrigin,
} from '../lib/roadmap'
import {
  CONDENSED_HORIZONS,
  HORIZON_SHORT,
  isRoadmapRowCondensed,
  loadRoadmapRowDisplayMode,
  saveRoadmapRowDisplayMode,
  toggleRoadmapRowDisplayMode,
  type RoadmapRowDisplayMode,
} from '../lib/roadmapDisplayMode'
import {
  ROADMAP_DESCRIPTION_OPEN_STORAGE_KEY,
  ROADMAP_FRAMING_OPEN_STORAGE_KEY,
  ROADMAP_PANEL_EXPANDED_STORAGE_KEY,
  ROADMAP_PANEL_HIDDEN_STORAGE_KEY,
  loadRoadmapFlag,
  loadRoadmapSelectedKey,
  loadRoadmapTab,
  saveRoadmapFlag,
  saveRoadmapSelectedKey,
  saveRoadmapTab,
  loadRoadmapGroupAxis,
  saveRoadmapGroupAxis,
  loadRoadmapFoldedSections,
  saveRoadmapFoldedSections,
} from '../lib/roadmapViewPrefs'
import {
  DRAG_EPIC_KEYS,
  groupEpics,
  isGroupableTab,
  parseDraggedEpicKeys,
  planAxisDrop,
  type EpicGroupAxis,
  type EpicSection,
} from '../lib/epicGrouping'
import {
  isSelectionClick,
  pruneSelection,
  rangeSelection,
  shouldEscapeClearSelection,
  toggleSelected,
} from '../lib/boardSelection'
import { locateEpic } from '../lib/roadmapFocus'
import {
  EPIC_PRIORITIES,
  EPIC_PRIORITY_LEVEL,
  EPIC_READINESS,
  epicPriorityLabel,
  matchesPriority,
  normalizeQuarter,
  seedProposals,
  sortByPriority,
  type PriorityFilter,
  type PrioritySort,
  type SeedLine,
} from '../lib/epicAxes'
import {
  TRACKER_TARGET_PREFIX,
  applyTargetPickerValue,
  isDefaultOriginSelection,
  loadOriginSelection,
  macroOrigin,
  matchesOrigins,
  normalizeOriginSelection,
  offeredOrigins,
  roadmapTargetOptions,
  rowOrigin,
  saveOriginSelection,
  selectionRevealing,
  targetPickerValue,
} from '../lib/roadmapOrigins'
import type { EpicPriority, EpicReadiness, MacroHorizon, MacroMeta, MacroStoryBatch, MacroTodo, MacroTodoSource } from '../types'
import { MacroRealignButton } from './MacroRealignButton'

/**
 * Macro roadmap, after the "Roadmap Epics.dc.html" design.
 *
 * Two jobs, not one. NOW and NEXT are operational: they check that a macro's
 * stories sit in a sprint (active for NOW, upcoming for NEXT), and whatever
 * does not must stand out. LATER is macro framing: description and TODO are
 * worked on before there are stories.
 *
 * Display text comes from `t.planning.roadmap`; horizon labels are product
 * vocabulary and read the same in both languages.
 */

/** An epic's free label on its row (#626), styled like the squad chip. */
const EPIC_LABEL_BADGE =
  'text-[9.5px] px-1 rounded font-mono bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)]'
/** How many free labels the condensed row shows before counting the rest. */
const CONDENSED_LABELS = 2

/**
 * The tabs. Horizon tabs show the horizon label as is; the two others take
 * their label from the catalog (`t.planning.roadmap.tabs`).
 */
const TABS: { id: HorizonTab; label?: string; icon: React.ReactNode }[] = [
  { id: 'now', label: 'NOW', icon: <Target size={14} /> },
  { id: 'next', label: 'NEXT', icon: <Route size={14} /> },
  { id: 'later', label: 'LATER', icon: <Compass size={14} /> },
  { id: 'unclassified', icon: <HelpCircle size={14} /> },
  { id: 'hidden', icon: <EyeOff size={14} /> },
]

/**
 * An on or off state of the view, kept across visits (see roadmapViewPrefs).
 * The setter takes a value or an updater, like the one of useState.
 */
function usePersistedFlag(key: string, fallback: boolean) {
  const [value, setValue] = useState(() => loadRoadmapFlag(key, fallback))
  const set = useCallback(
    (next: boolean | ((prev: boolean) => boolean)) => {
      setValue(prev => {
        const resolved = typeof next === 'function' ? next(prev) : next
        saveRoadmapFlag(key, resolved)
        return resolved
      })
    },
    [key]
  )
  return [value, set] as const
}

export const RoadmapView: React.FC = () => {
  const {
    tasks,
    projects,
    currentProject,
    settings,
    setSelectedTask,
    fetchProjectMacros,
    saveMacroMeta,
    createStoryFromMacroTodo,
    createStoriesFromMacroTodos,
    produceMacroSlicing,
    setTaskMacro,
    createStoryUnderMacro,
    createMacro,
    deleteMacro,
    moveTasksToMacro,
    setTaskSprint,
    setTasksSprint,
    assigneeFilter,
    setAssigneeFilter,
    myTasksOnly,
    setMyTasksOnly,
    sprintFilter,
    setSprintFilter,
    teamFilter,
    setTeamFilter,
    labelFilter,
    setLabelFilter,
    pinnedOnly,
    setPinnedOnly,
    searchQuery,
    setSearchQuery,
    activeJobCount,
    addToast,
    migrateMacro,
    refineMacro,
    pendingHorizonPushes,
    pushPendingHorizons,
    importMacroHorizons,
    createBatchTasks,
    isLoading,
    setActiveView,
    roadmapFocus,
    consumeRoadmapFocus,
    openEpicTickets,
    selectedTask,
    selectedActivity,
    isQuickAddOpen,
    isCommandPaletteOpen,
    isProfileOpen,
    t,
  } = useApp()
  const strings = t.planning.roadmap
  const language = settings.language
  // Les macros sont celles du projet affiché : c'est son réglage qui compte.
  const epicColorsOn = useEpicColors()()

  // The tab and the selected macro survive a change of view (see
  // roadmapViewPrefs). The tab is saved by its setter, whoever calls it: a
  // click, the search going where it finds, the creation of a macro. Keeping
  // only the click would make the memory unpredictable.
  const [tab, setTabState] = useState<HorizonTab>(() => loadRoadmapTab())
  const setTab = useCallback((next: HorizonTab) => {
    setTabState(next)
    saveRoadmapTab(next)
  }, [])
  const [displayMode, setDisplayMode] = useState<'framing' | 'execution' | 'phases' | 'goals'>('execution')
  const [macroMeta, setMacroMeta] = useState<MacroMeta[]>([])
  // The selected macro is kept per project. The choice is held with its
  // project, and switching project swaps in the other project's memory during
  // the render, once, instead of from an effect. Only a choice writes it: the
  // fallback on the first visible macro does not, so a remembered macro that is
  // filtered out for a while is selected again once it shows.
  const projectId = currentProject?.id || ''
  const [selection, setSelection] = useState(() => ({ projectId, key: loadRoadmapSelectedKey(projectId) }))
  let currentSelection = selection
  if (selection.projectId !== projectId) {
    currentSelection = { projectId, key: loadRoadmapSelectedKey(projectId) }
    setSelection(currentSelection)
  }
  const selectedKey = currentSelection.key
  const setSelectedKey = useCallback(
    (key: string | null) => {
      setSelection({ projectId, key })
      saveRoadmapSelectedKey(projectId, key)
    },
    [projectId]
  )
  const [onlyIssues, setOnlyIssues] = useState(false)
  const [showClosed, setShowClosed] = useState(false)
  // The epic priority filter and sort (#627). Not remembered: the view opens in
  // backlog order with every priority, as it always did.
  const [priorityFilter, setPriorityFilter] = useState<PriorityFilter>(null)
  const [prioritySort, setPrioritySort] = useState<PrioritySort>('backlog')
  // The grouping of the tabs (#628) and its folded sections are reading
  // settings, kept per browser like the tab.
  const [groupAxis, setGroupAxisState] = useState<EpicGroupAxis>(() => loadRoadmapGroupAxis())
  const setGroupAxis = useCallback((next: EpicGroupAxis) => {
    setGroupAxisState(next)
    saveRoadmapGroupAxis(next)
  }, [])
  const [foldedSections, setFoldedSections] = useState<ReadonlySet<string>>(() => new Set(loadRoadmapFoldedSections()))
  const toggleSection = useCallback((id: string) => {
    setFoldedSections(prev => {
      const next = toggleSelected(prev, id)
      saveRoadmapFoldedSections(next)
      return next
    })
  }, [])
  // The epics picked with Ctrl, Cmd or Shift click, carried together by a drag.
  const [pickedKeys, setPickedKeys] = useState<ReadonlySet<string>>(() => new Set())
  const [pickAnchor, setPickAnchor] = useState<string | null>(null)
  const [dropSection, setDropSection] = useState<string | null>(null)
  const [isDropping, setIsDropping] = useState(false)
  // The seeding preview: the proposals, and which of their values are kept.
  const [seedLines, setSeedLines] = useState<SeedLine[] | null>(null)
  const [seedKept, setSeedKept] = useState<Record<string, boolean>>({})
  const [isSeeding, setIsSeeding] = useState(false)
  // The quarter field of the panel, validated on Enter or blur. The edit is
  // tied to the epic and value it started from, so selecting another epic or
  // saving drops it without an effect.
  const [quarterEdit, setQuarterEdit] = useState<{ origin: string; value: string; error: string } | null>(null)
  // The epic labels picked in the toolbar filter (#626). Not remembered between
  // visits: a label filter kept without the user knowing is what makes a
  // roadmap look empty.
  const [selectedLabels, setSelectedLabels] = useState<string[]>([])
  // The Jira projects whose epics the roadmap shows (#632). Remembered per
  // project, unlike the filters above: a declared project can hold hundreds of
  // epics, and choosing which to read is a way of reading the roadmap, like its
  // tab. A choice made here holds for its project whatever the storage accepts;
  // another project reads its own, and the own key alone when none is stored.
  const originProjectId = currentProject?.id || ''
  const [chosenOrigins, setChosenOrigins] = useState<{ projectId: string; keys: string[] } | null>(null)
  const storedOrigins = useMemo(() => loadOriginSelection(originProjectId), [originProjectId])
  const originSelection = chosenOrigins && chosenOrigins.projectId === originProjectId ? chosenOrigins.keys : storedOrigins
  const chooseOrigins = useCallback(
    (next: string[]) => {
      setChosenOrigins({ projectId: originProjectId, keys: next })
      saveOriginSelection(originProjectId, next)
    },
    [originProjectId]
  )

  // The shape of the rows. Remembered per browser: it is a reading setting, it
  // depends neither on the project nor on the tab, and resetting it on every
  // reload is the reason nobody used it.
  const [rowMode, setRowMode] = useState<RoadmapRowDisplayMode>(() => loadRoadmapRowDisplayMode())
  const chooseRowMode = (next: RoadmapRowDisplayMode) => {
    setRowMode(next)
    saveRoadmapRowDisplayMode(next)
  }

  // The macros classified here whose "roadmap:" label is not on the tracker
  // yet, or no longer says the same thing. The count is read again on every
  // move of the activity queue, since the queue is what pushes.
  const [pendingPushes, setPendingPushes] = useState(0)
  const [isPushing, setIsPushing] = useState(false)
  const [isImporting, setIsImporting] = useState(false)

  const [isRefining, setIsRefining] = useState(false)
  const [refinePreview, setRefinePreview] = useState<RefineMacroResult | null>(null)
  const [selectedProposedTasks, setSelectedProposedTasks] = useState<Record<number, boolean>>({})
  const [isCreatingBatch, setIsCreatingBatch] = useState(false)

  const [showMigrateModal, setShowMigrateModal] = useState(false)
  const [migrateTargetProjectId, setMigrateTargetProjectId] = useState('')
  const [migrateIncludeTasks, setMigrateIncludeTasks] = useState(true)
  const [isMigrating, setIsMigrating] = useState(false)

  // Synchronisation du mode par défaut selon l'horizon sélectionné
  useEffect(() => {
    if (tab === 'later') {
      setDisplayMode('framing')
    } else if (tab === 'now' || tab === 'next') {
      setDisplayMode('execution')
    }
  }, [tab])

  const [showCreateMacroModal, setShowCreateMacroModal] = useState(false)
  const [createMacroTitle, setCreateMacroTitle] = useState('')
  const [createMacroHorizon, setCreateMacroHorizon] = useState<MacroHorizon>('now')
  const [createMacroProjectId, setCreateMacroProjectId] = useState<string>('')

  // Les trois dialogues de cette vue se ferment comme leur croix : clic à côté
  // et Échap appellent le setter que le bouton appelle déjà.
  const closeCreateMacro = useCallback(() => setShowCreateMacroModal(false), [])
  const createMacroBackdrop = useBackdropDismiss(closeCreateMacro)
  useEscapeKey(showCreateMacroModal, closeCreateMacro)

  const closeMigrate = useCallback(() => setShowMigrateModal(false), [])
  const migrateBackdrop = useBackdropDismiss(closeMigrate)
  useEscapeKey(showMigrateModal, closeMigrate)

  const closeSeed = useCallback(() => {
    if (!isSeeding) setSeedLines(null)
  }, [isSeeding])
  const seedBackdrop = useBackdropDismiss(closeSeed)
  useEscapeKey(seedLines !== null, closeSeed)
  const seedValueCount = Object.values(seedKept).filter(Boolean).length

  const closeRefinePreview = useCallback(() => setRefinePreview(null), [])
  const refinePreviewBackdrop = useBackdropDismiss(closeRefinePreview)
  useEscapeKey(refinePreview !== null, closeRefinePreview)

  // Copy the macro's own link, or its reference when the tracker gives no
  // page. Writing to the clipboard needs a secure context and the API can be
  // missing behind a plain-HTTP proxy: the failure is said, with the text to
  // copy by hand, rather than letting one believe the copy happened.
  const [copiedLink, setCopiedLink] = useState(false)
  const copyMacroLink = async (row: MacroRow) => {
    const payload = macroCopyPayload(row)
    try {
      if (!navigator.clipboard) throw new Error(strings.panel.clipboardUnavailable)
      await navigator.clipboard.writeText(payload.text)
      setCopiedLink(true)
      window.setTimeout(() => setCopiedLink(false), 1800)
      addToast({
        type: 'success',
        title: payload.kind === 'link' ? strings.panel.linkCopied : strings.panel.refCopied,
        description: payload.text,
      })
    } catch (err) {
      const reason = err instanceof Error ? err.message : String(err)
      addToast({
        type: 'error',
        title: strings.panel.copyFailed,
        description: `${reason} ${format(strings.panel.copyByHand, { text: payload.text })}`,
        duration: 9000,
      })
    }
  }

  const [isEditingTitle, setIsEditingTitle] = useState(false)
  const [editingTitleValue, setEditingTitleValue] = useState('')

  useEffect(() => {
    setIsEditingTitle(false)
    setEditingTitleValue('')
  }, [selectedKey])

  // The room the panel takes and the folded framing sections, kept across
  // visits. Sections open by default: folding is something one asks for.
  const [isPanelExpanded, setIsPanelExpanded] = usePersistedFlag(ROADMAP_PANEL_EXPANDED_STORAGE_KEY, false)
  const [isDescExpanded, setIsDescExpanded] = usePersistedFlag(ROADMAP_DESCRIPTION_OPEN_STORAGE_KEY, true)
  const [isFramingExpanded, setIsFramingExpanded] = usePersistedFlag(ROADMAP_FRAMING_OPEN_STORAGE_KEY, true)

  /**
   * A hidden panel gives the whole width to the list.
   *
   * Expanding gives the panel all the room, the split handle some; this gives
   * it none, which is what browsing many condensed macros asks for. The choice
   * belongs to the view, not to the selection: clicking another macro does not
   * bring the panel back, or browsing a list would change half the screen on
   * every click.
   *
   * Hiding and expanding speak of the same room, so they exclude each other: a
   * panel is never both full screen and absent.
   */
  const [isPanelHidden, setIsPanelHidden] = usePersistedFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, false)
  const hidePanel = () => {
    setIsPanelExpanded(false)
    setIsPanelHidden(true)
  }
  const showPanel = () => setIsPanelHidden(false)
  const toggleExpanded = () => {
    setIsPanelHidden(false)
    setIsPanelExpanded(prev => !prev)
  }

  // Le cadrage n'est enregistré qu'à la demande
  const [draftDescription, setDraftDescription] = useState('')
  const [draftDirty, setDraftDirty] = useState(false)
  const [draftFramingComment, setDraftFramingComment] = useState('')
  const [draftFramingDirty, setDraftFramingDirty] = useState(false)
  const [newTodo, setNewTodo] = useState('')
  const [creatingTodoId, setCreatingTodoId] = useState<string | null>(null)
  // Batch story creation (#634). The selection is interface state only,
  // distinct from the done checkbox; the report lasts until the next batch,
  // another macro or a reload. batchMacroKey names the macro whose slicing a
  // running batch locks.
  const [selectedTodoIds, setSelectedTodoIds] = useState<Set<string>>(() => new Set())
  const [batchMacroKey, setBatchMacroKey] = useState<string | null>(null)
  const [batchReport, setBatchReport] = useState<{ macroKey: string; batch: MacroStoryBatch } | null>(null)
  // La source en cours de lecture, pour que le bouton cliqué soit celui qui
  // tourne : deux sources côte à côte, un seul témoin, et on ne sait plus
  // laquelle on a demandée.
  const [slicingSource, setSlicingSource] = useState<MacroTodoSource | null>(null)
  // Prototypage de la macro : créer une story a la volée, ou y pousser un ticket existant
  const [newStory, setNewStory] = useState('')
  const [attachQuery, setAttachQuery] = useState('')
  const [busyKey, setBusyKey] = useState<string | null>(null)
  // Découpe : les stories cochées partent vers une autre macro
  const [checked, setChecked] = useState<Record<string, boolean>>({})
  const [cutAt, setCutAt] = useState<number>(-1)
  const [draggingCut, setDraggingCut] = useState(false)
  const rowRefs = useRef<(HTMLDivElement | null)[]>([])
  const [moveTarget, setMoveTarget] = useState('')
  const [newMacroTitle, setNewMacroTitle] = useState('')

  // Largeur du panneau de droite. C'est là que le travail se fait (les tickets de
  // l'épic, la découpe), alors que la colonne de gauche n'a qu'à lister : la
  // répartition par défaut donne donc la place au panneau, et la poignée permet
  // de la régler. La valeur est mémorisée par navigateur.
  // Sprints proposables : ceux du board du projet, actifs et futurs. Un sprint
  // clos n'accueille plus de travail, et l'API Agile déplace par identifiant, donc
  // ceux qui n'en portent pas (import antérieur) sont écartés.
  const sprintOptions = useMemo(
    () =>
      (currentProject?.sprints || [])
        .filter(sp => sp.id && sp.state !== 'closed')
        .map(sp => ({ id: sp.id as string, name: sp.name, state: sp.state })),
    [currentProject?.sprints]
  )
  const [sprintTarget, setSprintTarget] = useState<{ id: string; name: string }>({ id: '', name: '' })

  // Épics et sprints se cherchent au clavier : cette vue en liste cent quarante
  // et dix, et l'épic cible d'une découpe se choisissait dans un menu déroulant
  // de tout le projet.
  const sprintKinds = t.taskDetail.lookups.sprintKinds
  const searchSprint = useMemo(
    () => sprintLookup(currentProject?.sprints || [], sprintKinds),
    [currentProject?.sprints, sprintKinds]
  )

  // Ce qui restreint la liste de tickets sur laquelle la roadmap est construite.
  const activeFilterChips = useMemo(() => {
    const chips: { label: string; clear: () => void }[] = []
    if (assigneeFilter) {
      chips.push({
        label: assigneeFilter === '__unassigned__' ? strings.filters.unassigned : assigneeFilter,
        clear: () => setAssigneeFilter(null),
      })
    }
    if (myTasksOnly) chips.push({ label: t.nav.myTasks, clear: () => setMyTasksOnly(false) })
    if (sprintFilter) chips.push({ label: sprintFilter, clear: () => setSprintFilter(null) })
    if (teamFilter) chips.push({ label: teamFilter, clear: () => setTeamFilter(null) })
    if (labelFilter) chips.push({ label: `#${labelFilter.replace(/^#+/, '')}`, clear: () => setLabelFilter(null) })
    if (pinnedOnly) chips.push({ label: strings.filters.pinnedOnly, clear: () => setPinnedOnly(false) })
    if (searchQuery) chips.push({ label: format(strings.filters.search, { query: searchQuery }), clear: () => setSearchQuery('') })
    if (priorityFilter) {
      const value = priorityFilter === 'none' ? strings.axes.noPriority : epicPriorityLabel(priorityFilter)
      chips.push({ label: format(strings.axes.filterChip, { value }), clear: () => setPriorityFilter(null) })
    }
    selectedLabels.forEach(label =>
      chips.push({
        label: format(strings.epicLabels.chip, { label }),
        clear: () => setSelectedLabels(prev => prev.filter(l => l !== label)),
      })
    )
    return chips
  }, [
    selectedLabels,
    assigneeFilter,
    myTasksOnly,
    sprintFilter,
    teamFilter,
    labelFilter,
    pinnedOnly,
    searchQuery,
    priorityFilter,
    setAssigneeFilter,
    setMyTasksOnly,
    setSprintFilter,
    setTeamFilter,
    setLabelFilter,
    setPinnedOnly,
    setSearchQuery,
    t,
    strings,
  ])

  const PANEL_MIN = 420
  const LIST_MIN = 280
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    const stored = Number(localStorage.getItem('sectile_roadmap_panel_width') || localStorage.getItem('taskacao_roadmap_panel_width') || '')
    return Number.isFinite(stored) && stored >= PANEL_MIN ? stored : 720
  })
  const splitRef = useRef<HTMLDivElement>(null)
  const [isDraggingSplit, setIsDraggingSplit] = useState(false)

  const startSplitDrag = (e: React.PointerEvent) => {
    e.preventDefault()
    const container = splitRef.current
    if (!container) return
    setIsDraggingSplit(true)

    const onMove = (ev: PointerEvent) => {
      const rect = container.getBoundingClientRect()
      const next = Math.round(rect.right - ev.clientX)
      const max = Math.max(PANEL_MIN, rect.width - LIST_MIN)
      setPanelWidth(Math.min(max, Math.max(PANEL_MIN, next)))
    }
    const onUp = () => {
      setIsDraggingSplit(false)
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      setPanelWidth(current => {
        try {
          localStorage.setItem('sectile_roadmap_panel_width', String(current))
        } catch {
          // stockage indisponible : la largeur vaut pour cette session
        }
        return current
      })
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  // The project the loaded macros belong to: a ticket's epic is looked for
  // only once they are this project's, never in the list of the previous one.
  const [macrosFor, setMacrosFor] = useState('')
  useEffect(() => {
    if (!currentProject?.id) {
      setMacroMeta([])
      setMacrosFor('')
      return
    }
    const projectId = currentProject.id
    fetchProjectMacros(projectId).then(list => {
      setMacroMeta(list)
      setMacrosFor(projectId)
    })
  }, [currentProject?.id, fetchProjectMacros, activeJobCount])

  // Whether a push is late is read from the tracker, not locally: a failure
  // leaves the classification in the database exactly as a success does, and
  // only the remote label tells the two apart. A project with no tracker, or
  // one whose read fails, answers zero rather than raising an alert that
  // nothing could clear.
  //
  // The function goes through a ref rather than the dependency array. The
  // context actions are rebuilt on every render of the provider: listing them
  // would fire this read on every toast and every updated work item, and this
  // read queries the tracker. The project and the activity queue are enough to
  // say when the lateness can have changed, the queue being what pushes.
  const readPendingPushes = useRef(pendingHorizonPushes)
  useEffect(() => {
    readPendingPushes.current = pendingHorizonPushes
  })

  useEffect(() => {
    if (!currentProject?.id) {
      setPendingPushes(0)
      return
    }
    let alive = true
    readPendingPushes.current(currentProject.id).then(list => {
      if (alive) setPendingPushes(list.length)
    })
    return () => {
      alive = false
    }
  }, [currentProject?.id, activeJobCount])

  const allRows = useMemo(() => buildMacroRows(tasks, currentProject, macroMeta), [tasks, currentProject, macroMeta])

  // A ticket asked for its epic (#630). The request is answered once, when
  // this project's tickets and epics are loaded, and dropped before anything
  // else so a later filter or tab change never brings the epic back. What
  // could hide the epic is cleared; how the roadmap is read (grouping, sort,
  // row shape, expanded panel) is left as the user set it.
  useEffect(() => {
    if (!roadmapFocus || !currentProject?.id) return
    if (roadmapFocus.projectId !== currentProject.id || macrosFor !== currentProject.id || isLoading) return
    consumeRoadmapFocus()
    const { epicKey, from } = roadmapFocus
    const place = locateEpic(allRows, epicKey)
    if (!place) {
      addToast({ type: 'error', title: strings.focus.unknownTitle, description: format(strings.focus.unknown, { key: epicKey }) })
      if (from !== 'roadmap') setActiveView(from)
      return
    }
    if (searchQuery) setSearchQuery('')
    // An epic of a roadmap project hides while its project is not ticked (#632).
    const focused = allRows.find(r => r.key === epicKey)
    const reveal = selectionRevealing(originSelection, currentProject.jiraProject || '', focused ? rowOrigin(focused) : '')
    if (reveal && currentProject.issueTracker === 'jira') chooseOrigins(reveal)
    setSelectedLabels([])
    setPriorityFilter(null)
    setOnlyIssues(false)
    if (place.closed) setShowClosed(true)
    setTab(place.tab)
    setSelectedKey(epicKey)
    setIsPanelHidden(false)
  }, [
    roadmapFocus,
    currentProject?.id,
    macrosFor,
    isLoading,
    allRows,
    consumeRoadmapFocus,
    addToast,
    strings.focus,
    setActiveView,
    searchQuery,
    setSearchQuery,
    setTab,
    setSelectedKey,
    setIsPanelHidden,
    originSelection,
    chooseOrigins,
  ])

  // The label filter offers what the epics the other filters let through
  // carry, across every horizon tab, so that its counts do not change with the
  // tab being read.
  const unlabelledRows = useMemo(() => {
    let list = allRows
    if (!showClosed) list = list.filter(r => !r.closed)
    if (searchQuery.trim()) list = list.filter(r => matchesMacroSearch(r, searchQuery))
    if (priorityFilter) list = list.filter(r => matchesPriority(r, priorityFilter))
    return list
  }, [allRows, showClosed, searchQuery, priorityFilter])

  // The origin selection offers every origin the epics carry, and counts those
  // the other filters let through, labels included; the rows then keep the
  // selected origins only. With nothing to choose, nothing is filtered.
  const ownOrigin = (currentProject?.jiraProject || '').trim().toUpperCase()
  const origins = useMemo(
    () => offeredOrigins(currentProject, allRows, unlabelledRows.filter(r => matchesEpicLabels(r, selectedLabels))),
    [currentProject, allRows, unlabelledRows, selectedLabels]
  )
  const selectedOrigins = useMemo(
    () => (origins.length > 0 ? normalizeOriginSelection(originSelection, origins, ownOrigin) : []),
    [origins, originSelection, ownOrigin]
  )
  const originRows = useMemo(
    () => unlabelledRows.filter(r => matchesOrigins(r, selectedOrigins, ownOrigin)),
    [unlabelledRows, selectedOrigins, ownOrigin]
  )

  // The origin chip joins the others once the origins are known: they come
  // from the rows, which the chips above are computed before.
  const filterChips = useMemo(() => {
    if (origins.length === 0 || isDefaultOriginSelection(selectedOrigins, ownOrigin)) return activeFilterChips
    return [
      {
        label: format(strings.origins.chip, { keys: selectedOrigins.join(', ') }),
        clear: () => chooseOrigins([ownOrigin]),
      },
      ...activeFilterChips,
    ]
  }, [activeFilterChips, origins, selectedOrigins, ownOrigin, strings, chooseOrigins])

  const labelInventory = useMemo(() => epicLabelInventory(originRows), [originRows])
  // The editor suggests every free label of the project's epics, closed and
  // searched-away ones included: a label is reused, not typed anew.
  const labelSuggestions = useMemo(() => epicLabelInventory(allRows).map(entry => entry.label), [allRows])

  // A picked label no epic of the view carries any more stops being picked,
  // rather than leaving an empty list nobody can explain.
  useEffect(() => {
    setSelectedLabels(prev => pruneSelectedLabels(prev, labelInventory))
  }, [labelInventory])

  const rows = useMemo(
    () => originRows.filter(r => matchesEpicLabels(r, selectedLabels)),
    [originRows, selectedLabels]
  )

  const hiddenMatches = useMemo(() => {
    const q = searchQuery.trim()
    if (!q) return 0
    return allRows.filter(r => {
      if (!matchesMacroSearch(r, q)) return false
      if (!showClosed && r.closed) return true
      return false
    }).length
  }, [allRows, searchQuery, showClosed])

  const closedCount = allRows.filter(r => r.closed).length

  const counts = useMemo(
    () => ({
      now: rows.filter(r => r.horizon === 'now').length,
      next: rows.filter(r => r.horizon === 'next').length,
      later: rows.filter(r => r.horizon === 'later').length,
      unclassified: rows.filter(r => !r.horizon).length,
      hidden: rows.filter(r => r.horizon === 'hidden').length,
    }),
    [rows]
  )

  // Les onglets « non classés » et « masqués » n'ont pas d'horizon propre : le
  // panneau y montre le cadrage, pas la vérification de sprint.
  /**
   * The "Masqués" tab keeps the unfolded shape, whatever the preference says.
   * A condensed row only offers NOW, NEXT and LATER: on that tab none of the
   * three is active, and every macro would read as unclassified there when it
   * is precisely the one carrying a classification.
   */
  const condensedHere = isRoadmapRowCondensed(rowMode) && tab !== 'hidden'

  const horizonOfTab: Horizon =
    tab === 'next' ? 'next' : tab === 'later' ? 'later' : tab === 'hidden' ? 'hidden' : 'now'

  const visibleRows = useMemo(() => {
    const inTab = tab === 'unclassified' ? rows.filter(r => !r.horizon) : rows.filter(r => r.horizon === tab)
    const list = sortByPriority(inTab, prioritySort)
    if (displayMode === 'execution' && onlyIssues) {
      return list.filter(r => placementIssues(r, horizonOfTab).length > 0)
    }
    return list
  }, [rows, tab, displayMode, onlyIssues, horizonOfTab, prioritySort])

  // Chercher une macro et rester devant un onglet vide n'aide personne
  useEffect(() => {
    if (!searchQuery.trim() || visibleRows.length > 0) return
    const target = TABS.find(candidate =>
      candidate.id === 'unclassified'
        ? rows.some(r => !r.horizon)
        : rows.some(r => r.horizon === candidate.id)
    )
    if (target && target.id !== tab) setTab(target.id)
  }, [searchQuery, visibleRows.length, rows, tab, setTab])

  const selected: MacroRow | null = visibleRows.find(r => r.key === selectedKey) || visibleRows[0] || null

  /**
   * The sections of the tab (#628), built on the rows the flat list would
   * show, in its order, so the filters and the sort apply first and the tab
   * counts do not change. The Hidden tab stays flat.
   */
  const grouped = groupAxis !== 'none' && isGroupableTab(tab)
  const sections = useMemo<EpicSection<MacroRow>[] | null>(
    () => (groupAxis !== 'none' && isGroupableTab(tab) ? groupEpics(visibleRows, groupAxis, new Date()) : null),
    [tab, groupAxis, visibleRows]
  )
  // What a Shift click ranges over: the shown epics, folded sections skipped.
  const pickOrder = useMemo(
    () => (sections ? sections.filter(sec => !foldedSections.has(sec.id)).flatMap(sec => sec.rows.map(r => r.key)) : []),
    [sections, foldedSections]
  )

  // An epic no longer shown (another tab, a filter, a fold, no grouping)
  // leaves the selection for good, adjusted while rendering as the board does.
  // pruneSelection returns the same Set when nothing drops out, so this settles.
  const prunedPicked = pruneSelection(pickedKeys, pickOrder)
  if (prunedPicked !== pickedKeys) setPickedKeys(prunedPicked)
  if (pickAnchor && !pickOrder.includes(pickAnchor)) setPickAnchor(null)

  // Escape clears the selection only when nothing else would take the key,
  // as on the board.
  const appSurfaceOpen = Boolean(
    isCommandPaletteOpen || isQuickAddOpen || selectedTask || selectedActivity || isProfileOpen || searchQuery,
  )
  const hasPicked = pickedKeys.size > 0
  useEffect(() => {
    if (!hasPicked) return
    const handleKeyDown = (e: KeyboardEvent) => {
      const active = document.activeElement
      const activeTag = (active?.tagName || '').toLowerCase()
      const clear = shouldEscapeClearSelection({
        key: e.key,
        defaultPrevented: e.defaultPrevented,
        appSurfaceOpen,
        inputFocused: activeTag === 'input' || activeTag === 'textarea' || activeTag === 'select' || Boolean(active?.closest('.xterm')),
        modalOpen: Boolean(document.querySelector('[aria-modal="true"]')),
      })
      if (clear) {
        setPickedKeys(new Set())
        setPickAnchor(null)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [hasPicked, appSurfaceOpen])

  /**
   * A row click: Ctrl or Cmd toggles the epic in the selection, Shift adds the
   * range from the last toggled one, anything else opens it in the panel and
   * keeps the selection. Selection only exists while the tab is grouped.
   */
  const onRowClick = (e: React.MouseEvent, key: string) => {
    if (grouped && isSelectionClick(e)) {
      setPickedKeys(prev => toggleSelected(prev, key))
      setPickAnchor(key)
      return
    }
    if (grouped && e.shiftKey) {
      const range = rangeSelection(pickOrder, pickAnchor, key)
      setPickedKeys(prev => new Set([...prev, ...range]))
      if (!pickAnchor) setPickAnchor(key)
      return
    }
    setSelectedKey(key)
  }

  /** Dragging a picked epic carries the whole selection, another one only itself. */
  const onRowDragStart = (e: React.DragEvent, key: string) => {
    const keys = pickedKeys.has(key) ? pickOrder.filter(k => pickedKeys.has(k)) : [key]
    e.dataTransfer.setData(DRAG_EPIC_KEYS, JSON.stringify(keys))
    e.dataTransfer.effectAllowed = 'move'
  }

  /** The drag and selection props of a row, empty while the tab is flat. */
  const rowDragProps = (key: string) =>
    grouped
      ? {
          draggable: true,
          onDragStart: (e: React.DragEvent) => onRowDragStart(e, key),
          onDragEnd: () => setDropSection(null),
          // A Shift click extends the selection; it must not select text too.
          onMouseDown: (e: React.MouseEvent) => {
            if (e.shiftKey) e.preventDefault()
          },
          'aria-selected': pickedKeys.has(key),
          title: strings.grouping.dragTitle,
        }
      : {}

  const pickedOutline = (key: string): React.CSSProperties =>
    pickedKeys.has(key) ? { outline: '2px solid var(--accent-color)', outlineOffset: 1 } : {}

  // Another macro starts with no selection and no report. The refs let the
  // end of a batch, and a save racing it, read the state of that moment.
  const shownMacroKey = selected?.key || ''
  const shownMacroKeyRef = useRef(shownMacroKey)
  const batchMacroKeyRef = useRef<string | null>(null)
  useEffect(() => {
    shownMacroKeyRef.current = shownMacroKey
    setSelectedTodoIds(new Set())
    setBatchReport(null)
  }, [shownMacroKey])

  const runStoryBatch = async (row: MacroRow) => {
    if (!currentProject?.id || batchMacroKeyRef.current) return
    // A line that became attached or was removed leaves the selection.
    const ids = [...pruneTodoSelection(selectedTodoIds, todosOf(row))]
    if (ids.length === 0) return
    batchMacroKeyRef.current = row.key
    setBatchMacroKey(row.key)
    setBatchReport(null)
    const batch = await createStoriesFromMacroTodos(currentProject.id, row.key, ids)
    batchMacroKeyRef.current = null
    setBatchMacroKey(null)
    if (!batch) return
    if (batch.macro) {
      const macro = batch.macro
      setMacroMeta(prev => prev.map(m => (m.key === macro.key ? macro : m)))
    }
    // The report belongs to the macro it ran on: shown only if it is still
    // the one on screen.
    if (shownMacroKeyRef.current === row.key) {
      setSelectedTodoIds(new Set())
      setBatchReport({ macroKey: row.key, batch })
    }
  }

  // An expanded panel takes the whole view, the toolbar included: it is there
  // to work on one macro. Both need a macro shown, so the toolbar and the list
  // come back by themselves when the last one leaves the tab.
  const panelShown = Boolean(selected) && !isPanelHidden
  const expandedHere = panelShown && isPanelExpanded

  const selectedQuarter = selected?.quarter || ''
  const quarterOrigin = `${selected?.key || ''}|${selectedQuarter}`
  const quarterEditHere = quarterEdit && quarterEdit.origin === quarterOrigin ? quarterEdit : null
  const quarterDraft = quarterEditHere ? quarterEditHere.value : selectedQuarter
  const quarterError = quarterEditHere ? quarterEditHere.error : ''
  const setQuarterDraft = (value: string) => setQuarterEdit({ origin: quarterOrigin, value, error: '' })
  const setQuarterError = (error: string) => setQuarterEdit({ origin: quarterOrigin, value: quarterDraft, error })

  // Les tickets de la macro dans l'ordre chronologique de leur sprint
  const orderedOpen = useMemo(
    () => (selected ? tasksBySprintOrder(selected.open, currentProject) : []),
    [selected?.key, selected?.open, currentProject?.id, currentProject?.sprints]
  )

  // Glissement du cran de coupe
  useEffect(() => {
    if (!draggingCut) return

    const onMove = (e: PointerEvent) => {
      const rows = rowRefs.current.slice(0, orderedOpen.length).filter(Boolean) as HTMLDivElement[]
      if (rows.length === 0) return

      let best = 0
      let bestDistance = Number.POSITIVE_INFINITY
      rows.forEach((row, index) => {
        const rect = row.getBoundingClientRect()
        const distance = Math.abs(rect.top - e.clientY)
        if (distance < bestDistance) {
          bestDistance = distance
          best = index
        }
      })

      const last = rows[rows.length - 1].getBoundingClientRect()
      if (e.clientY > last.bottom) {
        setCutAt(-1)
        return
      }
      setCutAt(best)
    }

    const onUp = () => setDraggingCut(false)
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
  }, [draggingCut, orderedOpen.length])

  const cutIds = useMemo(() => {
    const manual = Object.entries(checked).filter(([, on]) => on).map(([id]) => id)
    if (manual.length > 0) return manual
    if (cutAt >= 0) return orderedOpen.slice(cutAt).map(t => t.id)
    return []
  }, [checked, cutAt, orderedOpen])

  useEffect(() => {
    setDraftDescription(selected?.meta?.description || '')
    setDraftDirty(false)
    setDraftFramingComment(selected?.meta?.framingComment || '')
    setDraftFramingDirty(false)
    setNewTodo('')
    setChecked({})
    setCutAt(-1)
    rowRefs.current = []
    setMoveTarget('')
    setNewMacroTitle('')
  }, [selected?.key, selected?.meta?.description, selected?.meta?.framingComment])

  const attachCandidates = useMemo(() => {
    const q = attachQuery.trim().toLowerCase()
    if (!q || !selected) return []
    return tasks
      .filter(t => t.parentKey !== selected.key)
      .filter(t => t.key.toLowerCase().includes(q) || t.title.toLowerCase().includes(q))
      .slice(0, 6)
  }, [attachQuery, tasks, selected])

  const activeSprints = (currentProject?.sprints || []).filter(s => s.state === 'active')
  const futureSprints = (currentProject?.sprints || []).filter(s => s.state === 'future')

  const persist = async (
    key: string,
    patch: { horizon?: MacroHorizon | ''; description?: string; framingComment?: string; todos?: MacroTodo[] }
  ) => {
    if (!currentProject?.id) {
      addToast({
        type: 'error',
        title: strings.noProjectTitle,
        description: strings.noProjectBody,
      })
      return
    }
    // A slicing saved while a batch runs would be older than the keys the
    // batch is recording (#634).
    if (patch.todos && batchMacroKeyRef.current === key) return
    const saved = await saveMacroMeta(currentProject.id, key, patch)
    if (saved) {
      setMacroMeta(prev => [...prev.filter(m => m.key !== saved.key), saved])
      if (patch.horizon) {
        setTab(patch.horizon)
      }
      setSelectedKey(saved.key)
    }
  }

  const todosOf = (row: MacroRow | null): MacroTodo[] => row?.meta?.todos || []

  /** Saves the epic's priority, quarter or readiness and takes the stored macro back. */
  const saveAxes = async (key: string, patch: { priority?: EpicPriority | ''; quarter?: string; readiness?: EpicReadiness | '' }) => {
    if (!currentProject?.id) return
    setBusyKey('axes')
    const saved = await saveMacroMeta(currentProject.id, key, patch)
    // Replaced in place: appending it would move an epic without tickets to the
    // end of its tab at every click, the list order following this array.
    if (saved) {
      setMacroMeta(prev =>
        prev.some(m => m.key === saved.key) ? prev.map(m => (m.key === saved.key ? saved : m)) : [...prev, saved]
      )
    }
    setBusyKey(null)
  }

  /**
   * Applies the kept seeding values, one epic at a time through the same save
   * as the panel, then reports once: how many epics took their values, and
   * which ones were refused. Tracker refusals arrive later in the activities,
   * as for any queued write.
   */
  const runSeed = async () => {
    if (!currentProject?.id || !seedLines) return
    setIsSeeding(true)
    let done = 0
    const refused: string[] = []
    for (const line of seedLines) {
      const patch: { priority?: EpicPriority; quarter?: string } = {}
      if (line.priority && seedKept[`${line.key}:priority`]) patch.priority = line.priority
      if (line.quarter && seedKept[`${line.key}:quarter`]) patch.quarter = line.quarter
      if (!patch.priority && !patch.quarter) continue
      const saved = await saveMacroMeta(currentProject.id, line.key, patch, { quiet: true, bulk: true })
      if (saved) done++
      else refused.push(line.key)
    }
    const fresh = await fetchProjectMacros(currentProject.id)
    setMacroMeta(fresh)
    setIsSeeding(false)
    setSeedLines(null)
    if (refused.length > 0) {
      addToast({
        type: 'error',
        title: strings.axes.seedFailedTitle,
        description: `${plural(language, done, strings.axes.seedDone)}. ${format(strings.axes.seedFailed, { keys: refused.join(', ') })}`,
      })
    } else {
      addToast({ type: 'success', title: strings.axes.seedDoneTitle, description: plural(language, done, strings.axes.seedDone) })
    }
  }

  /** A section's name: its priority, its quarter, or the no-value one. */
  const sectionLabel = (section: EpicSection<MacroRow>): string => {
    if (!section.value) return section.axis === 'priority' ? strings.grouping.noPriority : strings.grouping.noQuarter
    return section.axis === 'priority' ? epicPriorityLabel(section.value as EpicPriority) : section.value
  }

  /**
   * Sets the section's value on the dropped epics, one at a time through the
   * panel's save, skipping those already there and never stopping on a
   * failure; then reloads once and reports once. The epics that moved leave
   * the selection, those refused stay in it so the drop can be retried.
   */
  const dropOnSection = async (section: EpicSection<MacroRow>, keys: string[]) => {
    if (!currentProject?.id || isDropping) return
    const { toSave, skipped } = planAxisDrop(visibleRows, keys, section.axis, section.value)
    // A drop on the section the epics come from changes nothing, and says nothing.
    if (toSave.length === 0) return
    setIsDropping(true)
    const patch = section.axis === 'priority' ? { priority: section.value as EpicPriority | '' } : { quarter: section.value }
    let done = 0
    const refused: string[] = []
    const moved: string[] = []
    for (const key of toSave) {
      const saved = await saveMacroMeta(currentProject.id, key, patch, { quiet: true })
      if (saved) {
        done++
        moved.push(key)
      } else refused.push(key)
    }
    const fresh = await fetchProjectMacros(currentProject.id)
    setMacroMeta(fresh)
    setIsDropping(false)
    setPickedKeys(prev => {
      const next = new Set(prev)
      moved.forEach(key => next.delete(key))
      return next
    })
    const parts = [plural(language, done, strings.grouping.done)]
    if (skipped.length > 0) parts.push(plural(language, skipped.length, strings.grouping.skipped))
    if (refused.length > 0) {
      addToast({
        type: 'error',
        title: strings.grouping.failedTitle,
        description: `${parts.join(', ')}. ${format(strings.grouping.failed, { keys: refused.join(', ') })}`,
      })
    } else {
      addToast({ type: 'success', title: strings.grouping.doneTitle, description: parts.join(', ') })
    }
  }

  /** Header and body of a section both take a drop, folded or empty alike. */
  const sectionDropProps = (section: EpicSection<MacroRow>) => ({
    onDragOver: (e: React.DragEvent) => {
      if (!e.dataTransfer.types.includes(DRAG_EPIC_KEYS)) return
      e.preventDefault()
      e.dataTransfer.dropEffect = 'move'
      if (dropSection !== section.id) setDropSection(section.id)
    },
    onDragLeave: (e: React.DragEvent) => {
      if (e.currentTarget.contains(e.relatedTarget as Node | null)) return
      setDropSection(prev => (prev === section.id ? null : prev))
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      setDropSection(null)
      const keys = parseDraggedEpicKeys(e.dataTransfer.getData(DRAG_EPIC_KEYS))
      if (keys.length > 0) void dropOnSection(section, keys)
    },
  })

  const renderSection = (section: EpicSection<MacroRow>) => {
    const folded = foldedSections.has(section.id)
    const label = sectionLabel(section)
    const over = dropSection === section.id
    return (
      <section
        key={section.id}
        data-section={section.id}
        {...sectionDropProps(section)}
        className="rounded-xl border transition-colors"
        style={{
          borderColor: over ? 'rgb(var(--accent-rgb) / 0.6)' : 'transparent',
          background: over ? 'var(--accent-light)' : 'transparent',
        }}
      >
        <button
          type="button"
          onClick={() => toggleSection(section.id)}
          aria-expanded={!folded}
          title={format(folded ? strings.grouping.unfold : strings.grouping.fold, { section: label })}
          className="w-full flex items-center gap-1.5 px-1.5 py-1 text-[11px] font-bold uppercase tracking-[.06em] text-[var(--text-secondary)] cursor-pointer"
        >
          {folded ? <ChevronRight size={12} /> : <ChevronDown size={12} />}
          <span>{label}</span>
          <span className="font-mono font-normal text-[var(--text-muted)]">{section.rows.length}</span>
        </button>
        {!folded && (
          <div className="space-y-2 px-0.5 pb-1">
            {section.rows.length === 0 ? (
              <div className="text-[10.5px] italic text-[var(--text-muted)] px-2 py-1.5 rounded-lg border border-dashed border-[var(--border-color)]">
                {format(strings.grouping.dropHere, { section: label })}
              </div>
            ) : (
              section.rows.map(row => (condensedHere ? renderCondensedRow(row) : renderMacroRow(row)))
            )}
          </div>
        )}
      </section>
    )
  }

  /**
   * Validates the quarter field and saves it when it changed. An unreadable
   * value stays in the field with its message, and nothing is sent.
   */
  const commitQuarter = (key: string, current: string) => {
    const normalized = normalizeQuarter(quarterDraft)
    if (normalized === null) {
      setQuarterError(format(strings.axes.invalidQuarter, { value: quarterDraft.trim() }))
      return
    }
    if (normalized === current) {
      setQuarterEdit(null)
      return
    }
    saveAxes(key, { quarter: normalized })
  }

  // The mark of an epic of a roadmap project (#632): its key already names its
  // project, the lock says Sectile reads it without writing on it.
  const foreignBadge = (row: MacroRow) =>
    row.meta?.foreign ? (
      <span
        className="shrink-0 inline-flex items-center text-[var(--text-muted)]"
        title={format(strings.origins.badgeTitle, { origin: row.meta.origin || '' })}
        aria-label={format(strings.origins.badgeTitle, { origin: row.meta.origin || '' })}
      >
        <Lock size={10} />
      </span>
    ) : null

  /**
   * The epic's readiness (#633): the level a person decided, in solid colours,
   * or, while nobody did, Sectile's suggestion in a dashed outline followed by
   * "?". Not clickable: the level is decided from the panel or by a drop.
   */
  const readinessBadge = (row: MacroRow, className: string) => {
    const level = row.readiness || row.suggestedReadiness
    const meta = READINESS_META[level]
    const name = strings.readiness.levels[level]
    if (row.readiness) {
      return (
        <span
          className={className}
          data-readiness={level}
          style={{ color: meta.color, background: meta.bg, border: `1px solid ${meta.border}` }}
          title={format(strings.readiness.decidedTitle, { level: name })}
        >
          {name}
        </span>
      )
    }
    return (
      <span
        className={className}
        data-readiness-suggested={level}
        style={{ color: meta.color, background: 'transparent', border: `1px dashed ${meta.border}` }}
        title={strings.readiness.suggestedTitle}
      >
        {name} ?
      </span>
    )
  }

  /**
   * The epic's own priority (#627), in the colour of the level it maps to. An
   * epic without one says so in a muted badge rather than borrowing a value
   * from its tickets.
   */
  const priorityBadge = (row: MacroRow, className: string) => {
    if (!row.priority) {
      return (
        <span className={className} style={{ color: 'var(--text-muted)', background: 'var(--bg-tertiary)' }}>
          {strings.axes.noPriority}
        </span>
      )
    }
    const prio = PRIORITY_META[EPIC_PRIORITY_LEVEL[row.priority]]
    return (
      <span className={className} style={{ color: prio.color, background: prio.bg }} title={strings.axes.priorityTitle}>
        {epicPriorityLabel(row.priority)}
      </span>
    )
  }

  /**
   * The two shapes of a macro row.
   *
   * The unfolded shape carries everything one came to the roadmap to check:
   * the sprint placement of the work items, the maturity, the progress. The
   * condensed shape fits on one line and keeps only what serves browsing: the
   * key, the title, the priority, an anomaly signal and the three classifying
   * buttons. Classifying without unfolding is the whole point: a long list is
   * exactly where one wants to move a macro from one horizon to another, and
   * doing so meant scrolling through six-line cards.
   */
  const renderMacroRow = (row: MacroRow) => {
    const isSel = selected?.key === row.key
    const issues = placementIssues(row, horizonOfTab)
    const mat = MATURITY_META[row.maturity]
    return (
      <div
        key={row.key}
        data-epic-key={row.key}
        onClick={e => onRowClick(e, row.key)}
        {...rowDragProps(row.key)}
        className="relative rounded-xl border p-2.5 cursor-pointer transition-colors"
        style={{
          background: isSel ? 'var(--accent-light)' : 'var(--bg-secondary)',
          borderColor: isSel ? 'rgb(var(--accent-rgb) / 0.45)' : 'var(--border-color)',
          ...pickedOutline(row.key),
        }}
      >
        {epicColorsOn && <EpicBar parentKey={row.key} />}
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-[11px] font-mono font-bold" style={{ color: 'var(--accent-color)' }}>{row.key}</span>
          {foreignBadge(row)}
          <span className="text-[9.5px] px-1 rounded font-mono truncate max-w-[150px] bg-[var(--bg-tertiary)] text-[var(--text-muted)] border border-[var(--border-color)]" title={row.squad}>
            {row.squad}
          </span>
          {priorityBadge(row, 'text-[9.5px] px-1 rounded font-bold')}
          {row.quarter && (
            <span className="text-[9.5px] px-1 rounded font-mono bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)]">
              {row.quarter}
            </span>
          )}
          {readinessBadge(row, 'text-[9.5px] px-1 rounded font-bold')}
          <span className="text-[9px] font-bold px-1.5 rounded uppercase tracking-[.06em]"
            style={{ color: mat.color, background: mat.bg, border: `1px solid ${mat.border}` }}
            title={strings.maturityTitle}>
            {strings.maturity[row.maturity]}
          </span>
          {freeEpicLabels(row.meta).map(label => (
            <span key={label} className={EPIC_LABEL_BADGE}>{label}</span>
          ))}

          {displayMode === 'execution' ? (
            issues.length > 0 ? (
              <span className="ml-auto text-[10px] font-bold px-1.5 py-0.5 rounded inline-flex items-center gap-1"
                style={{ color: 'var(--status-danger)', background: 'rgb(var(--status-danger-rgb) / 0.13)', border: '1px solid rgb(var(--status-danger-rgb) / 0.32)' }}>
                <AlertTriangle size={10} />
                {plural(language, issues.length, strings.toFix)}
              </span>
            ) : (
              <span className="ml-auto text-[10px] font-bold px-1.5 py-0.5 rounded inline-flex items-center gap-1"
                style={{ color: 'var(--status-ok)', background: 'rgb(var(--status-ok-rgb) / 0.13)', border: '1px solid rgb(var(--status-ok-rgb) / 0.32)' }}>
                <Check size={10} />
                {strings.allPlaced}
              </span>
            )
          ) : (
            <span className="ml-auto text-[10px] font-mono text-[var(--text-muted)]">
              {format(strings.todosCount, { done: todosOf(row).filter(t => t.done).length, total: todosOf(row).length })}
            </span>
          )}
        </div>

        <div className="text-[12px] font-semibold leading-snug mt-1.5">{row.title}</div>

        <div className="flex items-center gap-2 mt-2 flex-wrap text-[10px] font-mono text-[var(--text-muted)]">
          <span>{plural(language, row.open.length, strings.openOfTotal, { total: row.tasks.length })}</span>
          {row.inActiveSprint.length > 0 && (
            <span style={{ color: 'var(--status-ok)' }}>{format(strings.activeSprintCount, { count: row.inActiveSprint.length })}</span>
          )}
          {row.inFutureSprint.length > 0 && (
            <span style={{ color: 'var(--status-info)' }}>{format(strings.futureSprintCount, { count: row.inFutureSprint.length })}</span>
          )}
          {row.inStaleSprint.length > 0 && (
            <span style={{ color: 'var(--status-warn)' }}>{format(strings.staleSprintCount, { count: row.inStaleSprint.length })}</span>
          )}
          {row.unscheduled.length > 0 && (
            <span style={{ color: 'var(--status-danger)' }}>{format(strings.noSprintCount, { count: row.unscheduled.length })}</span>
          )}
        </div>

        {/* Classification : un clic, et la suggestion est mise en avant */}
        <div className="flex items-center gap-1.5 mt-2">
          {(['now', 'next', 'later', 'hidden'] as MacroHorizon[]).map(h => {
            const active = row.horizon === h
            const isSuggestion = !row.horizon && row.suggested === h
            return (
              <button
                key={h}
                type="button"
                onClick={e => {
                  e.stopPropagation()
                  persist(row.key, { horizon: h })
                }}
                className="px-1.5 py-0.5 rounded text-[9.5px] font-bold uppercase tracking-[.06em] border transition-colors cursor-pointer"
                style={{
                  color: active ? '#fff' : HORIZON_META[h].color,
                  background: active ? HORIZON_META[h].color : isSuggestion ? HORIZON_META[h].bg : 'transparent',
                  borderColor: active || isSuggestion ? HORIZON_META[h].border : 'var(--border-color)',
                }}
                title={format(isSuggestion ? strings.suggestedHorizon : strings.classifyAs, { horizon: HORIZON_META[h].label })}
              >
                {HORIZON_META[h].label}
                {isSuggestion && ' ?'}
              </button>
            )
          })}
        </div>
      </div>
    )
  }

  /**
   * The condensed row keeps two free labels and counts the rest: it has one line
   * to spend, and the tooltip names what the counter hides.
   */
  const renderCondensedLabels = (row: MacroRow) => {
    const labels = freeEpicLabels(row.meta)
    if (labels.length === 0) return null
    const hidden = labels.slice(CONDENSED_LABELS)
    return (
      <>
        {labels.slice(0, CONDENSED_LABELS).map(label => (
          <span key={label} className={`shrink-0 max-w-[110px] truncate ${EPIC_LABEL_BADGE}`} title={label}>{label}</span>
        ))}
        {hidden.length > 0 && (
          <span className={`shrink-0 ${EPIC_LABEL_BADGE}`} title={format(strings.epicLabels.moreTitle, { labels: hidden.join(', ') })}>
            {format(strings.epicLabels.more, { count: hidden.length })}
          </span>
        )}
      </>
    )
  }

  const renderCondensedRow = (row: MacroRow) => {
    const isSel = selected?.key === row.key
    const issues = placementIssues(row, horizonOfTab)
    return (
      <div
        key={row.key}
        data-epic-key={row.key}
        onClick={e => onRowClick(e, row.key)}
        {...rowDragProps(row.key)}
        className="relative flex items-center gap-2 px-2.5 py-1.5 rounded-lg border cursor-pointer transition-colors"
        style={{
          background: isSel ? 'var(--accent-light)' : 'var(--bg-secondary)',
          borderColor: isSel ? 'var(--accent-color)' : 'var(--border-color)',
          ...pickedOutline(row.key),
        }}
      >
        {epicColorsOn && <EpicBar parentKey={row.key} />}
        <span className="shrink-0 text-[10.5px] font-mono font-bold" style={{ color: 'var(--accent-color)' }}>
          {row.key}
        </span>
        {foreignBadge(row)}
        <span className="flex-1 min-w-0 truncate text-[11.5px] text-[var(--text-primary)]" title={row.title}>
          {row.title}
        </span>
        {renderCondensedLabels(row)}
        <span className="shrink-0 text-[9.5px] font-mono text-[var(--text-muted)]">
          {row.open.length}/{row.tasks.length}
        </span>
        {/* The placement anomaly shrinks to its count: it is the only signal of
            the unfolded shape that calls for an action, and losing it would make
            the condensed shape a view where one no longer sees what is wrong.
            Outside NOW and NEXT, placementIssues answers nothing and the badge
            does not show. */}
        {issues.length > 0 && (
          <span
            className="shrink-0 text-[9.5px] font-bold px-1 rounded inline-flex items-center gap-0.5"
            style={{
              color: 'var(--status-danger)',
              background: 'rgb(var(--status-danger-rgb) / 0.13)',
              border: '1px solid rgb(var(--status-danger-rgb) / 0.32)',
            }}
            title={plural(language, issues.length, strings.issuesTitle)}
          >
            <AlertTriangle size={9} />
            {issues.length}
          </span>
        )}
        {priorityBadge(row, 'shrink-0 text-[9.5px] px-1 rounded font-bold')}
        {readinessBadge(row, 'shrink-0 text-[9.5px] px-1 rounded font-bold')}
        <span className="shrink-0 flex items-center gap-0.5">
          {CONDENSED_HORIZONS.map(h => {
            const active = row.horizon === h
            const isSuggestion = !row.horizon && row.suggested === h
            return (
              <button
                key={h}
                type="button"
                onClick={e => {
                  e.stopPropagation()
                  persist(row.key, { horizon: h })
                }}
                className="px-1 py-0.5 rounded text-[9px] font-bold border transition-colors cursor-pointer"
                style={{
                  color: active ? '#fff' : HORIZON_META[h].color,
                  background: active ? HORIZON_META[h].color : isSuggestion ? HORIZON_META[h].bg : 'transparent',
                  borderColor: active || isSuggestion ? HORIZON_META[h].border : 'var(--border-color)',
                }}
                title={
                  isSuggestion
                    ? format(strings.suggestedHorizon, { horizon: HORIZON_META[h].label })
                    : format(strings.classifyAs, { horizon: HORIZON_META[h].label })
                }
                aria-label={format(strings.classifyAs, { horizon: HORIZON_META[h].label })}
                aria-pressed={active}
              >
                {HORIZON_SHORT[h]}
              </button>
            )
          })}
        </span>
      </div>
    )
  }


  const addTodo = (row: MacroRow) => {
    const text = newTodo.trim()
    if (!text) return
    setNewTodo('')
    persist(row.key, { todos: [...todosOf(row), { id: '', text, done: false }] })
  }

  const handleRefineMacro = async () => {
    if (!selected) return
    const text = (draftDescription || selected.meta?.description || '').trim()
    if (!text) {
      addToast({
        type: 'warning',
        title: strings.framingRequired,
        description: strings.framingRequiredBody,
      })
      return
    }

    if (draftDirty) {
      await persist(selected.key, { description: draftDescription })
      setDraftDirty(false)
    }

    setIsRefining(true)
    const result = await refineMacro(selected.key, currentProject?.id)
    setIsRefining(false)

    if (result && ((result.todos && result.todos.length > 0) || (result.proposedTasks && result.proposedTasks.length > 0))) {
      setRefinePreview(result)
      const initialMap: Record<number, boolean> = {}
      result.proposedTasks?.forEach((_: unknown, idx: number) => { initialMap[idx] = true })
      setSelectedProposedTasks(initialMap)
    } else if (result) {
      addToast({
        type: 'info',
        title: strings.noGeneratedData,
        description: strings.noGeneratedDataBody,
      })
    }
  }

  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-hidden bg-[var(--bg-primary)] text-[var(--text-primary)]">
      {/* Barre d'outils : classification, et en mode opérationnel les sprints visés.
          Hidden while the panel is expanded. */}
      {!expandedHere && (
          <div className="flex items-center justify-between gap-3 flex-wrap px-4 py-2 border-b border-[var(--border-color)] bg-[var(--bg-secondary)]/50 shrink-0">
        <div className="flex items-center gap-3 flex-wrap min-w-0">
          <div className="flex items-center p-0.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
            {TABS.map(t => {
              const active = tab === t.id
              const meta = t.id === 'unclassified' ? null : HORIZON_META[t.id as Horizon]
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => setTab(t.id)}
                  className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] transition-colors cursor-pointer"
                  style={{
                    fontWeight: active ? 700 : 500,
                    background: active ? meta?.color || 'var(--text-muted)' : 'transparent',
                    color: active ? '#fff' : 'var(--text-secondary)',
                  }}
                  title={t.id === 'unclassified' ? strings.unclassifiedHint : strings.horizonHints[t.id]}
                >
                  {t.icon}
                  <span>{t.label ?? (t.id === 'hidden' ? strings.tabs.hidden : strings.tabs.unclassified)}</span>
                  <span
                    className="text-[10px] font-mono px-1.5 rounded-full"
                    style={{
                      background: active ? 'rgb(255 255 255 / 0.25)' : 'var(--bg-secondary)',
                      color: active ? '#fff' : 'var(--text-muted)',
                    }}
                  >
                    {counts[t.id]}
                  </span>
                </button>
              )
            })}
          </div>

          {closedCount > 0 && (
            <button
              type="button"
              onClick={() => setShowClosed(v => !v)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold cursor-pointer border"
              style={{
                background: showClosed ? 'var(--accent-light)' : 'var(--bg-tertiary)',
                borderColor: showClosed ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
                color: showClosed ? 'var(--accent-color)' : 'var(--text-secondary)',
              }}
              title={plural(language, closedCount, strings.closedTitle)}
            >
              {showClosed ? <Eye size={12} /> : <EyeOff size={12} />}
              {plural(language, closedCount, showClosed ? strings.closedShown : strings.closedHidden)}
            </button>
          )}

          <EpicOriginFilter offered={origins} selected={selectedOrigins} onChange={chooseOrigins} />
          <EpicLabelFilter inventory={labelInventory} selected={selectedLabels} onChange={setSelectedLabels} />

          {/*
            The classification of a macro is written on the tracker as a
            "roadmap:now / next / later" label, posed on every horizon change.
            These two buttons catch up the two moments where the local side and
            the tracker disagree: what was classified before the mirroring
            existed or during a write outage, and what somebody classified on
            the tracker without coming through here.

            The push button only shows when something is late: offered
            permanently, it would invite a write where there is nothing to write.
          */}
          {pendingPushes > 0 && currentProject && (
            <button
              type="button"
              disabled={isPushing}
              onClick={async () => {
                setIsPushing(true)
                await pushPendingHorizons(currentProject.id)
                setIsPushing(false)
              }}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold cursor-pointer border disabled:opacity-60"
              style={{
                background: 'rgb(var(--status-warn-rgb) / 0.14)',
                borderColor: 'rgb(var(--status-warn-rgb) / 0.4)',
                color: 'var(--status-warn)',
              }}
              title={plural(language, pendingPushes, strings.pendingPushesTitle)}
            >
              {isPushing ? <Loader2 size={12} className="animate-spin" /> : <Tag size={12} />}
              {plural(language, pendingPushes, strings.pendingPushes)}
            </button>
          )}

          {currentProject && (
            <button
              type="button"
              disabled={isImporting}
              onClick={async () => {
                setIsImporting(true)
                const ok = await importMacroHorizons(currentProject.id)
                if (ok) fetchProjectMacros(currentProject.id).then(setMacroMeta)
                const pending = await pendingHorizonPushes(currentProject.id)
                setPendingPushes(pending.length)
                setIsImporting(false)
              }}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold cursor-pointer border disabled:opacity-60"
              style={{
                background: 'var(--bg-tertiary)',
                borderColor: 'var(--border-color)',
                color: 'var(--text-secondary)',
              }}
              title={strings.importTitle}
            >
              {isImporting ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
              {strings.importLabels}
            </button>
          )}

          {/* The epic's own priority (#627): filter and sort. Both apply to
              every tab, and the tab counts follow the filter. */}
          <select
            value={priorityFilter || ''}
            onChange={e => setPriorityFilter((e.target.value || null) as PriorityFilter)}
            aria-label={strings.axes.filterLabel}
            className="px-2 py-1 rounded-md text-[11px] font-semibold cursor-pointer border bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)]"
          >
            <option value="">{strings.axes.filterAll}</option>
            {EPIC_PRIORITIES.map(p => (
              <option key={p} value={p}>{epicPriorityLabel(p)}</option>
            ))}
            <option value="none">{strings.axes.noPriority}</option>
          </select>
          <select
            value={prioritySort}
            onChange={e => setPrioritySort(e.target.value as PrioritySort)}
            aria-label={strings.axes.sortLabel}
            className="px-2 py-1 rounded-md text-[11px] font-semibold cursor-pointer border bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)]"
          >
            <option value="backlog">{strings.axes.sortBacklog}</option>
            <option value="priority-desc">{strings.axes.sortDesc}</option>
            <option value="priority-asc">{strings.axes.sortAsc}</option>
          </select>
          {/* Sections by priority or quarter (#628), on every tab but Hidden. */}
          <select
            value={groupAxis}
            onChange={e => setGroupAxis(e.target.value as EpicGroupAxis)}
            aria-label={strings.grouping.label}
            title={strings.grouping.label}
            className="px-2 py-1 rounded-md text-[11px] font-semibold cursor-pointer border bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)]"
          >
            <option value="none">{strings.grouping.none}</option>
            <option value="priority">{strings.grouping.priority}</option>
            <option value="quarter">{strings.grouping.quarter}</option>
          </select>
          {hasPicked && (
            <span
              data-epic-selection
              className="flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold border"
              style={{ color: 'var(--accent-color)', background: 'var(--accent-light)', borderColor: 'rgb(var(--accent-rgb) / 0.45)' }}
            >
              {isDropping && <Loader2 size={11} className="animate-spin" />}
              {plural(language, pickedKeys.size, strings.grouping.selected)}
              <button
                type="button"
                onClick={() => {
                  setPickedKeys(new Set())
                  setPickAnchor(null)
                }}
                aria-label={strings.grouping.clearSelection}
                title={strings.grouping.clearSelection}
                className="p-0.5 rounded cursor-pointer hover:bg-[var(--bg-tertiary)]"
              >
                <X size={11} />
              </button>
            </span>
          )}
          {currentProject && (
            <button
              type="button"
              onClick={() => {
                const lines = seedProposals(allRows.filter(r => showClosed || !r.closed))
                const kept: Record<string, boolean> = {}
                lines.forEach(line => {
                  if (line.priority) kept[`${line.key}:priority`] = true
                  if (line.quarter) kept[`${line.key}:quarter`] = true
                })
                setSeedKept(kept)
                setSeedLines(lines)
              }}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold cursor-pointer border"
              style={{
                background: 'var(--bg-tertiary)',
                borderColor: 'var(--border-color)',
                color: 'var(--text-secondary)',
              }}
              title={strings.axes.seedTitle}
            >
              <Sparkles size={12} />
              {strings.axes.seedButton}
            </button>
          )}

          {/* Les filtres globaux */}
          {filterChips.length > 0 && (
            <div className="flex items-center gap-1.5 flex-wrap">
              {filterChips.map(chip => (
                <button
                  key={chip.label}
                  type="button"
                  onClick={chip.clear}
                  className="flex items-center gap-1 px-2 py-1 rounded-md text-[10.5px] font-semibold cursor-pointer border"
                  style={{
                    background: 'rgb(var(--status-warn-rgb) / 0.14)',
                    borderColor: 'rgb(var(--status-warn-rgb) / 0.34)',
                    color: 'var(--status-warn)',
                  }}
                  title={format(strings.activeFilterTitle, { label: chip.label })}
                >
                  <Filter size={10} />
                  {chip.label}
                  <X size={10} />
                </button>
              ))}
            </div>
          )}

          {/* Toggle Mode: Framing | Execution */}
          <div className="flex items-center p-0.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
            <button
              type="button"
              onClick={() => setDisplayMode('framing')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition-all cursor-pointer ${
                displayMode === 'framing'
                  ? 'bg-[var(--accent-color)] text-white shadow-xs'
                  : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
              title={strings.modes.framingTitle}
            >
              <Compass size={12} />
              <span>{strings.modes.framing}</span>
            </button>
            <button
              type="button"
              onClick={() => setDisplayMode('execution')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition-all cursor-pointer ${
                displayMode === 'execution'
                  ? 'bg-[var(--accent-color)] text-white shadow-xs'
                  : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
              title={strings.modes.executionTitle}
            >
              <Target size={12} />
              <span>{strings.modes.execution}</span>
            </button>
            {/* Les deux axes de découpe. Ils sont des modes du panneau et non
                une vue à part : on répartit les tickets d'une macro en la
                lisant, pas en quittant son panneau. */}
            <button
              type="button"
              onClick={() => setDisplayMode('phases')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition-all cursor-pointer ${
                displayMode === 'phases'
                  ? 'bg-[var(--accent-color)] text-white shadow-xs'
                  : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
              title={strings.modes.phasesTitle}
            >
              <Layers size={12} />
              <span>{t.planning.macro.axes.phase.plural}</span>
            </button>
            <button
              type="button"
              onClick={() => setDisplayMode('goals')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition-all cursor-pointer ${
                displayMode === 'goals'
                  ? 'bg-[var(--accent-color)] text-white shadow-xs'
                  : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
              title={strings.modes.goalsTitle}
            >
              <Goal size={12} />
              <span>{t.planning.macro.axes.goal.plural}</span>
            </button>
          </div>

          <button
            type="button"
            disabled={busyKey === 'macro'}
            onClick={() => {
              const defaultProjId = currentProject?.id || projects.find(p => p.isDefault)?.id || projects[0]?.id || ''
              setCreateMacroProjectId(defaultProjId)
              setCreateMacroHorizon(tab === 'unclassified' || tab === 'hidden' ? 'now' : (tab as MacroHorizon))
              setCreateMacroTitle('')
              setShowCreateMacroModal(true)
            }}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold text-white accent-bg cursor-pointer disabled:opacity-40"
            title={strings.createMacroTitle}
          >
            <Plus size={12} /> {strings.createMacro}
          </button>

          {displayMode === 'execution' && (
            <button
              type="button"
              onClick={() => setOnlyIssues(v => !v)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold transition-colors cursor-pointer border"
              style={{
                background: onlyIssues ? 'rgb(var(--status-danger-rgb) / 0.14)' : 'var(--bg-tertiary)',
                borderColor: onlyIssues ? 'rgb(var(--status-danger-rgb) / 0.4)' : 'var(--border-color)',
                color: onlyIssues ? 'var(--status-danger)' : 'var(--text-secondary)',
              }}
              title={strings.onlyIssuesTitle}
            >
              <AlertTriangle size={12} />
              {strings.onlyIssues}
            </button>
          )}

          {/* The shape of the rows. The button stays offered on the "Masqués"
              tab and keeps its state there: the preference holds for the whole
              roadmap, and turning it off because one tab does not apply it would
              suggest it was lost by changing tab. */}
          <button
            type="button"
            onClick={() => chooseRowMode(toggleRoadmapRowDisplayMode(rowMode))}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold transition-colors cursor-pointer border"
            style={{
              background: isRoadmapRowCondensed(rowMode) ? 'var(--accent-light)' : 'var(--bg-tertiary)',
              borderColor: isRoadmapRowCondensed(rowMode) ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
              color: isRoadmapRowCondensed(rowMode) ? 'var(--accent-color)' : 'var(--text-secondary)',
            }}
            title={
              tab === 'hidden'
                ? strings.condensedTitleHidden
                : strings.condensedTitle
            }
            aria-pressed={isRoadmapRowCondensed(rowMode)}
          >
            <Rows3 size={12} />
            {strings.condensed}
          </button>

          {/* La barre de recherche est dans l'en-tête, loin de la liste : sans
              ce rappel, on ne comprend pas pourquoi la roadmap est réduite. */}
          {searchQuery.trim() && (
            <span
              className="flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-semibold border"
              style={{
                background: 'var(--accent-light)',
                borderColor: 'rgb(var(--accent-rgb) / 0.4)',
                color: 'var(--accent-color)',
              }}
            >
              <Search size={11} />
              {plural(language, rows.length, strings.searchResults, { query: searchQuery.trim() })}
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                className="cursor-pointer opacity-70 hover:opacity-100"
                title={strings.clearSearch}
              >
                <X size={11} />
              </button>
            </span>
          )}
        </div>

        {displayMode === 'execution' && (tab === 'now' || tab === 'next') && (
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] bg-[var(--bg-tertiary)] border border-[var(--border-color)] shrink-0">
            <CalendarDays size={13} style={{ color: HORIZON_META[horizonOfTab].color }} />
            <span className="text-[var(--text-muted)]">{tab === 'now' ? strings.activeSprints : strings.upcomingSprints}</span>
            {(tab === 'now' ? activeSprints : futureSprints).length === 0 ? (
              <span className="font-mono" style={{ color: 'var(--status-warn)' }}>{strings.noKnownSprint}</span>
            ) : (
              <span className="font-mono font-bold truncate max-w-[320px]">
                {(tab === 'now' ? activeSprints : futureSprints).map(s => s.name).join(' · ')}
              </span>
            )}
          </div>
        )}
      </div>
      )}

      <div className="flex-1 flex min-h-0 min-w-0 overflow-hidden" ref={splitRef}>
        {/* Macros de l'horizon courant (masqué si panneau en plein écran) */}
        {!expandedHere && (
          <div className="flex-1 overflow-y-auto p-3 min-w-0 space-y-2">
            {sections ? (
              sections.map(renderSection)
            ) : visibleRows.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center gap-2 text-center px-6">
                <Compass size={26} className="text-[var(--text-muted)]" />
                <p className="text-sm font-semibold">
                  {searchQuery.trim() ? format(strings.noMacroMatch, { query: searchQuery.trim() }) : strings.noMacroHere}
                </p>
                <p className="text-[11px] text-[var(--text-muted)] max-w-sm">
                  {searchQuery.trim()
                    ? hiddenMatches > 0
                      ? plural(language, hiddenMatches, strings.hiddenMatches)
                      : strings.searchScope
                    : tab === 'hidden'
                    ? strings.emptyHidden
                    : tab === 'unclassified'
                    ? strings.emptyUnclassified
                    : onlyIssues
                      ? strings.emptyNoIssue
                      : strings.emptyHorizon}
                </p>
              </div>
            ) : (
              visibleRows.map(row => (condensedHere ? renderCondensedRow(row) : renderMacroRow(row)))
            )}
          </div>
        )}

        {/* Poignée de répartition (masquée si plein écran ou panneau masqué) */}
        {panelShown && !isPanelExpanded && (
          <div
            role="separator"
            aria-orientation="vertical"
            aria-label={strings.splitLabel}
            onPointerDown={startSplitDrag}
            onDoubleClick={() => setPanelWidth(720)}
            title={strings.splitTitle}
            className="w-1.5 shrink-0 cursor-col-resize transition-colors"
            style={{ background: isDraggingSplit ? 'var(--accent-color)' : 'var(--border-color)' }}
          />
        )}

        {/* The rail of a hidden panel: without it, getting the panel back would
            mean selecting another macro, which no longer brings it back. */}
        {selected && isPanelHidden && (
          <button
            type="button"
            onClick={showPanel}
            className="shrink-0 w-6 flex items-center justify-center border-l border-[var(--border-color)] bg-[var(--bg-secondary)] text-[var(--text-muted)] hover:text-[var(--accent-color)] hover:bg-[var(--accent-light)] cursor-pointer transition-colors"
            title={strings.panel.show}
            aria-label={strings.panel.show}
          >
            <PanelRightOpen size={13} />
          </button>
        )}

        {/* Panneau : vérification des sprints en NOW/NEXT, cadrage en LATER */}
        {selected && panelShown && (
          <aside className="flex flex-col min-h-0 shrink-0 bg-[var(--bg-secondary)]"
            style={{ width: isPanelExpanded ? '100%' : panelWidth, flex: isPanelExpanded ? 1 : undefined }}>
            <div className="px-4 pt-3.5 pb-3 shrink-0 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 mb-1.5 flex-wrap">
                <span className="text-[11px] font-mono font-bold" style={{ color: 'var(--accent-color)' }}>{selected.key}</span>
                {selected.horizon && (
                  <span
                    className="px-1.5 py-0.5 rounded text-[9.5px] font-bold uppercase tracking-[.06em] text-white"
                    style={{ background: HORIZON_META[selected.horizon].color }}
                  >
                    {HORIZON_META[selected.horizon].label}
                  </span>
                )}
                <div className="ml-auto flex items-center gap-1.5 flex-wrap">
                  <button
                    type="button"
                    onClick={toggleExpanded}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--accent-color)] hover:opacity-90 cursor-pointer border border-[var(--accent-color)]/40 bg-[var(--accent-light)] transition-colors"
                    title={isPanelExpanded ? strings.panel.collapseTitle : strings.panel.expandTitle}
                  >
                    {isPanelExpanded ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
                    <span>{isPanelExpanded ? strings.panel.collapse : strings.panel.expand}</span>
                  </button>

                  <button
                    type="button"
                    onClick={hidePanel}
                    className="inline-flex items-center px-1.5 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                    title={strings.panel.hide}
                    aria-label={strings.panel.hide}
                  >
                    <PanelRightClose size={12} />
                  </button>

                  <button
                    type="button"
                    onClick={() => copyMacroLink(selected)}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                    title={format(selected.externalUrl ? strings.panel.copyLinkTitle : strings.panel.copyRefTitle, { key: selected.key })}
                  >
                    {copiedLink ? <Check size={12} className="text-[var(--status-ok)]" /> : <Copy size={12} />}
                    <span>{strings.panel.copy}</span>
                  </button>

                  {selected.externalUrl && (
                    <a href={selected.externalUrl} target="_blank" rel="noreferrer"
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold transition-colors"
                      style={{
                        color: 'var(--status-info)',
                        background: 'rgb(var(--status-info-rgb) / 0.12)',
                        border: '1px solid rgb(var(--status-info-rgb) / 0.32)',
                      }}
                      title={format(strings.panel.openRemoteTitle, { key: selected.key })}>
                      <ExternalLink size={13} />
                      <span>{strings.panel.link}</span>
                    </a>
                  )}
                  {/* Back to the ticket views, filtered on the epic (#630). An
                      epic without a ticket would open an empty list. */}
                  {selected.tasks.length > 0 && (
                    <button
                      type="button"
                      onClick={() => openEpicTickets(selected.key)}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                      title={format(strings.panel.openTicketsTitle, { key: selected.key })}
                    >
                      <ListFilter size={12} />
                      <span>{strings.panel.openTickets}</span>
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      setEditingTitleValue(selected.title)
                      setIsEditingTitle(true)
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                    title={strings.panel.renameTitle}
                  >
                    <Pencil size={12} />
                    <span>{strings.panel.rename}</span>
                  </button>
                  <button
                    type="button"
                    disabled={busyKey === 'migrate' || isMigrating}
                    onClick={() => {
                      const compatible = projects.filter(p => isProjectCompatible(currentProject, p))
                      setMigrateTargetProjectId(compatible[0]?.id || '')
                      setMigrateIncludeTasks(true)
                      setShowMigrateModal(true)
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 transition-colors"
                    title={strings.panel.migrateTitle}
                  >
                    <ArrowRightLeft size={12} />
                    <span>{strings.panel.migrate}</span>
                  </button>
                  <button
                    type="button"
                    disabled={busyKey === 'delete'}
                    onClick={async () => {
                      if (!confirm(format(strings.panel.deleteConfirm, { key: selected.key, title: selected.title }))) return
                      if (!currentProject?.id) return
                      setBusyKey('delete')
                      const ok = await deleteMacro(currentProject.id, selected.key)
                      if (ok) {
                        setMacroMeta(prev => prev.filter(m => m.key !== selected.key))
                        setSelectedKey(null)
                      }
                      setBusyKey(null)
                    }}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-semibold text-[var(--text-muted)] hover:text-rose-400 cursor-pointer border border-[var(--border-color)] hover:border-rose-500/30 transition-colors disabled:opacity-50"
                    title={format(strings.panel.deleteTitle, { key: selected.key })}
                  >
                    <Trash2 size={12} />
                    <span>{strings.panel.delete}</span>
                  </button>
                </div>
              </div>

              <div className="mt-1">
                {isEditingTitle ? (
                  <form
                    onSubmit={async (e) => {
                      e.preventDefault()
                      if (!selected) return
                      const nextTitle = editingTitleValue.trim()
                      if (!nextTitle || nextTitle === selected.title) {
                        setIsEditingTitle(false)
                        return
                      }
                      const targetProjId = currentProject?.id || projects.find(p => p.isDefault)?.id || projects[0]?.id
                      if (!targetProjId) return
                      setBusyKey('editTitle')
                      const updated = await saveMacroMeta(targetProjId, selected.key, { title: nextTitle })
                      if (updated) {
                        setMacroMeta(prev => prev.map(m => m.key === selected.key ? { ...m, title: nextTitle } : m))
                        addToast({ type: 'success', title: format(strings.panel.renamed, { key: selected.key }), description: nextTitle })
                      }
                      setIsEditingTitle(false)
                      setBusyKey(null)
                    }}
                    className="flex items-center gap-2"
                  >
                    <input
                      type="text"
                      autoFocus
                      value={editingTitleValue}
                      onChange={(e) => setEditingTitleValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Escape') {
                          setIsEditingTitle(false)
                        }
                      }}
                      className="flex-1 px-2.5 py-1 text-sm font-bold rounded-xl bg-[var(--bg-primary)] border border-[var(--accent-color)] text-[var(--text-primary)] focus:outline-none"
                    />
                    <button
                      type="submit"
                      disabled={!editingTitleValue.trim() || busyKey === 'editTitle'}
                      className="p-1.5 rounded-lg text-white accent-bg hover:opacity-90 transition-opacity cursor-pointer disabled:opacity-50"
                      title={strings.panel.saveName}
                    >
                      {busyKey === 'editTitle' ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                    </button>
                    <button
                      type="button"
                      onClick={() => setIsEditingTitle(false)}
                      className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                      title={strings.panel.cancel}
                    >
                      <X size={13} />
                    </button>
                  </form>
                ) : (
                  <div className="group flex items-center gap-2">
                    <h2
                      onClick={() => {
                        setEditingTitleValue(selected.title)
                        setIsEditingTitle(true)
                      }}
                      className="text-[16px] font-bold leading-[1.25] cursor-pointer hover:text-[var(--accent-color)] transition-colors"
                      title={strings.panel.clickToRename}
                    >
                      {selected.title}
                    </h2>
                    <button
                      type="button"
                      onClick={() => {
                        setEditingTitleValue(selected.title)
                        setIsEditingTitle(true)
                      }}
                      className="opacity-0 group-hover:opacity-100 p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-all cursor-pointer"
                      title={strings.panel.renameShortTitle}
                    >
                      <Pencil size={12} />
                    </button>
                  </div>
                )}
              </div>

              {selected.meta?.status && (
                <div className="mt-1.5 text-[10px] font-mono" style={{ color: selected.closed ? 'var(--status-ok)' : 'var(--text-muted)' }}>
                  {format(selected.closed ? strings.panel.statusClosed : strings.panel.statusOpen, { status: selected.meta.status })}
                </div>
              )}

              {/* The epic's own priority and quarter (#627) and its readiness
                  (#633), stored here first, then written as labels when the
                  tracker can carry them. */}
              <div className="mt-2.5 flex items-start gap-4 flex-wrap">
                <div>
                  <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1">
                    {strings.axes.priorityLabel}
                  </div>
                  <div className="flex items-center gap-1" role="group" aria-label={strings.axes.priorityLabel}>
                    {EPIC_PRIORITIES.map(p => {
                      const active = selected.priority === p
                      const prio = PRIORITY_META[EPIC_PRIORITY_LEVEL[p]]
                      return (
                        <button
                          key={p}
                          type="button"
                          aria-pressed={active}
                          disabled={busyKey === 'axes'}
                          onClick={() => saveAxes(selected.key, { priority: active ? '' : p })}
                          className="px-1.5 py-0.5 rounded text-[10.5px] font-bold border cursor-pointer disabled:opacity-60"
                          style={{
                            color: prio.color,
                            background: active ? prio.bg : 'transparent',
                            borderColor: active ? prio.color : 'var(--border-color)',
                          }}
                          title={strings.axes.priorityTitle}
                        >
                          {epicPriorityLabel(p)}
                        </button>
                      )
                    })}
                    {selected.priority && (
                      <button
                        type="button"
                        disabled={busyKey === 'axes'}
                        onClick={() => saveAxes(selected.key, { priority: '' })}
                        className="p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
                        title={strings.axes.clearPriority}
                        aria-label={strings.axes.clearPriority}
                      >
                        <X size={11} />
                      </button>
                    )}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1">
                    {strings.readiness.label}
                  </div>
                  <div className="flex items-center gap-1" role="group" aria-label={strings.readiness.label}>
                    {EPIC_READINESS.map(level => {
                      const active = selected.readiness === level
                      const isSuggestion = !selected.readiness && selected.suggestedReadiness === level
                      const meta = READINESS_META[level]
                      return (
                        <button
                          key={level}
                          type="button"
                          aria-pressed={active}
                          disabled={busyKey === 'axes'}
                          onClick={() => saveAxes(selected.key, { readiness: active ? '' : level })}
                          className="px-1.5 py-0.5 rounded text-[10.5px] font-bold border cursor-pointer disabled:opacity-60"
                          style={{
                            color: meta.color,
                            background: active ? meta.bg : 'transparent',
                            borderColor: active ? meta.color : isSuggestion ? meta.border : 'var(--border-color)',
                            borderStyle: isSuggestion ? 'dashed' : 'solid',
                          }}
                          title={active ? strings.readiness.chipTitle : isSuggestion ? strings.readiness.suggestedTitle : undefined}
                        >
                          {strings.readiness.levels[level]}
                          {isSuggestion && ' ?'}
                        </button>
                      )
                    })}
                  </div>
                </div>
                <div>
                  <label
                    htmlFor="roadmap-quarter"
                    className="block text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1"
                  >
                    {strings.axes.quarterLabel}
                  </label>
                  <div className="flex items-center gap-1">
                    <input
                      id="roadmap-quarter"
                      type="text"
                      value={quarterDraft}
                      placeholder={strings.axes.quarterPlaceholder}
                      aria-invalid={quarterError ? true : undefined}
                      onChange={e => setQuarterDraft(e.target.value)}
                      onKeyDown={e => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          commitQuarter(selected.key, selected.quarter)
                        } else if (e.key === 'Escape') {
                          setQuarterEdit(null)
                        }
                      }}
                      onBlur={() => commitQuarter(selected.key, selected.quarter)}
                      className="w-[150px] px-2 py-0.5 text-[11px] font-mono rounded-md bg-[var(--bg-primary)] border text-[var(--text-primary)] focus:outline-none"
                      style={{ borderColor: quarterError ? 'var(--status-danger)' : 'var(--border-color)' }}
                    />
                    {selected.quarter && (
                      <button
                        type="button"
                        disabled={busyKey === 'axes'}
                        onClick={() => {
                          setQuarterEdit(null)
                          saveAxes(selected.key, { quarter: '' })
                        }}
                        className="p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
                        title={strings.axes.clearQuarter}
                        aria-label={strings.axes.clearQuarter}
                      >
                        <X size={11} />
                      </button>
                    )}
                  </div>
                  {quarterError && (
                    <div className="mt-1 text-[10px]" style={{ color: 'var(--status-danger)' }} role="alert">
                      {quarterError}
                    </div>
                  )}
                </div>
              </div>
              {/* The priority and the quarter follow axesWritable, which only an
                  older server leaves out: labelsWritable says the same there. */}
              {(selected.meta?.axesWritable ?? selected.meta?.labelsWritable) === false && (
                <div className="mt-1.5 text-[10px] text-[var(--text-muted)]">
                  {selected.meta?.foreign
                    ? format(strings.axes.keptLocalForeign, { origin: selected.meta.origin || '' })
                    : strings.axes.keptLocal}
                </div>
              )}
            </div>

            <div className="flex-1 overflow-y-auto px-4 pt-3.5 pb-7 flex flex-col gap-4">
              {selected.meta?.foreign && (
                <div
                  className="rounded-md border px-2.5 py-2 text-[11px] flex gap-2 items-start"
                  style={{ borderColor: 'var(--border-color)', background: 'var(--bg-tertiary)' }}
                  role="note"
                >
                  <Lock size={12} className="shrink-0 mt-0.5 text-[var(--text-muted)]" />
                  <div>
                    <div className="font-semibold text-[var(--text-primary)]">{strings.origins.readOnly}</div>
                    <div className="text-[var(--text-secondary)]">
                      {format(strings.origins.readOnlyBody, { origin: selected.meta.origin || '' })}
                    </div>
                  </div>
                </div>
              )}
              {/* The free labels of a roadmap project's epic are its team's, and
                  Sectile never writes them (#632). */}
              {currentProject && !selected.meta?.foreign && (
                <EpicLabelEditor key={selected.key} project={currentProject} row={selected} suggestions={labelSuggestions} />
              )}
              {/* La clé porte l'axe, et ce n'est pas cosmétique : les deux vues
                  montent le même composant au même endroit de l'arbre, donc
                  React le réutiliserait en ne changeant que la prop. Son état
                  interne survivrait au passage d'une vue à l'autre, si bien
                  qu'un groupe nommé en Phases apparaîtrait dans les Objectifs,
                  avec la recherche et les groupes dépliés de l'autre axe. */}
              {displayMode === 'phases' ? (
                <MacroLabelGroups key="phase" axis="phase" tasks={selected.tasks} />
              ) : displayMode === 'goals' ? (
                <MacroLabelGroups key="goal" axis="goal" tasks={selected.tasks} />
              ) : displayMode === 'execution' ? (
                <>
                  {/* Prototypage : ajouter une story a la volée, ou pousser un
                      ticket existant dans la macro. */}
                  <div>
                    <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                      {strings.execution.compose}
                    </div>

                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={newStory}
                        onChange={e => setNewStory(e.target.value)}
                        onKeyDown={async e => {
                          if (e.key === 'Enter' && newStory.trim() && currentProject?.id) {
                            e.preventDefault()
                            setBusyKey('new')
                            await createStoryUnderMacro(currentProject.id, selected.key, newStory.trim())
                            setNewStory('')
                            setBusyKey(null)
                          }
                        }}
                        placeholder={format(strings.execution.newStoryPlaceholder, { key: selected.key })}
                        className="flex-1 px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <button
                        type="button"
                        disabled={!newStory.trim() || busyKey === 'new' || !currentProject?.id}
                        onClick={async () => {
                          setBusyKey('new')
                          await createStoryUnderMacro(currentProject!.id, selected.key, newStory.trim())
                          setNewStory('')
                          setBusyKey(null)
                        }}
                        className="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0"
                      >
                        <Plus size={12} /> {busyKey === 'new' ? '…' : strings.execution.create}
                      </button>
                    </div>

                    <div className="mt-2">
                      <input
                        type="text"
                        value={attachQuery}
                        onChange={e => setAttachQuery(e.target.value)}
                        placeholder={strings.execution.attachPlaceholder}
                        className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      {attachCandidates.length > 0 && (
                        <div className="mt-1.5 flex flex-col gap-1">
                          {attachCandidates.map(candidate => (
                            <button
                              key={candidate.id}
                              type="button"
                              disabled={busyKey === candidate.id}
                              onClick={async () => {
                                setBusyKey(candidate.id)
                                await setTaskMacro(candidate.id, selected.key)
                                setBusyKey(null)
                                setAttachQuery('')
                              }}
                              className="flex items-center gap-2 px-2 py-1.5 rounded-lg text-left bg-[var(--bg-primary)] border border-[var(--border-color)] hover:border-[var(--accent-color)]/50 cursor-pointer disabled:opacity-50"
                            >
                              <span className="text-[10px] font-mono font-bold shrink-0" style={{ color: 'var(--status-info)' }}>
                                {candidate.key}
                              </span>
                              <span className="text-[11px] truncate flex-1 text-[var(--text-secondary)]">{candidate.title}</span>
                              {candidate.parentKey && (
                                <span className="text-[9px] font-mono shrink-0 text-[var(--text-muted)]" title={strings.execution.currentMacroTitle}>
                                  {candidate.parentKey} →
                                </span>
                              )}
                              <Plus size={11} className="shrink-0 text-[var(--text-muted)]" />
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <div className="px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <div className="text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">
                        {tab === 'now' ? strings.execution.inActiveSprint : strings.execution.inUpcomingSprint}
                      </div>
                      <div className="text-[15px] font-bold mt-0.5" style={{ color: 'var(--status-ok)' }}>
                        {tab === 'now' ? selected.inActiveSprint.length : selected.inFutureSprint.length}
                        <span className="text-[11px] font-normal text-[var(--text-muted)]"> {plural(language, selected.open.length, strings.execution.openSuffix)}</span>
                      </div>
                    </div>
                    <div className="px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <div className="text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">{strings.onlyIssues}</div>
                      <div className="text-[15px] font-bold mt-0.5"
                        style={{ color: placementIssues(selected, horizonOfTab).length ? 'var(--status-danger)' : 'var(--status-ok)' }}>
                        {placementIssues(selected, horizonOfTab).length}
                      </div>
                    </div>
                  </div>

                  <div>
                    <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                      {plural(language, selected.open.length, strings.execution.storiesHeading)}
                    </div>
                    <div className="flex flex-col gap-1.5">
                      {selected.open.length === 0 && (
                        <p className="text-[11px] text-[var(--text-muted)]">
                          {strings.execution.noOpenStory}
                        </p>
                      )}
                      {orderedOpen.map((task, index) => {
                        const state = placementOf(task, selected, horizonOfTab)
                        const meta = PLACEMENT_META[state]
                        const beyondCut = cutAt >= 0 && index >= cutAt
                        const sprintLabel = sprintLabelOf(task, strings.execution.noSprint)
                        const startsSprint = index === 0 || sprintLabelOf(orderedOpen[index - 1], strings.execution.noSprint) !== sprintLabel
                        return (
                          <React.Fragment key={task.id}>
                            {startsSprint && (
                              <div className="flex items-center gap-1.5 mt-1 first:mt-0 pl-[20px]">
                                <span className="text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">
                                  {sprintLabel}
                                </span>
                                <span className="flex-1 h-px bg-[var(--border-color)]" />
                              </div>
                            )}
                          <div
                            ref={el => { rowRefs.current[index] = el }}
                            className="grid gap-1.5"
                            style={{ gridTemplateColumns: '14px 1fr' }}
                          >
                            {/* Gouttière de coupe : le cran est à la frontière
                                haute de la ligne, donc l'alignement suit les
                                lignes quel que soit leur hauteur. */}
                            <div className="relative">
                              <span
                                className="absolute left-1/2 top-0 bottom-0 w-px -translate-x-1/2"
                                style={{ background: beyondCut ? 'var(--accent-color)' : 'var(--border-color)' }}
                              />
                              <button
                                type="button"
                                onPointerDown={e => {
                                  e.preventDefault()
                                  setCutAt(index)
                                  setDraggingCut(true)
                                }}
                                onKeyDown={e => {
                                  if (e.key === 'ArrowUp') { e.preventDefault(); setCutAt(Math.max(0, index - 1)) }
                                  if (e.key === 'ArrowDown') {
                                    e.preventDefault()
                                    const next = index + 1
                                    setCutAt(next >= orderedOpen.length ? -1 : next)
                                  }
                                }}
                                className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full cursor-ns-resize"
                                style={{
                                  top: 0,
                                  width: cutAt === index ? 11 : 7,
                                  height: cutAt === index ? 11 : 7,
                                  background: cutAt === index ? 'var(--accent-color)' : 'var(--bg-secondary)',
                                  border: `1px solid ${cutAt === index ? 'var(--accent-color)' : 'var(--border-color)'}`,
                                }}
                                title={plural(language, orderedOpen.length - index, strings.execution.cutHereTitle)}
                                aria-label={format(strings.execution.cutBefore, { key: task.key })}
                              />
                            </div>

                          <div className="px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border"
                            style={{
                              borderColor: beyondCut
                                ? 'var(--accent-color)'
                                : state === 'ok'
                                  ? 'var(--border-color)'
                                  : meta.border,
                              opacity: cutAt >= 0 && !beyondCut ? 0.55 : 1,
                            }}>
                            <div className="flex items-center gap-2">
                              <button
                                type="button"
                                onClick={() => setChecked(prev => ({ ...prev, [task.id]: !prev[task.id] }))}
                                className="w-3.5 h-3.5 rounded shrink-0 flex items-center justify-center cursor-pointer"
                                style={{
                                  background: checked[task.id] ? 'var(--accent-color)' : 'transparent',
                                  border: `1px solid ${checked[task.id] ? 'var(--accent-color)' : 'var(--border-color)'}`,
                                }}
                                title={strings.execution.selectToMove}
                              >
                                {checked[task.id] && <Check size={10} className="text-white" />}
                              </button>
                              <button type="button" onClick={() => setSelectedTask(task)}
                                className="text-[10.5px] font-mono font-bold hover:underline cursor-pointer"
                                style={{ color: 'var(--status-info)' }}>
                                {task.key}
                              </button>
                              {sprintOptions.length > 0 ? (
                                <div className="ml-auto shrink-0 w-[165px]">
                                  <LookupField
                                    value={task.sprint || ''}
                                    icon={<CalendarRange size={10} />}
                                    placeholder={strings.placement[state]}
                                    clearLabel={t.planning.triage.backlogNoSprint}
                                    onSearch={searchSprint}
                                    onPick={option => setTaskSprint(task.id, option?.id || '', option?.label)}
                                  />
                                </div>
                              ) : (
                                <span className="text-[9px] px-1 rounded font-mono ml-auto shrink-0 truncate max-w-[150px]"
                                  style={{ color: meta.color, background: meta.bg, border: `1px solid ${meta.border}` }}>
                                  {task.sprint || strings.placement[state]}
                                </span>
                              )}

                              <button
                                type="button"
                                disabled={busyKey === task.id}
                                onClick={async () => {
                                  setBusyKey(task.id)
                                  await setTaskMacro(task.id, '')
                                  setBusyKey(null)
                                }}
                                className="p-1 rounded text-[var(--text-muted)] hover:text-rose-400 cursor-pointer disabled:opacity-50"
                                title={format(strings.execution.removeFromMacro, { key: task.key })}
                              >
                                <X size={12} />
                              </button>
                            </div>
                            <div className="text-[11px] mt-1 leading-snug text-[var(--text-secondary)]">{task.title}</div>
                            {state !== 'ok' && (
                              <div className="text-[9.5px] mt-1 font-mono" style={{ color: meta.color }}>
                                {state === 'missing' && strings.execution.stateMissing}
                                {state === 'stale' && strings.execution.stateStale}
                                {state === 'other-horizon' &&
                                  (tab === 'now' ? strings.execution.stateFutureNotActive : strings.execution.stateActiveNotFuture)}
                              </div>
                            )}
                          </div>
                          </div>
                          </React.Fragment>
                        )
                      })}
                    </div>
                  </div>

                  {/* Résumé de la coupe. Le curseur lui-même est vertical, dans
                      la gouttière de la liste : couper se lit alors comme une
                      ligne tracée entre deux tickets, et non comme un rail
                      horizontal détaché de ce qu'il découpe. */}
                  {orderedOpen.length > 1 && (
                    <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <Scissors size={11} style={{ color: cutAt >= 0 ? 'var(--accent-color)' : 'var(--text-muted)' }} />
                      <span className="text-[10px] text-[var(--text-secondary)]">
                        {cutAt < 0
                          ? strings.execution.cutHint
                          : plural(language, orderedOpen.length - cutAt, strings.execution.cutSummary, { sprint: sprintLabelOf(orderedOpen[cutAt], strings.execution.noSprint) })}
                      </span>
                      {cutAt >= 0 && (
                        <button
                          type="button"
                          onClick={() => setCutAt(-1)}
                          className="ml-auto text-[10px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
                        >
                          {strings.execution.cancelCut}
                        </button>
                      )}
                    </div>
                  )}

                  {/* Planification : la même sélection sert à replanifier, ce qui
                      est l'autre moitié du travail sur un épic trop gros. Couper
                      change de contenant, changer de sprint change la date. */}
                  {cutIds.length > 0 && sprintOptions.length > 0 && (
                    <div className="p-2.5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-primary)]">
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] mb-1.5 text-[var(--text-muted)]">
                        {plural(language, cutIds.length, strings.execution.rescheduleHeading)}
                      </div>
                      <div className="flex items-center gap-2">
                        <div className="flex-1">
                          <LookupField
                            value={sprintTarget.name}
                            icon={<CalendarRange size={11} />}
                            placeholder={strings.execution.targetSprintPlaceholder}
                            clearLabel={strings.execution.backlogRemove}
                            onSearch={searchSprint}
                            onPick={option =>
                              setSprintTarget({ id: option?.id || '', name: option?.label || t.planning.triage.backlog })
                            }
                          />
                        </div>
                        <button
                          type="button"
                          disabled={!sprintTarget.name || busyKey === 'sprint'}
                          onClick={async () => {
                            setBusyKey('sprint')
                            await setTasksSprint(
                              currentProject!.id,
                              cutIds,
                              sprintTarget.id,
                              sprintTarget.name
                            )
                            setChecked({})
                            setCutAt(-1)
                            setSprintTarget({ id: '', name: '' })
                            setBusyKey(null)
                          }}
                          className="px-2.5 py-1.5 rounded-lg text-[11px] font-bold cursor-pointer shrink-0 disabled:opacity-40"
                          style={{ color: 'var(--status-ok)', background: 'rgb(var(--status-ok-rgb) / 0.12)', border: '1px solid rgb(var(--status-ok-rgb) / 0.32)' }}
                          title={strings.execution.rescheduleTitle}
                        >
                          {busyKey === 'sprint' ? '…' : strings.execution.reschedule}
                        </button>
                      </div>
                    </div>
                  )}

                  {/* Découpe : les stories cochées, ou celles au delà du
                      curseur, quittent la macro pour une autre. */}
                  {cutIds.length > 0 && (
                    <div className="p-2.5 rounded-xl border" style={{ background: 'var(--accent-light)', borderColor: 'rgb(var(--accent-rgb) / 0.4)' }}>
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] mb-1.5" style={{ color: 'var(--accent-color)' }}>
                        {plural(language, cutIds.length, strings.execution.cutHeading)}
                      </div>
                      <div className="flex items-center gap-2">
                        <div className="flex-1">
                          <LookupField
                            value={moveTarget}
                            icon={<Target size={11} />}
                            placeholder={strings.execution.existingMacroPlaceholder}
                            allowClear={false}
                            emptyHint={t.planning.triage.noMacroMatch}
                            onSearch={async query => {
                              const q = query.trim().toLowerCase()
                              return allRows
                                .filter(r => r.key !== selected.key && !r.closed)
                                .filter(r => !q || r.key.toLowerCase().includes(q) || r.title.toLowerCase().includes(q))
                                .slice(0, 40)
                                .map(r => ({ id: r.key, label: r.key, sublabel: r.title }))
                            }}
                            onPick={option => setMoveTarget(option?.id || '')}
                          />
                        </div>
                        <button
                          type="button"
                          disabled={!moveTarget || busyKey === 'move'}
                          onClick={async () => {
                            setBusyKey('move')
                            await moveTasksToMacro(currentProject!.id, cutIds, moveTarget)
                            setChecked({})
                            setCutAt(-1)
                            setBusyKey(null)
                          }}
                          className="px-2.5 py-1.5 rounded-lg text-[11px] font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0"
                        >
                          {strings.execution.move}
                        </button>
                      </div>
                      <div className="flex items-center gap-2 mt-2">
                        <input
                          type="text"
                          value={newMacroTitle}
                          onChange={e => setNewMacroTitle(e.target.value)}
                          placeholder={strings.execution.newMacroPlaceholder}
                          className="flex-1 px-2 py-1.5 text-[11px] rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                        />
                        <button
                          type="button"
                          disabled={!newMacroTitle.trim() || busyKey === 'move'}
                          onClick={async () => {
                            setBusyKey('move')
                            await moveTasksToMacro(currentProject!.id, cutIds, '', newMacroTitle.trim())
                            setChecked({})
                            setCutAt(-1)
                            setNewMacroTitle('')
                            setBusyKey(null)
                          }}
                          title={strings.execution.createAndMoveTitle}
                          className="px-2.5 py-1.5 rounded-lg text-[11px] font-bold cursor-pointer shrink-0 disabled:opacity-40"
                          style={{ color: 'var(--status-info)', background: 'rgb(var(--status-info-rgb) / 0.12)', border: '1px solid rgb(var(--status-info-rgb) / 0.32)' }}
                        >
                          {busyKey === 'move' ? '…' : strings.execution.createAndCut}
                        </button>
                      </div>
                    </div>
                  )}

</>
              ) : (
                <>
                  {/* Mode Framing : description de la macro, commentaire framing et checklist TODO / tickets */}
                  <div className="space-y-3">
                    {/* Textbox 1 : Description de la Macro */}
                    <div className="rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] overflow-hidden">
                      <div
                        className="flex items-center justify-between p-3 cursor-pointer select-none bg-[var(--bg-tertiary)]/40 hover:bg-[var(--bg-tertiary)]/70 transition-colors"
                        onClick={() => setIsDescExpanded(prev => !prev)}
                      >
                        <div className="flex items-center gap-1.5 min-w-0">
                          {isDescExpanded ? <ChevronDown size={14} className="text-[var(--accent-color)] shrink-0" /> : <ChevronRight size={14} className="text-[var(--text-muted)] shrink-0" />}
                          <span className="text-[11px] font-bold uppercase tracking-[.08em] text-[var(--text-primary)] shrink-0">
                            {strings.framing.descriptionHeading}
                          </span>
                          {!isDescExpanded && draftDescription.trim() && (
                            <span className="text-[10px] text-[var(--text-muted)] italic truncate max-w-[280px]">
                              - {draftDescription.trim().slice(0, 50)}…
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0" onClick={e => e.stopPropagation()}>
                          <button
                            type="button"
                            disabled={isRefining}
                            onClick={handleRefineMacro}
                            className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-orange-300 bg-orange-500/10 border border-orange-500/30 hover:bg-orange-500/20 disabled:opacity-50 cursor-pointer"
                            title={strings.framing.refineTitle}
                          >
                            {isRefining ? <Loader2 size={10} className="animate-spin text-orange-400" /> : <Sparkles size={10} className="text-orange-400" />}
                            <span>{strings.framing.refine}</span>
                          </button>
                          {currentProject?.id && (
                            <MacroRealignButton
                              projectId={currentProject.id}
                              macroKey={selected.key}
                              onError={message => addToast({ type: 'error', title: strings.framing.realignFailed, description: message })}
                              onLaunched={message => addToast({ type: 'success', title: strings.framing.realignLaunched, description: message })}
                            />
                          )}
                          {draftDirty && (
                            <button
                              type="button"
                              onClick={() => {
                                persist(selected.key, { description: draftDescription })
                                setDraftDirty(false)
                              }}
                              className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-white accent-bg cursor-pointer"
                            >
                              <Save size={10} /> {strings.framing.save}
                            </button>
                          )}
                        </div>
                      </div>

                      {isDescExpanded && (
                        <div className="p-3 pt-1 border-t border-[var(--border-color)]">
                          <MarkdownEditor
                            value={draftDescription}
                            onChange={value => {
                              setDraftDescription(value)
                              setDraftDirty(true)
                            }}
                            minHeight={120}
                            placeholder={strings.framing.descriptionPlaceholder}
                            maximizable
                            maximizeTitle={strings.framing.descriptionHeading}
                          />
                        </div>
                      )}
                    </div>

                    {/* Textbox 2 : Commentaire Framing / Notes de Cadrage */}
                    <div className="rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] overflow-hidden">
                      <div
                        className="flex items-center justify-between p-3 cursor-pointer select-none bg-[var(--bg-tertiary)]/40 hover:bg-[var(--bg-tertiary)]/70 transition-colors"
                        onClick={() => setIsFramingExpanded(prev => !prev)}
                      >
                        <div className="flex items-center gap-1.5 min-w-0">
                          {isFramingExpanded ? <ChevronDown size={14} className="text-amber-400 shrink-0" /> : <ChevronRight size={14} className="text-[var(--text-muted)] shrink-0" />}
                          <span className="text-[11px] font-bold uppercase tracking-[.08em] text-[var(--text-primary)] flex items-center gap-1 shrink-0">
                            <MessageSquare size={12} className="text-amber-400" />
                            <span>{strings.framing.commentHeading}</span>
                          </span>
                          {!isFramingExpanded && draftFramingComment.trim() && (
                            <span className="text-[10px] text-[var(--text-muted)] italic truncate max-w-[280px]">
                              - {draftFramingComment.trim().slice(0, 50)}…
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0" onClick={e => e.stopPropagation()}>
                          {draftFramingDirty && (
                            <button
                              type="button"
                              onClick={() => {
                                persist(selected.key, { framingComment: draftFramingComment })
                                setDraftFramingDirty(false)
                              }}
                              className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-white accent-bg cursor-pointer"
                            >
                              <Save size={10} /> {strings.framing.save}
                            </button>
                          )}
                        </div>
                      </div>

                      {isFramingExpanded && (
                        <div className="p-3 pt-1 border-t border-[var(--border-color)]">
                          <MarkdownEditor
                            value={draftFramingComment}
                            onChange={value => {
                              setDraftFramingComment(value)
                              setDraftFramingDirty(true)
                            }}
                            minHeight={100}
                            placeholder={strings.framing.commentPlaceholder}
                            maximizable
                            maximizeTitle={strings.framing.commentHeading}
                          />
                        </div>
                      )}
                    </div>
                  </div>

                  {(() => {
                    // A running batch locks this macro's slicing (#634).
                    const slicingLocked = batchMacroKey === selected.key
                    const selectable = selectableTodoIds(todosOf(selected))
                    const selectedCount = selectable.filter(id => selectedTodoIds.has(id)).length
                    const allSelected = selectable.length > 0 && selectedCount === selectable.length
                    const report = batchReport?.macroKey === selected.key ? batchReport.batch : null
                    const outcomeOf = (todoId: string) => report?.results.find(r => r.todoId === todoId)
                    const originLabel = (todo: MacroTodo) => {
                      const origin = todoOrigin(todo)
                      switch (origin.kind) {
                        case 'tasks': return strings.framing.originTasks
                        case 'spec': return strings.framing.originSpec
                        case 'stories': return strings.framing.originStories
                        case 'manual': return strings.framing.originManual
                        default: return origin.raw
                      }
                    }
                    return (
                  <div>
                    <div className="flex items-center gap-2 mb-1.5">
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] flex-1">
                        {format(strings.framing.checklistHeading, { done: todosOf(selected).filter(t => t.done).length, total: todosOf(selected).length })}
                      </div>
                      {selectable.length > 0 && (
                        <>
                          <button
                            type="button"
                            disabled={slicingLocked}
                            onClick={() => setSelectedTodoIds(allSelected ? new Set() : new Set(selectable))}
                            className="text-[10px] font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
                          >
                            {allSelected ? strings.framing.deselectAll : strings.framing.selectAll}
                          </button>
                          <button
                            type="button"
                            disabled={selectedCount === 0 || batchMacroKey !== null}
                            onClick={() => runStoryBatch(selected)}
                            className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-white accent-bg disabled:opacity-40 cursor-pointer"
                            title={strings.framing.createStoriesTitle}
                          >
                            {slicingLocked && <Loader2 size={10} className="animate-spin" />}
                            {slicingLocked ? strings.framing.creatingStories : format(strings.framing.createStories, { count: selectedCount })}
                          </button>
                        </>
                      )}
                    </div>
                    {report && (
                      <div className="text-[10.5px] mb-1.5 font-semibold" style={{ color: report.failed > 0 ? 'var(--status-warn)' : 'var(--status-ok)' }}>
                        {batchSummary(language, report, {
                          created: strings.framing.batchCreated,
                          skipped: strings.framing.batchSkipped,
                          failed: strings.framing.batchFailed,
                        })}
                      </div>
                    )}
                    <div className="flex flex-col gap-1.5">
                      {todosOf(selected).map(todo => (
                        <div key={todo.id} className="flex items-start gap-2 px-2.5 py-2 rounded-lg bg-[var(--bg-primary)] border"
                          style={{ borderColor: todo.done ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)' }}>
                          {/* The batch selection, a native box so it never reads
                              as the done checkbox beside it. An attached line
                              keeps the room, so the texts stay aligned. */}
                          {todo.storyKey ? (
                            <span className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
                          ) : (
                            <input
                              type="checkbox"
                              checked={selectedTodoIds.has(todo.id)}
                              disabled={slicingLocked}
                              onChange={() =>
                                setSelectedTodoIds(prev => {
                                  const next = new Set(prev)
                                  if (next.has(todo.id)) next.delete(todo.id)
                                  else next.add(todo.id)
                                  return next
                                })
                              }
                              aria-label={format(strings.framing.selectTodo, { todo: todo.text })}
                              className="w-3.5 h-3.5 mt-0.5 shrink-0 cursor-pointer disabled:opacity-50"
                              style={{ accentColor: 'var(--accent-color)' }}
                            />
                          )}
                          <button
                            type="button"
                            disabled={slicingLocked}
                            onClick={() =>
                              persist(selected.key, {
                                todos: todosOf(selected).map(t => (t.id === todo.id ? { ...t, done: !t.done } : t)),
                              })
                            }
                            className="w-3.5 h-3.5 mt-0.5 rounded shrink-0 flex items-center justify-center cursor-pointer disabled:opacity-50"
                            style={{
                              background: todo.done ? 'var(--accent-color)' : 'transparent',
                              border: `1px solid ${todo.done ? 'var(--accent-color)' : 'var(--border-color)'}`,
                            }}
                            title={todo.done ? strings.framing.reopen : strings.framing.check}
                          >
                            {todo.done && <Check size={10} className="text-white" />}
                          </button>
                          <div className="flex-1 min-w-0 flex flex-col gap-0.5">
                            <span className="text-[11.5px] leading-snug"
                              style={{
                                color: todo.done ? 'var(--text-muted)' : 'var(--text-primary)',
                                textDecoration: todo.done ? 'line-through' : 'none',
                              }}>
                              {todo.text}
                            </span>
                            {/* Second row: where the line came from, and what the
                                last batch did with it. */}
                            <div className="flex flex-wrap items-center gap-1.5">
                              <span
                                className="text-[9px] px-1.5 py-px rounded-full border border-[var(--border-color)] text-[var(--text-muted)]"
                                title={todoOrigin(todo).entry || format(strings.framing.originTitle, { origin: originLabel(todo) })}
                              >
                                {originLabel(todo)}
                              </span>
                              {(() => {
                                const outcome = outcomeOf(todo.id)
                                if (!outcome) return null
                                return (
                                  <>
                                    {outcome.status === 'skipped' && (
                                      <span className="text-[10px] text-[var(--text-muted)]">
                                        {format(strings.framing.lineSkipped, { key: outcome.storyKey || '' })}
                                      </span>
                                    )}
                                    {outcome.status === 'failed' && (
                                      <span className="text-[10px] text-rose-400">{outcome.error}</span>
                                    )}
                                    {outcome.notice && (
                                      <span className="text-[10px] text-amber-400">{outcome.notice}</span>
                                    )}
                                  </>
                                )
                              })()}
                            </div>
                          </div>
                          {currentProject && (() => {
                            // Where the line's story lands: the macro's project by
                            // default, or another project of the same tracker
                            // instance, where the epic can still be its parent.
                            // Its roadmap projects join them (#632): the story is then
                            // created in that Jira project and stays there.
                            const options = targetProjectOptions(currentProject, projects, { jiraUrl: settings.jiraUrl, githubApiUrl: settings.githubApiUrl, gitlabUrl: settings.gitlabUrl, gitlabProject: settings.gitlabProject })
                            const remoteOptions = roadmapTargetOptions(currentProject)
                            const saved = targetPickerValue(todo, currentProject.id)
                            const savedRemote = saved.startsWith(TRACKER_TARGET_PREFIX) ? saved.slice(TRACKER_TARGET_PREFIX.length) : ''
                            const savedName = savedRemote || projects.find(p => p.id === saved)?.name || saved
                            const invalid = saved !== '' && (savedRemote ? !remoteOptions.includes(savedRemote) : !options.some(p => p.id === saved))
                            if (todo.storyKey) {
                              // Where the story was created, read-only; worth saying only
                              // where another project could have received it.
                              return saved || options.length > 0 || remoteOptions.length > 0 ? (
                                <span className="text-[9.5px] px-1.5 py-0.5 rounded shrink-0 text-[var(--text-muted)] border border-[var(--border-color)]" title={strings.framing.storyProjectTitle}>
                                  {saved ? savedName : currentProject.name}
                                </span>
                              ) : null
                            }
                            if (options.length === 0 && remoteOptions.length === 0 && !saved) return null
                            return (
                              <select
                                aria-label={format(strings.framing.targetProjectLabel, { todo: todo.text })}
                                value={saved}
                                disabled={slicingLocked}
                                onChange={e =>
                                  persist(selected.key, {
                                    todos: todosOf(selected).map(t => (t.id === todo.id ? applyTargetPickerValue(t, e.target.value) : t)),
                                  })
                                }
                                className={`text-[9.5px] max-w-[120px] px-1 py-0.5 rounded shrink-0 bg-[var(--bg-secondary)] border cursor-pointer ${invalid ? 'border-rose-500 text-rose-300' : 'border-[var(--border-color)] text-[var(--text-secondary)]'}`}
                                title={invalid ? strings.framing.incompatibleProjectTitle : strings.framing.targetProjectTitle}
                              >
                                <option value="">{currentProject.name}</option>
                                {options.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
                                {remoteOptions.length > 0 && (
                                  <optgroup label={strings.framing.trackerProjectsGroup}>
                                    {remoteOptions.map(key => (
                                      <option key={key} value={TRACKER_TARGET_PREFIX + key}>
                                        {format(strings.framing.trackerProjectOption, { key })}
                                      </option>
                                    ))}
                                  </optgroup>
                                )}
                                {invalid && <option value={saved}>{format(strings.framing.incompatibleOption, { name: savedName })}</option>}
                              </select>
                            )
                          })()}
                          {todo.storyKey ? (
                            <>
                            <button
                              type="button"
                              onClick={() => {
                                const created = tasks.find(t => t.key === todo.storyKey)
                                if (created) setSelectedTask(created)
                              }}
                              className="text-[9.5px] font-mono font-bold px-1.5 py-0.5 rounded shrink-0 cursor-pointer"
                              style={{
                                color: 'var(--status-ok)',
                                background: 'rgb(var(--status-ok-rgb) / 0.13)',
                                border: '1px solid rgb(var(--status-ok-rgb) / 0.32)',
                              }}
                              title={format(
                                // A story of a roadmap project stays in Jira (#632),
                                // which its key's project says, whatever is loaded.
                                currentProject && roadmapTargetOptions(currentProject).includes(macroOrigin(todo.storyKey))
                                  ? strings.framing.storyStaysInTracker
                                  : strings.framing.storyCreated,
                                { key: todo.storyKey }
                              )}
                            >
                              {todo.storyKey}
                            </button>
                            {/* Le lien vers le tracker, distinct de l'ouverture
                                dans Sectile : consulter la fiche et aller
                                commenter le ticket ne sont pas le même geste. */}
                            {(() => {
                              // A story of a roadmap project was never imported
                              // (#632): its page is built from the Jira site.
                              const created = tasks.find(t => t.key === todo.storyKey)
                              const jiraSite = currentProject?.issueTracker === 'jira' ? (currentProject.trackerUrl || settings.jiraUrl || '').replace(/\/+$/, '') : ''
                              const href = created?.externalUrl || (!created && jiraSite ? `${jiraSite}/browse/${todo.storyKey}` : '')
                              return href ? (
                                <a
                                  href={href}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="shrink-0 text-[var(--text-muted)] hover:text-[var(--accent-color)] transition-colors"
                                  title={format(t.planning.macro.openOnTracker, { key: todo.storyKey })}
                                >
                                  <ExternalLink size={10} />
                                </a>
                              ) : null
                            })()}
                            </>
                          ) : (
                            <button
                              type="button"
                              disabled={creatingTodoId === todo.id || slicingLocked}
                              onClick={async () => {
                                setCreatingTodoId(todo.id)
                                const result = await createStoryFromMacroTodo(currentProject!.id, selected.key, todo.id)
                                setCreatingTodoId(null)
                                if (result?.macro) {
                                  setMacroMeta(prev => [...prev.filter(m => m.key !== result.macro!.key), result.macro!])
                                }
                              }}
                              className="text-[9.5px] font-mono font-bold px-1.5 py-0.5 rounded shrink-0 cursor-pointer disabled:opacity-50"
                              style={{
                                color: 'var(--status-info)',
                                background: 'rgb(var(--status-info-rgb) / 0.12)',
                                border: '1px solid rgb(var(--status-info-rgb) / 0.32)',
                              }}
                              title={format(strings.framing.createStoryTitle, { key: selected.key })}
                            >
                              {creatingTodoId === todo.id ? '…' : strings.framing.createStory}
                            </button>
                          )}
                          <button
                            type="button"
                            disabled={slicingLocked}
                            onClick={() => persist(selected.key, { todos: todosOf(selected).filter(t => t.id !== todo.id) })}
                            className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 cursor-pointer shrink-0 disabled:opacity-40"
                            title={strings.framing.remove}
                          >
                            <Trash2 size={11} />
                          </button>
                        </div>
                      ))}
                      {todosOf(selected).length === 0 && (
                        <p className="text-[11px] text-[var(--text-muted)]">
                          {strings.framing.noTodo}
                        </p>
                      )}
                    </div>

                    <div className="flex items-center gap-2 mt-2">
                      <input
                        type="text"
                        value={newTodo}
                        disabled={slicingLocked}
                        onChange={e => setNewTodo(e.target.value)}
                        onKeyDown={e => {
                          if (e.key === 'Enter') {
                            e.preventDefault()
                            addTodo(selected)
                          }
                        }}
                        placeholder={strings.framing.addTodoPlaceholder}
                        className="flex-1 px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <button type="button" onClick={() => addTodo(selected)} disabled={!newTodo.trim() || slicingLocked}
                        className="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0">
                        <Plus size={12} /> {strings.framing.add}
                      </button>
                    </div>

                    {/* Produire la découpe depuis les artefacts SDD du dépôt.
                        Le bouton est offert dès que le projet déclare un dépôt,
                        et non selon la présence du fichier : une action qui
                        disparaît exactement quand elle aurait servi n'explique
                        rien, là où un refus nomme sa cause. */}
                    <div className="flex items-center gap-2 mt-2 pt-2 border-t border-[var(--border-color)]">
                      <span className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] shrink-0">
                        {strings.framing.importHeading}
                      </span>
                      {([
                        { source: 'tasks' as const, label: 'tasks.md', hint: strings.framing.importTasksHint },
                        { source: 'spec' as const, label: 'spec.md', hint: strings.framing.importSpecHint },
                        // L'inverse de « Créer story » : celui-ci descend d'une
                        // ligne vers un ticket, celui-là remonte d'un ticket
                        // vers sa ligne. Une macro dont les stories ont été
                        // créées ailleurs avait une découpe vide alors que le
                        // travail était déjà découpé.
                        { source: 'stories' as const, label: strings.framing.importStories, hint: strings.framing.importStoriesHint },
                      ]).map(option => (
                        <button
                          key={option.source}
                          type="button"
                          disabled={slicingSource !== null || slicingLocked}
                          title={option.hint}
                          onClick={async () => {
                            setSlicingSource(option.source)
                            const macro = await produceMacroSlicing(currentProject!.id, selected.key, option.source)
                            setSlicingSource(null)
                            if (macro) {
                              setMacroMeta(prev => [...prev.filter(m => m.key !== macro.key), macro])
                              setSelectedKey(macro.key)
                            }
                          }}
                          className="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-semibold bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
                        >
                          {slicingSource === option.source ? <Loader2 size={12} className="animate-spin" /> : <FileCode size={12} />}
                          {option.label}
                        </button>
                      ))}
                    </div>
                  </div>
                    )
                  })()}

                  {selected.tasks.length > 0 && (
                    <div className="pt-2 border-t border-[var(--border-color)]">
                      <div className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] mb-1.5">
                        {format(strings.framing.createdTickets, { count: selected.tasks.length })}
                      </div>
                      {/* Une seule colonne, la même ligne que les groupes par
                          objectif : une pastille qui ne porte que la clé oblige
                          à survoler chaque ticket pour savoir de quoi il parle. */}
                      <div className="flex flex-col gap-1">
                        {selected.tasks.map(task => (
                          <MacroTaskRow key={task.id} task={task} onOpen={setSelectedTask} />
                        ))}
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>
          </aside>
        )}
      </div>

      {showCreateMacroModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150" {...createMacroBackdrop}>
          <div className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl w-full max-w-md shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 text-sm font-bold text-[var(--text-primary)]">
                <Target size={16} className="text-[var(--accent-color)]" />
                <span>{strings.createModal.title}</span>
              </div>
              <button
                type="button"
                onClick={() => setShowCreateMacroModal(false)}
                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              >
                <X size={16} />
              </button>
            </div>

            <form
              onSubmit={async (e) => {
                e.preventDefault()
                const targetProjId = createMacroProjectId || currentProject?.id || projects[0]?.id
                if (!createMacroTitle.trim() || !targetProjId) return
                setBusyKey('macro')
                const created = await createMacro(
                  targetProjId,
                  createMacroTitle.trim(),
                  createMacroHorizon
                )
                if (created) {
                  setMacroMeta(prev => [...prev.filter(m => m.key !== created.key), created])
                  if (createMacroHorizon) {
                    setTab(createMacroHorizon)
                  }
                  setSelectedKey(created.key)
                  setShowCreateMacroModal(false)
                }
                setBusyKey(null)
              }}
              className="p-5 space-y-4"
            >
              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  {strings.createModal.titleLabel}
                </label>
                <input
                  type="text"
                  autoFocus
                  value={createMacroTitle}
                  onChange={(e) => setCreateMacroTitle(e.target.value)}
                  placeholder={strings.createModal.titlePlaceholder}
                  className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                    {strings.createModal.horizon}
                  </label>
                  <select
                    value={createMacroHorizon}
                    onChange={(e) => setCreateMacroHorizon(e.target.value as MacroHorizon)}
                    className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                  >
                    <option value="now">{strings.createModal.horizonNow}</option>
                    <option value="next">{strings.createModal.horizonNext}</option>
                    <option value="later">{strings.createModal.horizonLater}</option>
                  </select>
                </div>

                {projects.length > 1 && (
                  <div>
                    <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                      {strings.createModal.project}
                    </label>
                    <select
                      value={createMacroProjectId}
                      onChange={(e) => setCreateMacroProjectId(e.target.value)}
                      className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                    >
                      {projects.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                    </select>
                  </div>
                )}
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateMacroModal(false)}
                  className="px-3 py-1.5 text-xs font-semibold rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                >
                  {strings.panel.cancel}
                </button>
                <button
                  type="submit"
                  disabled={!createMacroTitle.trim() || busyKey === 'macro'}
                  className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-bold text-white accent-bg rounded-xl cursor-pointer disabled:opacity-50"
                >
                  {busyKey === 'macro' ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
                  {strings.createModal.submit}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showMigrateModal && selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150" {...migrateBackdrop}>
          <div className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl w-full max-w-md shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 text-sm font-bold text-[var(--text-primary)]">
                <ArrowRightLeft size={16} className="text-[var(--accent-color)]" />
                <span>{format(strings.migrateModal.title, { key: selected.key })}</span>
              </div>
              <button
                type="button"
                onClick={() => setShowMigrateModal(false)}
                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              >
                <X size={16} />
              </button>
            </div>

            <form
              onSubmit={async (e) => {
                e.preventDefault()
                if (!migrateTargetProjectId || !currentProject?.id) return
                setIsMigrating(true)
                const res = await migrateMacro(
                  currentProject.id,
                  selected.key,
                  migrateTargetProjectId,
                  migrateIncludeTasks
                )
                setIsMigrating(false)
                if (res.success) {
                  setShowMigrateModal(false)
                  setSelectedKey(null)
                }
              }}
              className="p-5 space-y-4"
            >
              <div>
                <div className="text-xs text-[var(--text-muted)] mb-1 font-medium">{strings.migrateModal.macroToMove}</div>
                <div className="p-3 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
                  <div className="text-xs font-bold text-[var(--text-primary)]">{selected.title}</div>
                  <div className="text-[11px] font-mono text-[var(--text-muted)] mt-0.5">
                    {plural(language, selected.tasks.length, strings.migrateModal.currentProject, { name: currentProject?.name || '' })}
                  </div>
                </div>
              </div>

              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  {strings.migrateModal.targetProject}
                </label>
                {projects.filter(p => isProjectCompatible(currentProject, p)).length > 0 ? (
                  <select
                    value={migrateTargetProjectId}
                    onChange={(e) => setMigrateTargetProjectId(e.target.value)}
                    className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                  >
                    {projects.filter(p => isProjectCompatible(currentProject, p)).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} ({p.issueTracker || 'local'}{p.githubRepo ? ` · ${p.githubRepo}` : ''})
                      </option>
                    ))}
                  </select>
                ) : (
                  <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs">
                    {strings.migrateModal.noCompatible}
                  </div>
                )}
              </div>

              {selected.tasks.length > 0 && (
                <label className="flex items-center gap-2.5 p-3 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] cursor-pointer">
                  <input
                    type="checkbox"
                    checked={migrateIncludeTasks}
                    onChange={(e) => setMigrateIncludeTasks(e.target.checked)}
                    className="rounded accent-[var(--accent-color)]"
                  />
                  <span className="text-xs text-[var(--text-primary)] font-medium">
                    {strings.migrateModal.includeTasksBefore}{' '}<strong>{plural(language, selected.tasks.length, strings.migrateModal.includeTasksCount)}</strong>{' '}{plural(language, selected.tasks.length, strings.migrateModal.includeTasksAfter)}
                  </span>
                </label>
              )}

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowMigrateModal(false)}
                  className="px-3 py-1.5 text-xs font-semibold rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                >
                  {strings.panel.cancel}
                </button>
                <button
                  type="submit"
                  disabled={!migrateTargetProjectId || isMigrating}
                  className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-bold text-white accent-bg rounded-xl cursor-pointer disabled:opacity-50"
                >
                  {isMigrating ? <Loader2 size={13} className="animate-spin" /> : <ArrowRightLeft size={13} />}
                  <span>{strings.migrateModal.submit}</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal d'aperçu du raffinage de macro (AI) */}
      {refinePreview && selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs" {...refinePreviewBackdrop}>
          <div className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl p-5 max-w-lg w-full shadow-2xl flex flex-col max-h-[85vh]">
            <div className="flex items-start justify-between border-b border-[var(--border-color)] pb-3">
              <div>
                <div className="flex items-center gap-2">
                  <Sparkles size={16} className="text-orange-400" />
                  <h3 className="text-sm font-bold text-[var(--text-primary)]">
                    {format(strings.refineModal.title, { key: selected.key })}
                  </h3>
                  <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded uppercase ${
                    refinePreview.specFramework === 'openspec'
                      ? 'bg-purple-500/10 text-purple-400 border border-purple-500/30'
                      : 'bg-blue-500/10 text-blue-400 border border-blue-500/30'
                  }`}>
                    {refinePreview.specFramework === 'openspec' ? 'OpenSpec SDD' : 'SpecKit SDD'}
                  </span>
                </div>
                <p className="text-[11px] text-[var(--text-muted)] mt-1">
                  {plural(language, refinePreview.todos.length, strings.refineModal.generated)}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setRefinePreview(null)}
                className="text-[var(--text-muted)] hover:text-[var(--text-primary)] p-1 rounded-lg cursor-pointer"
              >
                <X size={14} />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto py-3 space-y-3 my-2 pr-1">
              {refinePreview.todos && refinePreview.todos.length > 0 && (
                <div className="space-y-1.5">
                  <span className="text-xs font-bold text-[var(--text-muted)] flex items-center gap-1">
                    <ListChecks size={13} className="text-orange-400" />
                    {format(strings.refineModal.checklist, { count: refinePreview.todos.length })}
                  </span>
                  {refinePreview.todos.map((todo, idx) => (
                    <div key={todo.id || idx} className="flex items-start gap-2 p-2 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)]">
                      <ListChecks size={13} className="text-orange-400 shrink-0 mt-0.5" />
                      <span className="text-[11.5px] leading-snug text-[var(--text-primary)] flex-1 font-mono">
                        {todo.text}
                      </span>
                    </div>
                  ))}
                </div>
              )}

              {refinePreview.proposedTasks && refinePreview.proposedTasks.length > 0 && (
                <div className="pt-2 border-t border-[var(--border-color)] space-y-1.5">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[var(--text-primary)] flex items-center gap-1.5">
                      <FolderGit2 size={13} className="text-cyan-400" />
                      {format(strings.refineModal.proposed, { count: refinePreview.proposedTasks.length })}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        const total = refinePreview.proposedTasks?.length || 0
                        const allChecked = Object.keys(selectedProposedTasks).length === total
                        const newMap: Record<number, boolean> = {}
                        if (!allChecked) {
                          refinePreview.proposedTasks?.forEach((_, idx) => { newMap[idx] = true })
                        }
                        setSelectedProposedTasks(newMap)
                      }}
                      className="text-[10px] font-bold text-[var(--accent-color)] hover:underline cursor-pointer"
                    >
                      {strings.refineModal.toggleAll}
                    </button>
                  </div>
                  <div className="space-y-1.5 max-h-52 overflow-y-auto">
                    {refinePreview.proposedTasks.map((pt, idx) => (
                      <label
                        key={idx}
                        className="flex items-start gap-2.5 p-2 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] hover:border-cyan-500/30 cursor-pointer select-none"
                      >
                        <input
                          type="checkbox"
                          checked={Boolean(selectedProposedTasks[idx])}
                          onChange={e => setSelectedProposedTasks(prev => ({ ...prev, [idx]: e.target.checked }))}
                          className="mt-0.5 rounded border-[var(--border-color)] text-cyan-500 focus:ring-0 cursor-pointer"
                        />
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1.5">
                            <span className={`text-[9px] font-bold px-1.5 py-0.2 rounded uppercase ${
                              pt.issueType === 'Bug'
                                ? 'bg-rose-500/10 text-rose-400 border border-rose-500/30'
                                : pt.issueType === 'Task'
                                ? 'bg-indigo-500/10 text-indigo-400 border border-indigo-500/30'
                                : 'bg-blue-500/10 text-blue-400 border border-blue-500/30'
                            }`}>
                              {pt.issueType}
                            </span>
                            <span className="text-xs font-semibold text-[var(--text-primary)] truncate">{pt.title}</span>
                          </div>
                          {pt.description && (
                            <p className="text-[10.5px] text-[var(--text-muted)] line-clamp-1 mt-0.5">{pt.description}</p>
                          )}
                        </div>
                      </label>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-[var(--border-color)] pt-3 mt-auto flex-wrap">
              <button
                type="button"
                onClick={() => setRefinePreview(null)}
                className="px-3 py-1.5 rounded-xl text-xs font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] cursor-pointer"
              >
                {strings.panel.cancel}
              </button>
              {refinePreview.todos && refinePreview.todos.length > 0 && (
                <button
                  type="button"
                  onClick={async () => {
                    const existing = todosOf(selected)
                    await persist(selected.key, { todos: [...existing, ...refinePreview.todos] })
                    setRefinePreview(null)
                    addToast({
                      type: 'success',
                      title: strings.refineModal.todosAdded,
                      description: plural(language, refinePreview.todos.length, strings.refineModal.todosAddedBody, { key: selected.key }),
                    })
                  }}
                  className="px-3 py-1.5 rounded-xl text-xs font-bold text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:border-[var(--accent-color)] cursor-pointer"
                >
                  {strings.refineModal.addToTodos}
                </button>
              )}
              {refinePreview.proposedTasks && refinePreview.proposedTasks.length > 0 && (
                <button
                  type="button"
                  disabled={isCreatingBatch}
                  onClick={async () => {
                    const selectedTasksToCreate = refinePreview.proposedTasks?.filter((_, idx) => selectedProposedTasks[idx]) || []
                    if (selectedTasksToCreate.length === 0) {
                      addToast({ type: 'warning', title: strings.refineModal.noTicketSelected, description: strings.refineModal.noTicketSelectedBody })
                      return
                    }
                    setIsCreatingBatch(true)
                    const payload = selectedTasksToCreate.map(pt => ({
                      projectId: currentProject?.id || selected.meta?.projectId || '',
                      title: pt.title,
                      issueType: pt.issueType,
                      description: pt.description,
                      parentKey: selected.key,
                      parentTitle: selected.title,
                      parentType: 'Macro',
                      status: 'backlog' as const,
                    }))
                    await createBatchTasks(payload)
                    setIsCreatingBatch(false)
                    setRefinePreview(null)
                  }}
                  className="px-3.5 py-1.5 rounded-xl text-xs font-bold text-white bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 cursor-pointer flex items-center gap-1.5"
                >
                  {isCreatingBatch ? <Loader2 size={13} className="animate-spin" /> : <FolderGit2 size={13} />}
                  <span>{format(strings.refineModal.generate, { count: Object.values(selectedProposedTasks).filter(Boolean).length })}</span>
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      {seedLines && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150" {...seedBackdrop}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label={strings.axes.seedModalTitle}
            className="bg-[var(--bg-secondary)] border border-[var(--border-color)] rounded-2xl w-full max-w-2xl max-h-[85vh] flex flex-col shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150"
          >
            <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2 text-sm font-bold text-[var(--text-primary)]">
                <Sparkles size={16} className="text-[var(--accent-color)]" />
                <span>{strings.axes.seedModalTitle}</span>
              </div>
              <button
                type="button"
                onClick={closeSeed}
                className="p-1 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
                aria-label={strings.axes.seedCancel}
              >
                <X size={16} />
              </button>
            </div>
            <div className="px-5 py-3 text-xs text-[var(--text-secondary)]">
              {seedLines.length === 0 ? strings.axes.seedEmpty : strings.axes.seedIntro}
            </div>
            {seedLines.length > 0 && (
              <div className="flex-1 overflow-y-auto px-5 pb-3">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-[10px] uppercase tracking-[.08em] text-[var(--text-muted)]">
                      <th className="text-left font-bold py-1.5">{strings.axes.seedMacro}</th>
                      <th className="text-left font-bold py-1.5 w-[90px]">{strings.axes.priorityLabel}</th>
                      <th className="text-left font-bold py-1.5 w-[110px]">{strings.axes.quarterLabel}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {seedLines.map(line => (
                      <tr key={line.key} className="border-t border-[var(--border-color)]">
                        <td className="py-1.5 pr-3">
                          <span className="font-mono font-bold text-[var(--accent-color)] mr-2">{line.key}</span>
                          <span className="text-[var(--text-primary)]">{line.title}</span>
                        </td>
                        {(['priority', 'quarter'] as const).map(axis => {
                          const value = line[axis]
                          const id = `${line.key}:${axis}`
                          return (
                            <td key={axis} className="py-1.5">
                              {value && (
                                <label className="inline-flex items-center gap-1.5 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={Boolean(seedKept[id])}
                                    onChange={e => setSeedKept(prev => ({ ...prev, [id]: e.target.checked }))}
                                  />
                                  <span className="font-mono font-bold">
                                    {axis === 'priority' ? epicPriorityLabel(value as EpicPriority) : value}
                                  </span>
                                </label>
                              )}
                            </td>
                          )
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <div className="flex items-center justify-end gap-2 px-5 py-3 border-t border-[var(--border-color)]">
              <button
                type="button"
                onClick={closeSeed}
                className="px-3.5 py-1.5 rounded-xl text-xs font-semibold text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
              >
                {strings.axes.seedCancel}
              </button>
              {seedLines.length > 0 && (
                <button
                  type="button"
                  disabled={isSeeding || seedValueCount === 0}
                  onClick={runSeed}
                  className="px-3.5 py-1.5 rounded-xl text-xs font-bold text-white accent-bg hover:opacity-90 disabled:opacity-50 cursor-pointer flex items-center gap-1.5"
                >
                  {isSeeding ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
                  <span>{plural(language, seedValueCount, strings.axes.seedConfirm)}</span>
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
